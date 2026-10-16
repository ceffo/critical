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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
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

// ADR 0021 step 9.7: the fixture CI runs on carries the PHB's real numbers,
// those of the real artifact's rule tables (which TestRepoSeeds_BootTheWholeStack
// boots on, the seeds holding none since step 9.6.3), so the PHB-page value
// tests on the fixture check the tables the app uses.
func TestRealCatalogue_Step9_7_TheFixtureCarriesTheRealArtifactsTables(t *testing.T) {
	realCatalogue(t)
	real, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	fixture, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	fromReal, err := ruletablesadpt.Parse(real.RuleTables)
	require.NoError(t, err)
	fromFixture, err := ruletablesadpt.Parse(fixture.RuleTables)
	require.NoError(t, err)
	// Compared by lookup, not row by row: the builder may group equal rows
	// differently (one row per book line) for the same numbers.
	for str := 1; str <= 25; str++ {
		for exc := 0; exc <= ruletablesvc.ExceptionalMax; exc++ {
			rh, rd, rerr := fromReal.StrengthMods(str, exc)
			fh, fd, ferr := fromFixture.StrengthMods(str, exc)
			require.NoError(t, rerr)
			require.NoError(t, ferr)
			assert.Equal(t, [2]int{rh, rd}, [2]int{fh, fd}, "STR %d/%d", str, exc)
		}
	}
	for dex := 1; dex <= 25; dex++ {
		rm, ra, rerr := fromReal.DexMods(dex)
		fm, fa, ferr := fromFixture.DexMods(dex)
		require.NoError(t, rerr)
		require.NoError(t, ferr)
		assert.Equal(t, [2]int{rm, ra}, [2]int{fm, fa}, "DEX %d", dex)
	}
	for level := 1; level <= ruletablesvc.MaxBardLevel; level++ {
		rs, err := fromReal.BardSpellsPerDay(level)
		require.NoError(t, err)
		fs, err := fromFixture.BardSpellsPerDay(level)
		require.NoError(t, err)
		assert.Equal(t, rs, fs, "bard spells, level %d", level)
		rx, err := fromReal.BardXPThreshold(level)
		require.NoError(t, err)
		fx, err := fromFixture.BardXPThreshold(level)
		require.NoError(t, err)
		assert.Equal(t, rx, fx, "XP, level %d", level)
	}
	_, err = os.Stat(filepath.Join(seedsDir, "rule_tables.json"))
	assert.ErrorIs(t, err, os.ErrNotExist, "the seeds hold no rule tables")
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
		// The English books' texts (the fallback language) are what the scrape produced.
		name, desc := s.Name.Default(), s.Description.Default()
		id := s.Class + " " + name
		k := key{strings.ToLower(name), s.Class, s.Level}
		assert.False(t, seen[k], "duplicate spell %s (same name, class and level)", id)
		seen[k] = true

		assert.Contains(t, []string{"Wizard", "Priest"}, s.Class, id)
		assert.GreaterOrEqual(t, s.Level, 1, id)
		assert.LessOrEqual(t, s.Level, 9, id)
		if s.Class == "Priest" {
			assert.NotEmpty(t, s.Sphere, "%s: priest spells have a sphere", id)
		}

		stats := map[string]string{
			"name": name, "school": s.School.Default(), "casting_time": s.CastingTime.Default(),
			"range": s.Range.Default(), "duration": s.Duration.Default(), "area_of_effect": s.AreaOfEffect.Default(),
			"components": s.Components.Default(), "saving_throw": s.SavingThrow.Default(),
		}
		for field, v := range stats {
			assert.NotEmpty(t, strings.TrimSpace(v), "%s: %s is empty", id, field)
			assert.Equal(t, strings.TrimSpace(v), v, "%s: %s has stray whitespace", id, field)
			assert.NotRegexp(t, leak, v, "%s: %s leaks a field label or a bad character", id, field)
			assert.LessOrEqual(t, len(v), 100, "%s: %s looks like leaked prose", id, field)
		}
		assert.NotRegexp(t, leak, s.Sphere.Default(), "%s: sphere", id)
		assert.NotEmpty(t, strings.TrimSpace(desc), "%s: description is empty", id)
		head := desc[:min(len(desc), 120)]
		assert.NotRegexp(t, `(Saving Throw|Area of Effect|Casting Time|Components?|Range|Duration|Reversible)\s*:`, head,
			"%s: description starts with a leaked stat line", id)
		first, _ := utf8.DecodeRuneInString(desc)
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
		"Varies": true, "Variable": true, "The wizard's staff": true,
		"Creates a sphere": true,
	},
	"components":   {"V, S, special": true},
	"saving_throw": {"Variable": true, "None (special)": true},
}

// specialAllowedCount pins how many spell fields are allow-listed: adding a
// spell whose stat falls back to special makes this fail until it is looked at.
const specialAllowedCount = 21

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
			"casting_time": sp.CastingTime.Default(), "range": sp.Range.Default(), "duration": sp.Duration.Default(),
			"area_of_effect": sp.AreaOfEffect.Default(), "components": sp.Components.Default(), "saving_throw": sp.SavingThrow.Default(),
		}
		stats := spellstats.Parse(spellstats.Raw{
			CastingTime: sp.CastingTime.Default(), Range: sp.Range.Default(), Duration: sp.Duration.Default(),
			AreaOfEffect: sp.AreaOfEffect.Default(), Components: sp.Components.Default(), SavingThrow: sp.SavingThrow.Default(),
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
			t.Errorf("%s: %s %q falls back to special (add a parser, or allow-list it)", sp.Name.Default(), field, raw)
		}
		assert.NotEmpty(t, stats.Components.Raw, sp.Name.Default())
		assert.True(t, stats.Components.V || stats.Components.S || stats.Components.M, "%s: components %q", sp.Name.Default(), sp.Components.Default())
	}
	assert.Equal(t, specialAllowedCount, allowedSeen, "allow-listed special fields in the catalogue")
}

// areaReadingCounts pins, over the real catalogue, how many area texts the
// parser reads as a choice between whole readings and how many it holds only
// in part (ADR 0021 step 8; 111 partial before alternatives, "up to",
// "circle", per-level steps and thousands separators were read). Like
// specialAllowedCount, a new artifact or a parser change that moves them
// fails here until it is looked at.
var areaReadingCounts = map[string]int{"alternatives": 26, "partial": 14}

// TestRealCatalogue_AreaReadingsAndPerLevelDurations checks how the parser
// reads the real catalogue's areas (the pinned counts above) and that no
// duration keeps a level abbreviation ("1 turn/lvl") as a divisor it does
// not apply.
func TestRealCatalogue_AreaReadingsAndPerLevelDurations(t *testing.T) {
	spells := realCatalogue(t).Spells
	got := map[string]int{}
	for _, sp := range spells {
		a := spellstats.ParseArea(sp.AreaOfEffect.Default())
		switch {
		case a.Kind == spellstats.AreaAlternatives:
			got["alternatives"]++
		case a.Partial:
			got["partial"]++
		}
		for _, term := range spellstats.ParseDuration(sp.Duration.Default()).Time.Terms {
			assert.NotRegexp(t, `(?i)^(lvl|level)`, term.Per, "%s: duration %q", sp.ID, sp.Duration.Default())
		}
	}
	t.Logf("area readings over %d spells: %v", len(spells), got)
	assert.Equal(t, areaReadingCounts, got)
}

// countStatsWrites installs, in db, a trigger counting the
// rows whose stats an UPDATE writes, and returns the count so far.
func countStatsWrites(t *testing.T, db *sqlite.DB) func() int {
	t.Helper()
	ctx := context.Background()
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS test_stats_writes (n INTEGER)`,
		`CREATE TRIGGER IF NOT EXISTS test_count_stats_writes AFTER UPDATE OF stats ON spells
			BEGIN INSERT INTO test_stats_writes VALUES (1); END`,
	} {
		_, err := db.ExecContext(ctx, q)
		require.NoError(t, err)
	}
	return func() int {
		var n int
		require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM test_stats_writes`).Scan(&n))
		return n
	}
}

// A restart on the same artifact version re-types stats an older parser
// stored (ADR 0021 step 5.6): every spell's stats are its stored texts
// parsed by the current parser, and a restart with nothing stale writes no
// row (counted by a trigger). The re-parse runs on every start; its time is
// logged.
func TestRealCatalogue_ARestartReTypesStaleStats(t *testing.T) {
	cat := realCatalogue(t)
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "retype.db")
	start := func() {
		t.Helper()
		bs, err := composition.New(ctx, composition.Config{DSN: dsn, SeedsDir: seedsDir, Catalogue: catalogueFile})
		require.NoError(t, err)
		require.NoError(t, bs.Close(ctx))
	}
	start()
	db, err := sqlite.Open(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, `UPDATE spells SET stats = '{}'`)
	require.NoError(t, err)
	writes := countStatsWrites(t, db)

	began := time.Now()
	start()
	assert.Equal(t, len(cat.Spells), writes(), "every stale row rewritten")
	t.Logf("restart re-typing %d spells: %s", len(cat.Spells), time.Since(began))
	stats := map[string]string{}
	rows, err := db.QueryContext(ctx, `SELECT key, stats FROM spells WHERE key IS NOT NULL`)
	require.NoError(t, err)
	for rows.Next() {
		var k, v string
		require.NoError(t, rows.Scan(&k, &v))
		stats[k] = v
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	for _, sp := range cat.Spells {
		want, err := json.Marshal(spellstats.Parse(spellstats.Raw{
			CastingTime: sp.CastingTime.Default(), Range: sp.Range.Default(), Duration: sp.Duration.Default(),
			AreaOfEffect: sp.AreaOfEffect.Default(), Components: sp.Components.Default(), SavingThrow: sp.SavingThrow.Default(),
		}))
		require.NoError(t, err)
		assert.JSONEq(t, string(want), stats[sp.ID], sp.ID)
	}

	began = time.Now()
	start()
	t.Logf("restart with nothing stale: %s", time.Since(began))
	assert.Equal(t, len(cat.Spells), writes(), "a restart with nothing stale writes no row")
}

// TestRealCatalogue_UpgradesACopyOfALiveDatabase imports the real catalogue
// into a copy of the database named by CRITICAL_UPGRADE_CHECK_DB (e.g. the
// dev database, .dev/critical.db), the way a restart of that database would:
// migrations, then ADR 0021 step 5 with the real seeds. The import must pass
// (the backfilled keys match the artifact, nothing referenced is removed)
// and afterwards every spell id the seeds and the game state hold resolves.
// The start changes no game state when no migration is pending (every table
// but the catalogue's, row for row), every keyed spell keeps its key, and
// the second start writes nothing at all (every table, row for row).
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
	db, err := sqlite.Open(dst)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	var writes func() int
	if hasSpells(t, db) {
		writes = countStatsWrites(t, db)
	}
	inventory := inventoryRows(t, db)
	migrations, state, keys := migrationNames(t, db), gameState(t, db), spellKeys(t, db)
	require.NotEmpty(t, state, "the copy holds game-state tables")
	bs, err := composition.New(ctx, composition.Config{DSN: dst, SeedsDir: seedsDir, Catalogue: catalogueFile})
	require.NoError(t, err, "the live database upgrades")
	if slices.Equal(migrations, migrationNames(t, db)) {
		assert.Equal(t, state, gameState(t, db), "no migration pending: the start changes no game state")
	} else {
		t.Logf("migrations ran on the copy: game state not compared across the first start")
	}
	ids := cat.ByID()
	after := spellKeys(t, db)
	for id, key := range keys {
		if _, kept := ids[key]; kept {
			assert.Equal(t, key, after[id], "spell row %d keeps its key", id)
		}
	}
	t.Logf("%d keyed spells before the start, %d after", len(keys), len(after))
	if inventory != nil {
		// ADR 0023 steps 9 and 10: every item and weapon keeps its id,
		// position and counts, and has a name.
		assert.Equal(t, inventory, inventoryRows(t, db), "items and weapons keep their ids, positions and counts")
		items, err := bs.Items.List(ctx)
		require.NoError(t, err)
		for _, it := range items {
			assert.NotEmpty(t, it.Name.Any(), "item %s has a name", it.ID)
			t.Logf("item %s: %v", it.ID, it.Name)
		}
		weapons, err := bs.Combat.ListWeapons(ctx)
		require.NoError(t, err)
		for _, w := range weapons {
			assert.NotEmpty(t, w.Name.Any(), "weapon %s has a name", w.ID)
			t.Logf("weapon %s: %v", w.ID, w.Name)
		}
	}
	page, err := bs.Spells.Search(ctx, spellsvc.Filter{}, spellsvc.PageRequest{PageSize: 1})
	require.NoError(t, err)
	assert.Equal(t, len(cat.Spells), page.Total, "every catalogue spell is served")
	require.NoError(t, bs.Close(ctx))
	first := 0
	if writes != nil {
		first = writes()
	}
	writes = countStatsWrites(t, db) // again: a migration that rebuilt spells dropped the trigger
	first = max(first, writes())
	t.Logf("first start wrote the stats of %d spells", first)
	whole := tableRows(t, db, nil)
	bs, err = composition.New(ctx, composition.Config{DSN: dst, SeedsDir: seedsDir, Catalogue: catalogueFile})
	require.NoError(t, err, "the second start")
	require.NoError(t, bs.Close(ctx))
	assert.Equal(t, first, writes(), "the second start writes no stats")
	assert.Equal(t, whole, tableRows(t, db, nil), "the second start writes nothing")
	refs, err := sqlite.StateSpellRefs(ctx, db)
	require.NoError(t, err)
	book, err := spellbookadpt.New(seedsDir, cat)
	require.NoError(t, err)
	for _, e := range book.All() {
		refs = append(refs, catalogue.Ref{ID: e.ID, Where: "spellbook seed"})
	}
	for _, r := range refs {
		var n int
		require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM spells WHERE key = ?", r.ID).Scan(&n))
		_, inArtifact := ids[r.ID]
		assert.True(t, n == 1 && inArtifact, "%s (%s) resolves", r.ID, r.Where)
	}
	t.Logf("%d spell references in the seeds and the game state, all resolved", len(refs))
}

// catalogueTables are the tables the catalogue import owns (ADR 0021 step
// 2), with the migrations' bookkeeping and the tests' trigger table: every
// other table is game state.
var catalogueTables = map[string]bool{
	"spells": true, "catalogue_meta": true, "localized_texts": true,
	"schema_migrations": true, "test_stats_writes": true,
}

// spellTextColumns are the spells' columns that key localized_texts rows
// (migration 031): those rows are the catalogue's, the others (item and
// weapon names) game state.
var spellTextColumns = []string{
	"name", "description", "school", "sphere", "casting_time", "range", "duration",
	"area_of_effect", "components", "saving_throw",
}

// tableRows is every row of every table but those skip names, by table: each
// row printed in column order, the rows sorted.
func tableRows(t *testing.T, db *sqlite.DB, skip map[string]bool) map[string][]string {
	t.Helper()
	ctx := context.Background()
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	require.NoError(t, err)
	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		if !skip[name] {
			tables = append(tables, name)
		}
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	out := make(map[string][]string, len(tables))
	for _, table := range tables {
		out[table] = queryRows(t, db, `SELECT * FROM "`+table+`"`)
	}
	return out
}

// queryRows runs query and prints each row's columns in order, sorted.
func queryRows(t *testing.T, db *sqlite.DB, query string) []string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), query)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	require.NoError(t, err)
	out := []string{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		cells := make([]string, len(vals))
		for i, v := range vals {
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			cells[i] = fmt.Sprintf("%#v", v)
		}
		out = append(out, strings.Join(cells, "|"))
	}
	require.NoError(t, rows.Err())
	slices.Sort(out)
	return out
}

// gameState is every game-state row: every table the catalogue import does
// not own, and the localized texts no spell keys.
func gameState(t *testing.T, db *sqlite.DB) map[string][]string {
	t.Helper()
	state := tableRows(t, db, catalogueTables)
	if hasTable(t, db, "localized_texts") {
		owned := make([]string, len(spellTextColumns))
		for i, c := range spellTextColumns {
			owned[i] = `SELECT "` + c + `" FROM spells WHERE "` + c + `" IS NOT NULL`
		}
		state["localized_texts"] = queryRows(t, db,
			`SELECT * FROM localized_texts WHERE text_id NOT IN (`+strings.Join(owned, " UNION ")+`)`)
	}
	return state
}

// migrationNames are the migrations the database recorded (none before the
// first one).
func migrationNames(t *testing.T, db *sqlite.DB) []string {
	t.Helper()
	if !hasTable(t, db, "schema_migrations") {
		return nil
	}
	return queryRows(t, db, `SELECT name FROM schema_migrations`)
}

// spellKeys maps each keyed spell row to its key (none before the catalogue
// added the column).
func spellKeys(t *testing.T, db *sqlite.DB) map[int64]string {
	t.Helper()
	out := map[int64]string{}
	if !hasTable(t, db, "spells") {
		return out
	}
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM pragma_table_info('spells') WHERE name = 'key'`).Scan(&n))
	if n == 0 {
		return out
	}
	rows, err := db.QueryContext(context.Background(), `SELECT id, key FROM spells WHERE key IS NOT NULL`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var key string
		require.NoError(t, rows.Scan(&id, &key))
		out[id] = key
	}
	require.NoError(t, rows.Err())
	return out
}

// hasTable reports whether db has the table name.
func hasTable(t *testing.T, db *sqlite.DB, name string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n))
	return n > 0
}

// inventoryRows are the items' and weapons' ids, positions and counts (nil
// for a database older than those tables).
func inventoryRows(t *testing.T, db *sqlite.DB) []string {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('items', 'weapons')`).Scan(&n))
	if n < 2 {
		return nil
	}
	rows, err := db.QueryContext(context.Background(), `
		SELECT 'item ' || id || ' ' || position || ' ' || count || '/' || max_count FROM items
		UNION ALL SELECT 'weapon ' || id || ' ' || position FROM weapons ORDER BY 1`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var r string
		require.NoError(t, rows.Scan(&r))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// hasSpells reports whether db already has the spells table (a database
// older than every migration does not).
func hasSpells(t *testing.T, db *sqlite.DB) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'spells'`).Scan(&n))
	return n > 0
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
