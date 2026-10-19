package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// catalogueCodes are the language codes of the fixture catalogue (format 8).
func catalogueCodes(t *testing.T) []string {
	t.Helper()
	a, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	var rows []struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(a.Languages, &rows))
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Code
	}
	return out
}

// ADR 0032 step 8.1: the language mapping, en and fr, ignoring case, accents and punctuation: each
// language of the racial lists maps to its code, whichever spelling; anything else maps to nothing.
func TestMapLanguage_Step8_1_EnAndFrSpellings(t *testing.T) {
	for raw, want := range map[string]string{
		"Common": "common", "Commun": "common", "COMMUNE": "common",
		"Dwarvish": "dwarvish", "Dwarf": "dwarvish", "Nain": "dwarvish", "nanique": "dwarvish",
		"Elvish": "elvish", "Elfique": "elvish", "elf": "elvish", "Elfe": "elvish",
		"Gnomish": "gnomish", "Gnome": "gnomish", "gnomique": "gnomish",
		"Halfling": "halfling", "Halfelin": "halfling",
		"Goblin": "goblin", "Gobelin": "goblin",
		"Hobgoblin": "hobgoblin", "Hobgobelin": "hobgoblin",
		"Kobold": "kobold", "Kobolde": "kobold",
		"Orcish": "orcish", "Orc": "orcish", "Orque": "orcish",
		"Gnoll":                 "gnoll",
		"Burrowing mammals":     "burrowers",
		"Parler des fouisseurs": "burrowers",
		"  élfique ":            "elvish", "Dwarf.": "dwarvish", "Haut-elfique": "",
		// Already a code: a second run changes nothing.
		"elvish": "elvish", "burrowers": "burrowers",
		// Not recognised.
		"": "", "Klingon": "", "Draconique": "", "langue des signes": "",
	} {
		assert.Equal(t, want, mapLanguage(raw), "%q", raw)
	}
	for _, code := range catalogueCodes(t) {
		assert.Equal(t, code, mapLanguage(code), "every catalogue code maps to itself")
	}
}

// ADR 0032 step 8.1: a stored item naming several languages (commas, semicolons, "and", "et") is
// split; an unrecognised item is dropped on its own and the recognised ones stay; each code once.
func TestMapLanguageList_Step8_1_SplitsItemsDropsTheUnknownAndKeepsEachCodeOnce(t *testing.T) {
	codes, dropped := mapLanguageList([]string{"Elfique, Commun", "Orque et Nain; Klingon", "Gnome and Goblin", "commun", "Draconique"})
	assert.Equal(t, []string{"elvish", "common", "orcish", "dwarvish", "gnomish", "goblin"}, codes)
	assert.Equal(t, []string{"Klingon", "Draconique"}, dropped)

	codes, dropped = mapLanguageList(nil)
	assert.Equal(t, []string{}, codes)
	assert.Empty(t, dropped)

	// "et"/"and" inside a word is no separator.
	codes, _ = mapLanguageList([]string{"Kobold", "Hobgoblin", "Gnoll"})
	assert.Equal(t, []string{"kobold", "hobgoblin", "gnoll"}, codes)
}

// Every code the mapping can produce is a language of the catalogue artifact: the committed
// spellings and the catalogue data are one set (ADR 0021 step 14, ADR 0032 step 8.1).
func TestLanguageSpellings_Step8_1_AreCatalogueCodes(t *testing.T) {
	known := catalogueCodes(t)
	for spelling, code := range languageSpellings {
		assert.Contains(t, known, code, "%q maps to a code the catalogue lacks", spelling)
	}
	for _, code := range known {
		assert.Contains(t, languageSpellings, code, "the catalogue language %q has no spelling in the mapping", code)
	}
}

// logged runs fn with the default logger writing to a buffer and returns what it logged.
func logged(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)
	fn()
	return buf.String()
}

// ADR 0032 step 8 on existing data: the stored free-text languages become codes, an unrecognised
// word is dropped and logged with its character, the recognised words of the same list staying, a
// list of codes or an empty one is left as it is, a second run changes nothing, and an applied
// migration runs no step.
func TestMigrate_Step8_LanguagesFreeTextBecomeCodesTheUnknownAreDroppedAndLogged(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	store := newFakeStore()
	for _, c := range []struct{ name, languages string }{
		{"Fr", `["Elfique","Commun","Orque","Nain"]`},
		{"Mixed", `["Elvish, Dwarvish and Klingon","Gnome"]`},
		{"Junk", `["Draconique","???"]`},
		{"Empty", `[]`},
		{"Done", `["common","elvish"]`}, // codes already: a migrated character
	} {
		p := insertParams(c.name, "elf")
		p.Languages = c.languages
		_, err := store.InsertCharacter(context.Background(), p)
		require.NoError(t, err)
	}
	var recorded []string
	for _, m := range embedded {
		if m.name != languagesMigration {
			recorded = append(recorded, m.name)
		}
	}
	run := func(recorded []string) {
		f := &fakeMigrations{recorded: recorded, store: store}
		require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error {
			before, after := migrationDataSteps(repoSeeds(), catalogueWeapons)
			return runMigrations(ctx, f, embedded, before, after)
		}))
	}

	out := logged(t, func() { run(recorded) })
	ids := store.t.Characters
	for i, want := range []string{
		`["elvish","common","orcish","dwarvish"]`, `["elvish","dwarvish","gnomish"]`, `[]`, `[]`, `["common","elvish"]`,
	} {
		assert.Equal(t, want, store.t.Languages[ids[i]], "character %d", ids[i])
	}
	assert.Contains(t, out, "dropped an unrecognised language")
	assert.Contains(t, out, "Klingon", "the dropped word is logged")
	assert.Contains(t, out, "Draconique")
	assert.NotContains(t, out, "Elfique", "a recognised word is not logged")

	// Idempotent: the step run again changes nothing and logs nothing; an applied migration runs no step.
	before := copyTables(store.t)
	again := logged(t, func() {
		require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error { return mapLanguages(ctx, store) }))
	})
	assert.Equal(t, before, store.t)
	assert.Empty(t, again)
	store.t.Languages[ids[0]] = `["free text"]`
	run(append(recorded, languagesMigration))
	assert.Equal(t, `["free text"]`, store.t.Languages[ids[0]])
}

// ADR 0032 step 8 and the seed: the seed character (Kelsen) stored before the migration holds
// French words ("Elfique", "Commun", "Orque", "Nain"); after it he holds the codes his seed names,
// all of them languages of the catalogue, and no French word is left.
func TestMigrate_Step8_TheSeedCharactersFrenchLanguagesBecomeCodes(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	store := newFakeStore()
	require.NoError(t, seedOn(store, campaignSeeds()))
	kelsen := store.t.Characters[0]
	seed, ok, err := seedfile.ReadCharacter(seedfile.Path(campaignSeeds(), seedfile.CharacterFile))
	require.NoError(t, err)
	require.True(t, ok)
	for _, l := range seed.Languages {
		assert.Contains(t, catalogueCodes(t), l, "a new database seeds codes of the catalogue (ADR 0023)")
	}

	store.t.Languages[kelsen] = `["Elfique","Commun","Orque","Nain"]` // the words stored before the codes
	var recorded []string
	for _, m := range embedded {
		if m.name != languagesMigration {
			recorded = append(recorded, m.name)
		}
	}
	f := &fakeMigrations{recorded: recorded, store: store}
	logged(t, func() {
		require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error {
			before, after := migrationDataSteps(campaignSeeds(), catalogueWeapons)
			return runMigrations(ctx, f, embedded, before, after)
		}))
	})
	var got []string
	require.NoError(t, json.Unmarshal([]byte(store.t.Languages[kelsen]), &got))
	assert.Equal(t, []string{"elvish", "common", "orcish", "dwarvish"}, got)
	assert.ElementsMatch(t, seed.Languages, got, "the seed names the same four")
}
