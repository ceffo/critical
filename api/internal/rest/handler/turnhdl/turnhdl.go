// Package turnhdl serves turning undead (ADR 0030 step 16): the current
// character's cleric names an undead type, the server rolls.
package turnhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/turnsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the slice of chi the handler registers on.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc reads the current character's id from the request.
type CharacterIDFunc func(r *http.Request) int64

type turner interface {
	Turn(ctx context.Context, characterID int64, undead string) (*turnsvc.Result, error)
}

// RegisterRoutes registers the turning endpoint.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc turnsvc.SVC) {
	r.Post("/api/character/turn-undead", render.Handle(Turn(charID, svc)))
}

// TurnBody is the request: the undead type's code (a PHB Table 61 row).
type TurnBody struct {
	Undead string `json:"undead"`
}

// ResultDTO is the turn's full breakdown (step 16.5): the row, the cleric's
// level, the cell (`target` the d20 number or `outcome` turn, destroy,
// destroy_more), the d20 (0: none), the result and its `effect` (turn or
// destroy), the 2d6 `affected` and the 2d4 `more` of a D* cell.
type ResultDTO struct {
	EntryID     int64  `json:"entryId"`
	Undead      string `json:"undead"`
	HDMin       int    `json:"hdMin"`
	HDMax       int    `json:"hdMax"`
	ClericLevel int    `json:"clericLevel"`
	Target      int    `json:"target,omitempty"`
	Outcome     string `json:"outcome,omitempty"`
	D20         int    `json:"d20,omitempty"`
	Success     bool   `json:"success"`
	Effect      string `json:"effect,omitempty"`
	Affected    []int  `json:"affected"`
	More        []int  `json:"more"`
}

// Turn handles POST /api/character/turn-undead.
func Turn(charID CharacterIDFunc, svc turner) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b TurnBody
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		if b.Undead == "" {
			return render.BadRequestError("undead is required")
		}
		res, err := svc.Turn(r.Context(), charID(r), b.Undead)
		if err != nil {
			return err
		}
		out := ResultDTO{
			EntryID: res.EntryID, Undead: res.Undead, HDMin: res.HDMin, HDMax: res.HDMax, ClericLevel: res.ClericLevel,
			Target: res.Cell.Roll, Outcome: res.Cell.Outcome, D20: res.D20, Success: res.Success, Effect: res.Effect,
			Affected: append([]int{}, res.Affected...), More: append([]int{}, res.More...),
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}
