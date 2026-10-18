package spellbookadpt_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
)

var cat = catalogue.Catalogue{Spells: []catalogue.Spell{
	{ID: "mirror_image", Name: localized.Of("Mirror Image"), Class: "Wizard", Level: 2},
	{ID: "fireball", Name: localized.Of("Fireball"), Class: "Wizard", Level: 3},
	{ID: "cure_light_wounds", Name: localized.Of("Cure Light Wounds"), Class: "Priest", Level: 1},
}}

func TestParseSeed(t *testing.T) {
	tests := []struct {
		name, book, wantErr string
	}{
		{"ok", `[{"id":"mirror_image","name":"Mirror Image","level":2}]`, ""},
		{"id not in the catalogue", `[{"id":"mirror","name":"Mirror Image","level":2}]`, "not a Wizard spell"},
		{"unknown spell", `[{"id":"shield","name":"Shield","level":1}]`, "not a Wizard spell"},
		{"priest spell is not a wizard spell", `[{"id":"cure_light_wounds","name":"Cure Light Wounds","level":1}]`, "not a Wizard spell"},
		{"name differs from the catalogue", `[{"id":"fireball","name":"Fire Ball","level":3}]`, `named "Fireball"`},
		{"wrong level", `[{"id":"fireball","name":"Fireball","level":2}]`, "is level 3"},
		{"duplicate", `[{"id":"fireball","name":"Fireball","level":3},{"id":"fireball","name":"Fireball","level":3}]`, "duplicate"},
		{"unknown field", `[{"id":"fireball","name":"Fireball","level":3,"extra":1}]`, "parse"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := spellbookadpt.ParseSeed([]byte(tc.book), cat)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestReadSeed_MissingFile(t *testing.T) {
	_, err := spellbookadpt.ReadSeed(t.TempDir(), cat)
	require.Error(t, err)
}

// The seed's keys come back in the file's order, for the seed to write.
func TestParseSeed_KeysInFileOrder(t *testing.T) {
	keys, err := spellbookadpt.ParseSeed([]byte(`[{"id":"mirror_image","name":"Mirror Image","level":2},`+
		`{"id":"fireball","name":"Fireball","level":3}]`), cat)
	require.NoError(t, err)
	assert.Equal(t, []string{"mirror_image", "fireball"}, keys)
}

// books is a Store holding a spellbook per character.
type books map[int64][]string

func (b books) Keys(_ context.Context, characterID int64) ([]string, error) {
	return b[characterID], nil
}

// Each character reads its own spellbook, named and levelled from the
// catalogue, in its stored order; a character without one has none.
func TestBook_EachCharacterReadsItsOwnSpellbook(t *testing.T) {
	ctx := context.Background()
	book := spellbookadpt.New(books{1: {"fireball", "mirror_image"}, 2: {"mirror_image"}}, cat)

	one, err := book.All(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.SpellbookEntry{
		{ID: "fireball", Name: "Fireball", Level: 3}, {ID: "mirror_image", Name: "Mirror Image", Level: 2},
	}, one)
	two, err := book.All(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.SpellbookEntry{{ID: "mirror_image", Name: "Mirror Image", Level: 2}}, two)
	none, err := book.All(ctx, 3)
	require.NoError(t, err)
	assert.Empty(t, none)

	keys, err := book.SpellKeys(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"mirror_image"}, keys)
}

// A stored key the catalogue no longer holds is an error naming it, never a
// silently shorter spellbook.
func TestBook_AKeyTheCatalogueLacksIsAnError(t *testing.T) {
	_, err := spellbookadpt.New(books{1: {"shield"}}, cat).All(context.Background(), 1)
	require.ErrorContains(t, err, `"shield"`)
}
