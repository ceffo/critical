// Package xphdl exposes /api/xp, the XP ledger (ADR 0015).
package xphdl

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/go-chi/chi/v5"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// Compile-time check that *chi.Mux satisfies our small Router interface.
var _ Router = (*chi.Mux)(nil)

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type xpSvc interface {
	Get(ctx context.Context, characterID int64) (*xpsvc.Ledger, error)
	Preview(base int, bonus bool) (xpsvc.Amount, error)
	Add(ctx context.Context, characterID int64, in xpsvc.EntryInput) (*xpsvc.Ledger, error)
	Update(ctx context.Context, characterID, entryID int64, p xpsvc.EntryPatch) (*xpsvc.Ledger, error)
	Delete(ctx context.Context, characterID, entryID int64) (*xpsvc.Ledger, error)
}

// RegisterRoutes wires the XP endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc xpsvc.SVC) {
	r.Get("/api/xp", render.Handle(Get(charID, svc)))
	r.Get("/api/xp/preview", render.Handle(Preview(svc)))
	r.Post("/api/xp", render.Handle(Add(charID, svc)))
	r.Patch("/api/xp/{id}", render.Handle(Update(charID, svc)))
	r.Delete("/api/xp/{id}", render.Handle(Delete(charID, svc)))
}

// AmountDTO is what one entry adds.
type AmountDTO struct {
	Base        int `json:"base"`
	BonusAmount int `json:"bonusAmount"`
	Total       int `json:"total"`
}

// EntryDTO is one ledger entry; bonus is whether the +10% applies,
// bonusAmount and total are computed by the server.
type EntryDTO struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"`
	Session     string `json:"session"`
	Reason      string `json:"reason"`
	Base        int    `json:"base"`
	Bonus       bool   `json:"bonus"`
	BonusAmount int    `json:"bonusAmount"`
	Total       int    `json:"total"`
}

// LevelDTO is one level of Table 25; status is past, current, next or ahead.
type LevelDTO struct {
	Level     int    `json:"level"`
	Threshold int    `json:"threshold"`
	Status    string `json:"status"`
}

// LedgerDTO is the wire shape of every /api/xp answer. nextLevel and
// nextThreshold are null at the last level of the table.
type LedgerDTO struct {
	Opening         int        `json:"opening"`
	Total           int        `json:"total"`
	Level           int        `json:"level"`
	CurrentFloor    int        `json:"currentFloor"`
	NextLevel       *int       `json:"nextLevel"`
	NextThreshold   *int       `json:"nextThreshold"`
	ToNext          int        `json:"toNext"`
	Progress        float64    `json:"progress"`
	ProgressPercent int        `json:"progressPercent"`
	BonusEligible   bool       `json:"bonusEligible"`
	Levels          []LevelDTO `json:"levels"`
	Entries         []EntryDTO `json:"entries"`
}

// ToDTO maps a ledger to its wire shape.
func ToDTO(l *xpsvc.Ledger) LedgerDTO {
	out := LedgerDTO{
		Opening: l.Opening, Total: l.Total, Level: l.Level, CurrentFloor: l.CurrentFloor,
		NextThreshold: l.NextThreshold, ToNext: l.ToNext, Progress: l.Progress.Progress,
		ProgressPercent: l.ProgressPercent, BonusEligible: l.BonusEligible,
		Levels: make([]LevelDTO, 0, len(l.Levels)), Entries: make([]EntryDTO, 0, len(l.Entries)),
	}
	if l.NextLevel > 0 {
		n := l.NextLevel
		out.NextLevel = &n
	}
	for _, c := range l.Levels {
		out.Levels = append(out.Levels, LevelDTO{Level: c.Level, Threshold: c.Threshold, Status: string(c.Status)})
	}
	for _, e := range l.Entries {
		out.Entries = append(out.Entries, EntryDTO{
			ID: e.ID, Date: e.Date, Session: e.Session, Reason: e.Reason,
			Base: e.Base, Bonus: e.Bonus, BonusAmount: e.BonusAmount, Total: e.Total,
		})
	}
	return out
}

// Get returns the ledger.
func Get(charID CharacterIDFunc, svc xpSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		l, err := svc.Get(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(l))
		return nil
	}
}

// Preview returns what an entry would add: ?base=N&bonus=true|false.
func Preview(svc xpSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		q := r.URL.Query()
		base, err := strconv.Atoi(q.Get("base"))
		if err != nil {
			return render.BadRequestError("base must be a whole number")
		}
		bonus := false
		if s := q.Get("bonus"); s != "" {
			if bonus, err = strconv.ParseBool(s); err != nil {
				return render.BadRequestError("bonus must be true or false")
			}
		}
		a, err := svc.Preview(base, bonus)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, AmountDTO{Base: a.Base, BonusAmount: a.BonusAmount, Total: a.Total})
		return nil
	}
}

// EntryBody is the body of POST and PATCH /api/xp. On a PATCH only the
// fields present change; base is required on a POST.
type EntryBody struct {
	Base    *int    `json:"base"`
	Bonus   *bool   `json:"bonus"`
	Reason  *string `json:"reason"`
	Session *string `json:"session"`
}

func decodeBody(r *http.Request) (EntryBody, error) {
	var b EntryBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return b, render.BadRequestError("invalid JSON body")
	}
	return b, nil
}

func entryID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, render.BadRequestError("invalid entry id")
	}
	return id, nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// Add records an award.
func Add(charID CharacterIDFunc, svc xpSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		b, err := decodeBody(r)
		if err != nil {
			return err
		}
		if b.Base == nil {
			return render.BadRequestError("base is required")
		}
		l, err := svc.Add(r.Context(), charID(r), xpsvc.EntryInput{
			Base: *b.Base, Bonus: deref(b.Bonus), Reason: deref(b.Reason), Session: deref(b.Session),
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(l))
		return nil
	}
}

// Update edits the fields present in the body.
func Update(charID CharacterIDFunc, svc xpSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := entryID(r)
		if err != nil {
			return err
		}
		b, err := decodeBody(r)
		if err != nil {
			return err
		}
		l, err := svc.Update(r.Context(), charID(r), id, xpsvc.EntryPatch{
			Base: b.Base, Bonus: b.Bonus, Reason: b.Reason, Session: b.Session,
		})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(l))
		return nil
	}
}

// Delete removes an entry.
func Delete(charID CharacterIDFunc, svc xpSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := entryID(r)
		if err != nil {
			return err
		}
		l, err := svc.Delete(r.Context(), charID(r), id)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(l))
		return nil
	}
}
