// Package spellprephdl exposes the daily spell preparation:
// GET /api/spells/preparation (spellbook + per-level limits) and
// PUT /api/spells/prepared (replace today's preparation).
package spellprephdl

import (
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires the preparation endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc prepsvc.SVC) {
	r.Get("/api/spells/preparation", render.Handle(GetInfo(charID, svc)))
	r.Put("/api/spells/prepared", render.Handle(Prepare(charID, svc)))
}

// SpellbookEntryDTO is one spell of the spellbook.
type SpellbookEntryDTO struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	// Origin is how the entry came into the spellbook (ADR 0030 step 20.5):
	// "learned", "given", "starting" or "level_grant"; absent for a priest's
	// sphere spell, which is in no book. GrantLevel is the realized level
	// that granted a level_grant entry (absent otherwise).
	Origin     string `json:"origin,omitempty"`
	GrantLevel int    `json:"grant_level,omitempty"`
	// What casting the spell asks of the player (from the spell effects
	// data): the splits of the extra slots it grants (the first is the
	// default), the type of the inventory item it spends, and whether it
	// rolls a to-hit (then a target AC is required).
	SlotSplits      [][]int `json:"slot_splits,omitempty"`
	ConsumeItemType string  `json:"consume_item_type,omitempty"`
	AttackRoll      bool    `json:"attack_roll,omitempty"`
	// Modes are the cast-time modes the cast picks from, with the damage a
	// cast in each rolls (the first is the default; absent: none).
	Modes []spelleffectsvc.ModeAtWire `json:"modes,omitempty"`
	// Damage is what a cast rolls at the character's level, in the shape
	// of the spell lookup's (absent: the cast rolls no damage).
	Damage *spelleffectsvc.DamageAtWire `json:"damage,omitempty"`
	// Exposure is the damage of one round of exposure to the spell's area
	// at the character's level (absent: none; ADR 0008 step 16).
	Exposure *spelleffectsvc.ExposureAtWire `json:"exposure,omitempty"`
	// CastTarget is the choice of target a buff's cast offers (absent: none,
	// the cast lands where the spell does): where a cast that names no target
	// lands, and the one other target the caster may name (onSelf on the
	// cast). Values: "self", "other".
	CastTarget *CastTargetDTO `json:"cast_target,omitempty"`
	// Items are the inventory items the spell can spend now (consuming
	// spells only); the cast takes the one chosen, or the only one.
	Items []SpendableItemDTO `json:"items,omitempty"`
}

// CastTargetDTO is the target choice of a cast: the default and its alternate.
type CastTargetDTO struct {
	Default   string `json:"default"`
	Alternate string `json:"alternate"`
}

// castTargetToDTO maps the target choice to the wire (nil: none).
func castTargetToDTO(c *spelleffectsvc.TargetChoice) *CastTargetDTO {
	if c == nil {
		return nil
	}
	return &CastTargetDTO{Default: string(c.Default), Alternate: string(c.Alternate)}
}

// SpendableItemDTO is an inventory item a spell can spend.
type SpendableItemDTO struct {
	ID string `json:"id"`
	// Name is a language map (ADR 0023 step 9).
	Name  localized.Text `json:"name"`
	Count int            `json:"count"`
}

// LevelLimitDTO is the preparation limit of one spell level.
type LevelLimitDTO struct {
	Level int `json:"level"`
	Max   int `json:"max"`
}

// InfoDTO is the wire shape of GET /api/spells/preparation.
type InfoDTO struct {
	Spellbook []SpellbookEntryDTO `json:"spellbook"`
	Limits    []LevelLimitDTO     `json:"limits"`
}

// PrepareRequest is the body of PUT /api/spells/prepared.
type PrepareRequest struct {
	Spells []string `json:"spells"`
}

// GetInfo returns the spellbook and the limits.
func GetInfo(charID CharacterIDFunc, svc prepsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		info, err := svc.Info(r.Context(), charID(r))
		if err != nil {
			return err
		}
		out := InfoDTO{Spellbook: []SpellbookEntryDTO{}, Limits: []LevelLimitDTO{}}
		for _, e := range info.Spellbook {
			dto := SpellbookEntryDTO{
				ID: e.ID, Name: e.Name, Level: e.Level, Origin: string(e.Origin), GrantLevel: e.GrantLevel,
				SlotSplits: e.Cast.SlotSplits, ConsumeItemType: e.Cast.ConsumeItemType, AttackRoll: e.Cast.AttackRoll,
				Damage: e.Damage.Wire(), Modes: spelleffectsvc.ModesWire(e.Cast.Modes),
				Exposure: e.Exposure.Wire(), CastTarget: castTargetToDTO(e.Cast.Target),
			}
			for _, it := range e.Items {
				dto.Items = append(dto.Items, SpendableItemDTO{ID: it.ID, Name: it.Name, Count: it.Count})
			}
			out.Spellbook = append(out.Spellbook, dto)
		}
		for _, l := range info.Limits {
			out.Limits = append(out.Limits, LevelLimitDTO{Level: l.Level, Max: l.Max})
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// Prepare replaces today's preparation and answers with the new slot list.
func Prepare(charID CharacterIDFunc, svc prepsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req PrepareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		slots, err := svc.Prepare(r.Context(), charID(r), req.Spells)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, wire.SlotsToDTO(slots))
		return nil
	}
}
