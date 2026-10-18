// Package casthdl exposes POST /api/spells/{id}/cast,
// POST /api/spells/receive, POST /api/spells/effect and
// POST /api/spells/exposure.
package casthdl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/spellhdl"
)

// Router is the chi subset we need.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type castSvc interface {
	Cast(ctx context.Context, characterID int64, slotIndex int, req castsvc.CastRequest) (castsvc.CastResult, error)
	Receive(ctx context.Context, characterID int64, req castsvc.ReceiveRequest) (castsvc.CastResult, error)
	AddEffect(ctx context.Context, characterID int64, req castsvc.AddEffectRequest) (castsvc.CastResult, error)
	Expose(ctx context.Context, characterID int64, req castsvc.ExposureRequest) (castsvc.ExposureResult, error)
}

// RegisterRoutes wires the cast endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc castsvc.SVC) {
	r.Post("/api/spells/{id}/cast", render.Handle(Cast(charID, svc)))
	r.Post("/api/spells/receive", render.Handle(Receive(charID, svc)))
	r.Post("/api/spells/effect", render.Handle(AddEffect(charID, svc)))
	r.Post("/api/spells/exposure", render.Handle(Expose(charID, svc)))
}

// ExposureRequestDTO is the body of POST /api/spells/exposure: one round of
// exposure to a spell's area (ADR 0008 step 16).
type ExposureRequestDTO struct {
	SpellID     string `json:"spellId"`
	CasterLevel int    `json:"casterLevel"`
	// Mode: the key of the exposure mode (omitted: the first).
	Mode string `json:"mode,omitempty"`
	// Round: the round of exposure, from 1 (omitted: 1).
	Round int `json:"round,omitempty"`
	// OnSelf: the Bard is the one exposed; the damage comes off his HP.
	OnSelf bool `json:"onSelf,omitempty"`
}

// ExposureResultDTO is the wire shape of castsvc.ExposureResult.
type ExposureResultDTO struct {
	EntryID     int64  `json:"entryId"`
	SpellID     string `json:"spellId"`
	Name        string `json:"name"`
	CasterLevel int    `json:"casterLevel"`
	// Mode: the key of the exposure mode rolled (absent: none).
	Mode   string `json:"mode,omitempty"`
	Round  int    `json:"round"`
	OnSelf bool   `json:"onSelf"`
	// HPDelta: the HP change the ledger applied when the Bard was exposed
	// (the floor may make it smaller than the damage); 0 otherwise.
	HPDelta int        `json:"hpDelta"`
	Damage  *DamageDTO `json:"damage"`
	// Exposure: the spell's exposure damage at the caster level, for the
	// next round of exposure.
	Exposure *spellhdl.SpellExposureDTO `json:"exposure,omitempty"`
}

// Expose rolls one round of exposure to a spell's area and logs it.
func Expose(charID CharacterIDFunc, svc castSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body ExposureRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.Expose(r.Context(), charID(r), castsvc.ExposureRequest{
			SpellID: body.SpellID, CasterLevel: body.CasterLevel, Mode: body.Mode, Round: body.Round, OnSelf: body.OnSelf,
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ExposureToDTO(res))
		return nil
	}
}

// ExposureToDTO maps an exposure roll to its wire shape.
func ExposureToDTO(res castsvc.ExposureResult) ExposureResultDTO {
	return ExposureResultDTO{
		EntryID: res.EntryID, SpellID: res.SpellID, Name: res.Name, CasterLevel: res.CasterLevel,
		Mode: res.Mode, Round: res.Round, OnSelf: res.OnSelf, HPDelta: res.HPDelta, Damage: damageDTO(res.Damage),
		Exposure: spellhdl.ExposureToDTO(res.Exposure),
	}
}

// CastRequestDTO is the optional body of POST /api/spells/{id}/cast.
type CastRequestDTO struct {
	// OnSelf: does a buff spell affect the Bard? Omitted: the spell's
	// default (see castsvc).
	OnSelf *bool `json:"onSelf,omitempty"`
	// SpellID: the spell cast from a blank extra slot.
	SpellID string `json:"spellId,omitempty"`
	// ExtraSlots: the split of the levels a spell that grants slots adds
	// (one of the spellbook entry's slotSplits; omitted: the first).
	ExtraSlots []int `json:"extraSlots,omitempty"`
	// ItemID: the item a spell that consumes an item spends (omitted: the
	// only one of its type left).
	ItemID string `json:"itemId,omitempty"`
	// TargetAC: the target's AC, required for a spell with an attack roll.
	TargetAC *int `json:"targetAc,omitempty"`
	// Mode: the key of the cast-time mode of a spell with modes (omitted:
	// the first).
	Mode string `json:"mode,omitempty"`
}

// ReceiveRequestDTO is the body of POST /api/spells/receive.
type ReceiveRequestDTO struct {
	SpellID     string `json:"spellId"`
	CasterLevel int    `json:"casterLevel"`
	// Mode: the cast-time mode the caster picked (omitted: the first).
	Mode string `json:"mode,omitempty"`
}

// Cast spends slot {id} and applies its spell.
func Cast(charID CharacterIDFunc, svc castSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		idx, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			return render.BadRequestError("invalid slot index")
		}
		var body CastRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.Cast(r.Context(), charID(r), idx, castsvc.CastRequest{OnSelf: body.OnSelf, SpellID: body.SpellID, ExtraSlots: body.ExtraSlots, ItemID: body.ItemID, TargetAC: body.TargetAC, Mode: body.Mode})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(res))
		return nil
	}
}

// Receive applies a spell a party member cast on the Bard.
func Receive(charID CharacterIDFunc, svc castSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body ReceiveRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.Receive(r.Context(), charID(r), castsvc.ReceiveRequest{SpellID: body.SpellID, CasterLevel: body.CasterLevel, Mode: body.Mode})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(res))
		return nil
	}
}

// AddEffectRequestDTO is the body of POST /api/spells/effect: a spell added
// as an effect on the Bard at his level (ADR 0004 step 19).
type AddEffectRequestDTO struct {
	SpellID string `json:"spellId"`
}

// AddEffect puts a spell that adds a buff on the Bard at his level: no slot,
// no damage.
func AddEffect(charID CharacterIDFunc, svc castSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body AddEffectRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.AddEffect(r.Context(), charID(r), castsvc.AddEffectRequest{SpellID: body.SpellID})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(res))
		return nil
	}
}

// CastResultDTO is the wire shape of castsvc.CastResult.
type CastResultDTO struct {
	EntryID     int64  `json:"entryId"`
	Source      string `json:"source"`
	SpellID     string `json:"spellId"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	CasterLevel int    `json:"casterLevel"`
	SlotIndex   *int   `json:"slotIndex,omitempty"`
	// Mode: the key of the cast-time mode the spell was cast in (absent: the
	// spell has no modes).
	Mode       string                  `json:"mode,omitempty"`
	Kind       string                  `json:"kind,omitempty"`
	Confidence string                  `json:"confidence,omitempty"`
	Target     string                  `json:"target,omitempty"`
	Note       string                  `json:"note,omitempty"`
	Spell      *spellhdl.SpellDTO      `json:"spell,omitempty"`
	Save       *SaveDTO                `json:"save,omitempty"`
	TextOnly   string                  `json:"textOnly,omitempty"`
	Buff       *AppliedBuffDTO         `json:"buff,omitempty"`
	Damage     *DamageDTO              `json:"damage,omitempty"`
	Untracked  []UntrackedDTO          `json:"untracked"`
	Slots      []spellhdl.SpellSlotDTO `json:"slots,omitempty"`
	// ExtraSlots: levels of the extra slots this cast added.
	ExtraSlots []int `json:"extraSlots,omitempty"`
	// ConsumedItem: the inventory item the cast spent.
	ConsumedItem *ConsumedItemDTO `json:"consumedItem,omitempty"`
	// Attack: the spell's to-hit roll (one d20, no crit).
	Attack *SpellAttackDTO `json:"attack,omitempty"`
	// Exposure: the spell's exposure damage at the caster level, to roll
	// each round of exposure from (absent: none; ADR 0008 step 16).
	Exposure *spellhdl.SpellExposureDTO `json:"exposure,omitempty"`
	// Failure: the low-Wisdom spell failure d100 (ADR 0030 step 14; absent:
	// no die). `failed`: the spell failed (d100 at or under `chance`), its
	// slot spent and nothing applied.
	Failure *SpellFailureDTO `json:"failure,omitempty"`
}

// SpellFailureDTO is a cast's spell failure roll (ADR 0030 step 14).
type SpellFailureDTO struct {
	D100   int  `json:"d100"`
	Chance int  `json:"chance"`
	Failed bool `json:"failed"`
}

// ConsumedItemDTO is the item a cast spent and how many are left.
type ConsumedItemDTO struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Remaining int    `json:"remaining"`
}

// SpellAttackDTO is a spell's to-hit roll; what a hit does is the DM's call.
type SpellAttackDTO struct {
	D20Roll        int                 `json:"d20Roll"`
	BaseThac0      int                 `json:"baseThac0"`
	Modifiers      []SpellAttackModDTO `json:"modifiers"`
	EffectiveThac0 int                 `json:"effectiveThac0"`
	TargetAC       int                 `json:"targetAc"`
	TargetACNeeded int                 `json:"targetACNeeded"`
	Hit            bool                `json:"hit"`
	HitMargin      int                 `json:"hitMargin"`
}

// SpellAttackModDTO is one to-hit modifier of a spell's attack; the UI
// labels it by kind and detail, an item power's from its item (itemId, ADR
// 0016 step 14).
type SpellAttackModDTO struct {
	Kind           string `json:"kind"`
	Detail         string `json:"detail,omitempty"`
	ItemID         string `json:"itemId,omitempty"`
	Value          int    `json:"value"`
	StrExceptional int    `json:"strExceptional,omitempty"` // a Strength modifier's 18/xx percentile (ADR 0030 step 9)
}

// SaveDTO is the target's saving throw (the DM's side).
type SaveDTO struct {
	Type   string `json:"type"`
	Effect string `json:"effect"`
}

// DieDTO is one die thrown.
type DieDTO struct {
	Sides int `json:"sides"`
	Value int `json:"value"`
}

// RollDTO is one evaluated formula.
type RollDTO struct {
	Formula string   `json:"formula"`
	Dice    []DieDTO `json:"dice"`
	Total   int      `json:"total"`
	// Parts are the rolls of a damage instance's parts, in the order of
	// DamageDTO.Parts (absent: no parts).
	Parts []RollDTO `json:"parts,omitempty"`
	// Step is the 1-based round or instance of a damage made of dice steps
	// (absent: no steps).
	Step int `json:"step,omitempty"`
}

// AppliedBuffDTO is the buff and its rolls.
type AppliedBuffDTO struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Rounds     int      `json:"rounds"`
	Charges    int      `json:"charges"`
	Applied    bool     `json:"applied"`
	Duration   *RollDTO `json:"durationRoll,omitempty"`
	ChargeRoll *RollDTO `json:"chargesRoll,omitempty"`
	ChargeUnit string   `json:"chargeUnit,omitempty"`
	StrGain    *RollDTO `json:"strGainRoll,omitempty"`
	// Effects are the engine-tracked effects, keyed like the buff API
	// (to_hit, ac, str_bonus, ...).
	Effects map[string]float64 `json:"effects"`
}

// DamageDTO is a damage roll.
type DamageDTO struct {
	Dice      string    `json:"dice"`
	Type      string    `json:"type"`
	Instances int       `json:"instances"`
	Rounds    int       `json:"rounds"`
	Rolls     []RollDTO `json:"rolls"`
	Total     int       `json:"total"`
	// Parts are the parts of one instance and what a save does to each;
	// TotalOnSave is Total without the parts a save negates. Both are absent
	// when the damage has no parts.
	Parts       []DamagePartDTO `json:"parts,omitempty"`
	TotalOnSave *int            `json:"totalOnSave,omitempty"`
	// StepsPer is "round" or "instance" for a damage made of dice steps:
	// each roll is one step of its own dice (Dice is empty). Absent otherwise.
	StepsPer string `json:"stepsPer,omitempty"`
}

// DamagePartDTO is one damage part: its formula and its save effect key
// (none, negates).
type DamagePartDTO struct {
	Formula string `json:"formula"`
	Save    string `json:"save"`
}

// UntrackedDTO is an effect to apply by hand.
type UntrackedDTO struct {
	Key       string `json:"key"`
	Value     int    `json:"value"`
	Condition string `json:"condition,omitempty"`
}

// ToDTO maps a cast result to its wire shape.
func ToDTO(res castsvc.CastResult) CastResultDTO {
	out := CastResultDTO{
		EntryID: res.EntryID, Source: string(res.Source), SpellID: res.SpellID, Name: res.Name,
		Level: res.Level, CasterLevel: res.CasterLevel, SlotIndex: res.SlotIndex, Mode: res.Mode,
		Kind: string(res.Kind), Confidence: string(res.Confidence), Target: string(res.Target),
		Note: res.Note, TextOnly: string(res.TextOnly), Untracked: []UntrackedDTO{},
	}
	if len(res.Spell.Name) > 0 {
		s := spellhdl.SpellToDTO(res.Spell, res.CasterLevel)
		out.Spell = &s
	}
	if res.Save != nil {
		out.Save = &SaveDTO{Type: string(res.Save.Type), Effect: string(res.Save.Effect)}
	}
	if b := res.Buff; b != nil {
		out.Buff = &AppliedBuffDTO{
			ID: b.Buff.ID, Name: b.Buff.Name, Rounds: b.Buff.Duration, Charges: b.Buff.Charges,
			Applied: b.Applied, Duration: rollDTO(b.Duration), ChargeRoll: rollDTO(b.Charges),
			ChargeUnit: b.ChargeUnit, StrGain: rollDTO(b.StrGain), Effects: effects(b),
		}
	}
	out.Damage = damageDTO(res.Damage)
	out.Exposure = spellhdl.ExposureToDTO(res.Exposure)
	for _, u := range res.Untracked {
		out.Untracked = append(out.Untracked, UntrackedDTO{Key: u.Key, Value: u.Value, Condition: u.Condition})
	}
	out.ExtraSlots = res.ExtraSlots
	if c := res.Consumed; c != nil {
		out.ConsumedItem = &ConsumedItemDTO{ItemID: c.ItemID, Name: c.Name, Remaining: c.Remaining}
	}
	if f := res.Failure; f != nil {
		out.Failure = &SpellFailureDTO{D100: f.Roll, Chance: f.Chance, Failed: f.Failed}
	}
	if a := res.Attack; a != nil {
		out.Attack = &SpellAttackDTO{
			D20Roll: a.D20Roll, BaseThac0: a.BaseThac0, Modifiers: []SpellAttackModDTO{}, EffectiveThac0: a.EffectiveThac0,
			TargetAC: a.TargetAC, TargetACNeeded: a.TargetACNeeded, Hit: a.Hit, HitMargin: a.HitMargin,
		}
		for _, m := range a.Modifiers {
			out.Attack.Modifiers = append(out.Attack.Modifiers, SpellAttackModDTO{Kind: string(m.Kind), Detail: m.Detail, ItemID: m.ItemID, Value: m.Value, StrExceptional: m.StrExceptional})
		}
	}
	if res.Slots != nil {
		out.Slots = spellhdl.SlotsToDTO(res.Slots)
	}
	return out
}

func damageDTO(d *castsvc.DamageResult) *DamageDTO {
	if d == nil {
		return nil
	}
	out := &DamageDTO{
		Dice: d.Dice, Type: d.Type, Instances: d.Instances, Rounds: d.Rounds, Total: d.Total,
		Rolls: []RollDTO{}, TotalOnSave: d.TotalOnSave, StepsPer: string(d.StepsPer),
	}
	for _, p := range d.Parts {
		out.Parts = append(out.Parts, DamagePartDTO{Formula: p.Formula, Save: string(p.Save)})
	}
	for i := range d.Rolls {
		out.Rolls = append(out.Rolls, *rollDTO(&d.Rolls[i]))
	}
	return out
}

func rollDTO(r *castsvc.Roll) *RollDTO {
	if r == nil {
		return nil
	}
	out := &RollDTO{Formula: r.Formula, Total: r.Total, Dice: []DieDTO{}, Step: r.Step}
	for _, d := range r.Dice {
		out.Dice = append(out.Dice, DieDTO{Sides: d.Sides, Value: d.Value})
	}
	for i := range r.Parts {
		out.Parts = append(out.Parts, *rollDTO(&r.Parts[i]))
	}
	return out
}

func effects(b *castsvc.AppliedBuff) map[string]float64 {
	fx := b.Buff.Effects
	out := map[string]float64{}
	ints := map[string]*int{
		"min_str": fx.MinStr, "weapon_buff": fx.WeaponBuff, "save_bonus": fx.SaveBonus,
		"str_bonus": fx.StrBonus, "str_max": fx.StrMax,
		"ac": fx.AC, "to_hit": fx.ToHit, "damage": fx.Damage,
	}
	for k, v := range ints {
		if v != nil {
			out[k] = float64(*v)
		}
	}
	if fx.SpellsBuff != nil {
		out["spells_buff"] = *fx.SpellsBuff
	}
	if fx.Multiplier != nil {
		out["multiplier"] = *fx.Multiplier
	}
	return out
}
