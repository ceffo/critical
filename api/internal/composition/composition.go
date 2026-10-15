// Package composition is the composition root: the one place where concrete
// adapters meet service constructors. Per clean_architecture.md §3, every
// other package sees these dependencies through interfaces.
package composition

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/actionlogdal"
	"github.com/ceffo/critical/api/internal/adapters/buffdal"
	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/critdal"
	"github.com/ceffo/critical/api/internal/adapters/diceadpt"
	"github.com/ceffo/critical/api/internal/adapters/hpdal"
	"github.com/ceffo/critical/api/internal/adapters/itemdal"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/adapters/weapondal"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
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

	services []domain.Service
}

// Config is what cmd/api passes to New. Each field is set from env in
// internal/cmd/api/start.go.
type Config struct {
	DSN      string
	SeedsDir string
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
	// STR/DEX adjustments up in Tables 1-2. A gap in a table is a startup
	// error.
	ruleTables, err := ruletablesadpt.New(cfg.SeedsDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	spellbook, err := spellbookadpt.New(cfg.SeedsDir)
	if err != nil {
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
	logSvc, err := actionlogsvc.New(logDAL, db, roundReader{buffDAL})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	buffSvc, err := buffsvc.New(buffDAL, db, logSvc, charSvc)
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
		Slots: spellDAL, Spellbook: spellbook, Effects: effects, Limits: ruleTables,
		Characters: charSvc, Log: logSvc, Tx: db,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	logSvc.RegisterRestorer(prepsvc.StateKey, prepSvc)
	itemSvc, err := itemsvc.New(itemDAL, itemsvc.Deps{Dice: dice, Log: logSvc, Tx: db})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, itemSvc.Restorers())
	weaponSvc, err := weaponsvc.New(weaponEditor, weaponsvc.Deps{Log: logSvc, Tx: db})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registerRestorers(logSvc, weaponSvc.Restorers())
	combatSvc, err := combatsvc.New(combatsvc.Deps{
		Chars: charSvc, Buffs: buffSvc, Dice: dice, Weapons: weaponDAL, Crit: critDAL,
		Items: itemSvc, Rules: ruleTables, Log: logSvc, Tx: db,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	castSvc, err := castsvc.New(castsvc.Deps{
		Slots: spellDAL, Effects: effects, Texts: spellDAL, Buffs: buffSvc, Items: itemSvc, Attacks: combatSvc,
		Characters: charSvc, Dice: dice, Log: logSvc, Tx: db,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	restSvc, err := restsvc.New(spellSvc, itemSvc, logSvc, db)
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
		services:    []domain.Service{charSvc, buffSvc, logSvc, hpSvc, spellSvc, prepSvc, castSvc, combatSvc, itemSvc, weaponSvc, restSvc},
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
