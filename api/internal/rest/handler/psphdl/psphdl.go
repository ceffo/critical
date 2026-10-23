// Package psphdl exposes the PSP pool's hand edit (ADR 0043 step 5), as hphdl
// does for HP, the recovery over hours of an activity (step 6) and the known
// powers (step 9), a power's use (steps 10, 11 and 13) and the stop of a
// maintained power (step 16) over HTTP. The handler depends on a narrow interface
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
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
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

// user is a power's use and stop; psionicsvc.SVC satisfies it.
type user interface {
	Use(ctx context.Context, characterID int64, powerID string, req psionicsvc.UseRequest) (*psionicsvc.UseResult, error)
	Stop(ctx context.Context, characterID int64, powerID string) ([]buffsvc.BuffEffect, error)
}

// defender is a defence mode's raise and the contests against it;
// psionicsvc.SVC satisfies it.
type defender interface {
	RaiseDefence(ctx context.Context, characterID int64, powerID string) (*psionicsvc.RaiseResult, error)
	Defend(ctx context.Context, characterID int64, req psionicsvc.DefendRequest) (*psionicsvc.DefendResult, error)
}

// CharacterIDFunc resolves the active character ID from the request context.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires the PSP endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc changeAppender, rec recoverer, known knownSetter, use user, def defender) {
	r.Post("/api/psp/event", render.Handle(AppendChange(charID, svc)))
	r.Post("/api/psp/recover", render.Handle(Recover(charID, rec)))
	r.Put("/api/psionics/picks", render.Handle(SetKnownPowers(charID, known)))
	r.Post("/api/psionics/powers/{id}/use", render.Handle(Use(charID, use)))
	r.Post("/api/psionics/powers/{id}/stop", render.Handle(Stop(charID, use)))
	r.Post("/api/psionics/powers/{id}/raise", render.Handle(Raise(charID, def)))
	r.Post("/api/psionics/defend", render.Handle(Defend(charID, def)))
}

// RaiseDTO is a defence mode's raise as logged: the payload of the
// defence_raised entry, the pool after the cost, the entry's id and the buff
// row put.
type RaiseDTO struct {
	Raise      psionicsvc.RaiseRecord `json:"raise"`
	PSP        StateDTO               `json:"psp"`
	LogEntryID int64                  `json:"logEntryId"`
	BuffID     string                 `json:"buffId"`
}

// Raise raises the known defence mode {id} with no roll (ADR 0044 steps 4
// and 5). 409 not_a_defence_mode, not_in_combat, needs_contact,
// not_enough_psp.
func Raise(charID CharacterIDFunc, svc defender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		res, err := svc.RaiseDefence(r.Context(), charID(r), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RaiseDTO{
			Raise: res.Raise, PSP: StateDTO{Current: res.PSP.Current, Max: res.PSP.Max},
			LogEntryID: res.LogEntryID, BuffID: res.BuffID,
		})
		return nil
	}
}

type defendBody struct {
	Attack string                 `json:"attack"`
	Rolls  []psionics.ContestSide `json:"rolls"`
	Keep   *int                   `json:"keep"`
}

// DefendDTO is a contest as logged: the psionic_defence entry's payload and
// its id.
type DefendDTO struct {
	Defence    psionicsvc.DefenceRecord `json:"defence"`
	LogEntryID int64                    `json:"logEntryId"`
}

// Defend decides the DM's one or two rolls of an attack mode against the
// current defence (ADR 0044 step 8). 400 on a bad attack, rolls or keep.
func Defend(charID CharacterIDFunc, svc defender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body defendBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		res, err := svc.Defend(r.Context(), charID(r), psionicsvc.DefendRequest{
			Attack: body.Attack, Rolls: body.Rolls, Keep: body.Keep,
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, DefendDTO{Defence: res.Defence, LogEntryID: res.LogEntryID})
		return nil
	}
}

type useBody struct {
	Modifiers []psionicsvc.Modifier `json:"modifiers"`
	Cost      *int                  `json:"cost"`
	Score     *int                  `json:"score"`
	// MaintenanceCost is the maintenance of a power whose maintenance cost
	// is variable, and only then.
	MaintenanceCost *int `json:"maintenanceCost"`
}

// UseDTO is a power's use as logged: the check's breakdown (the power_check
// entry's payload), the pool after the spend and the entry's id. BuffID is
// the buff row a success of a power with maintenance put on (step 13),
// omitted otherwise.
type UseDTO struct {
	Check      psionicsvc.CheckRecord `json:"check"`
	PSP        StateDTO               `json:"psp"`
	LogEntryID int64                  `json:"logEntryId"`
	BuffID     string                 `json:"buffId,omitempty"`
}

// Use checks the known power {id} and spends its initial cost (ADR 0043
// steps 10 and 11). The body gives the labelled modifiers, and the cost or
// the score of a power whose catalogue entry has none.
func Use(charID CharacterIDFunc, svc user) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body useBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		res, err := svc.Use(r.Context(), charID(r), chi.URLParam(r, "id"), psionicsvc.UseRequest{
			Modifiers: body.Modifiers, Cost: body.Cost, Score: body.Score, MaintenanceCost: body.MaintenanceCost,
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, UseDTO{
			Check: res.Check, PSP: StateDTO{Current: res.PSP.Current, Max: res.PSP.Max}, LogEntryID: res.LogEntryID, BuffID: res.BuffID,
		})
		return nil
	}
}

// StopDTO names the power whose maintenance stopped; the buff list and the
// journal are read again by the client.
type StopDTO struct {
	PowerID string `json:"powerId"`
}

// Stop ends the maintained power {id} (ADR 0043 step 16): its buff row is
// dismissed, free and undoable. 409 power_not_maintained when no row keeps
// it up.
func Stop(charID CharacterIDFunc, svc user) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id := chi.URLParam(r, "id")
		if _, err := svc.Stop(r.Context(), charID(r), id); err != nil {
			return err
		}
		render.OK(r.Context(), w, StopDTO{PowerID: id})
		return nil
	}
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
// It answers the pool after the recovery; 409 psp_recovery_while_maintaining
// while a power is maintained (ADR 0043 step 6).
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
