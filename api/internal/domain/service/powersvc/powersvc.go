// Package powersvc serves the psionic powers catalogue (ADR 0043 step 8,
// ADR 0021 step 20.3): the disciplines and powers of the artifact, searched
// by name in every language and filtered by discipline and kind. It owns no
// state: the catalogue changes only when a server starts on a new artifact,
// so everything is read from memory (like profdescsvc). The lookup lists
// powers here; knowing and using them are other services' concern.
package powersvc

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Default and maximum page sizes of a search.
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
)

// Filter narrows a search. Zero fields do not filter.
type Filter struct {
	// Query is matched against the power's name in every language, ignoring
	// case and accents: exact, prefix or substring, the best language ranks.
	Query string
	// Discipline is a discipline id.
	Discipline string
	// Kind is one of catalogue.PowerKinds.
	Kind string
}

// PageRequest selects one page of a search: Page is 1-based. Zero values mean
// the first page of DefaultPageSize; a size above MaxPageSize is clamped.
type PageRequest struct {
	Page     int
	PageSize int
}

// Page is one page of matches; Total counts every match.
type Page struct {
	Items    []catalogue.Power
	Total    int
	Page     int
	PageSize int
}

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Search ranks the matches (exact name, prefix, substring) and keeps the
	// book order within a rank, then cuts the page. Filtering runs before
	// paging, so Total and the pages are exact.
	Search(ctx context.Context, f Filter, p PageRequest) (Page, error)
	// Get is the power with id, or domain.ErrNotFound.
	Get(ctx context.Context, id string) (catalogue.Power, error)
	// Disciplines lists the disciplines in book order; Kinds the power kinds.
	Disciplines(ctx context.Context) []catalogue.Discipline
	Kinds(ctx context.Context) []string
}

type service struct {
	disciplines []catalogue.Discipline
	powers      []catalogue.Power
	byID        map[string]int
}

// New builds the service over the artifact's disciplines and powers. The
// artifact's parse already refused a bad reference (catalogue.CheckPowers);
// it is checked again so a wiring mistake cannot serve a broken catalogue.
func New(disciplines []catalogue.Discipline, powers []catalogue.Power) (SVC, error) {
	if err := catalogue.CheckPowers(disciplines, powers); err != nil {
		return nil, err
	}
	s := &service{
		disciplines: slices.Clone(disciplines),
		powers:      slices.Clone(powers),
		byID:        make(map[string]int, len(powers)),
	}
	for i, p := range s.powers {
		s.byID[p.ID] = i
	}
	return s, nil
}

// Health always succeeds: the catalogue is in memory.
func (s *service) Health(context.Context) error { return nil }

// Close releases nothing.
func (s *service) Close(context.Context) error { return nil }

// Module declares no codec: the service writes nothing.
func (s *service) Module() domain.Module { return domain.Module{Name: "powersvc"} }

type tier int

const (
	tierNone tier = iota
	tierExact
	tierPrefix
	tierContains
)

// nameTier is the best tier of q (folded) over the power's names.
func nameTier(p catalogue.Power, q string) tier {
	if q == "" {
		return tierExact
	}
	best := tierNone
	for _, name := range p.Name {
		n := spellsvc.Fold(name)
		t := tierNone
		switch {
		case n == q:
			t = tierExact
		case strings.HasPrefix(n, q):
			t = tierPrefix
		case strings.Contains(n, q):
			t = tierContains
		}
		if t != tierNone && (best == tierNone || t < best) {
			best = t
		}
	}
	return best
}

func (s *service) Search(_ context.Context, f Filter, p PageRequest) (Page, error) {
	q := spellsvc.Fold(strings.TrimSpace(f.Query))
	type ranked struct {
		tier  tier
		power catalogue.Power
	}
	found := make([]ranked, 0, len(s.powers))
	for _, pw := range s.powers {
		if f.Discipline != "" && pw.Discipline != f.Discipline {
			continue
		}
		if f.Kind != "" && pw.Kind != f.Kind {
			continue
		}
		if t := nameTier(pw, q); t != tierNone {
			found = append(found, ranked{t, pw})
		}
	}
	slices.SortStableFunc(found, func(a, b ranked) int { return cmp.Compare(a.tier, b.tier) })
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = DefaultPageSize
	}
	p.PageSize = min(p.PageSize, MaxPageSize)
	start := min((p.Page-1)*p.PageSize, len(found))
	end := min(start+p.PageSize, len(found))
	items := make([]catalogue.Power, 0, end-start)
	for _, r := range found[start:end] {
		items = append(items, r.power)
	}
	return Page{Items: items, Total: len(found), Page: p.Page, PageSize: p.PageSize}, nil
}

func (s *service) Get(_ context.Context, id string) (catalogue.Power, error) {
	i, ok := s.byID[id]
	if !ok {
		return catalogue.Power{}, fmt.Errorf("power %q: %w", id, domain.ErrNotFound)
	}
	return s.powers[i], nil
}

func (s *service) Disciplines(context.Context) []catalogue.Discipline {
	return slices.Clone(s.disciplines)
}

func (s *service) Kinds(context.Context) []string { return slices.Clone(catalogue.PowerKinds) }
