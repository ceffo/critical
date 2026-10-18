package spelldal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelldal"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	spellmocks "github.com/ceffo/critical/api/internal/domain/service/spellsvc/mocks"
)

// The catalogue's spells, in the inner reader's order (level, name, class).
var (
	alarm    = spellsvc.Spell{ID: "alarm", Class: "Wizard", Level: 1, Name: localized.Text{"en": "Alarm"}}
	bless    = spellsvc.Spell{ID: "bless", Class: "Priest", Level: 1, Name: localized.Text{"en": "Bless"}}
	fireball = spellsvc.Spell{ID: "fireball", Class: "Wizard", Level: 3, Name: localized.Text{"en": "Fireball"}}
)

// loaded is a catalogue read from a mocked reader: one Search with no filter,
// then one Details call for every key it returned, in its order.
func loaded(t *testing.T) *spelldal.Catalogue {
	t.Helper()
	r := spellmocks.NewMockReader(t)
	rows := []spellsvc.Spell{{ID: "alarm"}, {ID: "bless"}, {ID: "fireball"}}
	search := r.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(rows, nil).Once()
	r.EXPECT().Details(mock.Anything, []string{"alarm", "bless", "fireball"}).
		Return([]spellsvc.Spell{alarm, bless, fireball}, nil).Once().NotBefore(search)
	c, err := spelldal.LoadCatalogue(context.Background(), r)
	require.NoError(t, err)
	return c
}

func ids(spells []spellsvc.Spell) []string {
	out := make([]string, len(spells))
	for i, sp := range spells {
		out[i] = sp.ID
	}
	return out
}

func TestCatalogue_SearchFiltersClassAndLevelsInTheCataloguesOrder(t *testing.T) {
	c := loaded(t)
	ctx := context.Background()
	tests := []struct {
		name string
		f    spellsvc.Filter
		want []string
	}{
		{"no filter", spellsvc.Filter{}, []string{"alarm", "bless", "fireball"}},
		{"class, any case", spellsvc.Filter{Class: "wIZARD"}, []string{"alarm", "fireball"}},
		{"levels", spellsvc.Filter{Levels: []int{3, 1}}, []string{"alarm", "bless", "fireball"}},
		{"one level", spellsvc.Filter{Levels: []int{1}}, []string{"alarm", "bless"}},
		{"class and level", spellsvc.Filter{Class: "priest", Levels: []int{3}}, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.Search(ctx, tc.f)
			require.NoError(t, err)
			assert.Equal(t, tc.want, ids(got))
		})
	}
}

func TestCatalogue_DetailsInTheOrderAskedSkippingUnknownKeys(t *testing.T) {
	c := loaded(t)
	got, err := c.Details(context.Background(), []string{"fireball", "gone", "alarm"})
	require.NoError(t, err)
	assert.Equal(t, []spellsvc.Spell{fireball, alarm}, got)

	none, err := c.Details(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestCatalogue_Describe(t *testing.T) {
	c := loaded(t)
	sp, ok, err := c.Describe(context.Background(), "bless")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, bless, sp)

	_, ok, err = c.Describe(context.Background(), "gone")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestLoadCatalogue_AFailedReadIsReturned(t *testing.T) {
	t.Run("search", func(t *testing.T) {
		r := spellmocks.NewMockReader(t)
		r.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(nil, errBoom).Once()
		_, err := spelldal.LoadCatalogue(context.Background(), r)
		require.ErrorIs(t, err, errBoom)
	})
	t.Run("details", func(t *testing.T) {
		r := spellmocks.NewMockReader(t)
		r.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return([]spellsvc.Spell{{ID: "alarm"}}, nil).Once()
		r.EXPECT().Details(mock.Anything, []string{"alarm"}).Return(nil, errBoom).Once()
		_, err := spelldal.LoadCatalogue(context.Background(), r)
		require.ErrorIs(t, err, errBoom)
	})
}
