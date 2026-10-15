// Package seedcheck holds the integrity tests for the repo's real seeds/
// directory and the real spell catalogue artifact (.catalogue/catalogue.json,
// fetched by `just catalogue`). They check only that the data is well formed
// (it loads, it matches the formats the adapters parse, references resolve).
// They assert no campaign values: editing a name, a stat, a spell number or a
// table entry must never break them. Every other test uses api/testdata.
//
// The real seeds refer to spells only the real catalogue holds, so every test
// that needs spells is skipped when the artifact is absent (CI has no access
// to the private rulebooks repo; it runs on the fixture elsewhere).
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
	"github.com/ceffo/critical/api/internal/storage/sqlite"
)

var (
	seedsDir      = filepath.Join("..", "..", "..", "seeds")
	catalogueFile = filepath.Join("..", "..", "..", ".catalogue", "catalogue.json")
)

// realCatalogue loads the real artifact, or skips the test without it.
func realCatalogue(t *testing.T) catalogue.Catalogue {
	t.Helper()
	if _, err := os.Stat(catalogueFile); err != nil {
		t.Skipf("no real spell catalogue at %s (run `just catalogue`): %v", catalogueFile, err)
	}
	c, err := catalogueadpt.Load(catalogueFile)
	require.NoError(t, err)
	return c
}

func readJSON(t *testing.T, name string, into any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(seedsDir, name)) //nolint:gosec // fixed test path
	require.NoError(t, err, name)
	require.NoError(t, json.Unmarshal(raw, into), name)
}

func TestRepoSeeds_BootTheWholeStack(t *testing.T) {
	realCatalogue(t)
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:       filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir:  seedsDir,
		Catalogue: catalogueFile,
	})
	require.NoError(t, err, "seeds load, migrate and parse")
	t.Cleanup(func() { _ = bs.Close(ctx) })

	char, err := bs.Character.GetCharacter(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.NotEmpty(t, char.Name)
	page, err := bs.Spells.Search(ctx, spellsvc.Filter{}, spellsvc.PageRequest{PageSize: 1})
	require.NoError(t, err)
	assert.Equal(t, len(realCatalogue(t).Spells), page.Total, "every catalogue spell is served")
	weapons, err := bs.Combat.ListWeapons(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, weapons)
}

func TestRepoSeeds_EveryWizardSpellHasAnEffectAtItsLevel(t *testing.T) {
	cat := realCatalogue(t)
	effects, err := spelleffectsadpt.New(seedsDir)
	require.NoError(t, err)

	wizard := map[string]int{}
	for _, s := range cat.Spells {
		if s.Class == "Wizard" {
			wizard[s.ID] = s.Level
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
		assert.True(t, ok, "effect %s is not a Wizard spell of the catalogue", e.ID)
	}
}

// The real seeds with one spell effect the real catalogue does not hold do
// not start: startup names the unknown id.
func TestRealCatalogue_AnUnknownSpellEffectIDRefusesTheStartup(t *testing.T) {
	realCatalogue(t)
	seeds := t.TempDir()
	require.NoError(t, os.CopyFS(seeds, os.DirFS(seedsDir)))
	path := filepath.Join(seeds, spelleffectsadpt.FileName)
	raw, err := os.ReadFile(path) //nolint:gosec // test temp dir
	require.NoError(t, err)
	var entries []json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &entries))
	entries = append(entries, json.RawMessage(`{"id":"magic_rocket","name":"Magic Rocket","level":1,`+
		`"kind":"utility","confidence":"high","target":"none","duration":{"kind":"instant"}}`))
	raw, err = json.Marshal(entries)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	_, err = composition.New(context.Background(), composition.Config{
		DSN: filepath.Join(t.TempDir(), "critical.db"), SeedsDir: seeds, Catalogue: catalogueFile,
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "spell effects: ids not in the spell catalogue: magic_rocket")
}

func TestRepoSeeds_SpellbookSpellsResolve(t *testing.T) {
	book, err := spellbookadpt.New(seedsDir, realCatalogue(t))
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
		if it.AmmoKind != "" {
			assert.True(t, launchers[it.AmmoKind], "ammo %s fits no weapon", it.ID)
		}
	}
}

// TestRepoSeeds_SpellCatalogInvariants checks what the schema cannot: the
// raw stat texts hold no glued field labels or leaked prose, Priest spells
// have a sphere, and no (name, class, level) appears twice.
func TestRepoSeeds_SpellCatalogInvariants(t *testing.T) {
	spells := realCatalogue(t).Spells
	require.NotEmpty(t, spells)

	// A field label ("Range:", "Component: V"), a half-decoded fraction ("_")
	// or a stat line swallowed by the neighbouring field means the scrape
	// glued two fields together.
	leak := regexp.MustCompile(`(?i)_|\b(range|duration|casting time|saving throw|area of effect|components?|reversible|sphere)\s*:|Reversible`)

	type key struct {
		name, class string
		level       int
	}
	seen := map[key]bool{}
	for _, s := range spells {
		id := s.Class + " " + s.Name
		k := key{strings.ToLower(s.Name), s.Class, s.Level}
		assert.False(t, seen[k], "duplicate spell %s (same name, class and level)", id)
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

// specialAllowed lists the raw PHB stat texts that parse to a special kind
// although they are not the literal word "Special". Keyed by field, then raw
// text. Two groups: prose the book gives instead of a measurable value, and
// "Special (...)" texts that keep their number in the parsed Time but stay
// special because the book says so. Any other non-literal fallback to
// special in the catalogue fails the test, so a new stat form must get a
// parser (or an entry here, on purpose).
var specialAllowed = map[string]map[string]bool{
	"casting_time": {"Special (8 turns)": true, "Special + 1": true},
	// "Change": a scrape glitch on Death Pact, which is not in the PHB text we hold.
	"range": {"Change": true, "Infinite + special": true},
	"duration": {
		"Time of chanting": true, "Variable": true,
		"Special (up to three rounds)": true, "Special (2 rounds/level maximum)": true, "Special; max. 10 rounds": true,
	},
	"area_of_effect": {
		"Circle of priests": true, "Hearing range": true, "Line of sight": true, "Script reader": true,
		"Varies": true, "Variable": true, "The wizard's staff": true, "Two or more creatures": true,
		"Creates a sphere": true,
	},
	"components":   {"V, S, special": true},
	"saving_throw": {"Variable": true, "None (special)": true},
}

// specialAllowedCount pins how many spell fields are allow-listed: adding a
// spell whose stat falls back to special makes this fail until it is looked at.
const specialAllowedCount = 22

// TestRepoSeeds_EverySpellStatParses checks parseability only: no spell
// field may fall back to special unless the PHB text is the literal word
// "Special" or it is in the allow-list above, whose size is pinned. It
// asserts no values.
func TestRepoSeeds_EverySpellStatParses(t *testing.T) {
	spells := realCatalogue(t).Spells
	require.NotEmpty(t, spells)

	allowedSeen := 0
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
			if strings.EqualFold(strings.TrimRight(strings.TrimSpace(raw), "."), "special") {
				continue
			}
			if specialAllowed[field][raw] {
				allowedSeen++
				continue
			}
			t.Errorf("%s: %s %q falls back to special (add a parser, or allow-list it)", sp.Name, field, raw)
		}
		assert.NotEmpty(t, stats.Components.Raw, sp.Name)
		assert.True(t, stats.Components.V || stats.Components.S || stats.Components.M, "%s: components %q", sp.Name, sp.Components)
	}
	assert.Equal(t, specialAllowedCount, allowedSeen, "allow-listed special fields in the catalogue")
}

// TestRealCatalogue_UpgradesACopyOfALiveDatabase imports the real catalogue
// into a copy of the database named by CRITICAL_UPGRADE_CHECK_DB (e.g. the
// dev database, .dev/critical.db), the way a restart of that database would:
// migrations, then ADR 0021 step 5 with the real seeds. The import must pass
// (the backfilled keys match the artifact, nothing referenced is removed)
// and afterwards every spell id the seeds and the game state hold resolves.
// The database itself is only read. Skipped unless the variable is set.
func TestRealCatalogue_UpgradesACopyOfALiveDatabase(t *testing.T) {
	src := os.Getenv("CRITICAL_UPGRADE_CHECK_DB")
	if src == "" {
		t.Skip("set CRITICAL_UPGRADE_CHECK_DB to a database file to check its upgrade")
	}
	cat := realCatalogue(t)
	dst := filepath.Join(t.TempDir(), "copy.db")
	for _, suffix := range []string{"", "-wal"} {
		raw, err := os.ReadFile(filepath.Clean(src + suffix))
		if suffix != "" && os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(dst+suffix, raw, 0o600))
	}

	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{DSN: dst, SeedsDir: seedsDir, Catalogue: catalogueFile})
	require.NoError(t, err, "the live database upgrades")
	page, err := bs.Spells.Search(ctx, spellsvc.Filter{}, spellsvc.PageRequest{PageSize: 1})
	require.NoError(t, err)
	assert.Equal(t, len(cat.Spells), page.Total, "every catalogue spell is served")
	require.NoError(t, bs.Close(ctx))

	db, err := sqlite.Open(dst)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	refs, err := sqlite.StateSpellRefs(ctx, db)
	require.NoError(t, err)
	book, err := spellbookadpt.New(seedsDir, cat)
	require.NoError(t, err)
	for _, e := range book.All() {
		refs = append(refs, catalogue.Ref{ID: e.ID, Where: "spellbook seed"})
	}
	ids := cat.ByID()
	for _, r := range refs {
		var n int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM spells WHERE key = ?", r.ID).Scan(&n))
		_, inArtifact := ids[r.ID]
		assert.True(t, n == 1 && inArtifact, "%s (%s) resolves", r.ID, r.Where)
	}
	t.Logf("%d spell references in the seeds and the game state, all resolved", len(refs))
}

// One database switched between the fixture catalogue (with the fixture
// seeds) and the real one: every switch imports, and a spell only the real
// catalogue holds that game state refers to blocks the switch back to the
// fixture, with a message naming the catalogue the database was built from
// and `just catalogue`, and leaves the database as it was.
func TestRealCatalogue_SwitchingADatabaseBetweenTheFixtureAndTheRealCatalogue(t *testing.T) {
	realCat := realCatalogue(t)
	fixtureFile := filepath.Join("..", "..", "testdata", "catalogue.json")
	fixtureSeeds := filepath.Join("..", "..", "testdata", "seeds")
	fixture, err := catalogueadpt.Load(fixtureFile)
	require.NoError(t, err)
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "scratch.db")
	start := func(seeds, file string) (*composition.BackingServices, error) {
		return composition.New(ctx, composition.Config{DSN: dsn, SeedsDir: seeds, Catalogue: file})
	}
	served := func(bs *composition.BackingServices) int {
		page, err := bs.Spells.Search(ctx, spellsvc.Filter{}, spellsvc.PageRequest{PageSize: 1})
		require.NoError(t, err)
		return page.Total
	}

	bs, err := start(fixtureSeeds, fixtureFile)
	require.NoError(t, err, "fixture")
	assert.Equal(t, len(fixture.Spells), served(bs))
	require.NoError(t, bs.Close(ctx))

	bs, err = start(seedsDir, catalogueFile)
	require.NoError(t, err, "fixture -> real")
	assert.Equal(t, len(realCat.Spells), served(bs))
	inFixture := fixture.ByID()
	var realOnly string
	for _, s := range realCat.Spells {
		if _, ok := inFixture[s.ID]; !ok {
			realOnly = s.ID
			break
		}
	}
	require.NotEmpty(t, realOnly)
	_, err = bs.DB.ExecContext(ctx, `INSERT INTO action_log (character_id, round, kind, summary, hp_delta, payload, transaction_id)
		VALUES (?, 0, 'cast', '', 0, ?, 999999)`, bs.CharacterID, `{"action":"cast","spellId":"`+realOnly+`","casterLevel":5}`)
	require.NoError(t, err)
	require.NoError(t, bs.Close(ctx))

	_, err = start(fixtureSeeds, fixtureFile)
	require.Error(t, err, "real -> fixture with a real-only spell in the log")
	assert.ErrorContains(t, err, realOnly+" (action_log ")
	assert.ErrorContains(t, err, "This database was built from catalogue version "+realCat.Version)
	assert.ErrorContains(t, err, "`just catalogue`")
	bs, err = start(seedsDir, catalogueFile)
	require.NoError(t, err, "the refused switch left the real catalogue in place")
	assert.Equal(t, len(realCat.Spells), served(bs))
	_, err = bs.DB.ExecContext(ctx, `DELETE FROM action_log WHERE transaction_id = 999999`)
	require.NoError(t, err)
	require.NoError(t, bs.Close(ctx))

	bs, err = start(fixtureSeeds, fixtureFile)
	require.NoError(t, err, "real -> fixture once nothing refers to the real-only spells")
	assert.Equal(t, len(fixture.Spells), served(bs))
	require.NoError(t, bs.Close(ctx))
}

// The web's list of PHB spell ids (web/src/i18n/phbSpellIds.json, the ids
// the fr spell names cover) names catalogue spells only.
func TestRealCatalogue_WebPHBSpellIdsAreCatalogueSpells(t *testing.T) {
	ids := realCatalogue(t).ByID()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "i18n", "phbSpellIds.json"))
	require.NoError(t, err)
	var phb []string
	require.NoError(t, json.Unmarshal(raw, &phb))
	require.NotEmpty(t, phb)
	for _, id := range phb {
		_, ok := ids[id]
		assert.True(t, ok, "%s is not a catalogue spell", id)
	}
}
