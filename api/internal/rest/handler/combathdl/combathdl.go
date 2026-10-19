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
	ResolveSavingThrow(ctx context.Context, characterID int64, category combatsvc.SaveCategory) (*combatsvc.RollResult, error)
	AdjustSave(ctx context.Context, characterID, entryID int64, adj combatsvc.SaveAdjustment) (*combatsvc.RollResult, error)
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

// ResolveAttack runs an attack and returns the RollResult JSON.
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
		render.OK(r.Context(), w, toDTO(res))
		return nil
	}
}

// RollSave runs a saving throw and returns the RollResult JSON.
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
		render.OK(r.Context(), w, toDTO(res))
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
// RollResult. 409 when the entry is voided or not the latest roll, 400 when
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
		render.OK(r.Context(), w, toDTO(res))
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
// rolled or spent (docs/adr/0012 step 4). Returns the updated RollResult.
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
		render.OK(r.Context(), w, toDTO(res))
		return nil
	}
}

// ClearAttackAC makes the target AC of the latest attack (log entry {id})
// unknown again (docs/adr/0012 step 8); no body. Returns the updated
// RollResult. 409 when the entry is voided or not the latest roll, 400 when
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
		render.OK(r.Context(), w, toDTO(res))
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
// updated RollResult. 409 when the entry is voided or not the latest save,
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
		render.OK(r.Context(), w, toDTO(res))
		return nil
	}
}

// RollResultDTO is the full breakdown of a roll (camelCase JSON). The attack
// fields match combatsvc's action-log payload, so web/src/types/combat.ts
// reads both.
type RollResultDTO struct {
	D20Roll      int `json:"d20Roll"`
	D100CritRoll int `json:"d100CritRoll"`
	// D100MultiplierRoll is the second d100 whose crit-table row gives the
	// multiplier (docs/adr/0001-critical-hits.md); absent on saves.
	D100MultiplierRoll int           `json:"d100MultiplierRoll,omitempty"`
	IsCrit             bool          `json:"isCrit"`
	CritMultiplier     float64       `json:"critMultiplier"`
	CritPlaceholder    bool          `json:"critPlaceholder"`
	CritFrom           int           `json:"critFrom"`
	CritTo             int           `json:"critTo"`
	BaseThac0          int           `json:"baseThac0"`
	Modifiers          []ModifierDTO `json:"modifiers"`
	EffectiveThac0     int           `json:"effectiveThac0"`
	TargetACNeeded     int           `json:"targetACNeeded"`
	// HitsAC is the lowest target AC the attack hits, CritsFromAC the
	// lowest it crits (docs/adr/0012 steps 2 and 3); sent on attacks.
	HitsAC      *int `json:"hitsAc,omitempty"`
	CritsFromAC *int `json:"critsFromAc,omitempty"`
	// CritPossible: some real AC (at most 10) is crit, i.e. critsFromAc is
	// not beyond every AC (ADR 0012 step 7). Attacks only.
	CritPossible *bool `json:"critPossible,omitempty"`
	// Hit, hitMargin, isCrit and the damage are final only when acKnown or
	// verdict is set; until then hit is false and the damage is the one of
	// a hit at x1.
	Hit       bool `json:"hit"`
	HitMargin int  `json:"hitMargin"`

	BaseDamage     int             `json:"baseDamage"`
	DamageDice     string          `json:"damageDice"`
	SpellsBuff     float64         `json:"spellsBuff"`
	WeaponBuff     int             `json:"weaponBuff"`
	Multipliers    float64         `json:"multipliers"`
	DiceMultiplier float64         `json:"diceMultiplier"`
	StrengthBonus  int             `json:"strengthBonus"`
	BonusDamage    int             `json:"bonusDamage"`
	DamageTerms    []DamageTermDTO `json:"damageTerms"`
	// DamageRolls are the damage dice as thrown (absent when there is no damage).
	DamageRolls []DieDTO `json:"damageRolls,omitempty"`
	TotalDamage int      `json:"totalDamage"`
	// UnroundedDamage is the damage formula's result before the final
	// floor (ADR 0003 step 9); the UI shows the formula from these fields.
	UnroundedDamage float64 `json:"unroundedDamage"`

	// Attack context; empty on saving throws.
	WeaponID   string `json:"weaponId,omitempty"`
	WeaponName string `json:"weaponName,omitempty"`
	// TargetAC is null on an attack whose AC is unknown (0 on saves).
	TargetAC   *int     `json:"targetAc"`
	TargetSize string   `json:"targetSize,omitempty"`
	Ammo       *AmmoDTO `json:"ammo,omitempty"`
	// ACKnown is sent on attacks; RolledACUnknown: rolled without the AC
	// (still set once the GM gave it); Verdict: the GM's "hit" or "miss"
	// while the AC is unknown.
	ACKnown         *bool  `json:"acKnown,omitempty"`
	RolledACUnknown bool   `json:"rolledAcUnknown,omitempty"`
	Verdict         string `json:"verdict,omitempty"`
	// Target AC and outcome as rolled; present while a correction differs
	// from the rolled target AC.
	RolledTargetAC *int  `json:"rolledTargetAc,omitempty"`
	RolledHit      *bool `json:"rolledHit,omitempty"`
	RolledCrit     *bool `json:"rolledCrit,omitempty"`

	// Saving-throw context; empty on attacks. adjustment is always sent on
	// saves (0: none).
	SaveCategory     string `json:"saveCategory,omitempty"`
	Adjustment       *int   `json:"adjustment,omitempty"`
	AdjustmentReason string `json:"adjustmentReason,omitempty"`
	// Sent on saves only. The adjustment moves the target (ADR 0006 step 5):
	// rollTotal (d20 + saveBonus, the buffs) is compared with adjustedTarget
	// (target - adjustment; baseThac0 is the target, effectiveThac0 the
	// adjusted one). saveTotal is rollTotal + adjustment, kept for the
	// clients that read it.
	SaveBonus      *int `json:"saveBonus,omitempty"`
	SaveTotal      *int `json:"saveTotal,omitempty"`
	RollTotal      *int `json:"rollTotal,omitempty"`
	AdjustedTarget *int `json:"adjustedTarget,omitempty"`

	LogEntryID int64 `json:"logEntryId,omitempty"`
}

// ModifierDTO is one to-hit (or save) modifier. The UI labels it in the
// player's language from Kind and Detail; an item power's carries its item's
// id (ADR 0016 step 14), which the UI names it from (Detail, the stored
// name, is the fallback).
type ModifierDTO struct {
	Kind           string `json:"kind"`
	Detail         string `json:"detail,omitempty"`
	ItemID         string `json:"itemId,omitempty"`
	Value          int    `json:"value"`
	StrExceptional int    `json:"strExceptional,omitempty"` // a Strength modifier's 18/xx percentile (ADR 0030 step 9)
}

// DamageTermDTO is one contribution to the damage formula (ItemID as on
// ModifierDTO).
type DamageTermDTO struct {
	Term           string  `json:"term"`
	Kind           string  `json:"kind"`
	Detail         string  `json:"detail,omitempty"`
	ItemID         string  `json:"itemId,omitempty"`
	Value          float64 `json:"value"`
	StrExceptional int     `json:"strExceptional,omitempty"` // a Strength modifier's 18/xx percentile (ADR 0030 step 9)
}

// AmmoDTO is the ammunition an attack spent and what is left.
type AmmoDTO struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Magic     int    `json:"magic"`
	Remaining int    `json:"remaining"`
}

func toDTO(r *combatsvc.RollResult) RollResultDTO {
	if r == nil {
		return RollResultDTO{Modifiers: []ModifierDTO{}, DamageTerms: []DamageTermDTO{}}
	}
	mods := make([]ModifierDTO, 0, len(r.Modifiers))
	for _, m := range r.Modifiers {
		mods = append(mods, ModifierDTO{Kind: string(m.Kind), Detail: m.Detail, ItemID: m.ItemID, Value: m.Value, StrExceptional: m.StrExceptional})
	}
	terms := make([]DamageTermDTO, 0, len(r.DamageTerms))
	for _, t := range r.DamageTerms {
		terms = append(terms, DamageTermDTO{Term: string(t.Term), Kind: string(t.Kind), Detail: t.Detail, ItemID: t.ItemID, Value: t.Value, StrExceptional: t.StrExceptional})
	}
	out := RollResultDTO{
		D20Roll: r.D20Roll, D100CritRoll: r.D100CritRoll, D100MultiplierRoll: r.D100MultiplierRoll, IsCrit: r.IsCrit, CritMultiplier: r.CritMultiplier,
		CritPlaceholder: r.CritPlaceholder, CritFrom: r.CritFrom, CritTo: r.CritTo,
		BaseThac0: r.BaseThac0, Modifiers: mods, EffectiveThac0: r.EffectiveThac0, TargetACNeeded: r.TargetACNeeded,
		Hit: r.Hit, HitMargin: r.HitMargin,
		BaseDamage: r.BaseDamage, DamageDice: r.DamageDice, SpellsBuff: r.SpellsBuff, WeaponBuff: r.WeaponBuff,
		Multipliers: r.Multipliers, DiceMultiplier: r.DiceMultiplier, StrengthBonus: r.StrengthBonus, BonusDamage: r.BonusDamage, DamageTerms: terms,
		DamageRolls: dieDTOs(r.DamageRolls),
		TotalDamage: r.TotalDamage, UnroundedDamage: r.UnroundedDamage,
		WeaponID: r.WeaponID, WeaponName: r.WeaponName, TargetSize: string(r.TargetSize),
		RolledACUnknown: r.RolledACUnknown, Verdict: string(r.Verdict),
		RolledTargetAC: r.RolledTargetAC, RolledHit: r.RolledHit, RolledCrit: r.RolledCrit,
		SaveCategory: string(r.SaveCategory), AdjustmentReason: r.AdjustmentReason,
		LogEntryID: r.LogEntryID,
	}
	if r.SaveCategory == "" {
		known, hits, crits := r.ACKnown, r.HitsAC, r.CritsFromAC
		out.ACKnown, out.HitsAC, out.CritsFromAC = &known, &hits, &crits
		possible := r.CritPossible()
		out.CritPossible = &possible
	}
	if r.SaveCategory != "" || r.ACKnown {
		ac := r.TargetAC
		out.TargetAC = &ac
	}
	if r.SaveCategory != "" {
		adj, bonus, total, roll, target := r.Adjustment, r.SaveBonus, r.SaveTotal, r.SaveRollTotal, r.SaveAdjustedTarget
		out.Adjustment, out.SaveBonus, out.SaveTotal, out.RollTotal, out.AdjustedTarget = &adj, &bonus, &total, &roll, &target
	}
	if a := r.Ammo; a != nil {
		out.Ammo = &AmmoDTO{ItemID: a.ItemID, Name: a.Name, Magic: a.Magic, Remaining: a.Remaining}
	}
	return out
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

// DieDTO is one die thrown.
type DieDTO struct {
	Sides int `json:"sides"`
	Value int `json:"value"`
}

// dieDTOs maps thrown dice to their wire shape.
func dieDTOs(in []combatsvc.DieRoll) []DieDTO {
	var out []DieDTO
	for _, d := range in {
		out = append(out, DieDTO{Sides: d.Sides, Value: d.Value})
	}
	return out
}
