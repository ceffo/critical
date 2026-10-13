// Package combathdl exposes /api/roll/attack, /api/roll/save, /api/weapons and
// /api/crit-table.
package combathdl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type combatSvc interface {
	ResolveAttack(ctx context.Context, characterID int64, req combatsvc.AttackRequest) (*combatsvc.RollResult, error)
	ResolveSavingThrow(ctx context.Context, characterID int64, category combatsvc.SaveCategory) (*combatsvc.RollResult, error)
	ListWeapons(ctx context.Context) ([]combatsvc.Weapon, error)
	GetCritTable(ctx context.Context) (combatsvc.CritTable, error)
	SetCritTable(ctx context.Context, rows []combatsvc.CritRow) (combatsvc.CritTable, error)
}

// RegisterRoutes wires combat endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc combatsvc.SVC) {
	r.Post("/api/roll/attack", render.Handle(ResolveAttack(charID, svc)))
	r.Post("/api/roll/save", render.Handle(RollSave(charID, svc)))
	r.Get("/api/weapons", render.Handle(ListWeapons(svc)))
	r.Get("/api/crit-table", render.Handle(GetCritTable(svc)))
	r.Put("/api/crit-table", render.Handle(PutCritTable(svc)))
}

// WeaponDTO is the wire shape of a weapon catalog entry.
type WeaponDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DamageSM    string `json:"damage_sm"`
	DamageL     string `json:"damage_l"`
	MagicToHit  int    `json:"magic_to_hit"`
	MagicDamage int    `json:"magic_damage"`
	Kind        string `json:"kind"`
	TwoHanded   bool   `json:"two_handed"`
	AmmoKind    string `json:"ammo_kind"`
}

// ListWeapons returns the weapons catalog.
func ListWeapons(svc combatSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		weapons, err := svc.ListWeapons(r.Context())
		if err != nil {
			return err
		}
		out := make([]WeaponDTO, 0, len(weapons))
		for _, wp := range weapons {
			out = append(out, WeaponDTO{
				ID: wp.ID, Name: wp.Name, DamageSM: wp.BaseDamage, DamageL: wp.DamageLarge,
				MagicToHit: wp.MagicToHit, MagicDamage: wp.MagicDamage, Kind: string(wp.Kind),
				TwoHanded: wp.TwoHanded, AmmoKind: wp.AmmoKind,
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
	TargetAC   int    `json:"target_ac"`
	// TargetSize is "sm" (small/medium, the default) or "l" (large).
	TargetSize  string          `json:"target_size,omitempty"`
	Situational *SituationalDTO `json:"situational,omitempty"`
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
			TargetAC: req.TargetAC, TargetSize: combatsvc.TargetSize(req.TargetSize),
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

// RollResultDTO is the full breakdown of a roll (camelCase JSON). The attack
// fields match combatsvc's action-log payload, so web/src/types/combat.ts
// reads both.
type RollResultDTO struct {
	D20Roll         int           `json:"d20Roll"`
	D100CritRoll    int           `json:"d100CritRoll"`
	IsCrit          bool          `json:"isCrit"`
	CritMultiplier  int           `json:"critMultiplier"`
	CritPlaceholder bool          `json:"critPlaceholder"`
	CritFrom        int           `json:"critFrom"`
	CritTo          int           `json:"critTo"`
	BaseThac0       int           `json:"baseThac0"`
	Modifiers       []ModifierDTO `json:"modifiers"`
	EffectiveThac0  int           `json:"effectiveThac0"`
	TargetACNeeded  int           `json:"targetACNeeded"`
	Hit             bool          `json:"hit"`
	HitMargin       int           `json:"hitMargin"`

	BaseDamage      int             `json:"baseDamage"`
	DamageDice      string          `json:"damageDice"`
	SpellsBuff      float64         `json:"spellsBuff"`
	WeaponBuff      int             `json:"weaponBuff"`
	Multipliers     float64         `json:"multipliers"`
	StrengthBonus   int             `json:"strengthBonus"`
	BonusDamage     int             `json:"bonusDamage"`
	DamageTerms     []DamageTermDTO `json:"damageTerms"`
	TotalDamage     int             `json:"totalDamage"`
	DamageBreakdown string          `json:"damageBreakdown"`

	// Attack context; empty on saving throws.
	WeaponID   string   `json:"weaponId,omitempty"`
	WeaponName string   `json:"weaponName,omitempty"`
	TargetAC   int      `json:"targetAc"`
	TargetSize string   `json:"targetSize,omitempty"`
	Ammo       *AmmoDTO `json:"ammo,omitempty"`
	LogEntryID int64    `json:"logEntryId,omitempty"`
}

// ModifierDTO is one to-hit (or save) modifier. Kind lets the UI label it
// in the player's language; Source is the English label.
type ModifierDTO struct {
	Kind   string `json:"kind"`
	Source string `json:"source"`
	Detail string `json:"detail,omitempty"`
	Value  int    `json:"value"`
}

// DamageTermDTO is one contribution to the damage formula.
type DamageTermDTO struct {
	Term   string  `json:"term"`
	Kind   string  `json:"kind"`
	Source string  `json:"source"`
	Detail string  `json:"detail,omitempty"`
	Value  float64 `json:"value"`
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
		mods = append(mods, ModifierDTO{Kind: string(m.Kind), Source: m.Source, Detail: m.Detail, Value: m.Value})
	}
	terms := make([]DamageTermDTO, 0, len(r.DamageTerms))
	for _, t := range r.DamageTerms {
		terms = append(terms, DamageTermDTO{Term: string(t.Term), Kind: string(t.Kind), Source: t.Source, Detail: t.Detail, Value: t.Value})
	}
	out := RollResultDTO{
		D20Roll: r.D20Roll, D100CritRoll: r.D100CritRoll, IsCrit: r.IsCrit, CritMultiplier: r.CritMultiplier,
		CritPlaceholder: r.CritPlaceholder, CritFrom: r.CritFrom, CritTo: r.CritTo,
		BaseThac0: r.BaseThac0, Modifiers: mods, EffectiveThac0: r.EffectiveThac0, TargetACNeeded: r.TargetACNeeded,
		Hit: r.Hit, HitMargin: r.HitMargin,
		BaseDamage: r.BaseDamage, DamageDice: r.DamageDice, SpellsBuff: r.SpellsBuff, WeaponBuff: r.WeaponBuff,
		Multipliers: r.Multipliers, StrengthBonus: r.StrengthBonus, BonusDamage: r.BonusDamage, DamageTerms: terms,
		TotalDamage: r.TotalDamage, DamageBreakdown: r.DamageBreakdown,
		WeaponID: r.WeaponID, WeaponName: r.WeaponName, TargetAC: r.TargetAC, TargetSize: string(r.TargetSize),
		LogEntryID: r.LogEntryID,
	}
	if a := r.Ammo; a != nil {
		out.Ammo = &AmmoDTO{ItemID: a.ItemID, Name: a.Name, Magic: a.Magic, Remaining: a.Remaining}
	}
	return out
}

// CritRowDTO is one inclusive D100 range of the crit table.
type CritRowDTO struct {
	From       int `json:"from"`
	To         int `json:"to"`
	Multiplier int `json:"multiplier"`
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
		// Numbers are decoded as text so a 2.5 gets a message about the row,
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
			mult, errMult := strconv.Atoi(in.Multiplier.String())
			label := fmt.Sprintf("row %d (%s-%s)", i+1, in.From, in.To)
			switch {
			case errFrom != nil || errTo != nil:
				return render.BadRequestError(label + ": from and to must be whole numbers")
			case errMult != nil:
				return render.BadRequestError(fmt.Sprintf("%s: multiplier must be a whole number, got %q", label, in.Multiplier.String()))
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
