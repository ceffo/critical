// Package combathdl exposes /api/roll/attack (and its target AC
// correction and GM verdict), /api/roll/save (and its adjustment), /api/weapons,
// /api/attack/form and /api/crit-table.
package combathdl

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type combatSvc interface {
	ResolveAttack(ctx context.Context, characterID int64, req combatsvc.AttackRequest) (*combatsvc.RollResult, error)
	ResolveSavingThrow(ctx context.Context, characterID int64, category combatsvc.SaveCategory) (*combatsvc.SaveBreakdown, error)
	AdjustSave(ctx context.Context, characterID, entryID int64, adj combatsvc.SaveAdjustment) (*combatsvc.SaveBreakdown, error)
	AdjustAttackAC(ctx context.Context, characterID, entryID int64, targetAC int) (*combatsvc.RollResult, error)
	SetAttackVerdict(ctx context.Context, characterID, entryID int64, hit bool) (*combatsvc.RollResult, error)
	ClearAttackAC(ctx context.Context, characterID, entryID int64) (*combatsvc.RollResult, error)
	ListWeapons(ctx context.Context, characterID int64) ([]combatsvc.Weapon, error)
	GetAttackForm(ctx context.Context, characterID int64) (*combatsvc.AttackFormView, error)
	UpdateAttackForm(ctx context.Context, characterID int64, p combatsvc.AttackFormPatch) (*combatsvc.AttackFormView, error)
	GetCritTable(ctx context.Context) (combatsvc.CritTable, error)
	SetCritTable(ctx context.Context, rows []combatsvc.CritRow) (combatsvc.CritTable, error)
}

// RegisterRoutes wires the combat endpoints of the current character onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc combatsvc.SVC) {
	r.Post("/api/roll/attack", render.Handle(ResolveAttack(charID, svc)))
	r.Post("/api/roll/attack/{id}/adjust", render.Handle(AdjustAttack(charID, svc)))
	r.Post("/api/roll/attack/{id}/verdict", render.Handle(AttackVerdict(charID, svc)))
	r.Post("/api/roll/attack/{id}/unknown-ac", render.Handle(ClearAttackAC(charID, svc)))
	r.Post("/api/roll/save", render.Handle(RollSave(charID, svc)))
	r.Post("/api/roll/save/{id}/adjust", render.Handle(AdjustSave(charID, svc)))
	r.Get("/api/weapons", render.Handle(ListWeapons(charID, svc)))
	r.Get("/api/attack/form", render.Handle(GetAttackForm(charID, svc)))
	r.Patch("/api/attack/form", render.Handle(UpdateAttackForm(charID, svc)))
}

// RegisterSharedRoutes wires the reading of the house crit table, which no
// character owns, onto r.
func RegisterSharedRoutes(r Router, svc combatsvc.SVC) {
	r.Get("/api/crit-table", render.Handle(GetCritTable(svc)))
}

// RegisterAdminRoutes wires the editing of the house crit table onto r, which
// the caller guards with mw.AdminOnly: the table is one rule set shared by
// every user, so only an admin changes it.
func RegisterAdminRoutes(r Router, svc combatsvc.SVC) {
	r.Put("/api/crit-table", render.Handle(PutCritTable(svc)))
}

// WeaponDTO is the wire shape of a weapon piece as the attack reads it (ADR
// 0033 step 3); the piece itself is served by GET /api/items.
type WeaponDTO struct {
	ID string `json:"id"`
	// Name is a language map (ADR 0023 step 9).
	Name        localized.Text `json:"name"`
	DamageSM    string         `json:"damage_sm"`
	DamageL     string         `json:"damage_l"`
	MagicToHit  int            `json:"magic_to_hit"`
	MagicDamage int            `json:"magic_damage"`
	Kind        string         `json:"kind"`
	TwoHanded   bool           `json:"two_handed"`
	AmmoKind    string         `json:"ammo_kind"`
	// WeaponType is a code of itemsvc.WeaponTypes ("" when untyped); the UI words it.
	WeaponType string `json:"weapon_type"`
	// Specialized is the fighter's weapon specialization (ADR 0030 step 12).
	Specialized bool `json:"specialized"`
	// Speed is the speed factor (0 to 20); Wielded whether the weapon is
	// wielded, the only ones the attack form holds (ADR 0033 step 8).
	Speed   int  `json:"speed"`
	Wielded bool `json:"wielded"`
}

// ListWeapons returns the character's weapons.
func ListWeapons(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		weapons, err := svc.ListWeapons(r.Context(), charID(r))
		if err != nil {
			return err
		}
		out := make([]WeaponDTO, 0, len(weapons))
		for _, wp := range weapons {
			out = append(out, WeaponDTO{
				ID: wp.ID, Name: wp.Name, DamageSM: wp.BaseDamage, DamageL: wp.DamageLarge,
				MagicToHit: wp.MagicToHit, MagicDamage: wp.MagicDamage, Kind: string(wp.Kind),
				TwoHanded: wp.TwoHanded, AmmoKind: wp.AmmoKind, WeaponType: wp.Type, Specialized: wp.Specialized,
				Speed: wp.Speed, Wielded: wp.Wielded,
			})
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// AttackRequestDTO is the body of POST /api/roll/attack.
type AttackRequestDTO struct {
	WeaponID string `json:"weapon_id"`
	// AmmoItemID is required for a weapon that fires ammunition (bows).
	AmmoItemID string `json:"ammo_item_id,omitempty"`
	// TargetAC absent or null: the target's AC is unknown (docs/adr/0012).
	TargetAC *int `json:"target_ac"`
	// TargetSize is "sm" (small/medium, the default) or "l" (large).
	TargetSize  string          `json:"target_size,omitempty"`
	Situational *SituationalDTO `json:"situational,omitempty"`
	// PointBlank is the point-blank option of a specialized launcher (ADR
	// 0030 step 12); refused for another weapon.
	PointBlank bool `json:"point_blank,omitempty"`
	// Backstab is a thief's backstab (ADR 0030 step 19): +4 to hit and the
	// damage dice multiplied by PHB Table 30; refused for another class.
	Backstab bool `json:"backstab,omitempty"`
}

// SituationalDTO is a to-hit modifier entered at the table.
type SituationalDTO struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

// SaveRequestDTO matches the previous request shape.
type SaveRequestDTO struct {
	Category combatsvc.SaveCategory `json:"category"`
}

// ResolveAttack runs an attack and returns its breakdown, the payload of its
// log entry plus the entry id (combatsvc.AttackBreakdown).
func ResolveAttack(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req AttackRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		in := combatsvc.AttackRequest{
			WeaponID: req.WeaponID, AmmoItemID: req.AmmoItemID,
			TargetAC: req.TargetAC, TargetSize: combatsvc.TargetSize(req.TargetSize), PointBlank: req.PointBlank,
			Backstab: req.Backstab,
		}
		if req.Situational != nil {
			in.Situational = &combatsvc.Situational{Value: req.Situational.Value, Label: req.Situational.Label}
		}
		res, err := svc.ResolveAttack(r.Context(), charID(r), in)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, combatsvc.NewAttackBreakdown(res))
		return nil
	}
}

// RollSave runs a saving throw and returns its breakdown, the payload of its
// log entry plus the entry id (combatsvc.SaveBreakdown, see saveAnswer).
func RollSave(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req SaveRequestDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		res, err := svc.ResolveSavingThrow(r.Context(), charID(r), req.Category)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, newSaveAnswer(res))
		return nil
	}
}

// AttackAdjustmentDTO is the body of POST /api/roll/attack/{id}/adjust: the
// corrected target AC.
type AttackAdjustmentDTO struct {
	TargetAC *int `json:"targetAc"`
}

// AdjustAttack corrects the target AC of the latest attack (log entry {id});
// the server recomputes hit, margin, crit and damage with the dice rolled
// with the attack (no reroll, no ammo spent) and returns the updated
// breakdown. 409 when the entry is voided or not the latest roll, 400 when
// it is not an attack or the AC is out of range, 404 when it does not exist.
func AdjustAttack(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			return render.BadRequestError("invalid log entry id")
		}
		var req AttackAdjustmentDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if req.TargetAC == nil {
			return render.BadRequestError("targetAc is required")
		}
		res, err := svc.AdjustAttackAC(r.Context(), charID(r), id, *req.TargetAC)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, combatsvc.NewAttackBreakdown(res))
		return nil
	}
}

// AttackVerdictDTO is the body of POST /api/roll/attack/{id}/verdict: the
// GM's hit (true) or miss (false).
type AttackVerdictDTO struct {
	Hit *bool `json:"hit"`
}

// AttackVerdict records the GM's hit or miss on the latest attack (log entry
// {id}) when its target AC is unknown: no crit, damage as rolled, nothing
// rolled or spent (docs/adr/0012 step 4). Returns the updated breakdown.
// 409 when the entry is voided or not the latest roll, 400 when it is not an
// attack or its AC is known, 404 when it does not exist.
func AttackVerdict(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			return render.BadRequestError("invalid log entry id")
		}
		var req AttackVerdictDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if req.Hit == nil {
			return render.BadRequestError("hit is required")
		}
		res, err := svc.SetAttackVerdict(r.Context(), charID(r), id, *req.Hit)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, combatsvc.NewAttackBreakdown(res))
		return nil
	}
}

// ClearAttackAC makes the target AC of the latest attack (log entry {id})
// unknown again (docs/adr/0012 step 8); no body. Returns the updated
// breakdown. 409 when the entry is voided or not the latest roll, 400 when
// it is not an attack or its AC is already unknown, 404 when it does not exist.
func ClearAttackAC(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			return render.BadRequestError("invalid log entry id")
		}
		res, err := svc.ClearAttackAC(r.Context(), charID(r), id)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, combatsvc.NewAttackBreakdown(res))
		return nil
	}
}

// SaveAdjustmentDTO is the body of POST /api/roll/save/{id}/adjust: the new
// signed adjustment (0 removes it) and an optional reason.
type SaveAdjustmentDTO struct {
	Adjustment *int   `json:"adjustment"`
	Reason     string `json:"reason,omitempty"`
}

// AdjustSave sets the adjustment of the latest save (log entry {id}); the
// server recomputes the verdict and margin with the same d20 and returns the
// updated breakdown. 409 when the entry is voided or not the latest save,
// 400 when it is not a save, 404 when it does not exist.
func AdjustSave(charID CharacterIDFunc, svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			return render.BadRequestError("invalid log entry id")
		}
		var req SaveAdjustmentDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if req.Adjustment == nil {
			return render.BadRequestError("adjustment is required")
		}
		res, err := svc.AdjustSave(r.Context(), charID(r), id, combatsvc.SaveAdjustment{Value: *req.Adjustment, Reason: req.Reason})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, newSaveAnswer(res))
		return nil
	}
}

// saveAnswer is the answer of the save endpoints: the save's breakdown
// (combatsvc.SaveBreakdown, the logged payload plus logEntryId) followed by
// hit, hitMargin and baseThac0, the passed, margin and target under the
// names the previous release's web reads from this answer. They are kept for
// one release so that a page of that release still shows the right verdict
// against this server during the rolling deploy (ADR 0024 step 5.2, as ADR
// 0032 step 8 did); they are never logged. Drop them in the next release.
type saveAnswer struct {
	combatsvc.SaveBreakdown
	Hit       bool `json:"hit"`
	HitMargin int  `json:"hitMargin"`
	BaseThac0 int  `json:"baseThac0"`
}

func newSaveAnswer(b *combatsvc.SaveBreakdown) saveAnswer {
	return saveAnswer{SaveBreakdown: *b, Hit: b.Passed, HitMargin: b.Margin, BaseThac0: b.Target}
}

// CritRowDTO is one inclusive D100 range of the crit table.
type CritRowDTO struct {
	From       int     `json:"from"`
	To         int     `json:"to"`
	Multiplier float64 `json:"multiplier"`
}

// CritTableDTO is the wire shape of the house crit table. Placeholder is true
// while it is still the unedited default.
type CritTableDTO struct {
	Rows        []CritRowDTO `json:"rows"`
	Placeholder bool         `json:"placeholder"`
}

func toCritTableDTO(t combatsvc.CritTable) CritTableDTO {
	rows := make([]CritRowDTO, 0, len(t.Rows))
	for _, r := range t.Rows {
		rows = append(rows, CritRowDTO{From: r.From, To: r.To, Multiplier: r.Multiplier})
	}
	return CritTableDTO{Rows: rows, Placeholder: t.Placeholder}
}

// GetCritTable returns the house crit table.
func GetCritTable(svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		t, err := svc.GetCritTable(r.Context())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toCritTableDTO(t))
		return nil
	}
}

// PutCritTable replaces the whole crit table; an invalid table is a 400 with
// {"error"} naming the bad row and the stored table is left alone.
func PutCritTable(svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		// Numbers are decoded as text so a bad number gets a message about the row,
		// not "invalid request body".
		var req struct {
			Rows []struct {
				From       json.Number `json:"from"`
				To         json.Number `json:"to"`
				Multiplier json.Number `json:"multiplier"`
			} `json:"rows"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if len(req.Rows) == 0 {
			return render.BadRequestError("rows must list at least one range covering 1-100")
		}
		rows := make([]combatsvc.CritRow, 0, len(req.Rows))
		for i, in := range req.Rows {
			from, errFrom := strconv.Atoi(in.From.String())
			to, errTo := strconv.Atoi(in.To.String())
			mult, errMult := strconv.ParseFloat(in.Multiplier.String(), 64)
			label := fmt.Sprintf("row %d (%s-%s)", i+1, in.From, in.To)
			switch {
			case errFrom != nil || errTo != nil:
				return render.BadRequestError(label + ": from and to must be whole numbers")
			case errMult != nil || math.IsNaN(mult) || math.IsInf(mult, 0):
				return render.BadRequestError(fmt.Sprintf("%s: multiplier must be a number, got %q", label, in.Multiplier.String()))
			}
			rows = append(rows, combatsvc.CritRow{From: from, To: to, Multiplier: mult})
		}
		t, err := svc.SetCritTable(r.Context(), rows)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toCritTableDTO(t))
		return nil
	}
}
