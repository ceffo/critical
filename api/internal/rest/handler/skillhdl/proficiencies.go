package skillhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// picksSvc is what the proficiency editor's endpoints need (ADR 0037 step 12).
type picksSvc interface {
	SetProficiencies(ctx context.Context, characterID int64, picks charactersvc.Picks) (*charactersvc.Character, error)
	PreviewProficiencies(ctx context.Context, characterID int64, picks charactersvc.Picks) (*charactersvc.Character, error)
}

// ProficienciesDTO is the answer of PUT /api/proficiencies and POST /api/proficiencies/preview:
// `proficiencySlots` the pools with their terms and each pick's cost, extra slots and check
// target (as GET /api/character's derived block names them; null without the catalogue's Table
// 34), `warnings` the slot warnings (ADR 0037 step 11: slots_over_budget, slots_to_spend,
// proficiency_unknown), never null.
type ProficienciesDTO struct {
	ProficiencySlots *wire.ProficiencySlotsDTO `json:"proficiencySlots"`
	Warnings         []wire.WarningDTO         `json:"warnings"`
}

func proficienciesDTO(c *charactersvc.Character) ProficienciesDTO {
	var ws []charactersvc.Warning
	if c.Derived.Proficiencies != nil {
		ws = charactersvc.SlotWarnings(*c.Derived.Proficiencies)
	}
	return ProficienciesDTO{ProficiencySlots: wire.ProficiencySlotsToDTO(c.Derived.Proficiencies), Warnings: wire.WarningsDTO(ws)}
}

// decodePicks reads the body {nonweapon: [{id, slots}]}; `nonweapon` is required (an empty list
// removes every pick).
func decodePicks(r *http.Request) (charactersvc.Picks, error) {
	var req wire.PicksDTO
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Nonweapon == nil {
		return charactersvc.Picks{}, render.BadRequestError("invalid request body")
	}
	return req.Picks(), nil
}

// SetProficiencies is PUT /api/proficiencies (ADR 0037 steps 12 and 13): the whole list of
// non-weapon picks, saved as one undoable move. A refused pick is a 400 naming the field
// (nonweapon.<id>: unknown, duplicate, out_of_range, not_allowed); a save that grows an
// overspend a 409 over_budget (field nonweapon, spent, limit); nothing is written then.
func SetProficiencies(charID CharacterIDFunc, svc picksSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		picks, err := decodePicks(r)
		if err != nil {
			return err
		}
		c, err := svc.SetProficiencies(r.Context(), charID(r), picks)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, proficienciesDTO(c))
		return nil
	}
}

// PreviewProficiencies is POST /api/proficiencies/preview: the same body and answer as the PUT,
// with the same refusals, writing nothing (the editor's live preview).
func PreviewProficiencies(charID CharacterIDFunc, svc picksSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		picks, err := decodePicks(r)
		if err != nil {
			return err
		}
		c, err := svc.PreviewProficiencies(r.Context(), charID(r), picks)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, proficienciesDTO(c))
		return nil
	}
}
