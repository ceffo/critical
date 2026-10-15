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
	actionlog.LatestAmender
	Void(ctx context.Context, characterID, id int64) (*actionlog.Entry, error)
	// VoidLatest pops the newest non-voided entry when it is of kind; ok is
	// false when the log has none, ErrNotLatest when it is another kind.
	VoidLatest(ctx context.Context, characterID int64, kind actionlog.Kind) (ok bool, err error)
	// Undo pops the newest non-voided entry, any kind, when its id is
	// expectedTopID (ADR 0010): ErrTopChanged when another entry is on top,
	// ErrNothingToUndo when the log has none. Returns the voided entry.
	Undo(ctx context.Context, characterID, expectedTopID int64) (*actionlog.Entry, error)
	// TopID is the newest non-voided entry, the one Undo pops; ok is false
	// when there is none.
	TopID(ctx context.Context, characterID int64) (id int64, ok bool, err error)
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
	// LatestActiveID returns the newest non-voided entry, any kind.
	LatestActiveID(ctx context.Context, characterID int64) (id int64, ok bool, err error)
	// LatestID returns the newest entry of kind, voided or not.
	LatestID(ctx context.Context, characterID int64, kind actionlog.Kind) (id int64, ok bool, err error)
	HPTotal(ctx context.Context, characterID int64) (int, error)
}

// Rounds supplies the current round number used to stamp new entries. It is
// called inside the recording transaction.
type Rounds interface {
	GetRound(ctx context.Context, characterID int64) (int, error)
}

// MaxHP supplies a character's max HP, which the HP floor needs (current HP is
// max HP plus the ledger total). Read inside the recording transaction.
type MaxHP interface {
	GetMaxHP(ctx context.Context, characterID int64) (int, error)
}

// Amendment is the only thing an amend may change: the entry's note (any
// kind; "" clears it). User, 2026-10-14: "no amending values. just the note".
type Amendment struct {
	Note string
}

type service struct {
	cfg    *Config
	dal    DAL
	tx     domain.TxRunner
	rounds Rounds
	maxHP  MaxHP

	mu        sync.RWMutex
	restorers map[string]actionlog.Restorer
}

// New constructs the service from environment-driven config.
func New(dal DAL, tx domain.TxRunner, rounds Rounds, maxHP MaxHP) (SVC, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("actionlogsvc: parse env: %w", err)
	}
	return NewWithConfig(cfg, dal, tx, rounds, maxHP)
}

// NewWithConfig is the test-friendly constructor.
func NewWithConfig(cfg *Config, dal DAL, tx domain.TxRunner, rounds Rounds, maxHP MaxHP) (SVC, error) {
	switch {
	case cfg == nil:
		return nil, fmt.Errorf("actionlogsvc: nil config")
	case dal == nil:
		return nil, fmt.Errorf("actionlogsvc: nil dal")
	case tx == nil:
		return nil, fmt.Errorf("actionlogsvc: nil tx runner")
	case rounds == nil:
		return nil, fmt.Errorf("actionlogsvc: nil rounds")
	case maxHP == nil:
		return nil, fmt.Errorf("actionlogsvc: nil max hp")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("actionlogsvc: invalid config: %w", err)
	}
	return &service{cfg: cfg, dal: dal, tx: tx, rounds: rounds, maxHP: maxHP, restorers: map[string]actionlog.Restorer{}}, nil
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
