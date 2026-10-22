// Package rulesethdl exposes the ruleset (ADR 0041 step 8): GET /api/ruleset
// and its audit for any logged-in user, and the admin-only edits, one request
// per house rule under /api/ruleset/. The wire carries structured data only;
// the UI words the proficiencies and the rules.
package rulesethdl

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/render"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
}

var _ Router = (*chi.Mux)(nil)

type rulesetSvc interface {
	Get(ctx context.Context) (ruleset.Ruleset, error)
	Changes(ctx context.Context) ([]ruleset.Change, error)
	SetCritTable(ctx context.Context, by uuid.UUID, rows []ruleset.CritRow) (ruleset.CritTable, error)
	SetRacialLift(ctx context.Context, by uuid.UUID, lift ruleset.RacialLift) (ruleset.RacialLift, error)
	SetWeaponLift(ctx context.Context, by uuid.UUID, lift ruleset.WeaponLift) (ruleset.WeaponLift, error)
	SetSuccessEffects(ctx context.Context, by uuid.UUID, effects []ruleset.SuccessEffect) ([]ruleset.SuccessEffect, error)
}

// RegisterSharedRoutes wires the reading of the ruleset, which no character
// owns, onto r (every logged-in user reads it).
func RegisterSharedRoutes(r Router, svc rulesetSvc) {
	r.Get("/api/ruleset", render.Handle(Get(svc)))
	r.Get("/api/ruleset/changes", render.Handle(ListChanges(svc)))
}

// RegisterAdminRoutes wires the edits onto r, which the caller guards with
// mw.AdminOnly: the ruleset is shared by every user, so only an admin edits
// it (ADR 0041 step 7).
func RegisterAdminRoutes(r Router, svc rulesetSvc) {
	r.Put("/api/ruleset/crit-table", render.Handle(PutCritTable(svc)))
	r.Put("/api/ruleset/racial-lift", render.Handle(PutRacialLift(svc)))
	r.Put("/api/ruleset/weapon-lift", render.Handle(PutWeaponLift(svc)))
	r.Put("/api/ruleset/success-effects", render.Handle(PutSuccessEffects(svc)))
}

// CritRowDTO is one inclusive D100 range of the crit table.
type CritRowDTO struct {
	From       int     `json:"from"`
	To         int     `json:"to"`
	Multiplier float64 `json:"multiplier"`
}

// CritTableDTO is the wire shape of the crit table. Placeholder is true while
// it is still the unedited default.
type CritTableDTO struct {
	Rows        []CritRowDTO `json:"rows"`
	Placeholder bool         `json:"placeholder"`
}

// RacialLiftDTO is the two racial lift switches.
type RacialLiftDTO struct {
	MulticlassAnyRace bool `json:"multiclassAnyRace"`
	DualclassAnyRace  bool `json:"dualclassAnyRace"`
}

// WeaponLiftDTO is the multi-class weapon lift switch (ADR 0041 step 2.5).
type WeaponLiftDTO struct {
	MulticlassAnyWeapon bool `json:"multiclassAnyWeapon"`
}

// SuccessEffectDTO is the buff a successful check of a proficiency puts on:
// the AC change (descending AC) and the rounds it lasts. The UI words the
// proficiency from its id.
type SuccessEffectDTO struct {
	ProficiencyID string `json:"proficiencyId"`
	AC            int    `json:"ac"`
	Rounds        int    `json:"rounds"`
}

// RulesetDTO is GET /api/ruleset.
type RulesetDTO struct {
	ID               int64              `json:"id"`
	Name             string             `json:"name"`
	CatalogueVersion string             `json:"catalogueVersion"`
	CritTable        CritTableDTO       `json:"critTable"`
	RacialLift       RacialLiftDTO      `json:"racialLift"`
	WeaponLift       WeaponLiftDTO      `json:"weaponLift"`
	LevelLimits      string             `json:"levelLimits"`
	SuccessEffects   []SuccessEffectDTO `json:"successEffects"`
}

// ChangeDTO is one line of the audit: who edited which rule when, with the
// rule's value before and after as the documents the matching PUT answers.
type ChangeDTO struct {
	ID     int64           `json:"id"`
	At     time.Time       `json:"at"`
	By     string          `json:"by"`
	Rule   string          `json:"rule"`
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}

// ChangesDTO is GET /api/ruleset/changes, newest first.
type ChangesDTO struct {
	Changes []ChangeDTO `json:"changes"`
}

func toCritTableDTO(t ruleset.CritTable) CritTableDTO {
	rows := make([]CritRowDTO, 0, len(t.Rows))
	for _, r := range t.Rows {
		rows = append(rows, CritRowDTO{From: r.From, To: r.To, Multiplier: r.Multiplier})
	}
	return CritTableDTO{Rows: rows, Placeholder: t.Placeholder}
}

func toEffectDTOs(effects []ruleset.SuccessEffect) []SuccessEffectDTO {
	out := make([]SuccessEffectDTO, 0, len(effects))
	for _, e := range effects {
		out = append(out, SuccessEffectDTO{ProficiencyID: e.ProficiencyID, AC: e.AC, Rounds: e.Rounds})
	}
	return out
}

// Get returns the default ruleset.
func Get(svc rulesetSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		rs, err := svc.Get(r.Context())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RulesetDTO{
			ID: rs.ID, Name: rs.Name, CatalogueVersion: rs.CatalogueVersion,
			CritTable:      toCritTableDTO(rs.Crit),
			RacialLift:     RacialLiftDTO{MulticlassAnyRace: rs.RacialLift.MulticlassAnyRace, DualclassAnyRace: rs.RacialLift.DualclassAnyRace},
			WeaponLift:     WeaponLiftDTO{MulticlassAnyWeapon: rs.WeaponLift.MulticlassAnyWeapon},
			LevelLimits:    string(rs.LevelLimits),
			SuccessEffects: toEffectDTOs(rs.SuccessEffects),
		})
		return nil
	}
}

// ListChanges returns the audit of the ruleset's edits, newest first.
func ListChanges(svc rulesetSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		changes, err := svc.Changes(r.Context())
		if err != nil {
			return err
		}
		out := ChangesDTO{Changes: make([]ChangeDTO, 0, len(changes))}
		for _, c := range changes {
			out.Changes = append(out.Changes, ChangeDTO{
				ID: c.ID, At: c.At, By: c.By.String(), Rule: c.Rule,
				Before: json.RawMessage(c.Before), After: json.RawMessage(c.After),
			})
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// editor is the request's user, who an edit is audited to.
func editor(r *http.Request) (uuid.UUID, error) {
	p := mw.PrincipalFrom(r.Context())
	if p == nil {
		return uuid.Nil, authsvc.ErrUnauthenticated
	}
	return p.User.ID, nil
}

// PutCritTable replaces the whole crit table; an invalid table is a 400 with
// {"error"} naming the bad row and the stored table is left alone.
func PutCritTable(svc rulesetSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		by, err := editor(r)
		if err != nil {
			return err
		}
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
		rows := make([]ruleset.CritRow, 0, len(req.Rows))
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
			rows = append(rows, ruleset.CritRow{From: from, To: to, Multiplier: mult})
		}
		t, err := svc.SetCritTable(r.Context(), by, rows)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toCritTableDTO(t))
		return nil
	}
}

// PutRacialLift stores the two racial lift switches; both are required.
func PutRacialLift(svc rulesetSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		by, err := editor(r)
		if err != nil {
			return err
		}
		var req struct {
			MulticlassAnyRace *bool `json:"multiclassAnyRace"`
			DualclassAnyRace  *bool `json:"dualclassAnyRace"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if req.MulticlassAnyRace == nil || req.DualclassAnyRace == nil {
			return render.BadRequestError("multiclassAnyRace and dualclassAnyRace are required")
		}
		lift, err := svc.SetRacialLift(r.Context(), by, ruleset.RacialLift{
			MulticlassAnyRace: *req.MulticlassAnyRace, DualclassAnyRace: *req.DualclassAnyRace,
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RacialLiftDTO{MulticlassAnyRace: lift.MulticlassAnyRace, DualclassAnyRace: lift.DualclassAnyRace})
		return nil
	}
}

// PutWeaponLift stores the multi-class weapon lift switch; it is required.
func PutWeaponLift(svc rulesetSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		by, err := editor(r)
		if err != nil {
			return err
		}
		var req struct {
			MulticlassAnyWeapon *bool `json:"multiclassAnyWeapon"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if req.MulticlassAnyWeapon == nil {
			return render.BadRequestError("multiclassAnyWeapon is required")
		}
		lift, err := svc.SetWeaponLift(r.Context(), by, ruleset.WeaponLift{MulticlassAnyWeapon: *req.MulticlassAnyWeapon})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, WeaponLiftDTO{MulticlassAnyWeapon: lift.MulticlassAnyWeapon})
		return nil
	}
}

// PutSuccessEffects replaces the success effects; an unknown proficiency or
// an effect of no rounds is a 400 and the stored effects stay.
func PutSuccessEffects(svc rulesetSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		by, err := editor(r)
		if err != nil {
			return err
		}
		var req struct {
			Effects *[]SuccessEffectDTO `json:"effects"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if req.Effects == nil {
			return render.BadRequestError("effects is required (an empty list removes them all)")
		}
		in := make([]ruleset.SuccessEffect, 0, len(*req.Effects))
		for _, e := range *req.Effects {
			in = append(in, ruleset.SuccessEffect{ProficiencyID: e.ProficiencyID, AC: e.AC, Rounds: e.Rounds})
		}
		out, err := svc.SetSuccessEffects(r.Context(), by, in)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, map[string][]SuccessEffectDTO{"effects": toEffectDTOs(out)})
		return nil
	}
}
