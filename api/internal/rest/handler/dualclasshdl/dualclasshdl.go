// Package dualclasshdl exposes POST /api/character/dual-class, the current
// character taking a new class (ADR 0042 step 3).
package dualclasshdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/dualclasssvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type switcher interface {
	Switch(ctx context.Context, characterID int64, newClass string) (*dualclasssvc.Result, error)
}

// RegisterRoutes wires the class switch onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc dualclasssvc.SVC) {
	r.Post("/api/character/dual-class", render.Handle(Switch(charID, svc)))
}

// Body is the body of POST /api/character/dual-class: the class taken.
type Body struct {
	Class string `json:"class"`
}

// ResultDTO is what the switch did: the class left with the level and XP it
// is frozen at, and the class taken (at level 1).
type ResultDTO struct {
	From        string `json:"from"`
	To          string `json:"to"`
	LeftAtLevel int    `json:"leftAtLevel"`
	XP          int    `json:"xp"`
}

// Switch is POST /api/character/dual-class.
func Switch(charID CharacterIDFunc, svc switcher) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b Body
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		if b.Class == "" {
			return render.BadRequestError("class is required")
		}
		res, err := svc.Switch(r.Context(), charID(r), b.Class)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ResultDTO{From: res.From, To: res.To, LeftAtLevel: res.LeftAtLevel, XP: res.XP})
		return nil
	}
}
