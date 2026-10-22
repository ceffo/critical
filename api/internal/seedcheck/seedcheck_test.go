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
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

var (
	seedsDir = filepath.Join("..", "..", "..", "seeds")
	// fixtureSeedsDir is the test fixture's seeds (api/testdata/seeds).
	fixtureSeedsDir = filepath.Join("..", "..", "testdata", "seeds")
	catalogueFile   = filepath.Join("..", "..", "..", ".catalogue", "catalogue.json")
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

// The real seeds and the real artifact load through the adapters the server
// starts with, without a database: the spell effects and the spellbook parse
// and every spell they name is in the catalogue (ADR 0021 step 5.4.3), the
// artifact's rule tables parse, and the seed files the storage reads are
// well-formed JSON. What the storage does with them (inserting, importing
// the catalogue) is checked at boot, by `just dev` and on staging, not in a
// Go test (ADR 0025 steps 6 and 7).
func TestRepoSeeds_LoadThroughTheServersAdapters(t *testing.T) {
	cat := realCatalogue(t)
	artifact, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	assert.NotEmpty(t, artifact.Catalogue.Spells)
	tables, err := ruletablesadpt.Parse(artifact.RuleTables, artifact.Equipment, artifact.Languages, artifact.Definitions)
	require.NoError(t, err, "the artifact's rule tables")
	defs, err := tables.Definitions()
	require.NoError(t, err, "the artifact's class definitions")

	effects, err := spelleffectsadpt.New(seedsDir)
	require.NoError(t, err)
	require.NoError(t, effects.CheckAgainst(cat), "every spell effect is a catalogue spell")
	book, err := spellbookadpt.ReadSeed(seedsDir, cat)
	require.NoError(t, err)
	assert.NotEmpty(t, book)
	_, err = spellbookadpt.ReadStartingSeed(seedsDir, cat, defs)
	require.NoError(t, err, "the starting spellbook's pre-ticks are wizard spells of the catalogue")

	for _, name := range []string{"character.json", "weapons.json", "items.json", "skills.json"} {
		var doc json.RawMessage
		readJSON(t, name, &doc)
		assert.NotEmpty(t, doc, name)
	}
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
	fromReal, err := ruletablesadpt.Parse(real.RuleTables, real.Equipment, real.Languages, real.Definitions)
	require.NoError(t, err)
	fromFixture, err := ruletablesadpt.Parse(fixture.RuleTables, fixture.Equipment, fixture.Languages, fixture.Definitions)
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

// ADR 0021 step 10.5: the fixture's class and race tables are the real
// artifact's, row for row (both are the builder's rows), so the PHB-page
// value tests on the fixture check the tables the app uses. Skipped while
func TestRealCatalogue_Step10_5_TheFixtureCarriesTheRealArtifactsClassTables(t *testing.T) {
	realCatalogue(t)
	real, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	fixture, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	fromReal, err := ruletablesadpt.Parse(real.RuleTables, real.Equipment, real.Languages, real.Definitions)
	require.NoError(t, err)
	fromFixture, err := ruletablesadpt.Parse(fixture.RuleTables, fixture.Equipment, fixture.Languages, fixture.Definitions)
	require.NoError(t, err)
	rc, ok := fromReal.Classes()
	require.True(t, ok)
	fc, ok := fromFixture.Classes()
	require.True(t, ok)
	assert.Equal(t, rc, fc)
}

// ADR 0021 step 11.5: the fixture's racial level limits are the real
// artifact's, cell for cell, so the DMG-page value tests on the fixture check
// the table the app uses.
func TestRealCatalogue_Step11_5_TheFixtureCarriesTheRealArtifactsLevelLimits(t *testing.T) {
	realCatalogue(t)
	real, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	fixture, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	fromReal, err := ruletablesadpt.Parse(real.RuleTables, real.Equipment, real.Languages, real.Definitions)
	require.NoError(t, err)
	fromFixture, err := ruletablesadpt.Parse(fixture.RuleTables, fixture.Equipment, fixture.Languages, fixture.Definitions)
	require.NoError(t, err)
	rl, ok := fromReal.LevelLimits()
	require.True(t, ok)
	fl, ok := fromFixture.LevelLimits()
	require.True(t, ok)
	defs, err := fromReal.Definitions()
	require.NoError(t, err)
	for _, race := range defs.RaceIDs() {
		for _, class := range defs.ClassIDs() {
			r, rok := rl.Limit(race, class)
			f, fok := fl.Limit(race, class)
			require.Equal(t, rok, fok, "%s %s", race, class)
			assert.Equal(t, r, f, "%s %s", race, class)
		}
	}
}

// ADR 0021 step 13.5: the fixture's equipment is the real artifact's, row for
// row but for the names (invented in the fixture), and Tables 52 and 56 cell for
// cell, so tests on the fixture check the numbers the app uses. Skipped while the
// real artifact is before format 7.
func TestRealCatalogue_Step13_5_TheFixtureCarriesTheRealArtifactsEquipment(t *testing.T) {
	realCatalogue(t)
	real, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	fixture, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	fromReal, err := ruletablesadpt.Parse(real.RuleTables, real.Equipment, real.Languages, real.Definitions)
	require.NoError(t, err)
	fromFixture, err := ruletablesadpt.Parse(fixture.RuleTables, fixture.Equipment, fixture.Languages, fixture.Definitions)
	require.NoError(t, err)
	re, ok := fromReal.Equipment()
	require.True(t, ok)
	fe, ok := fromFixture.Equipment()
	require.True(t, ok)
	nameless := func(e *ruletablesvc.Equipment) ruletablesvc.Equipment {
		out := ruletablesvc.Equipment{
			Weapons:    append([]ruletablesvc.Weapon(nil), e.Weapons...),
			Ammunition: append([]ruletablesvc.Ammunition(nil), e.Ammunition...),
			Armour:     append([]ruletablesvc.Armour(nil), e.Armour...),
			Shields:    append([]ruletablesvc.Shield(nil), e.Shields...),
		}
		for i := range out.Weapons {
			out.Weapons[i].Name = nil
		}
		for i := range out.Ammunition {
			out.Ammunition[i].Name = nil
		}
		for i := range out.Armour {
			out.Armour[i].Name = nil
		}
		for i := range out.Shields {
			out.Shields[i].Name = nil
		}
		return out
	}
	assert.Equal(t, nameless(re), nameless(fe))
	for _, row := range ruletablesvc.ArmorRows {
		r, rok := mustTable52(t, fromReal).Row(row)
		f, fok := mustTable52(t, fromFixture).Row(row)
		require.True(t, rok && fok, row)
		assert.Equal(t, r, f, row)
	}
	for _, kind := range ruletablesvc.MagicKinds {
		ri, _ := fromReal.InitiativeItems()
		fi, _ := fromFixture.InitiativeItems()
		r, rok := ri.Row(kind)
		f, fok := fi.Row(kind)
		require.True(t, rok && fok, kind)
		assert.Equal(t, r, f, kind)
	}
}

func mustTable52(t *testing.T, tables *ruletablesvc.Tables) *ruletablesvc.WeaponVsArmor {
	t.Helper()
	w, ok := tables.WeaponVsArmor()
	require.True(t, ok)
	return w
}

// Every Wizard spell has an effect entry at its level; every other entry is a Priest spell of the
// catalogue at its level (the Priest entries are the audit's, docs/priest-effects-audit.md), under
// the catalogue's own id.
func TestRepoSeeds_EveryEffectIsASpellAtItsLevelAndEveryWizardSpellHasOne(t *testing.T) {
	cat := realCatalogue(t)
	effects, err := spelleffectsadpt.New(seedsDir)
	require.NoError(t, err)

	wizard := map[string]int{}
	priest := map[string]int{}
	for _, s := range cat.Spells {
		switch s.Class {
		case "Wizard":
			wizard[s.ID] = s.Level
		case "Priest":
			priest[s.ID] = s.Level
		}
	}
	require.NotEmpty(t, wizard)
	require.NotEmpty(t, priest)
	for id, level := range wizard {
		if e, ok := effects.Get(id); assert.True(t, ok, "no effect for %s", id) {
			assert.Equal(t, level, e.Level, id)
		}
	}
	priestEntries := 0
	for _, e := range effects.All() {
		if level, ok := wizard[e.ID]; ok {
			assert.Equal(t, level, e.Level, e.ID)
			continue
		}
		level, ok := priest[e.ID]
		assert.True(t, ok, "effect %s is neither a Wizard nor a Priest spell of the catalogue", e.ID)
		assert.Equal(t, level, e.Level, e.ID)
		priestEntries++
	}
	assert.Equal(t, len(wizard)+priestEntries, effects.Len(), "no entry is counted twice")
	assert.NotZero(t, priestEntries, "the Priest spells the audit maps have entries")
}

// The real seeds with one spell effect the real catalogue does not hold do
// not pass the startup cross-check: it names the unknown id.
func TestRealCatalogue_AnUnknownSpellEffectIDIsRefused(t *testing.T) {
	cat := realCatalogue(t)
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

	effects, err := spelleffectsadpt.New(seeds)
	require.NoError(t, err)
	err = effects.CheckAgainst(cat)
	require.Error(t, err)
	assert.ErrorContains(t, err, "spell effects: ids not in the spell catalogue: magic_rocket")
}

func TestRepoSeeds_SpellbookSpellsResolve(t *testing.T) {
	book, err := spellbookadpt.ReadSeed(seedsDir, realCatalogue(t))
	require.NoError(t, err, "spellbook ids/levels match the catalog")
	effects, err := spelleffectsadpt.New(seedsDir)
	require.NoError(t, err)
	for _, id := range book {
		_, ok := effects.Get(id)
		assert.True(t, ok, "spellbook spell %s has no effect entry", id)
	}
}

// Every seeded weapon carries a weapon type from the fixed list (ADR 0030 step 23).
func TestRepoSeeds_EveryWeaponIsTyped(t *testing.T) {
	var weapons []struct {
		ID   string `json:"id"`
		Type string `json:"weapon_type"`
	}
	readJSON(t, "weapons.json", &weapons)
	require.NotEmpty(t, weapons)
	for _, w := range weapons {
		assert.NotEmpty(t, w.Type, "weapon %s has no weapon_type", w.ID)
		assert.True(t, itemsvc.ValidWeaponType(w.Type), "weapon %s: unknown weapon_type %q", w.ID, w.Type)
	}
}

// The web's WEAPON_TYPES is the same list as itemsvc.WeaponTypes, in the same order: the code list has
// one owner (Go) and this copy cannot drift.
func TestWebWeaponTypes_MatchTheServicesList(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "types", "combat.ts"))
	require.NoError(t, err)
	m := regexp.MustCompile(`(?s)export const WEAPON_TYPES = \[(.*?)\] as const`).FindSubmatch(raw)
	require.NotNil(t, m, "WEAPON_TYPES not found in web/src/types/combat.ts")
	var codes []string
	for _, q := range regexp.MustCompile(`"([a-z_]+)"`).FindAllSubmatch(m[1], -1) {
		codes = append(codes, string(q[1]))
	}
	assert.Equal(t, itemsvc.WeaponTypes, codes)
}

// ADR 0033 step 1.1: every component a spell spends (consume_item.type) is a
// code of itemsvc.Components, so a piece the editor marks can be spent.
func TestRepoSeeds_Step1_1_EverySpentComponentIsKnown(t *testing.T) {
	effects, err := spelleffectsadpt.New(seedsDir)
	require.NoError(t, err)
	n := 0
	for _, e := range effects.All() {
		if e.ConsumeItem != nil {
			n++
			assert.Contains(t, itemsvc.Components, e.ConsumeItem.Type, "spell %s spends an unknown component", e.ID)
		}
	}
	assert.NotZero(t, n, "the Chromatic Orb spends a gem")
}

// The web's COMPONENTS is itemsvc.Components, in the same order (one owner, no drift).
func TestWebComponents_MatchTheServicesList(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "web", "src", "types", "items.ts"))
	require.NoError(t, err)
	m := regexp.MustCompile(`(?s)export const COMPONENTS = \[(.*?)\] as const`).FindSubmatch(raw)
	require.NotNil(t, m, "COMPONENTS not found in web/src/types/items.ts")
	var codes []string
	for _, q := range regexp.MustCompile(`"([a-z_]+)"`).FindAllSubmatch(m[1], -1) {
		codes = append(codes, string(q[1]))
	}
	assert.Equal(t, itemsvc.Components, codes)
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

// ADR 0040 step 3: the real artifact's five classes list exactly the feature
// kinds of the step's table, with its params, and the same spell progression:
// the fixture's (whose lists the adapter test reads against the table,
// TestDefinitions_Step3_TheFeatureListsMapWithTheirParams), class by class.
// Skipped without the real artifact.
func TestFeatures_Step3_TheFiveClassesGrantTodaysMechanics(t *testing.T) {
	realCatalogue(t)
	real, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	fixture, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	fromReal, err := ruletablesadpt.Parse(real.RuleTables, real.Equipment, real.Languages, real.Definitions)
	require.NoError(t, err)
	fromFixture, err := ruletablesadpt.Parse(fixture.RuleTables, fixture.Equipment, fixture.Languages, fixture.Definitions)
	require.NoError(t, err)
	rd, err := fromReal.Definitions()
	require.NoError(t, err)
	fd, err := fromFixture.Definitions()
	require.NoError(t, err)
	for _, id := range fd.ClassIDs() {
		f, err := fd.Get(id)
		require.NoError(t, err)
		r, err := rd.Get(id)
		require.NoError(t, err, "the real artifact defines %s", id)
		assert.Equal(t, f.SpellProgression, r.SpellProgression, id)
		assert.Equal(t, f.Features, r.Features, id)
	}
}

// ADR 0021 step 20.1 on the real artifact: the psionicist's column of the
// experience and THAC0 grids equals its class entry and its group entry (the
// ones the app reads); the grid copy has no reader of its own.
func TestRealCatalogue_Step20_PsionicistGridsEqualItsClassAndGroupEntries(t *testing.T) {
	realCatalogue(t)
	artifact, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(artifact.RuleTables, artifact.Equipment, artifact.Languages, artifact.Definitions)
	require.NoError(t, err)
	defs, err := tables.Definitions()
	require.NoError(t, err)
	class, ok := defs.Class("psionicist")
	require.True(t, ok)
	var grids struct {
		Experience []struct{ Level, Psionicist int } `json:"experience"`
		THAC0      []struct{ Level, Psionicist int } `json:"thac0"`
	}
	require.NoError(t, json.Unmarshal(artifact.RuleTables, &grids))
	require.Len(t, grids.Experience, len(class.Experience))
	for i, r := range grids.Experience {
		assert.Equal(t, class.Experience[i], r.Psionicist, "experience at level %d", i+1)
	}
	for i, r := range grids.THAC0 {
		got, err := defs.THAC0(class.Group, i+1)
		require.NoError(t, err)
		assert.Equal(t, got, r.Psionicist, "THAC0 at level %d", i+1)
	}
}

// The psionicist's figures the character service reads, from the real
// artifact through the server's adapters: the hit die and the grids by group
// or class (ADR 0021 step 20.1, ADR 0030 step 7). The values are the ones the
// builder traced to the CPsiH's psionicist tables (its own checks hold the page
// references); the creation gate keeps the class off the API until its kinds
// are built, so this is the only place the real figures are read through the
// services.
func TestRealCatalogue_Step20_ThePsionicistsFiguresReadFromTheRealArtifact(t *testing.T) {
	realCatalogue(t)
	artifact, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(artifact.RuleTables, artifact.Equipment, artifact.Languages, artifact.Definitions)
	require.NoError(t, err)
	defs, err := tables.Definitions()
	require.NoError(t, err)
	class, ok := defs.Class("psionicist")
	require.True(t, ok)

	hd, err := tables.HitDiceFor(class.Group)
	require.NoError(t, err)
	assert.Equal(t, ruletablesvc.HitDiceRow{Group: class.Group, Die: 6, NameLevel: 9, GainAfter: 2}, hd)
	xp, err := tables.XPThreshold(class.ID, 5)
	require.NoError(t, err)
	assert.Equal(t, 16_500, xp)
	thac0, err := tables.THAC0For(class.Group, 1)
	require.NoError(t, err)
	assert.Equal(t, 20, thac0)
	saves, err := tables.SavesFor(class.Group, 1)
	require.NoError(t, err)
	assert.Equal(t, [5]int{13, 15, 10, 16, 15}, [5]int{saves.ParaPoison, saves.RodStaffWand, saves.PetriPoly, saves.Breath, saves.Spell})
	saves, err = tables.SavesFor(class.Group, 17)
	require.NoError(t, err)
	assert.Equal(t, [5]int{9, 7, 6, 11, 9}, [5]int{saves.ParaPoison, saves.RodStaffWand, saves.PetriPoly, saves.Breath, saves.Spell})
}
