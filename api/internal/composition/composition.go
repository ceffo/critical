// Package composition is the composition root: the one place where concrete
// adapters meet service constructors. Per clean_architecture.md §3, every
// other package sees these dependencies through interfaces.
package composition

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/adapters/attackformdal"
	"github.com/ceffo/critical/api/internal/adapters/buffdal"
	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/critdal"
	"github.com/ceffo/critical/api/internal/adapters/diceadpt"
	"github.com/ceffo/critical/api/internal/adapters/dicedal"
	"github.com/ceffo/critical/api/internal/adapters/hpdal"
	"github.com/ceffo/critical/api/internal/adapters/itemdal"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/adapters/skilldal"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/adapters/weapondal"
	"github.com/ceffo/critical/api/internal/adapters/xpdal"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

// BackingServices holds every business service plus the storage handle and the
// resolved character id (single-character mode). Routers select which slices
// to mount.
type BackingServices struct {
	DB          *sqlite.DB
	CharacterID int64

	Character charactersvc.SVC
	HP        hpsvc.SVC
	Log       actionlogsvc.SVC
	Buffs     buffsvc.SVC
	Spells    spellsvc.SVC
	Prep      prepsvc.SVC
	Cast      castsvc.SVC
	Combat    combatsvc.SVC
	Items     itemsvc.SVC
	Weapons   weaponsvc.SVC
	Rest      restsvc.SVC
	XP        xpsvc.SVC
	Details   detailssvc.SVC
	Skills    skillsvc.SVC
	Dice      dicesvc.SVC

	// LogPayloadViews fill, when the log is served, the payload fields an
	// entry's kind gained after the entry was logged.
	LogPayloadViews actionlog.PayloadViews

	services []domain.Service
}

// Config is what cmd/api passes to New. Each field is set from env in
// internal/cmd/api/start.go.
type Config struct {
	DSN      string
	SeedsDir string
	// Catalogue is the path of the spell catalogue artifact (ADR 0021),
	// imported into the database at startup.
	Catalogue string
	// Dice replaces the crypto dice (tests that need known rolls). Nil in
	// production.
	Dice Dice
}

// Dice is every dice port the services take (diceadpt satisfies it).
type Dice interface {
	RollDie(sides int) int
	RollD20() int
	RollD100() int
}

// New opens storage, runs migrations + seeding, builds adapters, then services.
// Returns an initialized BackingServices ready to be mounted on a router.
func New(ctx context.Context, cfg Config) (*BackingServices, error) {
	db, err := sqlite.Open(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := db.Seed(ctx, cfg.SeedsDir); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}

	characterDAL := characterdal.New(db)
	hpDAL := hpdal.New(db)
	logDAL := actionlogdal.New(db)
	buffDAL := buffdal.New(db)
	spellDAL := spelldal.New(db)
	var dice Dice = diceadpt.New()
	if cfg.Dice != nil {
		dice = cfg.Dice
	}
	weaponDAL := weapondal.New(db)
	weaponEditor := weapondal.NewEditor(db)
	critDAL := critdal.New(db)
	itemDAL := itemdal.New(db)

	// The spell effects casting applies; a malformed formula is a startup
	// error.
	effects, err := spelleffectsadpt.New(cfg.SeedsDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// The PHB rule tables: spell preparation takes Table 32, attacks look
	// STR/DEX adjustments up in Tables 1-2, the XP ledger Table 25. A gap in a table is a startup
	// error.
	ruleTables, err := ruletablesadpt.New(cfg.SeedsDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// The spell catalogue (ADR 0021 step 5): a missing or invalid artifact,
	// or one that removes a spell still referenced, is a startup error.
	cat, err := catalogueadpt.Load(cfg.Catalogue)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := effects.CheckAgainst(cat); err != nil {
		_ = db.Close()
		return nil, err
	}
	spellbook, err := spellbookadpt.New(cfg.SeedsDir, cat)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.ImportCatalogue(ctx, cat, seedSpellRefs(spellbook, effects)); err != nil {
		_ = db.Close()
		return nil, err
	}

	charID, err := characterDAL.FirstID(ctx)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("resolve character: %w", err)
	}

	charSvc, err := charactersvc.New(characterDAL)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// The action log stamps entries with the round read straight from
	// buffdal (buffsvc itself records into the log, so it cannot be the
	// log's dependency). Services that record snapshots register their
	// Restorers on logSvc here.
	//
	// tx is the one TxRunner every service gets: each outermost InTx is one
	// action-log transaction, so whatever a request records is popped by one
	// undo (ADR 0018 steps 1 and 2).
	tx := actionlogsvc.Transactions(db)
	logSvc, err := actionlogsvc.New(logDAL, tx, roundReader{buffDAL}, hpDAL)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	itemSvc, err := itemsvc.New(itemDAL, itemsvc.Deps{Dice: dice, Log: logSvc, Tx: tx})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, itemSvc.Restorers())
	// The carried items' powers join the buffs' aggregate (ADR 0016 step 11).
	buffSvc, err := buffsvc.New(buffDAL, tx, logSvc, charSvc, dice, itemSvc)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, buffSvc.Restorers())
	hpSvc, err := hpsvc.New(hpDAL, logSvc)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	spellSvc, err := spellsvc.New(spellDAL, spellDAL, spellbook)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	prepSvc, err := prepsvc.New(prepsvc.Deps{
		Slots: spellDAL, Spellbook: spellbook, Effects: effects, Limits: ruleTables, Items: itemSvc,
		Characters: charSvc, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	logSvc.RegisterRestorer(prepsvc.StateKey, prepSvc)
	weaponSvc, err := weaponsvc.New(weaponEditor, weaponsvc.Deps{Log: logSvc, Tx: tx})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, weaponSvc.Restorers())
	combatSvc, err := combatsvc.New(combatsvc.Deps{
		Chars: charSvc, Buffs: buffSvc, Dice: dice, Weapons: weaponDAL, Crit: critDAL,
		Items: itemSvc, Forms: attackformdal.New(db), Rules: ruleTables, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	castSvc, err := castsvc.New(castsvc.Deps{
		Slots: spellDAL, Effects: effects, Texts: spellDAL, Buffs: buffSvc, Items: itemSvc, Attacks: combatSvc,
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
	detailsSvc, err := detailssvc.New(characterDAL, tx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	xpSvc, err := xpsvc.New(xpsvc.Deps{
		DAL: xpdal.New(db), Characters: charSvc, Thresholds: ruleTables, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, xpSvc.Restorers())
	// Skill checks change no state: their entries carry no snapshot, so
	// there is no restorer to register.
	skillSvc, err := skillsvc.New(skillsvc.Deps{
		DAL: skilldal.New(db), Characters: charSvc, Dice: dice, Log: logSvc, Tx: tx,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	// Free dice rolls change no state: no snapshot, no restorer.
	diceSvc, err := dicesvc.New(dicesvc.Deps{DAL: dicedal.New(db), Dice: dice, Log: logSvc, Tx: tx})
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &BackingServices{
		DB:          db,
		CharacterID: charID,
		Character:   charSvc,
		HP:          hpSvc,
		Log:         logSvc,
		Buffs:       buffSvc,
		Spells:      spellSvc,
		Prep:        prepSvc,
		Cast:        castSvc,
		Combat:      combatSvc,
		Items:       itemSvc,
		Weapons:     weaponSvc,
		Rest:        restSvc,
		XP:          xpSvc,
		Details:     detailsSvc,
		Skills:      skillSvc,
		Dice:        diceSvc,
		LogPayloadViews: actionlog.PayloadViews{
			actionlog.KindSave: combatsvc.SavePayloadView,
		},
		services: []domain.Service{
			charSvc, buffSvc, logSvc, hpSvc, spellSvc, prepSvc, castSvc, combatSvc, itemSvc, weaponSvc, restSvc, xpSvc, skillSvc, detailsSvc, diceSvc,
		},
	}, nil
}

// Close shuts down every service in reverse order, then closes the DB.
func (b *BackingServices) Close(ctx context.Context) error {
	var firstErr error
	for i := len(b.services) - 1; i >= 0; i-- {
		if err := b.services[i].Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if b.DB != nil {
		if err := b.DB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// roundReader satisfies actionlogsvc.Rounds from buffdal. It runs inside the
// recording transaction (buffdal follows the ctx transaction).
type roundReader struct{ dal *buffdal.DAL }

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

// seedSpellRefs are the spell ids the seeds refer to: a catalogue import may
// not remove any of them (ADR 0021 step 5.4.3).
func seedSpellRefs(book *spellbookadpt.Book, effects *spelleffectsadpt.Catalog) []catalogue.Ref {
	var refs []catalogue.Ref
	for _, e := range book.All() {
		refs = append(refs, catalogue.Ref{ID: e.ID, Where: "spellbook seed"})
	}
	for _, e := range effects.All() {
		refs = append(refs, catalogue.Ref{ID: e.ID, Where: "spell effects seed"})
	}
	return refs
}
