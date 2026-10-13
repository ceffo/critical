// Package seedcheck holds the integrity tests for the repo's real seeds/
// directory. They check only that the data is well formed (it loads, it
// matches the formats the adapters parse, references resolve). They assert no
// campaign values: editing a name, a stat, a spell number or a table entry
// must never break them. Every other test uses api/testdata/seeds.
package seedcheck_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var seedsDir = filepath.Join("..", "..", "..", "seeds")

func readJSON(t *testing.T, name string, into any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(seedsDir, name)) //nolint:gosec // fixed test path
	require.NoError(t, err, name)
	require.NoError(t, json.Unmarshal(raw, into), name)
}

func TestRepoSeeds_BootTheWholeStack(t *testing.T) {
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:      filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir: seedsDir,
	})
	require.NoError(t, err, "seeds load, migrate and parse")
	t.Cleanup(func() { _ = bs.Close(ctx) })

	char, err := bs.Character.GetCharacter(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.NotEmpty(t, char.Name)
	weapons, err := bs.Combat.ListWeapons(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, weapons)
}

type spell struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	Level int    `json:"level"`
}

func TestRepoSeeds_EveryWizardSpellHasAnEffectAtItsLevel(t *testing.T) {
	var spells []spell
	readJSON(t, "spells.json", &spells)
	effects, err := spelleffectsadpt.New(seedsDir)
	require.NoError(t, err)

	wizard := map[string]int{}
	for _, s := range spells {
		if s.Class == "Wizard" {
			wizard[spelleffectsvc.Slug(s.Name)] = s.Level
		}
	}
	require.NotEmpty(t, wizard)
	assert.Equal(t, len(wizard), effects.Len(), "one effect entry per Wizard spell")
	for id, level := range wizard {
		if e, ok := effects.Get(id); assert.True(t, ok, "no effect for %s", id) {
			assert.Equal(t, level, e.Level, id)
		}
	}
	for _, e := range effects.All() {
		_, ok := wizard[e.ID]
		assert.True(t, ok, "effect %s is not a Wizard spell in spells.json", e.ID)
	}
}

func TestRepoSeeds_SpellbookSpellsResolve(t *testing.T) {
	book, err := spellbookadpt.New(seedsDir)
	require.NoError(t, err, "spellbook ids/levels match the catalog")
	effects, err := spelleffectsadpt.New(seedsDir)
	require.NoError(t, err)
	for _, e := range book.All() {
		_, ok := effects.Get(e.ID)
		assert.True(t, ok, "spellbook spell %s has no effect entry", e.ID)
	}
}

func TestRepoSeeds_ItemsReferenceRealWeapons(t *testing.T) {
	var weapons []struct {
		ID       string `json:"id"`
		AmmoKind string `json:"ammo_kind"`
	}
	var items []struct {
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		AmmoKind string `json:"ammo_kind"`
	}
	readJSON(t, "weapons.json", &weapons)
	readJSON(t, "items.json", &items)
	launchers := map[string]bool{}
	for _, w := range weapons {
		if w.AmmoKind != "" {
			launchers[w.AmmoKind] = true
		}
	}
	for _, it := range items {
		if it.Kind == "ammo" {
			assert.True(t, launchers[it.AmmoKind], "ammo %s fits no weapon", it.ID)
		}
	}
}

// catalogSpell is the full seeds/spells.json row; every stat is a string.
type catalogSpell struct {
	Name         string `json:"name"`
	Class        string `json:"class"`
	Level        int    `json:"level"`
	School       string `json:"school"`
	Sphere       string `json:"sphere"`
	CastingTime  string `json:"casting_time"`
	Range        string `json:"range"`
	Duration     string `json:"duration"`
	AreaOfEffect string `json:"area_of_effect"`
	Components   string `json:"components"`
	SavingThrow  string `json:"saving_throw"`
	Description  string `json:"description"`
}

func TestRepoSeeds_SpellCatalogInvariants(t *testing.T) {
	var spells []catalogSpell
	readJSON(t, "spells.json", &spells)
	require.NotEmpty(t, spells)

	// A field label ("Range:", "Component: V"), a half-decoded fraction ("_")
	// or a stat line swallowed by the neighbouring field means the scrape
	// glued two fields together.
	leak := regexp.MustCompile(`(?i)_|\b(range|duration|casting time|saving throw|area of effect|components?|reversible|sphere)\s*:|Reversible`)

	type key struct{ name, class string }
	seen := map[key]bool{}
	for _, s := range spells {
		id := s.Class + " " + s.Name
		k := key{strings.ToLower(s.Name), s.Class}
		assert.False(t, seen[k], "duplicate spell %s", id)
		seen[k] = true

		assert.Contains(t, []string{"Wizard", "Priest"}, s.Class, id)
		assert.GreaterOrEqual(t, s.Level, 1, id)
		assert.LessOrEqual(t, s.Level, 9, id)
		if s.Class == "Priest" {
			assert.NotEmpty(t, s.Sphere, "%s: priest spells have a sphere", id)
		}

		stats := map[string]string{
			"name": s.Name, "school": s.School, "casting_time": s.CastingTime,
			"range": s.Range, "duration": s.Duration, "area_of_effect": s.AreaOfEffect,
			"components": s.Components, "saving_throw": s.SavingThrow,
		}
		for field, v := range stats {
			assert.NotEmpty(t, strings.TrimSpace(v), "%s: %s is empty", id, field)
			assert.Equal(t, strings.TrimSpace(v), v, "%s: %s has stray whitespace", id, field)
			assert.NotRegexp(t, leak, v, "%s: %s leaks a field label or a bad character", id, field)
			assert.LessOrEqual(t, len(v), 100, "%s: %s looks like leaked prose", id, field)
		}
		assert.NotRegexp(t, leak, s.Sphere, "%s: sphere", id)
		assert.NotEmpty(t, strings.TrimSpace(s.Description), "%s: description is empty", id)
		head := s.Description[:min(len(s.Description), 120)]
		assert.NotRegexp(t, `(Saving Throw|Area of Effect|Casting Time|Components?|Range|Duration|Reversible)\s*:`, head,
			"%s: description starts with a leaked stat line", id)
		first, _ := utf8.DecodeRuneInString(s.Description)
		assert.False(t, unicode.IsLower(first), "%s: description starts mid-sentence (a stat swallowed its first words)", id)
	}
}

// specialAllowed lists the raw PHB stat texts that legitimately parse to a
// special kind although they do not literally say "Special": prose the book
// gives instead of a measurable value. Keyed by field, then raw text. Any
// other fallback to special in seeds/spells.json fails the test, so a new
// stat form must get a parser (or an entry here, on purpose).
var specialAllowed = map[string]map[string]bool{
	"casting_time": {},
	// "1": the bare "1" of Speak With Dead (1" indoors). `1"`: Life Force Transfer, same unit.
	// "Change": a scrape glitch on Death Pact (nothing in the scrape repairs it).
	"range":    {"1": true, `1"`: true, "Change": true},
	"duration": {"Time of chanting": true, "Variable": true},
	// Area prose the book gives instead of a measurable value (counts, "variable", or a
	// shape the area parser does not model); shown as written.
	"area_of_effect": {
		"Circle of priests": true, "Hearing range": true, "Line of sight": true, "Script reader": true,
		"Varies": true, "Variable": true, "1d6 creatures/level": true, `3" diameter circle`: true,
		"30' long + 5'/level": true, "1–7 targets": true, "The wizard's staff": true, "Creates 1d4+1": true,
		"Two or more creatures": true, "Create 7 eyes": true, "Creates a sphere": true,
	},
	"components":   {},
	"saving_throw": {"Variable": true}, // Wildfire: the effect decides
}

// TestRepoSeeds_EverySpellStatParses checks parseability only: no spell
// field may fall back to special unless the PHB text says so ("Special",
// "... special") or it is in the allow-list above. It asserts no values.
func TestRepoSeeds_EverySpellStatParses(t *testing.T) {
	var spells []struct {
		Name         string `json:"name"`
		CastingTime  string `json:"casting_time"`
		Range        string `json:"range"`
		Duration     string `json:"duration"`
		AreaOfEffect string `json:"area_of_effect"`
		Components   string `json:"components"`
		SavingThrow  string `json:"saving_throw"`
	}
	readJSON(t, "spells.json", &spells)
	require.NotEmpty(t, spells)

	for _, sp := range spells {
		raws := map[string]string{
			"casting_time": sp.CastingTime, "range": sp.Range, "duration": sp.Duration,
			"area_of_effect": sp.AreaOfEffect, "components": sp.Components, "saving_throw": sp.SavingThrow,
		}
		stats := spellstats.Parse(spellstats.Raw{
			CastingTime: sp.CastingTime, Range: sp.Range, Duration: sp.Duration,
			AreaOfEffect: sp.AreaOfEffect, Components: sp.Components, SavingThrow: sp.SavingThrow,
		})
		for _, field := range stats.Special() {
			raw := raws[field]
			if strings.Contains(strings.ToLower(raw), "special") || specialAllowed[field][raw] {
				continue
			}
			t.Errorf("%s: %s %q falls back to special (add a parser, or allow-list it)", sp.Name, field, raw)
		}
		assert.NotEmpty(t, stats.Components.Raw, sp.Name)
		assert.True(t, stats.Components.V || stats.Components.S || stats.Components.M, "%s: components %q", sp.Name, sp.Components)
	}
}
