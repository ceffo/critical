// Package actionlogsvc owns the action log: recording entries (atomically with
// the caller's state write), listing them with their correction status, and
// the audited amend/void corrections. Current HP is derived from it.
package actionlogsvc

import (
	"context"
	"fmt"
	"sync"

	"github.com/caarlos0/env/v11"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	actionlog.Recorder

	// List returns the newest entries first, every kind, with corrections.
	List(ctx context.Context, characterID int64, limit int) ([]actionlog.Entry, error)
	Amend(ctx context.Context, characterID, id int64, a Amendment) (*actionlog.Entry, error)
	Void(ctx context.Context, characterID, id int64) (*actionlog.Entry, error)
	// VoidLatest voids the newest non-voided entry of kind; ok is false when
	// there is none.
	VoidLatest(ctx context.Context, characterID int64, kind actionlog.Kind) (ok bool, err error)
	// HPTotal is the SUM of effective HP deltas, computed in storage.
	HPTotal(ctx context.Context, characterID int64) (int, error)
	// ActiveEntries returns the newest non-voided entries of kind.
	ActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error)

	// RegisterRestorer binds the service that owns stateKey, so voiding an
	// entry with a snapshot under that key can write the "before" state back.
	// Called by the composition root.
	RegisterRestorer(stateKey string, r actionlog.Restorer)
}

// DAL is the storage port. Every method must use the transaction carried by
// ctx when there is one (see domain.TxRunner).
type DAL interface {
	InsertEntry(ctx context.Context, e actionlog.NewEntry) (int64, error)
	InsertCorrection(ctx context.Context, c actionlog.Correction) error
	// GetEntry returns the entry with snapshots and corrections, or
	// domain.ErrNotFound.
	GetEntry(ctx context.Context, characterID, id int64) (*actionlog.Entry, error)
	ListEntries(ctx context.Context, characterID int64, limit int) ([]actionlog.Entry, error)
	ListActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error)
	LatestActiveID(ctx context.Context, characterID int64, kind actionlog.Kind) (id int64, ok bool, err error)
	// LaterEntryTouchingState returns the first non-voided entry after id
	// that has a snapshot under one of id's state keys.
	LaterEntryTouchingState(ctx context.Context, characterID, id int64) (laterID int64, ok bool, err error)
	HPTotal(ctx context.Context, characterID int64) (int, error)
}

// Rounds supplies the current round number used to stamp new entries. It is
// called inside the recording transaction.
type Rounds interface {
	GetRound(ctx context.Context, characterID int64) (int, error)
}

// Amendment is the new value of an hp_change entry. An empty Summary keeps
// the current one.
type Amendment struct {
	HPDelta int
	Summary string
}

type service struct {
	cfg    *Config
	dal    DAL
	tx     domain.TxRunner
	rounds Rounds

	mu        sync.RWMutex
	restorers map[string]actionlog.Restorer
}

// New constructs the service from environment-driven config.
func New(dal DAL, tx domain.TxRunner, rounds Rounds) (SVC, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("actionlogsvc: parse env: %w", err)
	}
	return NewWithConfig(cfg, dal, tx, rounds)
}

// NewWithConfig is the test-friendly constructor.
func NewWithConfig(cfg *Config, dal DAL, tx domain.TxRunner, rounds Rounds) (SVC, error) {
	switch {
	case cfg == nil:
		return nil, fmt.Errorf("actionlogsvc: nil config")
	case dal == nil:
		return nil, fmt.Errorf("actionlogsvc: nil dal")
	case tx == nil:
		return nil, fmt.Errorf("actionlogsvc: nil tx runner")
	case rounds == nil:
		return nil, fmt.Errorf("actionlogsvc: nil rounds")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("actionlogsvc: invalid config: %w", err)
	}
	return &service{cfg: cfg, dal: dal, tx: tx, rounds: rounds, restorers: map[string]actionlog.Restorer{}}, nil
}

func (s *service) RegisterRestorer(stateKey string, r actionlog.Restorer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restorers[stateKey] = r
}

// restorer is the Restorer registered for stateKey, else the one registered
// for its family ("items" restores "items:arrows_plus3").
func (s *service) restorer(stateKey string) actionlog.Restorer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if r, ok := s.restorers[stateKey]; ok {
		return r
	}
	return s.restorers[actionlog.Family(stateKey)]
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
