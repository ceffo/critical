package spellbookadpt_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
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
type books map[int64][]spellbook.Entry

func (b books) Keys(_ context.Context, characterID int64) ([]string, error) {
	keys := []string{}
	for _, e := range b[characterID] {
		keys = append(keys, e.Key)
	}
	return keys, nil
}

func (b books) Entries(_ context.Context, characterID int64) ([]spellbook.Entry, error) {
	return b[characterID], nil
}

// given is an entry of a spell given (ADR 0030 step 20.6).
func given(key string) spellbook.Entry {
	return spellbook.Entry{Key: key, Origin: spellbook.OriginGiven}
}

// Each character reads its own spellbook, named and levelled from the
// catalogue, in its stored order, each entry with its origin (ADR 0030 step
// 20.5); a character without one has none.
func TestBook_EachCharacterReadsItsOwnSpellbook(t *testing.T) {
	ctx := context.Background()
	book := spellbookadpt.New(books{
		1: {{Key: "fireball", Origin: spellbook.OriginLevelGrant, GrantLevel: 5}, {Key: "mirror_image", Origin: spellbook.OriginStarting}},
		2: {given("mirror_image")},
	}, cat)

	one, err := book.All(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.SpellbookEntry{
		{ID: "fireball", Name: "Fireball", Level: 3, Origin: spellbook.OriginLevelGrant, GrantLevel: 5},
		{ID: "mirror_image", Name: "Mirror Image", Level: 2, Origin: spellbook.OriginStarting},
	}, one)
	two, err := book.All(ctx, 2)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.SpellbookEntry{{ID: "mirror_image", Name: "Mirror Image", Level: 2, Origin: spellbook.OriginGiven}}, two)
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
	_, err := spellbookadpt.New(books{1: {given("shield")}}, cat).All(context.Background(), 1)
	require.ErrorContains(t, err, `"shield"`)
}

// A free spell's entry the catalogue no longer holds pays for no grant: Claimed
// skips it with a warning naming it instead of failing, since it runs on every
// read of a mage (ADR 0030 step 20.8); the other claims still count.
func TestBook_Step20_8_AClaimTheCatalogueLacksIsSkippedAndLogged(t *testing.T) {
	var logged bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	claimed, err := spellbookadpt.New(books{1: {
		{Key: "shield", Origin: spellbook.OriginLevelGrant, GrantLevel: 3},
		{Key: "fireball", Origin: spellbook.OriginLevelGrant, GrantLevel: 5},
		given("mirror_image"),
	}}, cat).Claimed(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, []spellbook.FreeSpell{{Level: 5, SpellLevel: 3}}, claimed)
	assert.Contains(t, logged.String(), "level=WARN")
	assert.Contains(t, logged.String(), "spell=shield")
	assert.Contains(t, logged.String(), "character=1")
}
