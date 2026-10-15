// Package actionlog holds the cross-service vocabulary of the action log: the
// one append-only record of table actions (HP changes, casts, buff changes,
// round advances, attacks, item uses). Entries are never edited: amend and
// void are correction events pointing at the original entry.
//
// Services that change state record an entry through Recorder inside the same
// domain.TxRunner transaction as their state write. Services whose state can
// be restored on void register a Restorer for their state key.
package actionlog

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

// Kind classifies an entry.
type Kind string

// Entry kinds. hp_change (hpsvc), buff_change and round_advance (buffsvc)
// are recorded today; the others are reserved for the services that plug
// into the log.
const (
	KindHPChange     Kind = "hp_change"
	KindCast         Kind = "cast"
	KindBuffChange   Kind = "buff_change"
	KindRoundAdvance Kind = "round_advance"
	KindRoundReset   Kind = "round_reset"
	KindRoundSet     Kind = "round_set"
	KindAttack       Kind = "attack"
	KindItemUse      Kind = "item_use"
	KindSpellPrep    Kind = "spell_prep"
	KindLongRest     Kind = "long_rest"
	// KindSave is a saving throw (combatsvc). It changes no state, so it has
	// no snapshots; its outcome lives in the payload and an adjustment after
	// the roll is an amend carrying the recomputed payload.
	KindSave Kind = "save"
	// KindSkillCheck is a proficiency or bard skill check (skillsvc, ADR
	// 0014). Like a save it changes no state and has no snapshots: popping it
	// only removes it. It is never adjusted.
	KindSkillCheck Kind = "skill_check"
)

// RollKinds are the entries whose result replaces the dashboard's result
// card (attack, save, cast or received spell, item use, skill check). Only
// the newest of them, the one on the card, can be adjusted after the fact.
var RollKinds = []Kind{KindAttack, KindSave, KindCast, KindItemUse, KindSkillCheck}

// IsRoll reports whether k is one of RollKinds.
func IsRoll(k Kind) bool {
	for _, r := range RollKinds {
		if r == k {
			return true
		}
	}
	return false
}

// KindTimePassed is a fast-forward of game time in hours (buffsvc PassTime):
// timed buffs count down, the round counter goes to 0.
const KindTimePassed Kind = "time_passed"

// KindInitiative is the round's initiative roll (buffsvc, ADR 0013), recorded
// right after the round_advance entry of Next Round and on a reroll. Its
// payload is {round, d10}. Not one of RollKinds: it leaves the result card and
// the adjustment of the latest roll alone.
const KindInitiative Kind = "initiative"

// KindItemRestock is a stock item's count going up at the table (itemsvc
// Restock). Not a roll: it leaves the result card alone.
const KindItemRestock Kind = "item_restock"

// Item editor entries (itemsvc Create, Update, Delete): an
// item added, edited or removed by hand. Not rolls. Each carries the item's
// "items:<id>" snapshot, so popping it undoes the change.
const (
	KindItemAdd    Kind = "item_add"
	KindItemEdit   Kind = "item_edit"
	KindItemRemove Kind = "item_remove"
)

// Weapon editor entries (weaponsvc Create, Update, Delete): a
// weapon added, edited or removed by hand. Not rolls. Each carries the
// weapon's "weapons:<id>" snapshot, so popping it undoes the change.
const (
	KindWeaponAdd    Kind = "weapon_add"
	KindWeaponEdit   Kind = "weapon_edit"
	KindWeaponRemove Kind = "weapon_remove"
)

// Snapshot is the state an entry changed, under a state key owned by one
// service (for example "buffs" or "spell_slots"). Before and After are that
// service's own JSON encoding of the state.
//
// A key may be "family:id" (for example "items:arrows_plus3") to scope the
// state to one thing: a void is then only blocked by later entries touching
// the same thing, and the Restorer registered for the family restores it.
type Snapshot struct {
	Key    string
	Before json.RawMessage
	After  json.RawMessage
}

// NewEntry is what a service records. Round 0 means "stamp the current round".
// HPDelta is the entry's contribution to current HP (0 for most kinds).
// Payload is the structured record of what happened (for example a full
// attack breakdown); the UI words it in the player's language. Summary is
// only free text the player typed (an HP change's source): the backend never
// composes display text. An entry needs a payload or a summary.
type NewEntry struct {
	CharacterID int64
	Round       int
	Kind        Kind
	Summary     string
	HPDelta     int
	Payload     json.RawMessage
	Snapshots   []Snapshot
}

// Recorder records an entry and returns its id. Call it inside the caller's
// domain.TxRunner transaction so the entry and the state write are atomic.
type Recorder interface {
	Record(ctx context.Context, e NewEntry) (int64, error)
}

// PayloadAmendment replaces an entry's payload; its HP delta is unchanged.
// The payload is required.
type PayloadAmendment struct {
	Payload json.RawMessage
}

// AmendFunc computes a PayloadAmendment from the entry being amended (with its
// corrections so far).
type AmendFunc func(e Entry) (PayloadAmendment, error)

// LatestAmender corrects an entry whose outcome lives in its payload (a save)
// after the fact. AmendLatest records an amend replacing the payload of
// entry id, which must be of kind (a roll kind), not voided, and
// the latest roll: the newest entry of any of RollKinds, voided ones
// included. fn computes the amendment from the entry inside the transaction.
type LatestAmender interface {
	AmendLatest(ctx context.Context, characterID, id int64, kind Kind, fn AmendFunc) (*Entry, error)
}

// KeySeparator splits a scoped state key into its family and id.
const KeySeparator = ":"

// Family is the part of a state key before KeySeparator ("items" for
// "items:arrows_plus3"); a key without one is its own family.
func Family(key string) string {
	family, _, _ := strings.Cut(key, KeySeparator)
	return family
}

// Restorer writes a "before" snapshot back when an entry is voided. It runs
// inside the void's transaction (use ctx for every storage call).
type Restorer interface {
	Restore(ctx context.Context, characterID int64, state json.RawMessage) error
}

// CorrectionKind is amend or void.
type CorrectionKind string

// Correction kinds.
const (
	CorrectionAmend CorrectionKind = "amend"
	CorrectionVoid  CorrectionKind = "void"
)

// Correction is an audited change to an entry. A note amend (the only kind
// the log's amend writes now) carries Note and nothing else.
// A value amend carries the new HP delta (nil: unchanged), summary (empty:
// unchanged) and payload (empty: unchanged): written by AmendLatest (post-roll
// adjustments, payload only) and, with an HP delta or a summary, by hp_change
// amends logged before amends became note-only. A void carries none of them.
type Correction struct {
	ID        int64
	ActionID  int64
	Kind      CorrectionKind
	HPDelta   *int
	Summary   string
	Payload   json.RawMessage
	Note      *string
	CreatedAt time.Time
}

// changesValue is true for an amend that changes the entry's HP delta,
// summary or payload (not a note-only amend).
func (c Correction) changesValue() bool {
	return c.Kind == CorrectionAmend && (c.HPDelta != nil || c.Summary != "" || len(c.Payload) > 0)
}

// Status is derived from an entry's corrections.
type Status string

// Entry statuses.
const (
	StatusActive  Status = "active"
	StatusAmended Status = "amended"
	StatusVoided  Status = "voided"
)

// Entry is a recorded action with its corrections in id order.
type Entry struct {
	ID          int64
	CharacterID int64
	Round       int
	Kind        Kind
	Summary     string
	HPDelta     int
	Payload     json.RawMessage
	Snapshots   []Snapshot
	CreatedAt   time.Time
	Corrections []Correction
}

// Status is voided when any void exists, amended when an amend changed a
// value, active otherwise (a note does not make an entry amended).
func (e Entry) Status() Status {
	status := StatusActive
	for _, c := range e.Corrections {
		switch c.Kind {
		case CorrectionVoid:
			return StatusVoided
		case CorrectionAmend:
			if c.changesValue() {
				status = StatusAmended
			}
		}
	}
	return status
}

// latestAmend is the newest amend for which set reports true.
func (e Entry) latestAmend(set func(Correction) bool) *Correction {
	for i := len(e.Corrections) - 1; i >= 0; i-- {
		if c := e.Corrections[i]; c.Kind == CorrectionAmend && set(c) {
			return &e.Corrections[i]
		}
	}
	return nil
}

// Note is the latest note amend's note ("" when none).
func (e Entry) Note() string {
	if a := e.latestAmend(func(c Correction) bool { return c.Note != nil }); a != nil {
		return *a.Note
	}
	return ""
}

// EffectivePayload is the payload of the latest amend that set one, the
// original payload otherwise.
func (e Entry) EffectivePayload() json.RawMessage {
	for i := len(e.Corrections) - 1; i >= 0; i-- {
		if c := e.Corrections[i]; c.Kind == CorrectionAmend && len(c.Payload) > 0 {
			return c.Payload
		}
	}
	return e.Payload
}

// EffectiveHPDelta is 0 for a voided entry, the latest amend's delta for an
// amended one, the original delta otherwise. It mirrors the SQL that computes
// current HP and is used for display only.
func (e Entry) EffectiveHPDelta() int {
	if e.Status() == StatusVoided {
		return 0
	}
	if a := e.latestAmend(func(c Correction) bool { return c.HPDelta != nil }); a != nil {
		return *a.HPDelta
	}
	return e.HPDelta
}

// EffectiveSummary is the latest amend's summary when it set one, the
// original summary otherwise.
func (e Entry) EffectiveSummary() string {
	if a := e.latestAmend(func(c Correction) bool { return c.Summary != "" }); a != nil {
		return a.Summary
	}
	return e.Summary
}
