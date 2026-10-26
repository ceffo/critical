// Package powerhdl exposes /api/powers (ADR 0043 step 8): the psionic powers
// catalogue, searched like the spells. A catalogue read no character owns, so
// it needs a session but no current character.
package powerhdl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/powersvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
}

type powerSvc interface {
	Search(ctx context.Context, f powersvc.Filter, p powersvc.PageRequest) (powersvc.Page, error)
	Get(ctx context.Context, id string) (catalogue.Power, error)
	Disciplines(ctx context.Context) []catalogue.Discipline
	Kinds(ctx context.Context) []string
}

// RegisterRoutes wires the power reads onto r.
func RegisterRoutes(r Router, svc powersvc.SVC) {
	r.Get("/api/powers/filters", render.Handle(FilterOptions(svc)))
	r.Get("/api/powers", render.Handle(Search(svc)))
	r.Get("/api/powers/{id}", render.Handle(Get(svc)))
}

// CostDTO is a power's cost: a number of PSPs, or the word `variable` or
// `contact` when the book prints none.
type CostDTO struct{ catalogue.Cost }

// MarshalJSON writes the amount as a number, a word as a string.
func (c CostDTO) MarshalJSON() ([]byte, error) {
	if c.Kind != "" {
		return json.Marshal(c.Kind)
	}
	return json.Marshal(c.Amount)
}

// ScoreDTO is a power's score: an ability code plus a modifier.
type ScoreDTO struct {
	Ability  string `json:"ability"`
	Modifier int    `json:"modifier"`
}

// MaintenanceDTO is what a maintained power costs every period rounds.
type MaintenanceDTO struct {
	Cost   CostDTO `json:"cost"`
	Period int     `json:"period"`
}

// PowerDTO is one catalogue power. Every text is a language map (ADR 0023);
// the client picks its language and falls back to English per field.
type PowerDTO struct {
	ID          string         `json:"id"`
	Name        localized.Text `json:"name"`
	Discipline  string         `json:"discipline"`
	Kind        string         `json:"kind"`
	Score       *ScoreDTO      `json:"score,omitempty"`
	InitialCost CostDTO        `json:"initial_cost"`
	// InitialCostTable names the rule table that gives the cost (read it at
	// GET /api/psionics/cost-tables/{table}); InitialCostPlus is what a
	// contact cost adds once contact is made. Both are omitted when empty;
	// initial_cost keeps its word or number.
	InitialCostTable string          `json:"initial_cost_table,omitempty"`
	InitialCostPlus  int             `json:"initial_cost_plus,omitempty"`
	Maintenance      *MaintenanceDTO `json:"maintenance,omitempty"`
	Prerequisites    []string        `json:"prerequisites"`
	MinLevel         int             `json:"min_level,omitempty"`
	Range            localized.Text  `json:"range"`
	PreparationTime  localized.Text  `json:"preparation_time"`
	Area             localized.Text  `json:"area"`
	Description      localized.Text  `json:"description"`
	PowerScoreResult localized.Text  `json:"power_score_result,omitempty"`
	Natural20Result  localized.Text  `json:"natural_20_result,omitempty"`
}

// ToDTO is the wire form of a power.
func ToDTO(p catalogue.Power) PowerDTO {
	out := PowerDTO{
		ID: p.ID, Name: wire.Text(p.Name), Discipline: p.Discipline, Kind: p.Kind,
		InitialCost: CostDTO{p.InitialCost}, InitialCostTable: p.InitialCost.Table, InitialCostPlus: p.InitialCost.Plus,
		Prerequisites: append([]string{}, p.Prerequisites...),
		MinLevel:      p.MinLevel,
		Range:         wire.Text(p.Range), PreparationTime: wire.Text(p.PreparationTime), Area: wire.Text(p.Area),
		Description:      wire.Text(p.Description),
		PowerScoreResult: wire.Text(p.PowerScoreResult), Natural20Result: wire.Text(p.Natural20Result),
	}
	if p.Score != nil {
		out.Score = &ScoreDTO{Ability: p.Score.Ability, Modifier: p.Score.Modifier}
	}
	if p.Maintenance != nil {
		out.Maintenance = &MaintenanceDTO{Cost: CostDTO{p.Maintenance.Cost}, Period: p.Maintenance.Period}
	}
	return out
}

// PowerPageDTO is one page of GET /api/powers; Total counts every match.
type PowerPageDTO struct {
	Items    []PowerDTO `json:"items"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
}

// DisciplineDTO is a discipline with its name in every language.
type DisciplineDTO struct {
	ID   string         `json:"id"`
	Name localized.Text `json:"name"`
}

// FilterOptionsDTO lists the choices of the lookup's filters: the disciplines
// (book order, with their names) and the power kinds.
type FilterOptionsDTO struct {
	Disciplines []DisciplineDTO `json:"disciplines"`
	Kinds       []string        `json:"kinds"`
}

// FilterOptions is GET /api/powers/filters.
func FilterOptions(svc powerSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ds := svc.Disciplines(r.Context())
		out := FilterOptionsDTO{Disciplines: make([]DisciplineDTO, 0, len(ds)), Kinds: svc.Kinds(r.Context())}
		for _, d := range ds {
			out.Disciplines = append(out.Disciplines, DisciplineDTO{ID: d.ID, Name: wire.Text(d.Name)})
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// Search is GET /api/powers. Parameters, a malformed one being a 400:
//   - q: text matched against the name in every language, case and accents ignored;
//   - discipline: a discipline id (GET /api/powers/filters);
//   - kind: science, devotion, defence_mode or attack_mode;
//   - page (1-based, default 1) and page_size (default 50, max 200).
func Search(svc powerSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		f, err := parseFilter(r.Context(), svc, r.URL.Query())
		if err != nil {
			return err
		}
		pr, err := parsePage(r.URL.Query())
		if err != nil {
			return err
		}
		page, err := svc.Search(r.Context(), f, pr)
		if err != nil {
			return err
		}
		items := make([]PowerDTO, 0, len(page.Items))
		for _, p := range page.Items {
			items = append(items, ToDTO(p))
		}
		render.OK(r.Context(), w, PowerPageDTO{Items: items, Total: page.Total, Page: page.Page, PageSize: page.PageSize})
		return nil
	}
}

// Get is GET /api/powers/{id}; an id the catalogue does not hold is a 404.
func Get(svc powerSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		p, err := svc.Get(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(p))
		return nil
	}
}

func parseFilter(ctx context.Context, svc powerSvc, q url.Values) (powersvc.Filter, error) {
	f := powersvc.Filter{Query: strings.TrimSpace(q.Get("q"))}
	if raw := q.Get("discipline"); raw != "" {
		known := false
		var ids []string
		for _, d := range svc.Disciplines(ctx) {
			ids = append(ids, d.ID)
			known = known || d.ID == raw
		}
		if !known {
			return f, render.BadRequestError("discipline must be one of " + strings.Join(ids, ", "))
		}
		f.Discipline = raw
	}
	if raw := q.Get("kind"); raw != "" {
		kinds := svc.Kinds(ctx)
		if !slices.Contains(kinds, raw) {
			return f, render.BadRequestError("kind must be one of " + strings.Join(kinds, ", "))
		}
		f.Kind = raw
	}
	return f, nil
}

func parsePage(q url.Values) (powersvc.PageRequest, error) {
	pr := powersvc.PageRequest{Page: 1, PageSize: powersvc.DefaultPageSize}
	if raw := q.Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return pr, render.BadRequestError("page must be a positive integer")
		}
		pr.Page = n
	}
	if raw := q.Get("page_size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > powersvc.MaxPageSize {
			return pr, render.BadRequestError("page_size must be an integer from 1 to " + strconv.Itoa(powersvc.MaxPageSize))
		}
		pr.PageSize = n
	}
	return pr, nil
}
