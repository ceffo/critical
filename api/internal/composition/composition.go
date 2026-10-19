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
// and the services over st, and registers every service's module on the
// action log. On an error it closes the services built so far and st. It
// touches storage only through the ports of st, so tests drive it with mocks.
func Build(ctx context.Context, st Storage, cfg Config) (_ *BackingServices, err error) {
	db := st.Store
	bs := &BackingServices{store: db}
	defer func() {
		if err != nil {
			_ = bs.Close(ctx)
		}
	}()
	// The spell catalogue (ADR 0021 step 5): a missing or invalid artifact,
	// or one that removes a spell still referenced, is a startup error. It is
	// read first: the migration's equipment conversion and the seed match
	// weapons to its rows (ADR 0033 step 26.4).
	artifact, err := catalogueadpt.LoadArtifact(cfg.Catalogue)
	if err != nil {
		return nil, err
	}
	cat := artifact.Catalogue
	// The PHB rule tables: spell preparation takes Table 32, attacks look
	// STR/DEX adjustments up in Tables 1-2, the XP ledger Table 25. A gap in a table is a startup
	// error.
	ruleTables, err := loadRuleTables(artifact, cfg)
	if err != nil {
		return nil, err
	}
	// The catalogue's weapons (format 7), none before.
	var catalogueWeapons []ruletablesvc.Weapon
	if eq, ok := ruleTables.Equipment(); ok {
		catalogueWeapons = eq.Weapons
	}
	if err := db.Migrate(ctx, cfg.SeedsDir, catalogueWeapons); err != nil {
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
		return nil, err
	}

	if err := effects.CheckAgainst(cat); err != nil {
		return nil, err
	}
	// The spellbook seed is checked against the catalogue before the seed
	// writes it into the seed character's spellbook: only catalogue spells
	// are stored.
	spellbookSeed, err := spellbookadpt.ReadSeed(cfg.SeedsDir, cat)
	if err != nil {
		return nil, err
	}
	if err := db.Seed(ctx, cfg.SeedsDir, spellbookSeed, catalogueWeapons); err != nil {
		return nil, fmt.Errorf("seed: %w", err)
	}
	if err := db.ImportCatalogue(ctx, cat, seedSpellRefs(spellbookSeed, effects)); err != nil {
		return nil, err
	}

	// The spell catalogue is served from memory, read once after the import
	// (spelldal.Catalogue): page loads read none of it from the database.
	catalogue, err := spelldal.LoadCatalogue(ctx, spellDAL)
	if err != nil {
		return nil, err
	}

	// Every service below joins bs.services (add) in the order it is built:
	// the one list of the services, whose modules are registered on the log
	// at the end and which Close walks in reverse.
	bs.Auth, err = newAuth(st, cfg)
	if err := bs.add(bs.Auth, err); err != nil {
		return nil, err
	}
	// The characters stored before characters had owners (the seed
	// character) are the root admin's once there is one (ADR 0029 step 5.3).
	if err := bs.Auth.AdoptUnownedCharacters(ctx); err != nil {
		return nil, fmt.Errorf("give the characters without an owner to the root admin: %w", err)
	}

	// Each character's spellbook, named from the catalogue.
	spellbook := spellbookadpt.New(st.Spellbook, cat)

	// The action log stamps entries with the round read straight from
	// buffdal (buffsvc itself records into the log, so it cannot be the
	// log's dependency).
	//
	// tx is the one TxRunner every service gets: each outermost InTx is one
	// action-log transaction, so whatever a request records is popped by one
	// undo (ADR 0018 steps 1 and 2).
	tx := actionlogsvc.Transactions(db)
	bs.Log, err = actionlogsvc.New(logDAL, tx, roundReader{buffDAL}, hpDAL)
	if err := bs.add(bs.Log, err); err != nil {
		return nil, err
	}
	logSvc := bs.Log
	// Choosing the current character is no game move: it runs on the store's
	// transactions, outside the action log.
	// The character's level, THAC0, saves and slots are derived from its
	// class, race and XP total (ADR 0030 steps 3 to 8): the class tables and
	// what the XP ledger's entries add.
	// The language catalogue (ADR 0032 step 4) is the artifact's: nil before
	// format 8, which knows no language.
	languages, _ := ruleTables.Languages()
	bs.Character, err = charactersvc.New(charactersvc.Deps{
		Reader: characterDAL, XP: xpsvc.Totals{DAL: st.XP}, Rules: ruleTables,
		Owners: characterDAL, Tx: tx, Sessions: bs.Auth, Log: logSvc, Languages: languages, HP: logSvc,
	})
	if err := bs.add(bs.Character, err); err != nil {
		return nil, err
	}
	charSvc := bs.Character
	// What a character prepares from: a cleric's sphere-open priest spells
	// (ADR 0030 step 15; no sphere access before catalogue format 6), every
	// other class's spellbook.
	sphereAccess, err := ruleTables.SphereAccess()
	if err != nil && !errors.Is(err, ruletablesvc.ErrNoSphereAccess) {
		return nil, err
	}
	prepSource, err := spheresvc.New(spheresvc.Deps{Book: spellbook, Characters: charSvc, Access: sphereAccess, Catalogue: cat})
	if err != nil {
		return nil, err
	}
	bs.Items, err = itemsvc.New(itemDAL, itemsvc.Deps{
		Dice: dice, Log: logSvc, Tx: tx, Chars: charSvc, Catalogue: catalogueWeapons,
	})
	if err := bs.add(bs.Items, err); err != nil {
		return nil, err
	}
	// The worn and wielded pieces' powers join the buffs' aggregate (ADR 0016
	// step 11, ADR 0033 step 9).
	bs.Buffs, err = buffsvc.New(buffDAL, tx, logSvc, charSvc, dice, bs.Items)
	if err := bs.add(bs.Buffs, err); err != nil {
		return nil, err
	}
	bs.Spells, err = spellsvc.New(spellDAL, catalogue, prepSource)
	if err := bs.add(bs.Spells, err); err != nil {
		return nil, err
	}
	bs.Prep, err = prepsvc.New(prepsvc.Deps{
		Slots: spellDAL, Spellbook: prepSource, Effects: effects, Items: bs.Items,
		Characters: charSvc, Log: logSvc, Tx: tx,
	})
	if err := bs.add(bs.Prep, err); err != nil {
		return nil, err
	}
	bs.Combat, err = combatsvc.New(combatsvc.Deps{
		Chars: charSvc, Buffs: bs.Buffs, Dice: dice, Weapons: combatsvc.ItemWeapons(bs.Items), Crit: critDAL,
		Items: bs.Items, Forms: st.Forms, Rules: ruleTables, Log: logSvc, Tx: tx,
	})
	if err := bs.add(bs.Combat, err); err != nil {
		return nil, err
	}
	bs.Cast, err = castsvc.New(castsvc.Deps{
		Slots: spellDAL, Effects: effects, Texts: catalogue, Buffs: bs.Buffs, Items: bs.Items, Attacks: bs.Combat,
		Characters: charSvc, Dice: dice, Log: logSvc, Tx: tx,
	})
	if err := bs.add(bs.Cast, err); err != nil {
		return nil, err
	}
	bs.Rest, err = restsvc.New(bs.Spells, bs.Items, logSvc, tx)
	if err := bs.add(bs.Rest, err); err != nil {
		return nil, err
	}
	bs.Details, err = detailssvc.New(characterDAL, tx, languages)
	if err := bs.add(bs.Details, err); err != nil {
		return nil, err
	}
	bs.XP, err = xpsvc.New(xpsvc.Deps{
		DAL: st.XP, Characters: charSvc, Thresholds: ruleTables, Log: logSvc, Tx: tx,
	})
	if err := bs.add(bs.XP, err); err != nil {
		return nil, err
	}
	// The level-up roll (ADR 0030 step 7) reads the pending levels from the
	// character's derived block and writes its hit points; its undo writes
	// them back.
	bs.LevelUp, err = levelupsvc.New(levelupsvc.Deps{
		DAL: characterDAL, Characters: charSvc, Rules: ruleTables, Dice: dice, Log: logSvc, Tx: tx,
	})
	if err := bs.add(bs.LevelUp, err); err != nil {
		return nil, err
	}
	// The creation's rolls (ADR 0032 steps 6 and 7) read the rule tables and
	// the dice, and write nothing.
	bs.Rolls, err = creationrollsvc.New(creationrollsvc.Deps{Rules: ruleTables, Dice: dice})
	if err := bs.add(bs.Rolls, err); err != nil {
		return nil, err
	}
	// Learning a spell (ADR 0030 step 20) reads the character's derived
	// spellbook rules and the catalogue, rolls the chance to learn and writes
	// the spellbook; its undo writes the spell's place back.
	bs.Learn, err = learnsvc.New(learnsvc.Deps{
		DAL: st.Spellbook, Book: st.Spellbook, Spells: spellbook, Characters: charSvc, Dice: dice, Log: logSvc, Tx: tx,
	})
	if err := bs.add(bs.Learn, err); err != nil {
		return nil, err
	}
	// Turning undead reads the cleric's Table 61 row (ADR 0030 step 16) and
	// rolls.
	bs.Turn, err = turnsvc.New(turnsvc.Deps{Characters: charSvc, Dice: dice, Log: logSvc, Tx: tx})
	if err := bs.add(bs.Turn, err); err != nil {
		return nil, err
	}
	// A thief's skill points spent on the sheet are undone by their
	// snapshot. The ability check's score in effect comes from buffsvc (ADR
	// 0028 step 2).
	bs.Skills, err = skillsvc.New(skillsvc.Deps{
		DAL: st.Skill, Characters: charSvc, Abilities: bs.Buffs, Dice: dice, Log: logSvc, Points: characterDAL, Tx: tx,
	})
	if err := bs.add(bs.Skills, err); err != nil {
		return nil, err
	}
	// The character's AC is derived on read from the pieces, the buffs and
	// powers that apply and the sheet's Dexterity (ADR 0033 step 10); its
	// override is a logged move undone by its snapshot (step 13).
	bs.AC, err = acsvc.New(acsvc.Deps{
		Items: bs.Items, Effects: bs.Buffs, Characters: charSvc, Tables: ruleTables,
		Overrides: characterDAL, Log: logSvc, Tx: tx,
	})
	if err := bs.add(bs.AC, err); err != nil {
		return nil, err
	}
	bs.Dice, err = dicesvc.New(dicesvc.Deps{DAL: st.Dice, Dice: dice, Log: logSvc, Tx: tx})
	if err := bs.add(bs.Dice, err); err != nil {
		return nil, err
	}

	// Every service's module on the log: the codecs that restore its
	// state-key families on undo and redo (ADR 0018 steps 2 and 8, ADR 0019
	// step 1) and its payload views. A family or a kind two modules claim
	// refuses the start.
	if err := registerModules(logSvc, bs.services); err != nil {
		return nil, err
	}
	return bs, nil
}

// add appends svc, just built (unless err), to the root's services.
func (b *BackingServices) add(svc domain.Service, err error) error {
	if err != nil {
		return err
	}
	b.services = append(b.services, svc)
	return nil
}

// registerModules registers the module of every service on the log, in
// order; the first refusal is the build's error.
func registerModules(log actionlogsvc.SVC, services []domain.Service) error {
	for _, svc := range services {
		if err := log.Register(svc.Module()); err != nil {
			return fmt.Errorf("register the modules: %w", err)
		}
	}
	return nil
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
