// Package achdl exposes the character's armour class (ADR 0033 steps 10 to
// 13): GET /api/ac (the derived AC, its breakdown and the override), PUT
// /api/ac/override (set it, logged) and DELETE /api/ac/override (clear it,
// logged). Every change answers the AC as GET /api/ac does. The breakdown is
// codes and numbers; the web words it (ADR 0023).
package achdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

var _ Router = (*chi.Mux)(nil)

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type acSvc interface {
	Get(ctx context.Context, characterID int64) (*acsvc.AC, error)
	SetOverride(ctx context.Context, characterID int64, o acsvc.Override) (*acsvc.AC, error)
	ClearOverride(ctx context.Context, characterID int64) (*acsvc.AC, error)
}

// RegisterRoutes wires the AC endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc acsvc.SVC) {
	r.Get("/api/ac", render.Handle(Get(charID, svc)))
	r.Put("/api/ac/override", render.Handle(SetOverride(charID, svc)))
	r.Delete("/api/ac/override", render.Handle(ClearOverride(charID, svc)))
}

// RowDTO is one row of the breakdown: the term (base, armour_magic, shield,
// shield_magic, dexterity, protection, buffs), its source (itemId: a piece
// the web names; buff and spellId: a buff the web names; none: no source),
// its value (the base AC for base, a change of the AC number otherwise),
// missileValue (the shield against missiles, when it differs), dexterity
// (the score, on the dexterity row) and off (why a row shown does not add:
// magic_armour, duplicate, armour_worn, not_best).
type RowDTO struct {
	Term         string `json:"term"`
	ItemID       string `json:"itemId,omitempty"`
	Buff         string `json:"buff,omitempty"`
	SpellID      string `json:"spellId,omitempty"`
	Value        int    `json:"value"`
	MissileValue *int   `json:"missileValue,omitempty"`
	Dexterity    int    `json:"dexterity,omitempty"`
	Off          string `json:"off,omitempty"`
}

// OverrideDTO is the override: a number -10 to 10 and the label the player
// typed ("" for none). Also the body of PUT /api/ac/override.
type OverrideDTO struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

// ACDTO is GET /api/ac: the derived AC (ac; missile against missiles when a
// body shield makes it differ, else absent), floored (the -10 floor raised
// it), the rows in the order they are summed, and the override (null: none)
// which the AC shown is while it is set.
type ACDTO struct {
	AC       int          `json:"ac"`
	Missile  *int         `json:"missile,omitempty"`
	Floored  bool         `json:"floored"`
	Rows     []RowDTO     `json:"rows"`
	Override *OverrideDTO `json:"override"`
	// Warnings are the class warnings of the worn armour and shield (ADR
	// 0033 step 20), codes with the piece's id; empty for none.
	Warnings []WarningDTO `json:"warnings"`
}

// WarningDTO is one class warning: the code the web words and the piece it
// is about.
type WarningDTO struct {
	ItemID string `json:"itemId"`
	Code   string `json:"code"`
}

// ToDTO maps the AC to its wire shape.
func ToDTO(a *acsvc.AC) ACDTO {
	out := ACDTO{AC: a.Value, Missile: a.Missile, Floored: a.Floored, Rows: make([]RowDTO, 0, len(a.Rows)), Warnings: make([]WarningDTO, 0, len(a.Warnings))}
	for _, r := range a.Rows {
		out.Rows = append(out.Rows, RowDTO{
			Term: string(r.Term), ItemID: r.ItemID, Buff: r.Buff, SpellID: r.SpellID, Value: r.Value,
			MissileValue: r.MissileValue, Dexterity: r.Dexterity, Off: string(r.Off),
		})
	}
	for _, w := range a.Warnings {
		out.Warnings = append(out.Warnings, WarningDTO{ItemID: w.ItemID, Code: w.Code})
	}
	if a.Override != nil {
		out.Override = &OverrideDTO{Value: a.Override.Value, Label: a.Override.Label}
	}
	return out
}

// Get answers the character's AC.
func Get(charID CharacterIDFunc, svc acSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		a, err := svc.Get(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(a))
		return nil
	}
}

// SetOverride sets the override and answers the AC. A value out of -10..10
// or a too long label is a 400 naming the field (value, label).
func SetOverride(charID CharacterIDFunc, svc acSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req struct {
			Value *int   `json:"value"`
			Label string `json:"label"`
		}
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil || req.Value == nil {
			return render.BadRequestError("invalid request body")
		}
		a, err := svc.SetOverride(r.Context(), charID(r), acsvc.Override{Value: *req.Value, Label: req.Label})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(a))
		return nil
	}
}

// ClearOverride clears the override and answers the AC.
func ClearOverride(charID CharacterIDFunc, svc acSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		a, err := svc.ClearOverride(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(a))
		return nil
	}
}
