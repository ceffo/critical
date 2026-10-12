// Package hpsvc owns a character's hit points: query current state, record
// damage/heal, undo the most recent change. HP changes are hp_change entries
// in the action log (the Log port, satisfied by actionlogsvc); current HP is
// max HP plus the log's SUM of effective HP deltas.
package hpsvc

import (
	"context"
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// SVC is the driver port — what handlers call. Consumers should depend on
// narrower per-handler interfaces (see clean_architecture.md §7.3); the fat
// interface is for the composition root.
type SVC interface {
	domain.Service
	GetState(ctx context.Context, characterID int64) (*HPState, error)
	AppendEvent(ctx context.Context, characterID int64, delta int, source string) (*HPState, error)
	UndoLast(ctx context.Context, characterID int64) (*HPState, error)
}

// DAL is the storage-driven port. Adapters in internal/adapters/hpdal satisfy
// it. The service never imports database/sql.
type DAL interface {
	GetMaxHP(ctx context.Context, characterID int64) (int, error)
}

// Log is the slice of the action log hpsvc uses (satisfied by actionlogsvc).
type Log interface {
	actionlog.Recorder
	HPTotal(ctx context.Context, characterID int64) (int, error)
	ActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error)
	VoidLatest(ctx context.Context, characterID int64, kind actionlog.Kind) (bool, error)
}

type service struct {
	cfg *Config
	dal DAL
	log Log
}

// New constructs the service from environment-driven config.
func New(dal DAL, log Log) (SVC, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("hpsvc: parse env: %w", err)
	}
	return NewWithConfig(cfg, dal, log)
}

// NewWithConfig is the test-friendly constructor — it takes a literal config
// and pre-built ports.
func NewWithConfig(cfg *Config, dal DAL, log Log) (SVC, error) {
	if cfg == nil {
		return nil, fmt.Errorf("hpsvc: nil config")
	}
	if dal == nil {
		return nil, fmt.Errorf("hpsvc: nil dal")
	}
	if log == nil {
		return nil, fmt.Errorf("hpsvc: nil log")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("hpsvc: invalid config: %w", err)
	}
	return &service{cfg: cfg, dal: dal, log: log}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
