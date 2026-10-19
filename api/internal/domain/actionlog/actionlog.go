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
	"slices"
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
	// KindAbilityCheck is a plain ability check, a d20 at or under the
	// ability score in effect (skillsvc, ADR 0028). It changes no state and
	// has no snapshots: popping it only removes it. Its DM adjustment after
	// the roll is an amend carrying the recomputed payload.
	KindAbilityCheck Kind = "ability_check"
	// KindExposure is one round of exposure to a spell's area rolled on
	// request (castsvc, ADR 0008 step 16). It has no snapshots; when the Bard
	// is the one exposed, its HP delta takes the damage off his HP, so
	// popping it gives the HP back.
	KindExposure Kind = "exposure"
	// KindTurnUndead is a cleric's turning undead roll (turnsvc, ADR 0030
	// step 16). It changes no state and has no snapshots: popping it only
	// removes it. It is never adjusted.
	KindTurnUndead Kind = "turn_undead"
)

// RollKinds are the entries whose result replaces the dashboard's result
// card (attack, save, cast or received spell, item use, skill check, ability
// check, spell exposure, turning undead). Only
// the newest of them, the one on the card, can be adjusted after the fact.
var RollKinds = []Kind{KindAttack, KindSave, KindCast, KindItemUse, KindSkillCheck, KindAbilityCheck, KindExposure, KindTurnUndead}

// IsRoll reports whether k is one of RollKinds.
func IsRoll(k Kind) bool {
	for _, r := range RollKinds {
		if r == k {
			return true
		}
	}
	return false
}

// PayloadView fills, on read, the fields an entry's payload gained after the
// entry was logged, so the API serves every entry of a kind in one shape. It
// computes them from the stored numbers, never changes the stored payload and
// never changes a recorded outcome. The service that owns the payload
// provides it; the composition root registers it per kind.
type PayloadView func(json.RawMessage) (json.RawMessage, error)

// PayloadViews are the payload views by entry kind.
type PayloadViews map[Kind]PayloadView

// View returns payload as served for an entry of kind k: unchanged when the
// kind has no view, the payload is empty or the view cannot read it.
func (v PayloadViews) View(k Kind, payload json.RawMessage) json.RawMessage {
	view, ok := v[k]
	if !ok || len(payload) == 0 {
		return payload
	}
	out, err := view(payload)
	if err != nil {
		return payload
	}
	return out
}

// Entry returns e as served: the payload and every amend's payload viewed
// for e's kind (so the effective payload is viewed too). e is not modified,
// its corrections are copied.
func (v PayloadViews) Entry(e Entry) Entry {
	if _, ok := v[e.Kind]; !ok {
		return e
	}
	e.Payload = v.View(e.Kind, e.Payload)
	e.Corrections = slices.Clone(e.Corrections)
	for i := range e.Corrections {
		e.Corrections[i].Payload = v.View(e.Kind, e.Corrections[i].Payload)
	}
	return e
}

// Entries is Entry for each of entries, in order.
func (v PayloadViews) Entries(entries []Entry) []Entry {
	out := make([]Entry, len(entries))
	for i, e := range entries {
		out[i] = v.Entry(e)
	}
	return out
}

// EntryPtr is Entry for a possibly nil entry.
func (v PayloadViews) EntryPtr(e *Entry) *Entry {
	if e == nil {
		return nil
	}
	out := v.Entry(*e)
	return &out
}

// KindTimePassed is a fast-forward of game time in hours (buffsvc PassTime):
// timed buffs count down, the round counter goes to 0.
const KindTimePassed Kind = "time_passed"

// KindInitiative is the round's initiative roll (buffsvc, ADR 0013), recorded
// right after the round_advance entry of Next Round and on a reroll. Its
// payload is {round, d10}. Not one of RollKinds: it leaves the result card and
// the adjustment of the latest roll alone.
const KindInitiative Kind = "initiative"

// KindDice is a free roll of the dice composer (dicesvc). Its payload is
// {formula, groups, modifier, total}. Not one of RollKinds: a stray composer
// roll must neither replace the dashboard's result card nor close the
// correction window of the latest save or attack (ADR 0006 step 6). It has
// no snapshots, so popping it only removes it.
const KindDice Kind = "dice"

// KindCorrection is a correction made after a roll (a target AC, a save
// adjustment, a GM verdict): its own step on the action stack (ADR 0010 step
// 6). Its payload is a CorrectionStep naming what changed; the corrected
// entry's new payload is an amend linked to it (Correction.StepID). It has no
// snapshots: popping it only puts that amend out of effect, so the corrected
// entry reads its previous payload again. Not one of RollKinds.
const KindCorrection Kind = "correction"

// CorrectionStep is the payload of a KindCorrection entry: which entry was
// corrected and what changed, From and To as the corrected kind encodes the
// field (nil: none, for example an AC that was unknown). The UI words it.
type CorrectionStep struct {
	EntryID   int64  `json:"entryId"`
	EntryKind Kind   `json:"entryKind"`
	Field     string `json:"field"`
	From      any    `json:"from"`
	To        any    `json:"to"`
}

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

// KindEquipmentState is a piece worn, taken off, wielded or put away (ADR
// 0033 step 7): one entry per move (a swap is one entry with both pieces),
// each piece's "items:<id>" snapshot, so popping it puts the pieces back.
// Not a roll.
const KindEquipmentState Kind = "equipment_state"

// Weapon editor entries, written before the one equipment model (ADR 0033
// step 1): a weapon added, edited or removed by hand. No longer written (a
// weapon is a piece, logged as item_*); the entries in the log still pop,
// their "weapons:<id>" snapshot restored into the piece of that id (step
// 26.2).
const (
	KindWeaponAdd    Kind = "weapon_add"
	KindWeaponEdit   Kind = "weapon_edit"
	KindWeaponRemove Kind = "weapon_remove"
)

// XP ledger entries (xpsvc Add, Update, Delete, ADR 0015 step 4): an award
// added, edited or removed. Not rolls. Each carries the entry's "xp:<id>"
// snapshot, so popping it gives the XP total back.
const (
	KindXPAdd    Kind = "xp_add"
	KindXPEdit   Kind = "xp_edit"
	KindXPRemove Kind = "xp_remove"
)

// KindLevelUp is a level-up roll (levelupsvc, ADR 0030 step 7.3): the level
// reached, the hit die and its value, the Constitution adjustment and the
// gain. Its "realized_level" snapshot carries the realized level and its max
// HP, so popping it makes the level-up pending again (and takes the level's
// benefits back). Not one of RollKinds:
// it is never adjusted after the fact.
const KindLevelUp Kind = "level_up"

// KindSpellLearn is a chance-to-learn roll (learnsvc, ADR 0030 step 20.2):
// the spell, the d100, Table 4's chance and whether it was learned. Its
// "spellbook:<spell key>" snapshot carries the spell's place in the
// spellbook (known, or blocked by a failure at a level), so popping it takes
// a learned spell out or lifts a failure's block. Not one of RollKinds: it
// is never adjusted after the fact.
const KindSpellLearn Kind = "spell_learn"

// KindSkillPoints is a thief spending skill points on the sheet (skillsvc,
// ADR 0030 step 17): the points per skill before and after. Its
// "skill_points" snapshot carries the points, so popping it gives them back.
// Not one of RollKinds: nothing is rolled.
const KindSkillPoints Kind = "skill_points"

// KindSkillArmor is a thief changing the armour it wears for its skills
// (charactersvc, ADR 0030 step 1): the armour before and after. Its
// "skill_armor" snapshot carries the armour, so popping it gives it back and
// the skills with it. Not one of RollKinds: nothing is rolled.
const KindSkillArmor Kind = "skill_armor"

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
// The payload and the change's field are required.
type PayloadAmendment struct {
	Payload json.RawMessage
	// Change names what the amendment changed, for the correction step.
	Change Change
}

// Change is the field a correction changed, with its value before and after
// (see CorrectionStep).
type Change struct {
	Field string
	From  any
	To    any
}

// AmendFunc computes a PayloadAmendment from the entry being amended (with its
// corrections so far).
type AmendFunc func(e Entry) (PayloadAmendment, error)

// LatestAmender corrects an entry whose outcome lives in its payload (a save)
// after the fact. AmendLatest records a correction step (a KindCorrection
// entry) and an amend linked to it replacing the payload of entry id, which
// must be of kind (a roll kind), not voided, and the latest roll: the newest
// entry of any of RollKinds, voided ones included. fn computes the amendment
// from the entry inside the transaction.
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

// Restorer writes a snapshot back: its "before" when an entry is voided, its
// "after" when the entry is replayed by a redo (ADR 0019 step 1). It runs
// inside the void's or the redo's transaction (use ctx for every storage
// call).
type Restorer interface {
	Restore(ctx context.Context, characterID int64, state json.RawMessage) error
}

// Replayer is implemented by a Restorer whose "after" cannot name the entry
// that recorded it, because the entry's id is assigned once its snapshots
// are taken (the initiative remembers the entry that rolled it). A redo
// writes such an "after" back through Replay, with the replayed entry's id;
// every other Restorer gets its "after" through Restore (ADR 0019 step 1).
type Replayer interface {
	Replay(ctx context.Context, characterID, entryID int64, after json.RawMessage) error
}

// CorrectionKind is amend, void or redo.
type CorrectionKind string

// Correction kinds. A void pops an entry (ADR 0018 step 2); a redo un-pops it
// when its transaction is replayed (ADR 0019 step 1). An entry may be popped
// and replayed several times: its newest void or redo decides (Entry.Status).
const (
	CorrectionAmend CorrectionKind = "amend"
	CorrectionVoid  CorrectionKind = "void"
	CorrectionRedo  CorrectionKind = "redo"
)

// Correction is an audited change to an entry. A note amend (the only kind
// the log's amend writes now) carries Note and nothing else.
// A value amend carries the new HP delta (nil: unchanged), summary (empty:
// unchanged) and payload (empty: unchanged): written by AmendLatest (post-roll
// adjustments, payload only) and, with an HP delta or a summary, by hp_change
// amends logged before amends became note-only. A void or a redo carries none
// of them.
//
// StepID is the KindCorrection entry an amend written by AmendLatest belongs
// to (nil for older amends, which stay in effect until their entry is popped).
// StepUndone is true once that step was popped: the amend is then out of
// effect, kept only for the audit.
type Correction struct {
	ID         int64
	ActionID   int64
	Kind       CorrectionKind
	HPDelta    *int
	Summary    string
	Payload    json.RawMessage
	Note       *string
	StepID     *int64
	StepUndone bool
	CreatedAt  time.Time
}

// InEffect is false for an amend whose correction step was popped.
func (c Correction) InEffect() bool {
	return c.StepID == nil || !c.StepUndone
}

// changesValue is true for an amend in effect that changes the entry's HP
// delta, summary or payload (not a note-only amend).
func (c Correction) changesValue() bool {
	return c.Kind == CorrectionAmend && c.InEffect() && (c.HPDelta != nil || c.Summary != "" || len(c.Payload) > 0)
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
//
// TransactionID is the transaction the entry belongs to (ADR 0018 step 1):
// the id of the first entry its request recorded, its own id when it is that
// first entry (and for every entry written before transactions existed).
type Entry struct {
	ID            int64
	TransactionID int64
	CharacterID   int64
	Round         int
	Kind          Kind
	Summary       string
	HPDelta       int
	Payload       json.RawMessage
	Snapshots     []Snapshot
	CreatedAt     time.Time
	Corrections   []Correction
}

// Status is voided when the newest void or redo is a void (ADR 0019 step 1),
// amended when an amend changed a value, active otherwise (a note does not
// make an entry amended).
func (e Entry) Status() Status {
	if e.Voided() {
		return StatusVoided
	}
	for _, c := range e.Corrections {
		if c.changesValue() {
			return StatusAmended
		}
	}
	return StatusActive
}

// Voided reports whether the entry is popped: its newest void or redo
// correction is a void (corrections are in id order).
func (e Entry) Voided() bool {
	for i := len(e.Corrections) - 1; i >= 0; i-- {
		switch e.Corrections[i].Kind {
		case CorrectionVoid:
			return true
		case CorrectionRedo:
			return false
		}
	}
	return false
}

// latestAmend is the newest amend in effect for which set reports true.
func (e Entry) latestAmend(set func(Correction) bool) *Correction {
	for i := len(e.Corrections) - 1; i >= 0; i-- {
		if c := e.Corrections[i]; c.Kind == CorrectionAmend && c.InEffect() && set(c) {
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

// EffectivePayload is the payload of the latest amend in effect that set one,
// the original payload otherwise (ADR 0010 step 6: popping a correction step
// gives back the payload before it).
func (e Entry) EffectivePayload() json.RawMessage {
	if a := e.latestAmend(func(c Correction) bool { return len(c.Payload) > 0 }); a != nil {
		return a.Payload
	}
	return e.Payload
}

// EffectiveHPDelta is 0 for a voided entry, the latest amend's delta for an
// amended one, the original delta otherwise. HPTotal sums it over the log.
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
