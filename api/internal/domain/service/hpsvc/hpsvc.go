// Package hpsvc owns the HP append-only ledger for a character: query current
// state, append damage/heal events, undo the most recent event. The service
// declares its own ports — the DAL is implemented in internal/adapters/hpdal.
package hpsvc

import (
	"context"
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/ceffo/critical/api/internal/domain"
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
	AppendEvent(ctx context.Context, characterID int64, delta int, source string) error
	GetCurrentHPSum(ctx context.Context, characterID int64) (int, error)
	GetHistory(ctx context.Context, characterID int64, limit int) ([]HpEvent, error)
	GetMaxHP(ctx context.Context, characterID int64) (int, error)
	LastEvent(ctx context.Context, characterID int64) (delta int, source string, ok bool, err error)
}

type service struct {
	cfg *Config
	dal DAL
}

// New constructs the service from environment-driven config.
func New(dal DAL) (SVC, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("hpsvc: parse env: %w", err)
	}
	return NewWithConfig(cfg, dal)
}

// NewWithConfig is the test-friendly constructor — it takes a literal config
// and pre-built DAL.
func NewWithConfig(cfg *Config, dal DAL) (SVC, error) {
	if cfg == nil {
		return nil, fmt.Errorf("hpsvc: nil config")
	}
	if dal == nil {
		return nil, fmt.Errorf("hpsvc: nil dal")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("hpsvc: invalid config: %w", err)
	}
	return &service{cfg: cfg, dal: dal}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
