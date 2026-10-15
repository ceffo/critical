// Package buffsvc owns active buffs, the round counter, and effect aggregation.
// Round advancement is here because it expires duration-based buffs — the
// round counter and buff list are one bounded concern.
//
// Every change (activate, edit, absorb, dismiss, Next Round) is recorded in
// the action log with before/after snapshots of the buff list (and the round
// for Next Round) in the same transaction as the state write, so voiding the
// entry restores the state through Restorers.
package buffsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	ListActive(ctx context.Context, characterID int64) ([]BuffEffect, error)
	// ListStatus is the active list with whether each buff's effects apply
	// at the current effective STR.
	ListStatus(ctx context.Context, characterID int64) ([]BuffStatus, error)
	Activate(ctx context.Context, characterID int64, buff BuffEffect) ([]BuffEffect, error)
	// Add puts a buff on the active list (replacing one with the same name)
	// without recording: the caller records the returned snapshot in its own
	// entry, inside the transaction ctx carries (a spell cast).
	Add(ctx context.Context, characterID int64, buff BuffEffect) (BuffEffect, actionlog.Snapshot, error)
	Edit(ctx context.Context, characterID int64, buffID string, e BuffEdit) ([]BuffEffect, error)
	Dismiss(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error)
	// Trigger uses one charge of a charged buff (a Stoneskin layer absorbed a
	// hit); the buff ends at 0.
	Trigger(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error)
	GetRound(ctx context.Context, characterID int64) (int, error)
	AdvanceRound(ctx context.Context, characterID int64) (active []BuffEffect, expired []BuffEffect, round int, err error)
	// ResetRound ends combat: the counter goes back to 0 (Next Round then
	// gives 1). Active buffs are untouched. Logged, voidable.
	ResetRound(ctx context.Context, characterID int64) (active []BuffEffect, round int, err error)
	// SetRound corrects the counter to `to` (>= 0, else ErrInvalidInput).
	// Active buffs are untouched. Logged, voidable.
	SetRound(ctx context.Context, characterID int64, to int) (active []BuffEffect, round int, err error)
	// PassTime fast-forwards game time in hours: timed buffs lose
	// RoundsForHours(hours) rounds (expired at <= 0), charge-only and untimed
	// buffs are untouched, the round counter goes to 0. Invalid hours (<= 0,
	// > MaxPassHours, under one round): ErrInvalidInput. Logged, voidable.
	PassTime(ctx context.Context, characterID int64, hours float64) (active []BuffEffect, expired []BuffEffect, rounds int, round int, err error)
	// PreviewPassTime is what PassTime(hours) would do, without writing or
	// logging anything: the rounds the hours give and the buffs that would
	// expire. Same validation (ErrInvalidInput).
	PreviewPassTime(ctx context.Context, characterID int64, hours float64) (rounds int, expiring []BuffEffect, err error)

	// ListSaved returns the saved buff templates, oldest first.
	ListSaved(ctx context.Context, characterID int64) ([]SavedBuff, error)
	// SaveBuff stores a template (replacing one with the same name) and logs
	// it; returns the list. Voidable.
	SaveBuff(ctx context.Context, characterID int64, saved SavedBuff) ([]SavedBuff, error)
	// DeleteSaved removes a template and logs it; active buffs are untouched.
	// Unknown id: ErrNotFound. Voidable.
	DeleteSaved(ctx context.Context, characterID int64, id int64) ([]SavedBuff, error)

	// Aggregate is consumed by combatsvc (see clean_architecture.md §4.5).
	// str is the base STR from the sheet; STR buffs raise it before MinStr
	// gating, and the result carries the effective STR used.
	Aggregate(ctx context.Context, characterID int64, str int) (BuffModifiers, error)
	// GetEffectiveStr is the character's base STR and its STR with buffs.
	GetEffectiveStr(ctx context.Context, characterID int64) (Strength, error)

	// Restorers maps each state key buffsvc snapshots to the Restorer that
	// writes a "before" state back on void. The composition root registers
	// every one on the action log.
	Restorers() map[string]actionlog.Restorer
}

// DAL is the storage-driven port. Every method must use the transaction
// carried by ctx when there is one (see domain.TxRunner).
type DAL interface {
	GetActiveBuffs(ctx context.Context, characterID int64) ([]BuffEffect, error)
	SaveActiveBuffs(ctx context.Context, characterID int64, buffs []BuffEffect) error
	GetRoundNumber(ctx context.Context, characterID int64) (int, error)
	SaveRoundNumber(ctx context.Context, characterID int64, round int) error
	ListSavedBuffs(ctx context.Context, characterID int64) ([]SavedBuff, error)
	// ReplaceSavedBuffs makes the stored list equal to saved: entries with an
	// ID keep it (a restore), entries without get a new one.
	ReplaceSavedBuffs(ctx context.Context, characterID int64, saved []SavedBuff) error
}

// Log is the slice of the action log buffsvc records into (actionlogsvc).
type Log interface {
	actionlog.Recorder
}

// Characters supplies the base STR from the sheet (charactersvc).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

type service struct {
	dal   DAL
	tx    domain.TxRunner
	log   Log
	chars Characters
}

// New constructs the service.
func New(dal DAL, tx domain.TxRunner, log Log, chars Characters) (SVC, error) {
	return NewWithConfig(DefaultConfig(), dal, tx, log, chars)
}

// NewWithConfig is the test-friendly constructor.
func NewWithConfig(cfg *Config, dal DAL, tx domain.TxRunner, log Log, chars Characters) (SVC, error) {
	switch {
	case cfg == nil:
		return nil, fmt.Errorf("buffsvc: nil config")
	case dal == nil:
		return nil, fmt.Errorf("buffsvc: nil dal")
	case tx == nil:
		return nil, fmt.Errorf("buffsvc: nil tx runner")
	case log == nil:
		return nil, fmt.Errorf("buffsvc: nil log")
	case chars == nil:
		return nil, fmt.Errorf("buffsvc: nil characters")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("buffsvc: invalid config: %w", err)
	}
	return &service{dal: dal, tx: tx, log: log, chars: chars}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
