// Package psphdl exposes the PSP pool's hand edit (ADR 0043 step 5), as hphdl
// does for HP, the recovery over hours of an activity (step 6) and the known
// powers (step 9) over HTTP. The handler depends on a narrow interface
// (clean_architecture.md §7.3) and translates between the HTTP DTOs and the
// domain types.
package psphdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the small subset of chi.Router we need for registration.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
}

// changeAppender is the one method the endpoint calls; actionlogsvc.SVC
// satisfies it.
type changeAppender interface {
	AppendPSPChange(ctx context.Context, characterID int64, delta int, source string) (actionlog.PSPState, error)
}

// recoverer is the recovery's one method; psionicsvc.SVC satisfies it.
type recoverer interface {
	Recover(ctx context.Context, characterID int64, hours float64, activity string) (actionlog.PSPState, error)
}

// knownSetter is the known powers' one method; psionicsvc.SVC satisfies it.
type knownSetter interface {
	SetKnownPowers(ctx context.Context, characterID int64, picks []psionics.Pick) (*charactersvc.PowerKnowledge, error)
}

// CharacterIDFunc resolves the active character ID from the request context.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires the PSP endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc changeAppender, rec recoverer, known knownSetter) {
	r.Post("/api/psp/event", render.Handle(AppendChange(charID, svc)))
	r.Post("/api/psp/recover", render.Handle(Recover(charID, rec)))
	r.Put("/api/psionics/picks", render.Handle(SetKnownPowers(charID, known)))
}

type picksBody struct {
	Picks []picksPickBody `json:"picks"`
}

type picksPickBody struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// SetKnownPowers saves the whole list of picks (ADR 0043 step 9) and answers
// the known powers as saved, in the derived character's `knownPowers` shape.
func SetKnownPowers(charID CharacterIDFunc, svc knownSetter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body picksBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		picks := make([]psionics.Pick, len(body.Picks))
		for i, p := range body.Picks {
			picks[i] = psionics.Pick(p)
		}
		k, err := svc.SetKnownPowers(r.Context(), charID(r), picks)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, knownPowersDTO(*k))
		return nil
	}
}

type recoverBody struct {
	Hours    float64 `json:"hours"`
	Activity string  `json:"activity"`
}

// Recover logs a psp_recovery entry: hours of one of Table 6's activities.
// It answers the pool after the recovery.
func Recover(charID CharacterIDFunc, svc recoverer) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body recoverBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		state, err := svc.Recover(r.Context(), charID(r), body.Hours, body.Activity)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, StateDTO{Current: state.Current, Max: state.Max})
		return nil
	}
}

type changeBody struct {
	Delta  int    `json:"delta"`
	Source string `json:"source"`
}

// AppendChange logs a psp_change entry: a signed delta, bounded by the pool,
// and an optional source. It answers the pool after the change.
func AppendChange(charID CharacterIDFunc, svc changeAppender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body changeBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		state, err := svc.AppendPSPChange(r.Context(), charID(r), body.Delta, body.Source)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, StateDTO{Current: state.Current, Max: state.Max})
		return nil
	}
}

// StateDTO is the pool after a change, the derived character's `psp` shape
// (characterhdl.PSPDTO, web/src/types).
type StateDTO struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

// Compile-time check that *chi.Mux satisfies our small Router interface.
var _ Router = (*chi.Mux)(nil)

// KnownPowersDTO is the saved known powers, the derived character's
// `knownPowers` shape (characterhdl.KnownPowersDTO).
type KnownPowersDTO struct {
	Allowance PowerCountsDTO `json:"allowance"`
	Used      PowerCountsDTO `json:"used"`
	Primary   string         `json:"primary"`
	Picks     []PickDTO      `json:"picks"`
}

// PowerCountsDTO counts disciplines, sciences, devotions and defence modes.
type PowerCountsDTO struct {
	Disciplines  int `json:"disciplines"`
	Sciences     int `json:"sciences"`
	Devotions    int `json:"devotions"`
	DefenceModes int `json:"defenceModes"`
}

// PickDTO is one pick: kind discipline, power or relearn, and its id.
type PickDTO struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

func knownPowersDTO(k charactersvc.PowerKnowledge) KnownPowersDTO {
	out := KnownPowersDTO{
		Allowance: PowerCountsDTO(k.Allowance), Used: PowerCountsDTO(k.Used),
		Primary: k.Primary, Picks: make([]PickDTO, len(k.Picks)),
	}
	for i, p := range k.Picks {
		out.Picks[i] = PickDTO(p)
	}
	return out
}
