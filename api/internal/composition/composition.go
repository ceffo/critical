// Package composition is the composition root: the one place where concrete
// adapters meet service constructors. Per clean_architecture.md §3, every
// other package sees these dependencies through interfaces.
package composition

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/adapters/buffdal"
	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/diceadpt"
	"github.com/ceffo/critical/api/internal/adapters/hpdal"
	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/adapters/weaponsadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/hpsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
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
	Buffs     buffsvc.SVC
	Spells    spellsvc.SVC
	Combat    combatsvc.SVC

	services []domain.Service
}

// Config is what cmd/api passes to New. Each field is set from env in
// internal/cmd/api/start.go.
type Config struct {
	DSN      string
	SeedsDir string
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
	buffDAL := buffdal.New(db, db)
	spellDAL := spelldal.New(db)
	dice := diceadpt.New()
	weapons, err := weaponsadpt.New(cfg.SeedsDir)
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
	hpSvc, err := hpsvc.New(hpDAL)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	buffSvc, err := buffsvc.New(buffDAL)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	spellSvc, err := spellsvc.New(spellDAL, spellDAL)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	combatSvc, err := combatsvc.New(charSvc, buffSvc, dice, weapons)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &BackingServices{
		DB:          db,
		CharacterID: charID,
		Character:   charSvc,
		HP:          hpSvc,
		Buffs:       buffSvc,
		Spells:      spellSvc,
		Combat:      combatSvc,
		services:    []domain.Service{charSvc, hpSvc, buffSvc, spellSvc, combatSvc},
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
