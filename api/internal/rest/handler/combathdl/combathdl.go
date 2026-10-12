// Package combathdl exposes /api/roll/attack and /api/roll/save.
package combathdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type combatSvc interface {
	ResolveAttack(ctx context.Context, characterID int64, weaponID string, targetAC int) (*combatsvc.RollResult, error)
	ResolveSavingThrow(ctx context.Context, characterID int64, category combatsvc.SaveCategory) (*combatsvc.RollResult, error)
	ListWeapons(ctx context.Context) ([]combatsvc.Weapon, error)
}

// RegisterRoutes wires combat endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc combatsvc.SVC) {
	r.Post("/api/roll/attack", render.Handle(ResolveAttack(charID, svc)))
	r.Post("/api/roll/save", render.Handle(RollSave(charID, svc)))
	r.Get("/api/weapons", render.Handle(ListWeapons(svc)))
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

// AttackRequestDTO matches the previous request shape.
type AttackRequestDTO struct {
	WeaponID string `json:"weapon_id"`
	TargetAC int    `json:"target_ac"`
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
		res, err := svc.ResolveAttack(r.Context(), charID(r), req.WeaponID, req.TargetAC)
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

// RollResultDTO mirrors the previous wire shape (camelCase JSON).
type RollResultDTO struct {
	D20Roll        int           `json:"d20Roll"`
	D100CritRoll   int           `json:"d100CritRoll"`
	IsCrit         bool          `json:"isCrit"`
	CritMultiplier int           `json:"critMultiplier"`
	BaseThac0      int           `json:"baseThac0"`
	Modifiers      []ModifierDTO `json:"modifiers"`
	EffectiveThac0 int           `json:"effectiveThac0"`
	TargetACNeeded int           `json:"targetACNeeded"`
	Hit            bool          `json:"hit"`
	HitMargin      int           `json:"hitMargin"`

	BaseDamage      int     `json:"baseDamage"`
	SpellsBuff      float64 `json:"spellsBuff"`
	WeaponBuff      int     `json:"weaponBuff"`
	Multipliers     float64 `json:"multipliers"`
	StrengthBonus   int     `json:"strengthBonus"`
	TotalDamage     int     `json:"totalDamage"`
	DamageBreakdown string  `json:"damageBreakdown"`
}

// ModifierDTO is the wire shape for a single modifier.
type ModifierDTO struct {
	Source string `json:"source"`
	Value  int    `json:"value"`
}

func toDTO(r *combatsvc.RollResult) RollResultDTO {
	if r == nil {
		return RollResultDTO{Modifiers: []ModifierDTO{}}
	}
	mods := make([]ModifierDTO, 0, len(r.Modifiers))
	for _, m := range r.Modifiers {
		mods = append(mods, ModifierDTO{Source: m.Source, Value: m.Value})
	}
	return RollResultDTO{
		D20Roll: r.D20Roll, D100CritRoll: r.D100CritRoll, IsCrit: r.IsCrit, CritMultiplier: r.CritMultiplier,
		BaseThac0: r.BaseThac0, Modifiers: mods, EffectiveThac0: r.EffectiveThac0, TargetACNeeded: r.TargetACNeeded,
		Hit: r.Hit, HitMargin: r.HitMargin,
		BaseDamage: r.BaseDamage, SpellsBuff: r.SpellsBuff, WeaponBuff: r.WeaponBuff,
		Multipliers: r.Multipliers, StrengthBonus: r.StrengthBonus,
		TotalDamage: r.TotalDamage, DamageBreakdown: r.DamageBreakdown,
	}
}
