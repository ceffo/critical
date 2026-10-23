// Package xpsvc owns the experience ledger (ADR 0015): the awards the
// player records, the total they add up to with the sheet's starting XP, the
// +10 percent prime-requisite bonus and the progress towards the next level
// from PHB Table 25. Every add, edit and delete is one action-log
// transaction with the entry's snapshot, so the journal's Undo pops it (ADR
// 0015 step 4, ADR 0018 scope). The level is derived from the total (ADR 0030
// step 3); the hit points a new level owes are levelupsvc's (step 7).
package xpsvc

import (
	"context"
	"fmt"
	"time"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// SVC is the driver port. Every write returns the whole ledger as it is
// afterwards, so a client renders it from one answer.
type SVC interface {
	domain.Service
	// Get returns the ledger with its totals and progress.
	Get(ctx context.Context, characterID int64) (*Ledger, error)
	// Preview returns what an entry of base XP would add to the character,
	// shared between its classes, without writing. A *domain.FieldError for
	// an invalid base.
	Preview(ctx context.Context, characterID int64, base int, bonus bool) (Amount, error)
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
	// Reinsert writes an entry back under its own id and date (the undo of
	// a delete).
	Reinsert(ctx context.Context, characterID int64, e Entry) error
	// Update overwrites the session, reason, base and bonus of an entry;
	// ErrEntryNotFound for an unknown id.
	Update(ctx context.Context, characterID int64, e Entry) error
	// Delete removes an entry; ErrEntryNotFound for an unknown id.
	Delete(ctx context.Context, characterID, entryID int64) error
}

// Characters reads the sheet: the starting XP, the level and the prime
// requisites.
type Characters interface {
	View(ctx context.Context, id int64) (charactersvc.View, error)
	// ClassEntries are the stored class entries, without the derivation: the
	// award's split and the journal's payload need no more.
	ClassEntries(ctx context.Context, id int64) ([]classes.Entry, error)
}

// Log is the slice of the action log XP changes are recorded into.
type Log interface {
	actionlog.Recorder
}

// Rules are the catalogue's class definitions (ADR 0040 step 5): a class's
// experience table (PHB Tables 14, 20, 23 and 25, read through
// classes.Class.XPThreshold) and its prime requisites (ADR 0040 step 6), which
// pick the bonus.
type Rules interface {
	Definitions() (*classes.Registry, error)
}

// Deps are the service's collaborators. Now is optional (time.Now).
type Deps struct {
	DAL        DAL
	Characters Characters
	Rules      Rules
	Log        Log
	Tx         domain.TxRunner
	Now        func() time.Time
}

type service struct {
	dal   DAL
	chars Characters
	rules Rules
	log   Log
	tx    domain.TxRunner
	now   func() time.Time
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Characters == nil || d.Rules == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("xpsvc: nil dependency")
	}
	now := d.Now
	if now == nil {
		now = time.Now
	}
	return &service{dal: d.DAL, chars: d.Characters, rules: d.Rules, log: d.Log, tx: d.Tx, now: now}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Totals is what a character's ledger entries add to the starting XP (ADR
// 0015 step 1), read through the ledger's DAL: the character service derives
// the level from it (ADR 0030 step 3) without depending on this service.
type Totals struct{ DAL DAL }

// Awards are the character's entries' amounts, each with its bonus (ADR 0015
// step 1), newest first: a multi-class character's entries share each award
// on its own (ADR 0042 step 5).
func (t Totals) Awards(ctx context.Context, characterID int64) ([]charactersvc.Award, error) {
	entries, err := t.DAL.List(ctx, characterID)
	if err != nil {
		return nil, err
	}
	out := make([]charactersvc.Award, len(entries))
	for i, e := range entries {
		out[i] = charactersvc.Award{ID: e.ID, Amount: EntryAmount(e.Base, e.Bonus).Total}
	}
	return out, nil
}
