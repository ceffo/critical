// Package composition is the composition root: the one place where concrete
// adapters meet service constructors. Per clean_architecture.md §3, every
// other package sees these dependencies through interfaces.
package composition

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/diceadpt"
	"github.com/ceffo/critical/api/internal/adapters/googleadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/creationrollsvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spheresvc"
	"github.com/ceffo/critical/api/internal/domain/service/turnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
	"github.com/ceffo/critical/api/internal/storage/postgres"
)

// BackingServices holds every business service plus the storage handle.
// Routers select which slices to mount; the character a request acts on comes
// from its user (ADR 0029 step 7), never from here.
type BackingServices struct {
	// DB is the PostgreSQL handle, set by New on a database only (nil when
	// built over mocks or in memory).
	DB *postgres.DB

	Character charactersvc.SVC
	Log       actionlogsvc.SVC
	Buffs     buffsvc.SVC
	Spells    spellsvc.SVC
	Prep      prepsvc.SVC
	Cast      castsvc.SVC
	Combat    combatsvc.SVC
	Items     itemsvc.SVC
	Rest      restsvc.SVC
	XP        xpsvc.SVC
	LevelUp   levelupsvc.SVC
	Rolls     creationrollsvc.SVC // the creation's score and max HP rolls (ADR 0032 steps 6 and 7)
	Learn     learnsvc.SVC
	Turn      turnsvc.SVC
	Details   detailssvc.SVC
	Skills    skillsvc.SVC
	// AC is the character's derived armour class and its override (ADR
	// 0033 steps 10 to 13).
	AC   acsvc.SVC
	Dice dicesvc.SVC
	// Auth is the login, the sessions and the users (ADR 0029).
	Auth authsvc.SVC

	services []domain.Service
	store    Store
}

// Config is what cmd/api passes to New. Each field is set from env in
// internal/cmd/api/start.go.
type Config struct {
	// DSN is the database New opens, unless InMemory.
	DSN string
	// InMemory makes New wire the in-memory storage instead of opening DSN
	// (ADR 0025 step 8: the e2e mode; the boot refuses it in staging and
	// production).
	InMemory bool
	SeedsDir string
	// Catalogue is the path of the spell catalogue artifact (ADR 0021),
	// imported into the database at startup.
	Catalogue string
	// Dice replaces the crypto dice (tests that need known rolls). Nil in
	// production.
	Dice Dice
	// Google is the OAuth client of the Google login (ADR 0029 step 3). Nil
	// runs the dev login instead (step 8.1); cmd/api refuses nil on a hosted
	// app (step 8.2).
	Google *googleadpt.Config
}

// Dice is every dice port the services take (diceadpt satisfies it).
type Dice interface {
	RollDie(sides int) int
	RollD20() int
	RollD100() int
}

// New opens the PostgreSQL database (cfg.DSN) and wires the real adapters over it (or, with
// cfg.InMemory, wires the in-memory storage and opens nothing), then builds
// the services (Build). Returns an initialized BackingServices ready to be
// mounted on a router.
func New(ctx context.Context, cfg Config) (*BackingServices, error) {
	if cfg.InMemory {
		return Build(ctx, memoryStorage(memory.New()), cfg)
	}
	db, err := postgres.Open(ctx, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	bs, err := Build(ctx, postgresStorage(db), cfg)
	if err != nil {
		return nil, err
	}
	bs.DB = db
	return bs, nil
}

// Build runs the startup steps (migrate, validate the catalogue and the
// seeds, seed, import the catalogue), then builds the adapters that need no storage
// and the services over st. On an error it closes st. It touches storage only
// through the ports of st, so tests drive it with mocks.
func Build(ctx context.Context, st Storage, cfg Config) (*BackingServices, error) {
	db := st.Store
	// The spell catalogue (ADR 0021 step 5): a missing or invalid artifact,
	// or one that removes a spell still referenced, is a startup error. It is
	// read first: the migration's equipment conversion and the seed match
	// weapons to its rows (ADR 0033 step 26.4).
	artifact, err := catalogueadpt.LoadArtifact(cfg.Catalogue)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	cat := artifact.Catalogue
	// The PHB rule tables: spell preparation takes Table 32, attacks look
	// STR/DEX adjustments up in Tables 1-2, the XP ledger Table 25. A gap in a table is a startup
	// error.
	ruleTables, err := loadRuleTables(artifact, cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// The catalogue's weapons (format 7), none before.
	var catalogueWeapons []ruletablesvc.Weapon
	if eq, ok := ruleTables.Equipment(); ok {
		catalogueWeapons = eq.Weapons
	}
	if err := db.Migrate(ctx, cfg.SeedsDir, catalogueWeapons); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	characterDAL, hpDAL, logDAL, buffDAL, spellDAL := st.Character, st.HP, st.Log, st.Buff, st.Spell
	var dice Dice = diceadpt.New()
	if cfg.Dice != nil {
		dice = cfg.Dice
	}
	critDAL, itemDAL := st.Crit, st.Items

	// The spell effects casting applies; a malformed formula is a startup
	// error.
	effects, err := spelleffectsadpt.New(cfg.SeedsDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := effects.CheckAgainst(cat); err != nil {
		_ = db.Close()
		return nil, err
	}
	// The spellbook seed is checked against the catalogue before the seed
	// writes it into the seed character's spellbook: only catalogue spells
	// are stored.
	spellbookSeed, err := spellbookadpt.ReadSeed(cfg.SeedsDir, cat)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.Seed(ctx, cfg.SeedsDir, spellbookSeed, catalogueWeapons); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}
	if err := db.ImportCatalogue(ctx, cat, seedSpellRefs(spellbookSeed, effects)); err != nil {
		_ = db.Close()
		return nil, err
	}

	// The spell catalogue is served from memory, read once after the import
	// (spelldal.Catalogue): page loads read none of it from the database.
	catalogue, err := spelldal.LoadCatalogue(ctx, spellDAL)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	authSvc, err := newAuth(st, cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// The characters stored before characters had owners (the seed
	// character) are the root admin's once there is one (ADR 0029 step 5.3).
	if err := authSvc.AdoptUnownedCharacters(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("give the characters without an owner to the root admin: %w", err)
	}

	// Each character's spellbook, named from the catalogue.
	spellbook := spellbookadpt.New(st.Spellbook, cat)

	// The action log stamps entries with the round read straight from
	// buffdal (buffsvc itself records into the log, so it cannot be the
	// log's dependency). Services that record snapshots register their
	// Restorers on logSvc here.
	//
	// tx is the one TxRunner every service gets: each outermost InTx is one
	// action-log transaction, so whatever a request records is popped by one
	// undo (ADR 0018 steps 1 and 2).
	tx := actionlogsvc.Transactions(db)
	// The log serves every entry through these views (payload fields a kind
	// gained after the entry was logged).
	logPayloadViews := actionlog.PayloadViews{
		actionlog.KindSave: combatsvc.SavePayloadView,
	}
	logSvc, err := actionlogsvc.New(logDAL, tx, roundReader{buffDAL}, hpDAL, logPayloadViews)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// Choosing the current character is no game move: it runs on the store's
	// transactions, outside the action log.
	// The character's level, THAC0, saves and slots are derived from its
	// class, race and XP total (ADR 0030 steps 3 to 8): the class tables and
	// what the XP ledger's entries add.
	// The language catalogue (ADR 0032 step 4) is the artifact's: nil before
	// format 8, which knows no language.
	languages, _ := ruleTables.Languages()
	charSvc, err := charactersvc.New(charactersvc.Deps{
		Reader: characterDAL, XP: xpsvc.Totals{DAL: st.XP}, Rules: ruleTables,
		Owners: characterDAL, Tx: tx, Sessions: authSvc, Log: logSvc, Languages: languages,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// What a character prepares from: a cleric's sphere-open priest spells
	// (ADR 0030 step 15; no sphere access before catalogue format 6), every
	// other class's spellbook.
	sphereAccess, err := ruleTables.SphereAccess()
	if err != nil && !errors.Is(err, ruletablesvc.ErrNoSphereAccess) {
		_ = db.Close()
		return nil, err
	}
	prepSource, err := spheresvc.New(spheresvc.Deps{Book: spellbook, Characters: charSvc, Access: sphereAccess, Catalogue: cat})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	itemSvc, err := itemsvc.New(itemDAL, itemsvc.Deps{
		Dice: dice, Log: logSvc, Tx: tx, Chars: charSvc, Catalogue: catalogueWeapons,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, itemSvc.Restorers())
	// The worn and wielded pieces' powers join the buffs' aggregate (ADR 0016
	// step 11, ADR 0033 step 9).
	buffSvc, err := buffsvc.New(buffDAL, tx, logSvc, charSvc, dice, itemSvc)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, buffSvc.Restorers())
	spellSvc, err := spellsvc.New(spellDAL, catalogue, prepSource)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	prepSvc, err := prepsvc.New(prepsvc.Deps{
		Slots: spellDAL, Spellbook: prepSource, Effects: effects, Items: itemSvc,
		Characters: charSvc, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	logSvc.RegisterRestorer(prepsvc.StateKey, prepSvc)
	combatSvc, err := combatsvc.New(combatsvc.Deps{
		Chars: charSvc, Buffs: buffSvc, Dice: dice, Weapons: combatsvc.ItemWeapons(itemSvc), Crit: critDAL,
		Items: itemSvc, Forms: st.Forms, Rules: ruleTables, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	castSvc, err := castsvc.New(castsvc.Deps{
		Slots: spellDAL, Effects: effects, Texts: catalogue, Buffs: buffSvc, Items: itemSvc, Attacks: combatSvc,
		Characters: charSvc, Dice: dice, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	restSvc, err := restsvc.New(spellSvc, itemSvc, logSvc, tx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	detailsSvc, err := detailssvc.New(characterDAL, tx, languages)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	xpSvc, err := xpsvc.New(xpsvc.Deps{
		DAL: st.XP, Characters: charSvc, Thresholds: ruleTables, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, xpSvc.Restorers())
	// The level-up roll (ADR 0030 step 7) reads the pending levels from the
	// character's derived block and writes its hit points; its undo writes
	// them back.
	levelUpSvc, err := levelupsvc.New(levelupsvc.Deps{
		DAL: characterDAL, Characters: charSvc, Rules: ruleTables, Dice: dice, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, levelUpSvc.Restorers())
	// The creation's rolls (ADR 0032 steps 6 and 7) read the rule tables and
	// the dice, and write nothing.
	creationRollSvc, err := creationrollsvc.New(creationrollsvc.Deps{Rules: ruleTables, Dice: dice})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// Learning a spell (ADR 0030 step 20) reads the character's derived
	// spellbook rules and the catalogue, rolls the chance to learn and writes
	// the spellbook; its undo writes the spell's place back.
	learnSvc, err := learnsvc.New(learnsvc.Deps{
		DAL: st.Spellbook, Book: st.Spellbook, Spells: spellbook, Characters: charSvc, Dice: dice, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, learnSvc.Restorers())
	// Turning undead reads the cleric's Table 61 row (ADR 0030 step 16) and
	// rolls; it changes no state, so it registers no restorer.
	turnSvc, err := turnsvc.New(turnsvc.Deps{Characters: charSvc, Dice: dice, Log: logSvc, Tx: tx})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// Skill and ability checks change no state: their entries carry no
	// snapshot. A thief's skill points spent on the sheet do: their undo
	// writes the points back. The ability check's score in effect comes from
	// buffSvc (ADR 0028 step 2).
	skillSvc, err := skillsvc.New(skillsvc.Deps{
		DAL: st.Skill, Characters: charSvc, Abilities: buffSvc, Dice: dice, Log: logSvc, Points: characterDAL, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, skillSvc.Restorers())
	registerRestorers(logSvc, charSvc.Restorers())
	// The character's AC is derived on read from the pieces, the buffs and
	// powers that apply and the sheet's Dexterity (ADR 0033 step 10); its
	// override is a logged move undone by its snapshot (step 13).
	acSvc, err := acsvc.New(acsvc.Deps{
		Items: itemSvc, Effects: buffSvc, Characters: charSvc, Tables: ruleTables,
		Overrides: characterDAL, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, acSvc.Restorers())
	// Free dice rolls change no state: no snapshot, no restorer.
	diceSvc, err := dicesvc.New(dicesvc.Deps{DAL: st.Dice, Dice: dice, Log: logSvc, Tx: tx})
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &BackingServices{
		Character: charSvc,
		Log:       logSvc,
		Buffs:     buffSvc,
		Spells:    spellSvc,
		Prep:      prepSvc,
		Cast:      castSvc,
		Combat:    combatSvc,
		Items:     itemSvc,
		Rest:      restSvc,
		XP:        xpSvc,
		LevelUp:   levelUpSvc,
		Rolls:     creationRollSvc,
		Learn:     learnSvc,
		Turn:      turnSvc,
		Details:   detailsSvc,
		Skills:    skillSvc,
		AC:        acSvc,
		Dice:      diceSvc,
		Auth:      authSvc,
		store:     db,
		services: []domain.Service{
			authSvc, charSvc, buffSvc, logSvc, spellSvc, prepSvc, castSvc, combatSvc, itemSvc, restSvc, xpSvc, levelUpSvc, creationRollSvc, learnSvc, skillSvc, acSvc, detailsSvc, diceSvc,
		},
	}, nil
}

// newAuth builds the login service over st: with Google when cfg.Google is
// set, else in dev login mode (ADR 0029 step 8.1).
func newAuth(st Storage, cfg Config) (authsvc.SVC, error) {
	deps := authsvc.Deps{DAL: st.Auth, Tx: st.Store}
	if cfg.Google != nil {
		p, err := googleadpt.New(*cfg.Google)
		if err != nil {
			return nil, err
		}
		deps.Provider = p
	}
	return authsvc.New(deps)
}

// OpenAuth opens the PostgreSQL database at dsn for the root-admin script
// (ADR 0029 step 5.1): the users only, no migration, seed or catalogue. The
// server must have migrated the database before. close releases it.
func OpenAuth(ctx context.Context, dsn string) (svc authsvc.SVC, closeDB func() error, err error) {
	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	svc, err = newAuth(postgresStorage(db), Config{})
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return svc, db.Close, nil
}

// Close shuts down every service in reverse order, then closes the DB.
func (b *BackingServices) Close(ctx context.Context) error {
	var firstErr error
	for i := len(b.services) - 1; i >= 0; i-- {
		if err := b.services[i].Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if b.store != nil {
		if err := b.store.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// roundReader satisfies actionlogsvc.Rounds from buffdal. It runs inside the
// recording transaction (buffdal follows the ctx transaction).
type roundReader struct{ dal BuffStore }

func (r roundReader) GetRound(ctx context.Context, characterID int64) (int, error) {
	return r.dal.GetRoundNumber(ctx, characterID)
}

// registerRestorers binds every state key a service snapshots to its
// Restorer, so voiding one of its entries can write the "before" state back.
func registerRestorers(log actionlogsvc.SVC, restorers map[string]actionlog.Restorer) {
	for key, r := range restorers {
		log.RegisterRestorer(key, r)
	}
}

// loadRuleTables parses the PHB rule tables the artifact carries (ADR 0021
// step 9.4), and a format-4 artifact's class and race tables (step 10, which
// the server needs: a format-3 artifact refuses the start), with
// the table rules of steps 9.3 and 10.3 (a missing or broken table refuses
// the start, naming it), at every start, whatever catalogue version the
// database recorded (step 9.5).
// The seeds hold no rule tables (step 9.6.3).
func loadRuleTables(artifact catalogueadpt.Artifact, cfg Config) (*ruletablesvc.Tables, error) {
	t, err := ruletablesadpt.Parse(artifact.Catalogue.Format, artifact.RuleTables, artifact.Equipment, artifact.Languages)
	if err != nil {
		return nil, fmt.Errorf("spell catalogue %s: rule_tables: %w", cfg.Catalogue, err)
	}
	// Every character's level, THAC0 and saves come from the class tables
	// (ADR 0030 steps 3 to 6): an artifact without them cannot serve one.
	if _, ok := t.Classes(); !ok {
		return nil, fmt.Errorf("spell catalogue %s: %w", cfg.Catalogue, ruletablesvc.ErrNoClassTables)
	}
	return t, nil
}

// seedSpellRefs are the spell ids the seeds refer to: a catalogue import may
// not remove any of them (ADR 0021 step 5.4.3).
func seedSpellRefs(spellbook []string, effects *spelleffectsadpt.Catalog) []catalogue.Ref {
	var refs []catalogue.Ref
	for _, id := range spellbook {
		refs = append(refs, catalogue.Ref{ID: id, Where: "spellbook seed"})
	}
	for _, e := range effects.All() {
		refs = append(refs, catalogue.Ref{ID: e.ID, Where: "spell effects seed"})
	}
	return refs
}
