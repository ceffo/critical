// Package spellhdl exposes /api/spells.
package spellhdl

import (
	"context"
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/render"
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
	ResetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	Search(ctx context.Context, query, class string) ([]spellsvc.Spell, error)
}

type characterGetter interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// RegisterRoutes wires spell endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc spellsvc.SVC, chars characterGetter) {
	r.Get("/api/spells/slots", render.Handle(GetSlots(charID, svc)))
	r.Get("/api/spells", render.Handle(Search(charID, svc, chars)))
}

// GetSlots returns the slot list.
func GetSlots(charID CharacterIDFunc, svc spellSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		slots, err := svc.GetSlots(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, SlotsToDTO(slots))
		return nil
	}
}

// Search runs the catalog query. The optional caster_level query parameter
// sets the level the computed values are for; it defaults to the character's.
func Search(charID CharacterIDFunc, svc spellSvc, chars characterGetter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		spells, err := svc.Search(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("class"))
		if err != nil {
			return err
		}
		level, err := casterLevel(r, charID, chars)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, spellsToDTO(spells, level))
		return nil
	}
}

func casterLevel(r *http.Request, charID CharacterIDFunc, chars characterGetter) (int, error) {
	if raw := r.URL.Query().Get("caster_level"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return 0, render.BadRequestError("caster_level must be a positive integer")
		}
		return n, nil
	}
	// No character (or a failed lookup) must not break the catalog: the
	// spells are returned without computed values.
	c, err := chars.GetCharacter(r.Context(), charID(r))
	if err != nil || c == nil {
		return 0, nil //nolint:nilerr // deliberate fallback, see above
	}
	return c.Level, nil
}

// SpellSlotDTO matches the previous wire shape.
type SpellSlotDTO struct {
	Level             int    `json:"level"`
	PreparedSpellID   string `json:"prepared_spell_id"`
	PreparedSpellName string `json:"prepared_spell_name"`
	Used              bool   `json:"used"`
}

// SpellDTO is the wire shape of a spell. The six stat fields keep their raw
// PHB text; Stats is their typed form and Values the computed values at
// CasterLevel (domain types carry the JSON tags).
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

	Stats       spellstats.Stats   `json:"stats"`
	CasterLevel int                `json:"caster_level,omitempty"`
	Values      *spellstats.Values `json:"values,omitempty"`
}

// SlotsToDTO maps slots to their wire shape.
func SlotsToDTO(in []spellsvc.SpellSlot) []SpellSlotDTO {
	out := make([]SpellSlotDTO, 0, len(in))
	for _, s := range in {
		out = append(out, SpellSlotDTO{Level: s.Level, PreparedSpellID: s.SpellID, PreparedSpellName: s.PreparedSpellName, Used: s.Used})
	}
	return out
}

func spellsToDTO(in []spellsvc.Spell, casterLevel int) []SpellDTO {
	out := make([]SpellDTO, 0, len(in))
	for _, s := range in {
		out = append(out, SpellToDTO(s, casterLevel))
	}
	return out
}

// SpellToDTO maps one spell to its wire shape, with values at casterLevel.
func SpellToDTO(s spellsvc.Spell, casterLevel int) SpellDTO {
	dto := SpellDTO{
		Name: s.Name, Class: s.Class, Level: s.Level, School: s.School, Sphere: s.Sphere,
		CastingTime: s.CastingTime, Range: s.Range, Duration: s.Duration,
		AreaOfEffect: s.AreaOfEffect, Components: s.Components, SavingThrow: s.SavingThrow,
		Description: s.Description,
		Stats:       s.Stats, CasterLevel: casterLevel,
	}
	if casterLevel > 0 {
		v := s.Stats.At(casterLevel)
		dto.Values = &v
	}
	return dto
}
