package skillhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// picksSvc is what the proficiency editor's endpoints need (ADR 0037 steps 12 and 18).
type picksSvc interface {
	SetProficiencies(ctx context.Context, characterID int64, e skillsvc.Edit) (*skillsvc.Proficiencies, error)
	PreviewProficiencies(ctx context.Context, characterID int64, e skillsvc.Edit) (*skillsvc.Proficiencies, error)
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

func proficienciesDTO(p *skillsvc.Proficiencies) ProficienciesDTO {
	return ProficienciesDTO{ProficiencySlots: wire.ProficiencySlotsToDTO(p.Slots), Warnings: wire.SkillWarningsDTO(p.Warnings)}
}

// proficienciesRequest is the body {nonweapon: [{id, slots}], weapon: [{type, slots}], languages:
// [code]} (ADR 0037 steps 12 and 18): each pool optional, at least one; an absent (or null) one is
// left as stored, an empty list removes its picks (the languages but the native one, which the
// server keeps).
type proficienciesRequest struct {
	wire.PicksDTO
	Languages []string `json:"languages"`
}

// decodeEdit reads the body; at least one pool is required.
func decodeEdit(r *http.Request) (skillsvc.Edit, error) {
	var req proficienciesRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || (req.Nonweapon == nil && req.Weapon == nil && req.Languages == nil) {
		return skillsvc.Edit{}, render.BadRequestError("invalid request body")
	}
	return skillsvc.Edit{Picks: req.Picks(), Languages: req.Languages}, nil
}

// SetProficiencies is PUT /api/proficiencies (ADR 0037 steps 7, 12, 13 and 18): the whole list of
// the non-weapon picks, the weapon picks, the languages or any of them, saved as one undoable
// move. A refused pick is a 400 naming the field (nonweapon.<id>: unknown, duplicate,
// out_of_range, not_allowed; weapon.<type>: unknown, duplicate, out_of_range,
// weapon_not_allowed; languages: empty, unknown, duplicate); a save that grows an overspend a 409
// over_budget (field nonweapon, weapon or languages, spent, limit); nothing is written then.
func SetProficiencies(charID CharacterIDFunc, svc picksSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		e, err := decodeEdit(r)
		if err != nil {
			return err
		}
		p, err := svc.SetProficiencies(r.Context(), charID(r), e)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, proficienciesDTO(p))
		return nil
	}
}

// PreviewProficiencies is POST /api/proficiencies/preview: the same body and answer as the PUT,
// with the same refusals, writing nothing (the editor's live preview).
func PreviewProficiencies(charID CharacterIDFunc, svc picksSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		e, err := decodeEdit(r)
		if err != nil {
			return err
		}
		p, err := svc.PreviewProficiencies(r.Context(), charID(r), e)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, proficienciesDTO(p))
		return nil
	}
}
