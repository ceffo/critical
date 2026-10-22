// Package actionlogsvc owns the action log: recording entries (atomically with
// the caller's state write), listing them with their correction status, and
// the audited amend/void corrections. Current HP is derived from it.
package actionlogsvc

import (
	"context"
	"errors"
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
	// Undo pops the transaction of the newest non-voided entry, whole, when
	// that entry's id is expectedTopID (ADR 0010, ADR 0018 step 2):
	// ErrTopChanged when another entry is on top, ErrNothingToUndo when the
	// log has none. Returns the transaction's first entry, voided.
	Undo(ctx context.Context, characterID, expectedTopID int64) (*actionlog.Entry, error)
	// Redo replays the newest popped transaction, when its first entry's id
	// is expectedRedoID (ADR 0019): ErrRedoChanged when another transaction
	// is the one to redo, ErrNothingToRedo when there is none. Returns the
	// transaction's first entry, live again.
	Redo(ctx context.Context, characterID, expectedRedoID int64) (*actionlog.Entry, error)
	// RedoTarget is the first entry of the transaction Redo replays, nil
	// when there is nothing to redo.
	RedoTarget(ctx context.Context, characterID int64) (*actionlog.Entry, error)
	// TopID is the newest non-voided entry, the one Undo pops; ok is false
	// when there is none.
	TopID(ctx context.Context, characterID int64) (id int64, ok bool, err error)
	// HPTotal is the SUM of effective HP deltas (actionlog.HPTotal over the log).
	HPTotal(ctx context.Context, characterID int64) (int, error)
	// PSPTotal is the SUM of effective PSP deltas (actionlog.PSPTotal over
	// the log, ADR 0043 step 5).
	PSPTotal(ctx context.Context, characterID int64) (int, error)
	// ActiveEntries returns the newest non-voided entries of kind.
	ActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error)
	// GetHPState is the HP read model: max HP, current HP (max plus the log's
	// total, within the floor and max, ADR 0007 step 6, ADR 0011 step 1), the
	// condition (ADR 0011 step 2) and the last HistoryLimit HP changes.
	GetHPState(ctx context.Context, characterID int64) (*actionlog.HPState, error)
	// AppendHPEvent logs an hp_change entry (delta signed, source optional)
	// and returns the new HP state. The cap and floor apply as in Record.
	AppendHPEvent(ctx context.Context, characterID int64, delta int, source string) (*actionlog.HPState, error)

	// Register binds a service's module: its codecs, so voiding an entry
	// with a snapshot under one of their families writes the "before" state
	// back (and a redo the "after"), and its payload views, applied to every
	// entry the log hands out. A family or an entry kind another module
	// claimed, or a codec that cannot restore, is refused and nothing of the
	// module is bound. Called by the composition root, before any request.
	Register(m domain.Module) error
}

// ErrClaimed is a state-key family or an entry kind two modules claim.
var ErrClaimed = errors.New("claimed by two modules")

// DAL is the storage port. Every method must use the transaction carried by
// ctx when there is one (see domain.TxRunner).
type DAL interface {
	// InsertEntry stores e in action-log transaction transactionID; 0 starts
	// a new transaction whose id is the new entry's (ADR 0018 step 1).
	InsertEntry(ctx context.Context, e actionlog.NewEntry, transactionID int64) (int64, error)
	InsertCorrection(ctx context.Context, c actionlog.Correction) error
	// GetEntry returns the entry with snapshots and corrections, or
	// domain.ErrNotFound.
	GetEntry(ctx context.Context, characterID, id int64) (*actionlog.Entry, error)
	ListEntries(ctx context.Context, characterID int64, limit int) ([]actionlog.Entry, error)
	// ListTransaction returns every entry of a transaction, voided ones
	// included, newest first.
	ListTransaction(ctx context.Context, characterID, transactionID int64) ([]actionlog.Entry, error)
	ListActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error)
	// LatestActiveID returns the newest non-voided entry, any kind.
	LatestActiveID(ctx context.Context, characterID int64) (id int64, ok bool, err error)
	// LatestPopped returns the entry whose standing void is the newest (the
	// last one popped and not redone since) and its transaction.
	LatestPopped(ctx context.Context, characterID int64) (id, transactionID int64, ok bool, err error)
	// LatestID returns the newest entry of kind, voided or not.
	LatestID(ctx context.Context, characterID int64, kind actionlog.Kind) (id int64, ok bool, err error)
	// HPTotal returns the sum of the effective HP deltas of every entry, by
	// the domain rule actionlog.HPTotal over the entries and their corrections.
	HPTotal(ctx context.Context, characterID int64) (int, error)
	// PSPTotal returns the sum of the effective PSP deltas of every entry,
	// by the domain rule actionlog.PSPTotal (ADR 0043 step 5).
	PSPTotal(ctx context.Context, characterID int64) (int, error)
}

// Rounds supplies the current round number used to stamp new entries. It is
// called inside the recording transaction.
type Rounds interface {
	GetRound(ctx context.Context, characterID int64) (int, error)
}

// Maxima supplies a character's stored maxima, read inside the recording
// transaction: max HP, which the HP floor needs (current HP is max HP plus the
// ledger total), and max PSP, which the PSP floor needs (ADR 0043 step 5).
type Maxima interface {
	GetMaxHP(ctx context.Context, characterID int64) (int, error)
	GetMaxPSP(ctx context.Context, characterID int64) (int, error)
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
	maxima Maxima
	mu     sync.RWMutex
	// restorers are the registered codecs by family, owners the module that
	// claimed each family and each kind's view.
	restorers map[string]actionlog.Restorer
	owners    map[string]string
	// views fill, on every read the service hands out, the payload fields an
	// entry's kind gained after it was logged. Internal reads (amend, undo,
	// redo logic) use the stored payload.
	views     actionlog.PayloadViews
	viewOwner map[actionlog.Kind]string
}

// New constructs the service from environment-driven config. The payload
// views registered modules declare (Register) are applied to every entry the
// service returns, so no reader serves an entry in the shape it was logged in.
func New(dal DAL, tx domain.TxRunner, rounds Rounds, maxima Maxima) (SVC, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("actionlogsvc: parse env: %w", err)
	}
	return NewWithConfig(cfg, dal, tx, rounds, maxima)
}

// NewWithConfig is the test-friendly constructor.
func NewWithConfig(cfg *Config, dal DAL, tx domain.TxRunner, rounds Rounds, maxima Maxima) (SVC, error) {
	switch {
	case cfg == nil:
		return nil, fmt.Errorf("actionlogsvc: nil config")
	case dal == nil:
		return nil, fmt.Errorf("actionlogsvc: nil dal")
	case tx == nil:
		return nil, fmt.Errorf("actionlogsvc: nil tx runner")
	case rounds == nil:
		return nil, fmt.Errorf("actionlogsvc: nil rounds")
	case maxima == nil:
		return nil, fmt.Errorf("actionlogsvc: nil maxima")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("actionlogsvc: invalid config: %w", err)
	}
	return &service{
		cfg: cfg, dal: dal, tx: Transactions(tx), rounds: rounds, maxima: maxima,
		restorers: map[string]actionlog.Restorer{}, owners: map[string]string{},
		views: actionlog.PayloadViews{}, viewOwner: map[actionlog.Kind]string{},
	}, nil
}

func (s *service) Register(m domain.Module) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	families := map[string]bool{}
	for _, c := range m.Codecs {
		family := c.Family()
		switch {
		case family == "" || actionlog.Family(family) != family:
			return fmt.Errorf("module %s: state-key family %q is not a family", m.Name, family)
		case actionlog.IsLogFamily(family):
			return fmt.Errorf("state-key family %q: %w (the log and %s)", family, ErrClaimed, m.Name)
		case families[family]:
			return fmt.Errorf("state-key family %q: %w (%s twice)", family, ErrClaimed, m.Name)
		}
		if owner, ok := s.owners[family]; ok {
			return fmt.Errorf("state-key family %q: %w (%s and %s)", family, ErrClaimed, owner, m.Name)
		}
		if err := c.CanRestore(); err != nil {
			return fmt.Errorf("module %s: %w", m.Name, err)
		}
		families[family] = true
	}
	for kind := range m.Views {
		if owner, ok := s.viewOwner[kind]; ok {
			return fmt.Errorf("payload view of %q: %w (%s and %s)", kind, ErrClaimed, owner, m.Name)
		}
	}
	for _, c := range m.Codecs {
		s.restorers[c.Family()] = c
		s.owners[c.Family()] = m.Name
	}
	for kind, v := range m.Views {
		s.views[kind] = v
		s.viewOwner[kind] = m.Name
	}
	return nil
}

// payloadViews are the registered payload views.
func (s *service) payloadViews() actionlog.PayloadViews {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.views
}

// restorer is the codec registered for stateKey's family ("items" restores
// "items:arrows_plus3").
func (s *service) restorer(stateKey string) actionlog.Restorer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.restorers[actionlog.Family(stateKey)]
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module is the log's own: it snapshots nothing.
func (s *service) Module() domain.Module { return domain.Module{Name: "actionlogsvc"} }
