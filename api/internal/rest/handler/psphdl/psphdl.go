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
	"github.com/ceffo/critical/api/internal/domain/localized"
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
	Get(pattern string, h http.HandlerFunc)
}

// changeAppender is the one method the endpoint calls; actionlogsvc.SVC
// satisfies it.
type changeAppender interface {
	AppendPSPChange(ctx context.Context, characterID int64, delta int, source string) (actionlog.PSPState, error)
}

// recoverer is the recovery's one method; psionicsvc.SVC satisfies it.
type recoverer interface {
	Recover(ctx context.Context, characterID int64, hours float64, activity string) (actionlog.PSPState, error)
	StopAndRecover(ctx context.Context, characterID int64, hours float64, activity string) (psionicsvc.StopRecoverResult, error)
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
	Attack(ctx context.Context, characterID int64, req psionicsvc.AttackRequest) (*psionicsvc.AttackResult, error)
	Contact(ctx context.Context, characterID int64, req psionicsvc.ContactRequest) (*psionicsvc.UseResult, error)
	ContactPreview(ctx context.Context, characterID int64, req psionicsvc.ContactRequest) (*psionicsvc.ContactPreview, error)
	CostTable(ctx context.Context, table string) (psionicsvc.CostTable, error)
}

// CharacterIDFunc resolves the active character ID from the request context.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires the PSP endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc changeAppender, rec recoverer, known knownSetter, use user, def defender) {
	r.Post("/api/psp/event", render.Handle(AppendChange(charID, svc)))
	r.Post("/api/psp/recover", render.Handle(Recover(charID, rec)))
	r.Post("/api/psionics/stop-and-recover", render.Handle(StopAndRecover(charID, rec)))
	r.Put("/api/psionics/picks", render.Handle(SetKnownPowers(charID, known)))
	r.Post("/api/psionics/powers/{id}/use", render.Handle(Use(charID, use)))
	r.Post("/api/psionics/powers/{id}/stop", render.Handle(Stop(charID, use)))
	r.Post("/api/psionics/powers/{id}/raise", render.Handle(Raise(charID, def)))
	r.Post("/api/psionics/defend", render.Handle(Defend(charID, def)))
	r.Post("/api/psionics/powers/{id}/attack", render.Handle(Attack(charID, def)))
	r.Post("/api/psionics/powers/{id}/contact", render.Handle(Contact(charID, def)))
	r.Post("/api/psionics/powers/{id}/contact/preview", render.Handle(ContactPreview(charID, def)))
	r.Get("/api/psionics/cost-tables/{table}", render.Handle(CostTable(def)))
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

type attackBody struct {
	Target    string                 `json:"target"`
	Mind      string                 `json:"mind"`
	Level     int                    `json:"level"`
	Defence   string                 `json:"defence"`
	Rolls     []psionics.ContestSide `json:"rolls"`
	Modifiers []psionicsvc.Modifier  `json:"modifiers"`
}

// AttackDTO is an attack as logged: the psionic_attack entry's payload, the
// pool after the cost and the entry's id.
type AttackDTO struct {
	Attack     psionicsvc.AttackRecord `json:"attack"`
	PSP        StateDTO                `json:"psp"`
	LogEntryID int64                   `json:"logEntryId"`
}

// Attack uses the known attack mode {id} on the opponent the body names: two
// checks for one cost (ADR 0044 step 7). 400 on a bad opponent or modifier;
// 409 not_an_attack_mode, needs_contact, not_enough_psp, power_not_known,
// not_a_psionicist.
func Attack(charID CharacterIDFunc, svc defender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body attackBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		res, err := svc.Attack(r.Context(), charID(r), psionicsvc.AttackRequest{
			Mode: chi.URLParam(r, "id"),
			Opponent: psionics.Opponent{
				Target: body.Target, Mind: body.Mind, Level: body.Level,
				Defence: body.Defence, Rolls: body.Rolls,
			},
			Modifiers: body.Modifiers,
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, AttackDTO{
			Attack: res.Attack, PSP: StateDTO{Current: res.PSP.Current, Max: res.PSP.Max}, LogEntryID: res.LogEntryID,
		})
		return nil
	}
}

type contactBody struct {
	Target  string `json:"target"`
	Mind    string `json:"mind"`
	Level   int    `json:"level"`
	Defence string `json:"defence"`
	// Cost is the DM's cost for a level Table 29 has no row for, and only
	// then (Q15).
	Cost *int `json:"cost"`
	// Modifiers are the cost table's modifier rows picked (kind, code), each
	// validated against the table; Labelled are the DM's labelled modifiers.
	Modifiers []psionicsvc.ContactPick `json:"modifiers"`
	Labelled  []psionicsvc.Modifier    `json:"labelled"`
}

func (b contactBody) request(power string) psionicsvc.ContactRequest {
	return psionicsvc.ContactRequest{
		Power: power,
		Opponent: psionics.Opponent{
			Target: b.Target, Mind: b.Mind, Level: b.Level, Defence: b.Defence,
		},
		Cost:  b.Cost,
		Picks: b.Modifiers, Labelled: b.Labelled,
	}
}

// ContactPreview is POST /api/psionics/powers/{id}/contact/preview: the
// contact's cost and score for the body Contact takes, from Contact's own
// computation, with nothing rolled or written. It answers the psionicsvc
// ContactPreview (cost, fromTable, target, lines, current, enough, warnings). The
// refusals are Contact's before the roll; not_enough_psp is not one, enough
// says it.
func ContactPreview(charID CharacterIDFunc, svc defender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body contactBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		res, err := svc.ContactPreview(r.Context(), charID(r), body.request(chi.URLParam(r, "id")))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, res)
		return nil
	}
}

// CostTable is GET /api/psionics/cost-tables/{table}: the rule table a
// power's initial cost names (PowerDTO.initial_cost_table) and its modifier
// rows (kind, language-neutral code, modifier), for the web to build its
// controls from; the cost for a level is the preview's. 404 for a name no power's cost carries.
func CostTable(svc defender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		t, err := svc.CostTable(r.Context(), chi.URLParam(r, "table"))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, t)
		return nil
	}
}

// Contact uses the known contact power {id} on the opponent the body names
// (ADR 0044 step 9): the check, Table 29's cost for its level and the
// distance, life-order and resisting lines. The answer is the Use one
// (UseDTO); a success is in check.contact. 400 on a bad opponent, distance,
// life order, modifier or cost, or a level Table 29 lacks with no cost given
// (level_out_of_table);
// 409 not_a_contact_power, mind_closed, not_enough_psp, power_not_known,
// not_a_psionicist.
func Contact(charID CharacterIDFunc, svc defender) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body contactBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		res, err := svc.Contact(r.Context(), charID(r), body.request(chi.URLParam(r, "id")))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, UseDTO{
			Check: res.Check, PSP: StateDTO{Current: res.PSP.Current, Max: res.PSP.Max}, LogEntryID: res.LogEntryID, BuffID: res.BuffID,
		})
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
	// Target is the mind a power whose cost is contact is used on.
	Target string `json:"target"`
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
			Target: body.Target,
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

// StopRecoverDTO is a stop-and-recover: the pool after it and the rows it
// ended, as the long rest's entry names them (ADR 0043 step 7). PowersEnded
// are catalogue power ids, PowerNames their names in every language,
// TargetsEnded the minds whose tangent or contact row ended.
type StopRecoverDTO struct {
	PSP          StateDTO                  `json:"psp"`
	PowersEnded  []string                  `json:"powersEnded"`
	PowerNames   map[string]localized.Text `json:"powerNames,omitempty"`
	TargetsEnded []string                  `json:"targetsEnded"`
}

// StopAndRecover ends every row that pays PSPs and recovers hours of an
// activity, as one undoable entry (ADR 0047 item 4). Same body and refusals as
// Recover, but never psp_recovery_while_maintaining.
func StopAndRecover(charID CharacterIDFunc, svc recoverer) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var body recoverBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		res, err := svc.StopAndRecover(r.Context(), charID(r), body.Hours, body.Activity)
		if err != nil {
			return err
		}
		powers, targets := buffsvc.SplitEnded(res.Ended)
		out := StopRecoverDTO{
			PSP:         StateDTO{Current: res.PSP.Current, Max: res.PSP.Max},
			PowersEnded: append([]string{}, powers...), TargetsEnded: append([]string{}, targets...),
			PowerNames: buffsvc.PowerNames(res.Ended),
		}
		render.OK(r.Context(), w, out)
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
