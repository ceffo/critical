// Package buffhdl exposes /api/buffs, /api/round and /api/stats/str.
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
	Patch(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type buffSvc interface {
	ListStatus(ctx context.Context, characterID int64) ([]buffsvc.BuffStatus, error)
	Activate(ctx context.Context, characterID int64, buff buffsvc.BuffEffect) ([]buffsvc.BuffEffect, error)
	Edit(ctx context.Context, characterID int64, buffID string, e buffsvc.BuffEdit) ([]buffsvc.BuffEffect, error)
	Dismiss(ctx context.Context, characterID int64, buffID string) ([]buffsvc.BuffEffect, error)
	Trigger(ctx context.Context, characterID int64, buffID string) ([]buffsvc.BuffEffect, error)
	GetRound(ctx context.Context, characterID int64) (int, error)
	AdvanceRound(ctx context.Context, characterID int64) (active []buffsvc.BuffEffect, expired []buffsvc.BuffEffect, round int, err error)
	GetEffectiveStr(ctx context.Context, characterID int64) (buffsvc.Strength, error)
}

// RegisterRoutes wires buff, round and effective-STR endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc buffsvc.SVC) {
	r.Get("/api/buffs", render.Handle(ListActive(charID, svc)))
	r.Post("/api/buffs", render.Handle(Activate(charID, svc)))
	r.Patch("/api/buffs/{id}", render.Handle(Edit(charID, svc)))
	r.Delete("/api/buffs/{id}", render.Handle(Dismiss(charID, svc)))
	r.Post("/api/buffs/{id}/trigger", render.Handle(Trigger(charID, svc)))
	r.Get("/api/round", render.Handle(GetRound(charID, svc)))
	r.Post("/api/round/advance", render.Handle(AdvanceRound(charID, svc)))
	r.Get("/api/stats/str", render.Handle(GetEffectiveStr(charID, svc)))
}

// renderList writes the active list with each buff's applies flag.
func renderList(w http.ResponseWriter, r *http.Request, charID CharacterIDFunc, svc buffSvc) error {
	buffs, err := svc.ListStatus(r.Context(), charID(r))
	if err != nil {
		return err
	}
	render.OK(r.Context(), w, toStatusDTOs(buffs))
	return nil
}

// ListActive returns the current buff list.
func ListActive(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return renderList(w, r, charID, svc)
	}
}

// Activate adds a buff to the active list.
func Activate(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var dto BuffEffectDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if _, err := svc.Activate(r.Context(), charID(r), dto.toDomain()); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
	}
}

// BuffEditDTO is the PATCH /api/buffs/{id} body; omitted fields are unchanged.
type BuffEditDTO struct {
	Duration *int `json:"duration"`
	Charges  *int `json:"charges"`
}

// Edit corrects a buff's remaining rounds and/or charges.
func Edit(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var dto BuffEditDTO
		if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
			return render.BadRequestError("invalid request body")
		}
		edit := buffsvc.BuffEdit{Duration: dto.Duration, Charges: dto.Charges}
		if _, err := svc.Edit(r.Context(), charID(r), chi.URLParam(r, "id"), edit); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
	}
}

// Dismiss removes the buff with the given id.
func Dismiss(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := svc.Dismiss(r.Context(), charID(r), chi.URLParam(r, "id")); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
	}
}

// Trigger uses one charge of a charged buff (absorbed a hit).
func Trigger(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if _, err := svc.Trigger(r.Context(), charID(r), chi.URLParam(r, "id")); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
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

// StrengthDTO is GET /api/stats/str.
type StrengthDTO struct {
	Base      int `json:"base"`
	Effective int `json:"effective"`
}

// GetEffectiveStr returns the sheet STR and the STR with buffs.
func GetEffectiveStr(charID CharacterIDFunc, svc buffSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		s, err := svc.GetEffectiveStr(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, StrengthDTO{Base: s.Base, Effective: s.Effective})
		return nil
	}
}

// BuffEffectDTO is the wire shape for a single buff (snake_case effect
// fields). duration is rounds left (0: not timed), charges what is left to
// absorb (0: none). applies is set on list responses: false while the buff's
// min_str is not met by the effective STR.
type BuffEffectDTO struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Charges  int            `json:"charges"`
	Duration int            `json:"duration"`
	Effects  BuffEffectsDTO `json:"effects"`
	Applies  *bool          `json:"applies,omitempty"`
}

// BuffEffectsDTO is the modifier payload.
type BuffEffectsDTO struct {
	MinStr     *int     `json:"min_str,omitempty"`
	SpellsBuff *float64 `json:"spells_buff,omitempty"`
	WeaponBuff *int     `json:"weapon_buff,omitempty"`
	Multiplier *float64 `json:"multiplier,omitempty"`
	SaveBonus  *int     `json:"save_bonus,omitempty"`
	StrSet     *int     `json:"str_set,omitempty"`
	StrBonus   *int     `json:"str_bonus,omitempty"`
	StrMax     *int     `json:"str_max,omitempty"`
	AC         *int     `json:"ac,omitempty"`
	ToHit      *int     `json:"to_hit,omitempty"`
	Damage     *int     `json:"damage,omitempty"`
}

// RoundAdvanceDTO mirrors the previous response shape from RoundHandler.Advance.
type RoundAdvanceDTO struct {
	RoundNumber  int             `json:"roundNumber"`
	ActiveBuffs  []BuffEffectDTO `json:"activeBuffs"`
	ExpiredBuffs []BuffEffectDTO `json:"expiredBuffs"`
}

func (d BuffEffectDTO) toDomain() buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: d.ID, Name: d.Name, Charges: d.Charges, Duration: d.Duration,
		Effects: buffsvc.BuffEffects(d.Effects),
	}
}

func toDTO(b buffsvc.BuffEffect) BuffEffectDTO {
	return BuffEffectDTO{
		ID: b.ID, Name: b.Name, Charges: b.Charges, Duration: b.Duration,
		Effects: BuffEffectsDTO(b.Effects),
	}
}

func toDTOs(bs []buffsvc.BuffEffect) []BuffEffectDTO {
	out := make([]BuffEffectDTO, 0, len(bs))
	for _, b := range bs {
		out = append(out, toDTO(b))
	}
	return out
}

func toStatusDTOs(bs []buffsvc.BuffStatus) []BuffEffectDTO {
	out := make([]BuffEffectDTO, 0, len(bs))
	for _, b := range bs {
		d := toDTO(b.BuffEffect)
		applies := b.Applies
		d.Applies = &applies
		out = append(out, d)
	}
	return out
}
