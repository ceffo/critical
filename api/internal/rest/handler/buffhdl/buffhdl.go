// Package buffhdl exposes /api/buffs and /api/round.
package buffhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/go-chi/chi/v5"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type buffSvc interface {
	ListActive(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, error)
	Activate(ctx context.Context, characterID int64, buff buffsvc.BuffEffect) ([]buffsvc.BuffEffect, error)
	Dismiss(ctx context.Context, characterID int64, buffID string) ([]buffsvc.BuffEffect, error)
	Trigger(ctx context.Context, characterID int64, buffID string) ([]buffsvc.BuffEffect, error)
	GetRound(ctx context.Context, characterID int64) (int, error)
	AdvanceRound(ctx context.Context, characterID int64) (active []buffsvc.BuffEffect, expired []buffsvc.BuffEffect, round int, err error)
}

// RegisterRoutes wires buff and round endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc buffsvc.SVC) {
	r.Get("/api/buffs", render.Handle(ListActive(charID, svc)))
	r.Post("/api/buffs", render.Handle(Activate(charID, svc)))
	r.Delete("/api/buffs/{id}", render.Handle(Dismiss(charID, svc)))
	r.Post("/api/buffs/{id}/trigger", render.Handle(Trigger(charID, svc)))
	r.Get("/api/round", render.Handle(GetRound(charID, svc)))
	r.Post("/api/round/advance", render.Handle(AdvanceRound(charID, svc)))
}

// ListActive returns the current buff list.
func ListActive(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		buffs, err := svc.ListActive(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTOs(buffs))
		return nil
	}
}

// Activate adds a buff to the active list.
func Activate(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var dto BuffEffectDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if dto.Name == "" {
			return render.BadRequestError("name is required")
		}
		buffs, err := svc.Activate(r.Context(), charID(r), dto.toDomain())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTOs(buffs))
		return nil
	}
}

// Dismiss removes the buff with the given id.
func Dismiss(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := chi.URLParam(r, "id")
		if id == "" {
			return render.BadRequestError("id is required")
		}
		buffs, err := svc.Dismiss(r.Context(), charID(r), id)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTOs(buffs))
		return nil
	}
}

// Trigger decrements charges on a charge buff.
func Trigger(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := chi.URLParam(r, "id")
		if id == "" {
			return render.BadRequestError("id is required")
		}
		buffs, err := svc.Trigger(r.Context(), charID(r), id)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTOs(buffs))
		return nil
	}
}

// GetRound returns the current round number.
func GetRound(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		round, err := svc.GetRound(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, map[string]int{"round": round})
		return nil
	}
}

// AdvanceRound increments the counter and ages duration buffs.
func AdvanceRound(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		active, expired, round, err := svc.AdvanceRound(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RoundAdvanceDTO{
			RoundNumber:  round,
			ActiveBuffs:  toDTOs(active),
			ExpiredBuffs: toDTOs(expired),
		})
		return nil
	}
}

// BuffEffectDTO is the wire shape for a single buff. Mirrors the previous
// domain.BuffEffect JSON layout (snake_case effect fields).
type BuffEffectDTO struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Charges  int            `json:"charges"`
	Duration int            `json:"duration"`
	Effects  BuffEffectsDTO `json:"effects"`
}

// BuffEffectsDTO is the modifier payload.
type BuffEffectsDTO struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
}

// RoundAdvanceDTO mirrors the previous response shape from RoundHandler.Advance.
type RoundAdvanceDTO struct {
	RoundNumber  int             `json:"roundNumber"`
	ActiveBuffs  []BuffEffectDTO `json:"activeBuffs"`
	ExpiredBuffs []BuffEffectDTO `json:"expiredBuffs"`
}

func (d BuffEffectDTO) toDomain() buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: d.ID, Name: d.Name, Type: d.Type, Charges: d.Charges, Duration: d.Duration,
		Effects: buffsvc.BuffEffects{
			MinStr: d.Effects.MinStr, SpellsBuff: d.Effects.SpellsBuff,
			WeaponBuff: d.Effects.WeaponBuff, Multiplier: d.Effects.Multiplier,
			SaveBonus: d.Effects.SaveBonus,
		},
	}
}

func toDTO(b buffsvc.BuffEffect) BuffEffectDTO {
	return BuffEffectDTO{
		ID: b.ID, Name: b.Name, Type: b.Type, Charges: b.Charges, Duration: b.Duration,
		Effects: BuffEffectsDTO{
			MinStr: b.Effects.MinStr, SpellsBuff: b.Effects.SpellsBuff,
			WeaponBuff: b.Effects.WeaponBuff, Multiplier: b.Effects.Multiplier,
			SaveBonus: b.Effects.SaveBonus,
		},
	}
}

func toDTOs(bs []buffsvc.BuffEffect) []BuffEffectDTO {
	out := make([]BuffEffectDTO, 0, len(bs))
	for _, b := range bs {
		out = append(out, toDTO(b))
	}
	return out
}
