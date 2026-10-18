package combathdl

import (
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// AttackFormDTO is the attack form's last choices.
type AttackFormDTO struct {
	WeaponID         string `json:"weaponId"`
	PreviousWeaponID string `json:"previousWeaponId"`
	AmmoItemID       string `json:"ammoItemId"`
	// TargetAC is null while the target's AC is unknown.
	TargetAC    *int           `json:"targetAc"`
	TargetSize  string         `json:"targetSize"`
	Situational SituationalDTO `json:"situational"`
}

// AmmoChoiceDTO is one ammunition the weapon in hand can fire.
type AmmoChoiceDTO struct {
	ItemID string `json:"itemId"`
	// Name is a language map (ADR 0023 step 9).
	Name  localized.Text `json:"name"`
	Magic int            `json:"magic"`
	Count int            `json:"count"`
}

// AttackPreviewDTO is the effective THAC0 of the current choices, before
// rolling (ADR 0002 step 12).
type AttackPreviewDTO struct {
	BaseThac0      int           `json:"baseThac0"`
	Modifiers      []ModifierDTO `json:"modifiers"`
	EffectiveThac0 int           `json:"effectiveThac0"`
	ToHit          int           `json:"toHit"`
	// RollNeeded is null while the target AC is unknown.
	RollNeeded  *int `json:"rollNeeded"`
	DamageBonus int  `json:"damageBonus"`
	// PointBlank is the same preview with the point-blank option (ADR 0030
	// step 12); absent when the weapon does not allow it.
	PointBlank *AttackPreviewDTO `json:"pointBlank,omitempty"`
}

// AttackRateDTO is the held weapon's attacks per round (ADR 0030 step 11):
// attacks every rounds rounds, thisRound in the current round.
type AttackRateDTO struct {
	Attacks   int `json:"attacks"`
	Rounds    int `json:"rounds"`
	ThisRound int `json:"thisRound"`
}

// AttackFormViewDTO is the body of GET and PATCH /api/attack/form.
type AttackFormViewDTO struct {
	Form            AttackFormDTO     `json:"form"`
	AmmoFromDefault bool              `json:"ammoFromDefault"`
	AmmoChoices     []AmmoChoiceDTO   `json:"ammoChoices"`
	Preview         *AttackPreviewDTO `json:"preview"`
	// AttackRate is absent when the weapon shows no rate (ADR 0030 step 11.3).
	AttackRate *AttackRateDTO `json:"attackRate,omitempty"`
	// MaxRealAC is the highest armor class the rules allow; the UI stops
	// the target AC stepper there (docs/adr/0012 step 7).
	MaxRealAC int `json:"maxRealAc"`
}

// AttackFormPatchDTO is the body of PATCH /api/attack/form: only the fields
// sent change. "targetAc": null makes the target AC unknown.
type AttackFormPatchDTO struct {
	WeaponID    *string         `json:"weaponId"`
	AmmoItemID  *string         `json:"ammoItemId"`
	TargetAC    json.RawMessage `json:"targetAc"`
	TargetSize  *string         `json:"targetSize"`
	Situational *SituationalDTO `json:"situational"`
}

// GetAttackForm returns the attack form with its preview.
func GetAttackForm(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		v, err := svc.GetAttackForm(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toFormViewDTO(v))
		return nil
	}
}

// UpdateAttackForm saves the fields sent and returns the form with its
// preview; 400 with the field for an unknown weapon or ammunition, an
// ammunition the weapon cannot fire or a value out of range.
func UpdateAttackForm(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req AttackFormPatchDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		p := combatsvc.AttackFormPatch{WeaponID: req.WeaponID, AmmoItemID: req.AmmoItemID}
		switch {
		case len(req.TargetAC) == 0:
		case string(req.TargetAC) == "null":
			p.UnknownTargetAC = true
		default:
			var ac int
			if err := json.Unmarshal(req.TargetAC, &ac); err != nil {
				return render.BadRequestError("targetAc must be a whole number or null")
			}
			p.TargetAC = &ac
		}
		if req.TargetSize != nil {
			size := combatsvc.TargetSize(*req.TargetSize)
			p.TargetSize = &size
		}
		if s := req.Situational; s != nil {
			p.Situational = &combatsvc.Situational{Value: s.Value, Label: s.Label}
		}
		v, err := svc.UpdateAttackForm(r.Context(), charID(r), p)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toFormViewDTO(v))
		return nil
	}
}

func toFormViewDTO(v *combatsvc.AttackFormView) AttackFormViewDTO {
	f := v.Form
	out := AttackFormViewDTO{
		Form: AttackFormDTO{
			WeaponID: f.WeaponID, PreviousWeaponID: f.PreviousWeaponID, AmmoItemID: f.AmmoItemID,
			TargetAC: f.TargetAC, TargetSize: string(f.TargetSize),
			Situational: SituationalDTO{Value: f.Situational.Value, Label: f.Situational.Label},
		},
		AmmoFromDefault: v.AmmoFromDefault,
		AmmoChoices:     make([]AmmoChoiceDTO, 0, len(v.AmmoChoices)),
		MaxRealAC:       combatsvc.MaxRealAC,
	}
	for _, a := range v.AmmoChoices {
		out.AmmoChoices = append(out.AmmoChoices, AmmoChoiceDTO{ItemID: a.ItemID, Name: a.Name, Magic: a.Magic, Count: a.Count})
	}
	out.Preview = toPreviewDTO(v.Preview)
	if r := v.AttackRate; r != nil {
		out.AttackRate = &AttackRateDTO{Attacks: r.Attacks, Rounds: r.Rounds, ThisRound: r.ThisRound}
	}
	return out
}

func toPreviewDTO(p *combatsvc.AttackPreview) *AttackPreviewDTO {
	if p == nil {
		return nil
	}
	mods := make([]ModifierDTO, 0, len(p.Modifiers))
	for _, m := range p.Modifiers {
		mods = append(mods, ModifierDTO{Kind: string(m.Kind), Detail: m.Detail, ItemID: m.ItemID, Value: m.Value})
	}
	return &AttackPreviewDTO{
		BaseThac0: p.BaseThac0, Modifiers: mods, EffectiveThac0: p.EffectiveThac0,
		ToHit: p.ToHit, RollNeeded: p.RollNeeded, DamageBonus: p.DamageBonus,
		PointBlank: toPreviewDTO(p.PointBlank),
	}
}
