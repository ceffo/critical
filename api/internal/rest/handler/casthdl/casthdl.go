// Package casthdl exposes POST /api/spells/{id}/cast and
// POST /api/spells/receive.
package casthdl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/spellhdl"
	"github.com/go-chi/chi/v5"
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
}

// RegisterRoutes wires the cast endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc castsvc.SVC) {
	r.Post("/api/spells/{id}/cast", render.Handle(Cast(charID, svc)))
	r.Post("/api/spells/receive", render.Handle(Receive(charID, svc)))
}

// CastRequestDTO is the optional body of POST /api/spells/{id}/cast.
type CastRequestDTO struct {
	// OnSelf: does a buff spell affect the Bard? Omitted: the spell's
	// default (see castsvc).
	OnSelf *bool `json:"onSelf,omitempty"`
	// SpellID: the spell cast from a blank Rary's extra slot.
	SpellID string `json:"spellId,omitempty"`
	// ExtraSlots: the levels of the slots Rary's grants ([1,1,1] by default).
	ExtraSlots []int `json:"extraSlots,omitempty"`
	// GemItemID / TargetAC: Chromatic Orb's gem (omitted: the only gem) and target AC.
	GemItemID string `json:"gemItemId,omitempty"`
	TargetAC  *int   `json:"targetAc,omitempty"`
}

// ReceiveRequestDTO is the body of POST /api/spells/receive.
type ReceiveRequestDTO struct {
	SpellID     string `json:"spellId"`
	CasterLevel int    `json:"casterLevel"`
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
		res, err := svc.Cast(r.Context(), charID(r), idx, castsvc.CastRequest{OnSelf: body.OnSelf, SpellID: body.SpellID, ExtraSlots: body.ExtraSlots, GemItemID: body.GemItemID, TargetAC: body.TargetAC})
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
		res, err := svc.Receive(r.Context(), charID(r), castsvc.ReceiveRequest{SpellID: body.SpellID, CasterLevel: body.CasterLevel})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(res))
		return nil
	}
}

// CastResultDTO is the wire shape of castsvc.CastResult.
type CastResultDTO struct {
	EntryID     int64                   `json:"entryId"`
	Source      string                  `json:"source"`
	SpellID     string                  `json:"spellId"`
	Name        string                  `json:"name"`
	Level       int                     `json:"level"`
	CasterLevel int                     `json:"casterLevel"`
	SlotIndex   *int                    `json:"slotIndex,omitempty"`
	Kind        string                  `json:"kind,omitempty"`
	Confidence  string                  `json:"confidence,omitempty"`
	Target      string                  `json:"target,omitempty"`
	Note        string                  `json:"note,omitempty"`
	Spell       *spellhdl.SpellDTO      `json:"spell,omitempty"`
	Save        *SaveDTO                `json:"save,omitempty"`
	TextOnly    string                  `json:"textOnly,omitempty"`
	Buff        *AppliedBuffDTO         `json:"buff,omitempty"`
	Damage      *DamageDTO              `json:"damage,omitempty"`
	Untracked   []UntrackedDTO          `json:"untracked"`
	Slots       []spellhdl.SpellSlotDTO `json:"slots,omitempty"`
	// ExtraSlots: levels of the Rary's slots this cast added.
	ExtraSlots []int `json:"extraSlots,omitempty"`
	// Orb: a Chromatic Orb's gem, attack roll and effect.
	Orb *OrbDTO `json:"orb,omitempty"`
}

// OrbDTO is a Chromatic Orb's outcome: the spent gem, the attack roll (no
// crit) and, on a hit, the gem's effect text for the DM to announce.
type OrbDTO struct {
	GemItemID      string      `json:"gemItemId"`
	Gem            string      `json:"gem"`
	Remaining      int         `json:"remaining"`
	D20Roll        int         `json:"d20Roll"`
	BaseThac0      int         `json:"baseThac0"`
	Modifiers      []OrbModDTO `json:"modifiers"`
	EffectiveThac0 int         `json:"effectiveThac0"`
	TargetAC       int         `json:"targetAc"`
	TargetACNeeded int         `json:"targetACNeeded"`
	Hit            bool        `json:"hit"`
	HitMargin      int         `json:"hitMargin"`
	Effect         string      `json:"effect,omitempty"`
}

// OrbModDTO is one to-hit modifier of the orb's attack.
type OrbModDTO struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Detail string `json:"detail,omitempty"`
	Value  int    `json:"value"`
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
		Level: res.Level, CasterLevel: res.CasterLevel, SlotIndex: res.SlotIndex,
		Kind: string(res.Kind), Confidence: string(res.Confidence), Target: string(res.Target),
		Note: res.Note, TextOnly: string(res.TextOnly), Untracked: []UntrackedDTO{},
	}
	if res.Spell.Name != "" {
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
	if d := res.Damage; d != nil {
		out.Damage = &DamageDTO{Dice: d.Dice, Type: d.Type, Instances: d.Instances, Rounds: d.Rounds, Total: d.Total, Rolls: []RollDTO{}}
		for i := range d.Rolls {
			out.Damage.Rolls = append(out.Damage.Rolls, *rollDTO(&d.Rolls[i]))
		}
	}
	for _, u := range res.Untracked {
		out.Untracked = append(out.Untracked, UntrackedDTO{Key: u.Key, Value: u.Value, Condition: u.Condition})
	}
	out.ExtraSlots = res.ExtraSlots
	if o := res.Orb; o != nil {
		out.Orb = &OrbDTO{
			GemItemID: o.GemItemID, Gem: o.Gem, Remaining: o.Remaining, D20Roll: o.Attack.D20Roll,
			BaseThac0: o.Attack.BaseThac0, Modifiers: []OrbModDTO{}, EffectiveThac0: o.Attack.EffectiveThac0,
			TargetAC: o.Attack.TargetAC, TargetACNeeded: o.Attack.TargetACNeeded, Hit: o.Attack.Hit,
			HitMargin: o.Attack.HitMargin, Effect: o.Effect,
		}
		for _, m := range o.Attack.Modifiers {
			out.Orb.Modifiers = append(out.Orb.Modifiers, OrbModDTO{Kind: string(m.Kind), Source: m.Source, Detail: m.Detail, Value: m.Value})
		}
	}
	if res.Slots != nil {
		out.Slots = spellhdl.SlotsToDTO(res.Slots)
	}
	return out
}

func rollDTO(r *castsvc.Roll) *RollDTO {
	if r == nil {
		return nil
	}
	out := &RollDTO{Formula: r.Formula, Total: r.Total, Dice: []DieDTO{}}
	for _, d := range r.Dice {
		out.Dice = append(out.Dice, DieDTO{Sides: d.Sides, Value: d.Value})
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
