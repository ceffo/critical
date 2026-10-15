// Package xpsvc owns the experience ledger (ADR 0015): the awards the
// player records, the total they add up to with the sheet's starting XP, the
// +10 percent prime-requisite bonus and the progress towards the next level
// from PHB Table 25. The ledger sits outside the action log: an entry is
// edited or deleted in place. Levelling up is not modelled, the level stays
// the sheet's.
package xpsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// SVC is the driver port. Every write returns the whole ledger as it is
// afterwards, so a client renders it from one answer.
type SVC interface {
	domain.Service
	// Get returns the ledger with its totals and progress.
	Get(ctx context.Context, characterID int64) (*Ledger, error)
	// Preview returns what an entry of base XP would add, without writing.
	// A *domain.FieldError for an invalid base.
	Preview(base int, bonus bool) (Amount, error)
	// Add records an award dated today. A *domain.FieldError for invalid input.
	Add(ctx context.Context, characterID int64, in EntryInput) (*Ledger, error)
	// Update changes the fields set in the patch. ErrEntryNotFound,
	// *domain.FieldError.
	Update(ctx context.Context, characterID, entryID int64, p EntryPatch) (*Ledger, error)
	// Delete removes an entry. ErrEntryNotFound.
	Delete(ctx context.Context, characterID, entryID int64) (*Ledger, error)
}

// DAL is the driven port over the stored entries. Entries come back with
// their stored fields only (BonusAmount and Total are the service's).
// Every method joins the transaction carried by ctx.
type DAL interface {
	// List returns the character's entries, newest first.
	List(ctx context.Context, characterID int64) ([]Entry, error)
	// Get returns one entry, or ErrEntryNotFound.
	Get(ctx context.Context, characterID, entryID int64) (Entry, error)
	// Insert writes a new entry and returns its id.
	Insert(ctx context.Context, characterID int64, e Entry) (int64, error)
	// Update overwrites the session, reason, base and bonus of an entry;
	// ErrEntryNotFound for an unknown id.
	Update(ctx context.Context, characterID int64, e Entry) error
	// Delete removes an entry; ErrEntryNotFound for an unknown id.
	Delete(ctx context.Context, characterID, entryID int64) error
}

// Characters reads the sheet: the starting XP, the level and the prime
// requisites.
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Thresholds is PHB Table 25 (Thief/Bard column): the XP that reaches a
// level, an error outside the table.
type Thresholds interface {
	BardXPThreshold(level int) (int, error)
}

// Deps are the service's collaborators. Now is optional (time.Now).
type Deps struct {
	DAL        DAL
	Characters Characters
	Thresholds Thresholds
	Tx         domain.TxRunner
	Now        func() time.Time
}

type service struct {
	dal   DAL
	chars Characters
	table Thresholds
	tx    domain.TxRunner
	now   func() time.Time
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Characters == nil || d.Thresholds == nil || d.Tx == nil {
		return nil, fmt.Errorf("xpsvc: nil dependency")
	}
	now := d.Now
	if now == nil {
		now = time.Now
	}
	return &service{dal: d.DAL, chars: d.Characters, table: d.Thresholds, tx: d.Tx, now: now}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
