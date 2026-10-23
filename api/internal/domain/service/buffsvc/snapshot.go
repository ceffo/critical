package buffsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// snapBuff is buffsvc's own JSON encoding of a buff inside action-log
// snapshots (StateKeyBuffs), the state of buffsSnaps. It must stay decodable
// for entries already in the log: add fields, never rename them.
type snapBuff struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Charges  int         `json:"charges"`
	Duration int         `json:"duration"`
	Effects  snapEffects `json:"effects"`
	Note     string      `json:"note,omitempty"`
	// StartRound is absent from entries recorded before it existed: they
	// restore it as 0.
	StartRound int `json:"start_round,omitempty"`
	// ChargesTotal is absent from entries recorded before it existed: they
	// restore it as 0 (unknown).
	ChargesTotal int `json:"charges_total,omitempty"`
	// SpellID is absent from entries recorded before it existed (and for a
	// buff not from a spell): they restore it empty.
	SpellID string `json:"spell_id,omitempty"`
	// ProficiencyID is absent from entries recorded before it existed and
	// for a buff not from a proficiency check: they restore it empty.
	ProficiencyID string `json:"proficiency_id,omitempty"`
	// OnOther is absent from entries recorded before it existed and for a
	// buff on the character (ADR 0004 step 20.5): they restore it false.
	OnOther bool `json:"on_other,omitempty"`
	// PowerID, MaintenanceCost and MaintenancePeriod are absent from entries
	// recorded before they existed and for a row not from a maintained
	// power: they restore empty (ADR 0043 step 13).
	PowerID string `json:"power_id,omitempty"`
	// PowerName is absent from entries recorded before it existed.
	PowerName         localized.Text `json:"power_name,omitempty"`
	MaintenanceCost   int            `json:"maintenance_cost,omitempty"`
	MaintenancePeriod int            `json:"maintenance_period,omitempty"`
	// RoundsKept is absent for a row not from a maintained power and for
	// one in its activation round: it restores 0 (ADR 0043 step 14).
	RoundsKept int `json:"rounds_kept,omitempty"`
	// Defence is absent for a row not of a defence mode: it restores ""
	// (ADR 0044 steps 4 and 5).
	Defence DefenceKind `json:"defence,omitempty"`
	// Target and Tangents are absent for a row not holding a mind: they
	// restore "" and 0 (ADR 0044 step 3).
	Target   string `json:"target,omitempty"`
	Tangents int    `json:"tangents,omitempty"`
	// ContactFrom is absent from rows not made contact rows (ADR 0044 step 10).
	ContactFrom int `json:"contact_from,omitempty"`
}

type snapEffects struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
	StrBonus   *int     `json:"str_bonus,omitempty"`
	StrMax     *int     `json:"str_max,omitempty"`
	AC         *int     `json:"ac,omitempty"`
	ToHit      *int     `json:"to_hit,omitempty"`
	Damage     *int     `json:"damage,omitempty"`
	BaseAC     *int     `json:"base_ac,omitempty"`
	// Initiative is the initiative effect (ADR 0033 step 11), absent from
	// buffs stored before it existed.
	Initiative *int `json:"initiative,omitempty"`
	// WeaponKind is the weapon-kind condition (ADR 0004 step 18), absent
	// from buffs stored before it existed and from buffs without one.
	WeaponKind WeaponKind `json:"weapon_kind,omitempty"`
	// WithoutMagicArmour is the AC effects' condition (ADR 0033 step 12).
	WithoutMagicArmour bool `json:"without_magic_armour,omitempty"`
}

// The snapshotters of buffsvc's state keys (ADR 0018 step 8): the active
// buff list, the saved templates, the round number and the initiative
// (absent: no initiative; its "after" names the entry that rolled it, ADR
// 0019 step 1).
var (
	buffsSnaps      = actionlog.NewSnapshotter[[]snapBuff](StateKeyBuffs)
	savedSnaps      = actionlog.NewSnapshotter[[]snapSaved](StateKeySavedBuffs)
	roundSnaps      = actionlog.NewSnapshotter[int](StateKeyRound)
	initiativeSnaps = actionlog.NewSnapshotter[snapInitiative](StateKeyInitiative).MayBeAbsent().
			WithReplay(func(st *snapInitiative, entryID int64) {
			if st.EntryID == 0 {
				st.EntryID = entryID
			}
		})
)

func toSnapBuffs(buffs []BuffEffect) []snapBuff {
	out := make([]snapBuff, len(buffs))
	for i, b := range buffs {
		out[i] = snapBuff{
			ID: b.ID, Name: b.Name, Charges: b.Charges, Duration: b.Duration,
			Effects: snapEffects(b.Effects), Note: b.Note, StartRound: b.StartRound, ChargesTotal: b.ChargesTotal,
			SpellID: b.SpellID, ProficiencyID: b.ProficiencyID, OnOther: b.OnOther,
			PowerID: b.PowerID, PowerName: b.PowerName, MaintenanceCost: b.MaintenanceCost, MaintenancePeriod: b.MaintenancePeriod, RoundsKept: b.RoundsKept, Defence: b.Defence,
			Target: b.Target, Tangents: b.Tangents, ContactFrom: b.ContactFrom,
		}
	}
	return out
}

func fromSnapBuffs(in []snapBuff) []BuffEffect {
	out := make([]BuffEffect, len(in))
	for i, b := range in {
		out[i] = BuffEffect{
			ID: b.ID, Name: b.Name, Charges: b.Charges, Duration: b.Duration,
			Effects: BuffEffects(b.Effects), Note: b.Note, StartRound: b.StartRound, ChargesTotal: b.ChargesTotal,
			SpellID: b.SpellID, ProficiencyID: b.ProficiencyID, OnOther: b.OnOther,
			PowerID: b.PowerID, PowerName: b.PowerName, MaintenanceCost: b.MaintenanceCost, MaintenancePeriod: b.MaintenancePeriod, RoundsKept: b.RoundsKept, Defence: b.Defence,
			Target: b.Target, Tangents: b.Tangents, ContactFrom: b.ContactFrom,
		}
	}
	return out
}

// EncodeRows encodes rows as buffsvc stores them in snapshots, for a caller
// that keeps rows in its own entry's payload to give them back later (a
// skill check keeps the rows its buff renewed, ADR 0037 step 20.8). The
// encoding stays decodable like the snapshots' (add fields, never rename).
func EncodeRows(rows []BuffEffect) (json.RawMessage, error) {
	raw, err := json.Marshal(toSnapBuffs(rows))
	if err != nil {
		return nil, fmt.Errorf("encode buff rows: %w", err)
	}
	return raw, nil
}

// DecodeRows reads rows EncodeRows encoded.
func DecodeRows(raw json.RawMessage) ([]BuffEffect, error) {
	var in []snapBuff
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("decode buff rows: %w", err)
	}
	return fromSnapBuffs(in), nil
}

// buffsSnapshot is the snapshot of the active buff list going from before to
// after.
func buffsSnapshot(before, after []BuffEffect) (actionlog.Snapshot, error) {
	b, a := toSnapBuffs(before), toSnapBuffs(after)
	return buffsSnaps.Snapshot("", &b, &a)
}

// roundSnapshot is the snapshot of the round number going from before to
// after.
func roundSnapshot(before, after int) (actionlog.Snapshot, error) {
	return roundSnaps.Snapshot("", &before, &after)
}

// Module declares the codecs that write the buff list, the saved templates,
// the round number or the initiative back from a snapshot. They run inside the
// void's transaction (ctx carries it).
func (s *service) Module() domain.Module {
	return domain.Module{Name: "buffsvc", Codecs: []actionlog.Codec{
		buffsSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, buffs []snapBuff) error {
			return s.dal.SaveActiveBuffs(ctx, characterID, fromSnapBuffs(buffs))
		}, nil),
		savedSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, saved []snapSaved) error {
			return s.dal.ReplaceSavedBuffs(ctx, characterID, fromSnapSaved(saved))
		}, nil),
		initiativeSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, st snapInitiative) error {
			if st.D10 < 1 || st.D10 > InitiativeDie {
				return fmt.Errorf("decode initiative snapshot: d10 %d out of range", st.D10)
			}
			return s.dal.SaveInitiative(ctx, characterID, &Initiative{D10: st.D10, Modifiers: fromTerms(st.Modifiers), LogEntryID: st.EntryID})
		}, func(ctx context.Context, characterID int64, _ string) error {
			return s.dal.SaveInitiative(ctx, characterID, nil)
		}),
		roundSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, round int) error {
			if round < 0 {
				return fmt.Errorf("decode round snapshot: negative round %d", round)
			}
			return s.dal.SaveRoundNumber(ctx, characterID, round)
		}, nil),
	}}
}
