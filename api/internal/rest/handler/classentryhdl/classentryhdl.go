// Package classentryhdl exposes POST /api/character/class-entry/class, the current character's
// class entry switched to another class of its group (ADR 0050 step 15).
package classentryhdl

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/classentrysvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type switcher interface {
	Switch(ctx context.Context, characterID int64, entry int, class string) (*classentrysvc.Result, error)
}

type lister interface {
	Targets(ctx context.Context, characterID int64, entry int) (*classentrysvc.TargetList, error)
}

// RegisterRoutes wires the entry switch onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc classentrysvc.SVC) {
	r.Post("/api/character/class-entry/class", render.Handle(Switch(charID, svc)))
	r.Get("/api/character/class-entry/targets", render.Handle(Targets(charID, svc)))
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

// TargetsDTO is GET /api/character/class-entry/targets?entry=N: the classes the entry could become
// (ADR 0050 step 15.6), what the school picker shows. Eligibility is the server's: the web only
// words it.
type TargetsDTO struct {
	Entry   int         `json:"entry"`
	From    string      `json:"from"`
	Targets []TargetDTO `json:"targets"`
}

// TargetDTO is one class an entry could become.
type TargetDTO struct {
	Class string `json:"class"`
	// Schools and OpposedSchools are school codes, empty for a mage.
	Schools        []string     `json:"schools"`
	OpposedSchools []string     `json:"opposedSchools"`
	Minimums       []MinimumDTO `json:"minimums"`
	// RaceWarning is a race outside PHB Table 22 for the specialist: a warning, never a refusal.
	RaceWarning bool `json:"raceWarning"`
	// Refusal is the code Switch would answer (class_switch_minimums_not_met, ...), empty when allowed.
	Refusal string `json:"refusal"`
}

// MinimumDTO is an ability minimum against the sheet's score.
type MinimumDTO struct {
	Ability string `json:"ability"`
	Min     int    `json:"min"`
	Score   int    `json:"score"`
	Met     bool   `json:"met"`
}

// Targets is GET /api/character/class-entry/targets.
func Targets(charID CharacterIDFunc, svc lister) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		entry, err := strconv.Atoi(r.URL.Query().Get("entry"))
		if err != nil {
			return render.BadRequestError("entry is required")
		}
		res, err := svc.Targets(r.Context(), charID(r), entry)
		if err != nil {
			return err
		}
		out := TargetsDTO{Entry: res.Entry, From: res.From, Targets: make([]TargetDTO, 0, len(res.Targets))}
		for _, t := range res.Targets {
			d := TargetDTO{
				Class: t.Class, Schools: t.Schools, OpposedSchools: t.OpposedSchools, RaceWarning: t.RaceWarning,
				Minimums: make([]MinimumDTO, 0, len(t.Minimums)),
			}
			if t.Refusal != nil {
				d.Refusal = render.Code(t.Refusal)
			}
			for _, m := range t.Minimums {
				d.Minimums = append(d.Minimums, MinimumDTO{Ability: m.Ability, Min: m.Min, Score: m.Score, Met: m.Met})
			}
			out.Targets = append(out.Targets, d)
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}
