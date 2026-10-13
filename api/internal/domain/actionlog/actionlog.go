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
	KindAttack       Kind = "attack"
	KindItemUse      Kind = "item_use"
	KindSpellPrep    Kind = "spell_prep"
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
// Payload is an optional JSON summary (for example a full attack breakdown).
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

// Correction is an audited change to an entry. An amend carries the new HP
// delta and summary; a void carries neither.
type Correction struct {
	ID        int64
	ActionID  int64
	Kind      CorrectionKind
	HPDelta   *int
	Summary   string
	CreatedAt time.Time
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

// Status is voided when any void exists, amended when any amend exists,
// active otherwise.
func (e Entry) Status() Status {
	status := StatusActive
	for _, c := range e.Corrections {
		switch c.Kind {
		case CorrectionVoid:
			return StatusVoided
		case CorrectionAmend:
			status = StatusAmended
		}
	}
	return status
}

func (e Entry) latestAmend() *Correction {
	for i := len(e.Corrections) - 1; i >= 0; i-- {
		if e.Corrections[i].Kind == CorrectionAmend {
			return &e.Corrections[i]
		}
	}
	return nil
}

// EffectiveHPDelta is 0 for a voided entry, the latest amend's delta for an
// amended one, the original delta otherwise. It mirrors the SQL that computes
// current HP and is used for display only.
func (e Entry) EffectiveHPDelta() int {
	if e.Status() == StatusVoided {
		return 0
	}
	if a := e.latestAmend(); a != nil && a.HPDelta != nil {
		return *a.HPDelta
	}
	return e.HPDelta
}

// EffectiveSummary is the latest amend's summary when it set one, the
// original summary otherwise.
func (e Entry) EffectiveSummary() string {
	if a := e.latestAmend(); a != nil && a.Summary != "" {
		return a.Summary
	}
	return e.Summary
}
