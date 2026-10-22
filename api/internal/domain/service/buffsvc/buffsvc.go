// Package buffsvc owns active buffs, the round counter and its initiative, and
// effect aggregation.
// Round advancement is here because it expires duration-based buffs — the
// round counter and buff list are one bounded concern.
//
// Every change (activate, edit, absorb, dismiss, Next Round) is recorded in
// the action log with before/after snapshots of the buff list (and the round
// for Next Round) in the same transaction as the state write, so voiding the
// entry restores the state through the codecs of its module (Module).
package buffsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
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
	// Remove takes buff buffID off the active list and puts putBack in its
	// place, without recording: the caller records the returned snapshot in
	// its own entry, inside the transaction ctx carries (a skill check's
	// verdict turned to a failure gives back the rows its buff renewed, ADR
	// 0037 step 20.8). removed is false, and nothing is written, when the
	// buff is not on the list.
	Remove(ctx context.Context, characterID int64, buffID string, putBack []BuffEffect) (removed bool, snap actionlog.Snapshot, err error)
	// EndMaintained takes every maintained power's row off without
	// recording (a long rest, ADR 0043 step 7): the caller records the
	// snapshot, inside the transaction ctx carries. ended is empty, and
	// nothing is written, when no power is maintained.
	EndMaintained(ctx context.Context, characterID int64) (ended []BuffEffect, snap actionlog.Snapshot, err error)
	Edit(ctx context.Context, characterID int64, buffID string, e BuffEdit) ([]BuffEffect, error)
	// Dismiss stops a row; cause is how it ended when its spell has end
	// effects (ADR 0004 step 21), and end what they did.
	Dismiss(ctx context.Context, characterID int64, buffID string, cause spelleffectsvc.EndCause) (active []BuffEffect, end EndResult, err error)
	// Trigger uses one charge of a charged buff (a Stoneskin layer absorbed a
	// hit); the buff ends at 0.
	Trigger(ctx context.Context, characterID int64, buffID string) ([]BuffEffect, error)
	GetRound(ctx context.Context, characterID int64) (int, error)
	// AdvanceRound is Next Round; it returns the new round's initiative
	// (ADR 0013 steps 1 and 2) and what the expired rows' end effects did
	// (ADR 0004 step 21).
	AdvanceRound(ctx context.Context, characterID int64) (active, expired []BuffEffect, round int, initiative Initiative, end EndResult, err error)
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
	// end is what the expired rows' end effects did (ADR 0004 step 21).
	PassTime(ctx context.Context, characterID int64, hours float64) (active, expired []BuffEffect, rounds, round int, end EndResult, err error)
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
	DeleteSaved(ctx context.Context, characterID, id int64) ([]SavedBuff, error)

	// ListPowers is the carried items' powers that are on at the current
	// effective STR (ADR 0016 step 13), in item and power order; a power that
	// is off is not in it.
	ListPowers(ctx context.Context, characterID int64) ([]ItemPower, error)

	// Aggregate is consumed by combatsvc (see clean_architecture.md §4.5).
	// str is the base STR from the sheet; STR buffs raise it before MinStr
	// gating and power triggers, and the result carries the effective STR
	// used. kind is the weapon kind of the roll it serves (WeaponNone for a
	// save or a spell's to-hit, ADR 0004 step 18).
	Aggregate(ctx context.Context, characterID int64, str BaseStrength, kind WeaponKind) (BuffModifiers, error)
	// GetEffectiveStr is the character's base STR and its STR with buffs.
	GetEffectiveStr(ctx context.Context, characterID int64) (Strength, error)
	// GetAbilityScore is the sheet's score in ability a and the score in
	// effect with the effects that apply to it (ADR 0028 step 2). An unknown
	// key wraps domain.ErrInvalidInput.
	GetAbilityScore(ctx context.Context, characterID int64, a charactersvc.Ability) (AbilityScore, error)

	// GetInitiative is the round's initiative, nil when there is none (ADR
	// 0013 step 5).
	GetInitiative(ctx context.Context, characterID int64) (*Initiative, error)
	// RerollInitiative rolls a new initiative for the round as its own
	// transaction, replacing the current one (ADR 0013 step 4, ADR 0018 step
	// 5). Refused (a conflict) when there is no initiative or a newer
	// transaction is on the log.
	RerollInitiative(ctx context.Context, characterID int64) (Initiative, error)
	// CorrectInitiative replaces the round's initiative by the same d10 with
	// a spell cast's or an item used's term (ADR 0035 item 6.2), logged as
	// its own entry inside the caller's transaction, which it must end. It
	// does nothing (nil) out of combat or with no initiative.
	CorrectInitiative(ctx context.Context, characterID int64, cause InitiativeModifier) (*Initiative, error)
}

// DAL is the storage-driven port. Every method must use the transaction
// carried by ctx when there is one (see domain.TxRunner).
type DAL interface {
	GetActiveBuffs(ctx context.Context, characterID int64) ([]BuffEffect, error)
	SaveActiveBuffs(ctx context.Context, characterID int64, buffs []BuffEffect) error
	GetRoundNumber(ctx context.Context, characterID int64) (int, error)
	SaveRoundNumber(ctx context.Context, characterID int64, round int) error
	// GetInitiative returns the stored initiative, nil when there is none.
	GetInitiative(ctx context.Context, characterID int64) (*Initiative, error)
	// SaveInitiative stores the initiative; nil clears it.
	SaveInitiative(ctx context.Context, characterID int64, initiative *Initiative) error
	ListSavedBuffs(ctx context.Context, characterID int64) ([]SavedBuff, error)
	// ReplaceSavedBuffs makes the stored list equal to saved: entries with an
	// ID keep it (a restore), entries without get a new one.
	ReplaceSavedBuffs(ctx context.Context, characterID int64, saved []SavedBuff) error
}

// Log is the slice of the action log buffsvc records into (actionlogsvc).
// TopID is the newest non-voided entry (ok false: none); the initiative
// reroll reads it inside its transaction.
type Log interface {
	actionlog.Recorder
	TopID(ctx context.Context, characterID int64) (id int64, ok bool, err error)
	// HPTotal is the ledger's HP total (ADR 0007 step 2), read for the HP
	// around an end effect's heal (ADR 0004 step 21).
	HPTotal(ctx context.Context, characterID int64) (int, error)
}

// Dice is the dice port (diceadpt): initiative is rolled through it.
type Dice interface {
	RollDie(sides int) int
}

// Items supplies the carried items' powers (itemsvc), whatever their count
// (ADR 0016 steps 9 and 11). Wired in the composition root.
type Items interface {
	ItemPowers(ctx context.Context, characterID int64) ([]ItemPower, error)
}

// Weapons supplies the weapon in hand (ADR 0033 step 8), whose speed Next
// Round adds to the initiative (ADR 0035 item 6); ok is false with no weapon
// in hand. Wired in the composition root over the attack form.
type Weapons interface {
	WeaponInHand(ctx context.Context, characterID int64) (weapon HeldWeapon, ok bool, err error)
}

// Characters supplies the base STR from the sheet (charactersvc).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

type service struct {
	dal     DAL
	tx      domain.TxRunner
	log     Log
	chars   Characters
	dice    Dice
	items   Items
	weapons Weapons
	spells  Spells
}

// New constructs the service.
func New(dal DAL, tx domain.TxRunner, log Log, chars Characters, dice Dice, items Items, weapons Weapons, spells Spells) (SVC, error) {
	return NewWithConfig(DefaultConfig(), dal, tx, log, chars, dice, items, weapons, spells)
}

// NewWithConfig is the test-friendly constructor.
func NewWithConfig(cfg *Config, dal DAL, tx domain.TxRunner, log Log, chars Characters, dice Dice, items Items, weapons Weapons, spells Spells) (SVC, error) {
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
	case dice == nil:
		return nil, fmt.Errorf("buffsvc: nil dice")
	case items == nil:
		return nil, fmt.Errorf("buffsvc: nil items")
	case weapons == nil:
		return nil, fmt.Errorf("buffsvc: nil weapons")
	case spells == nil:
		return nil, fmt.Errorf("buffsvc: nil spells")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("buffsvc: invalid config: %w", err)
	}
	return &service{dal: dal, tx: tx, log: log, chars: chars, dice: dice, items: items, weapons: weapons, spells: spells}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
