// Package spellhdl exposes /api/spells.
package spellhdl

import (
	"context"
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/go-chi/chi/v5"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type spellSvc interface {
	GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	Cast(ctx context.Context, characterID int64, slotIndex int) ([]spellsvc.SpellSlot, error)
	ResetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	Search(ctx context.Context, query, class string) ([]spellsvc.Spell, error)
}

// RegisterRoutes wires spell endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc spellsvc.SVC) {
	r.Get("/api/spells/slots", render.Handle(GetSlots(charID, svc)))
	r.Get("/api/spells", render.Handle(Search(svc)))
	r.Post("/api/spells/{id}/cast", render.Handle(Cast(charID, svc)))
	r.Post("/api/spells/reset", render.Handle(Reset(charID, svc)))
}

// GetSlots returns the slot list.
func GetSlots(charID CharacterIDFunc, svc spellSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		slots, err := svc.GetSlots(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, slotsToDTO(slots))
		return nil
	}
}

// Cast marks slot {id} used.
func Cast(charID CharacterIDFunc, svc spellSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		idStr := chi.URLParam(r, "id")
		idx, err := strconv.Atoi(idStr)
		if err != nil {
			return render.BadRequestError("invalid slot index")
		}
		slots, err := svc.Cast(r.Context(), charID(r), idx)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, slotsToDTO(slots))
		return nil
	}
}

// Reset marks every slot unused (long rest).
func Reset(charID CharacterIDFunc, svc spellSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		slots, err := svc.ResetSlots(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, slotsToDTO(slots))
		return nil
	}
}

// Search runs the catalog query.
func Search(svc spellSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		spells, err := svc.Search(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("class"))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, spellsToDTO(spells))
		return nil
	}
}

// SpellSlotDTO matches the previous wire shape.
type SpellSlotDTO struct {
	Level             int    `json:"level"`
	PreparedSpellName string `json:"prepared_spell_name"`
	Used              bool   `json:"used"`
}

// SpellDTO matches the previous wire shape.
type SpellDTO struct {
	Name         string `json:"name"`
	Class        string `json:"class"`
	Level        int    `json:"level"`
	School       string `json:"school"`
	Sphere       string `json:"sphere"`
	CastingTime  string `json:"casting_time"`
	Range        string `json:"range"`
	Duration     string `json:"duration"`
	AreaOfEffect string `json:"area_of_effect"`
	Components   string `json:"components"`
	SavingThrow  string `json:"saving_throw"`
	Description  string `json:"description"`
}

func slotsToDTO(in []spellsvc.SpellSlot) []SpellSlotDTO {
	out := make([]SpellSlotDTO, 0, len(in))
	for _, s := range in {
		out = append(out, SpellSlotDTO{Level: s.Level, PreparedSpellName: s.PreparedSpellName, Used: s.Used})
	}
	return out
}

func spellsToDTO(in []spellsvc.Spell) []SpellDTO {
	out := make([]SpellDTO, 0, len(in))
	for _, s := range in {
		out = append(out, SpellDTO{
			Name: s.Name, Class: s.Class, Level: s.Level, School: s.School, Sphere: s.Sphere,
			CastingTime: s.CastingTime, Range: s.Range, Duration: s.Duration,
			AreaOfEffect: s.AreaOfEffect, Components: s.Components, SavingThrow: s.SavingThrow,
			Description: s.Description,
		})
	}
	return out
}
