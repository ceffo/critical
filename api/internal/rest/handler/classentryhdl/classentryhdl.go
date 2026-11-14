// Package classentryhdl exposes POST /api/character/class-entry/class, the current character's
// class entry switched to another class of its group (ADR 0050 step 15).
package classentryhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/classentrysvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type switcher interface {
	Switch(ctx context.Context, characterID int64, entry int, class string) (*classentrysvc.Result, error)
}

// RegisterRoutes wires the entry switch onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc classentrysvc.SVC) {
	r.Post("/api/character/class-entry/class", render.Handle(Switch(charID, svc)))
}

// Body is the body of POST /api/character/class-entry/class: the entry's index in the
// character's class entries and the class it takes. Both are required (an absent entry is not
// entry 0).
type Body struct {
	Entry *int   `json:"entry"`
	Class string `json:"class"`
}

// ResultDTO is what the switch did.
type ResultDTO struct {
	Entry int    `json:"entry"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// Switch is POST /api/character/class-entry/class.
func Switch(charID CharacterIDFunc, svc switcher) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b Body
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		if b.Entry == nil {
			return render.BadRequestError("entry is required")
		}
		if b.Class == "" {
			return render.BadRequestError("class is required")
		}
		res, err := svc.Switch(r.Context(), charID(r), *b.Entry, b.Class)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ResultDTO{Entry: res.Entry, From: res.From, To: res.To})
		return nil
	}
}
