package powersvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

func fixture(t *testing.T) SVC {
	t.Helper()
	disciplines := []catalogue.Discipline{
		{ID: "telepathy", Name: localized.Text{"en": "Telepathy", "fr": "Télépathie"}},
		{ID: "clairsentience", Name: localized.Text{"en": "Clairsentience", "fr": "Clairsentience"}},
	}
	powers := []catalogue.Power{
		{ID: "contact", Name: localized.Text{"en": "Contact", "fr": "Contact"}, Discipline: "telepathy", Kind: catalogue.KindDevotion},
		{ID: "mind_thrust", Name: localized.Text{"en": "Mind Thrust", "fr": "Pression mentale"}, Discipline: "telepathy", Kind: catalogue.KindAttackMode},
		{ID: "aura_sight", Name: localized.Text{"en": "Aura Sight", "fr": "Vision de l'Aura"}, Discipline: "clairsentience", Kind: catalogue.KindScience},
		{ID: "mindlink", Name: localized.Text{"en": "Mindlink", "fr": "Lien spirituel"}, Discipline: "telepathy", Kind: catalogue.KindScience},
	}
	s, err := New(disciplines, powers)
	require.NoError(t, err)
	return s
}

func ids(p Page) []string {
	out := make([]string, 0, len(p.Items))
	for _, it := range p.Items {
		out = append(out, it.ID)
	}
	return out
}

// ADR 0043 step 8 / ADR 0023 step 7: a search text matches the name in every
// language, accents and case ignored; an exact name ranks before a prefix,
// a prefix before a substring, and the book order holds within a rank.
func TestSearch_Step8_MatchesEveryLanguageAndRanks(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()

	p, err := s.Search(ctx, Filter{Query: "vision"}, PageRequest{})
	require.NoError(t, err)
	assert.Equal(t, []string{"aura_sight"}, ids(p), "the French name matches")

	p, _ = s.Search(ctx, Filter{Query: "TELEPATHIE"}, PageRequest{})
	assert.Empty(t, ids(p), "only names are searched, not disciplines")

	p, _ = s.Search(ctx, Filter{Query: "pression"}, PageRequest{})
	assert.Equal(t, []string{"mind_thrust"}, ids(p))

	p, _ = s.Search(ctx, Filter{Query: "mind"}, PageRequest{})
	assert.Equal(t, []string{"mind_thrust", "mindlink"}, ids(p), "both prefix matches, in book order")

	p, _ = s.Search(ctx, Filter{Query: "ink"}, PageRequest{})
	assert.Equal(t, []string{"mindlink"}, ids(p), "substring")

	p, _ = s.Search(ctx, Filter{Query: "contact"}, PageRequest{})
	assert.Equal(t, []string{"contact"}, ids(p))
}

// ADR 0043 step 8: the discipline and kind filters narrow the book-order
// list, and Total counts every match whatever the page.
func TestSearch_Step8_FiltersByDisciplineAndKindAndPages(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()

	p, _ := s.Search(ctx, Filter{Discipline: "telepathy"}, PageRequest{})
	assert.Equal(t, []string{"contact", "mind_thrust", "mindlink"}, ids(p))

	p, _ = s.Search(ctx, Filter{Discipline: "telepathy", Kind: catalogue.KindScience}, PageRequest{})
	assert.Equal(t, []string{"mindlink"}, ids(p))

	p, _ = s.Search(ctx, Filter{}, PageRequest{Page: 2, PageSize: 3})
	assert.Equal(t, []string{"mindlink"}, ids(p))
	assert.Equal(t, 4, p.Total)
	assert.Equal(t, 2, p.Page)

	p, _ = s.Search(ctx, Filter{}, PageRequest{Page: 9, PageSize: 3})
	assert.Empty(t, ids(p))
	assert.Equal(t, 4, p.Total)
}

func TestGet_Step8_AnUnknownIDIsNotFound(t *testing.T) {
	s := fixture(t)
	pw, err := s.Get(context.Background(), "aura_sight")
	require.NoError(t, err)
	assert.Equal(t, "Aura Sight", pw.Name["en"])
	_, err = s.Get(context.Background(), "levitation")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestNew_Step8_RefusesABrokenCatalogue(t *testing.T) {
	_, err := New(nil, []catalogue.Power{{ID: "x", Discipline: "nope", Kind: catalogue.KindScience}})
	require.ErrorContains(t, err, `unknown discipline "nope"`)
}
