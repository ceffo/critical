package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// ADR 0032 step 8.1: the alignment mapping, en and fr, ignoring case, accents and punctuation:
// the nine names, the usual abbreviations, "neutral" / "neutre" alone as true neutral; anything
// else maps to nothing.
func TestMapAlignment_Step8_1_EnAndFrSpellings(t *testing.T) {
	for raw, want := range map[string]string{
		"Lawful Good": "lawful_good", "neutral good": "neutral_good", "CHAOTIC-GOOD": "chaotic_good",
		"lawful neutral": "lawful_neutral", "True Neutral": "true_neutral", "neutral": "true_neutral",
		"chaotic neutral": "chaotic_neutral", "Lawful Evil": "lawful_evil", "neutral evil": "neutral_evil",
		"chaotic evil": "chaotic_evil",
		"LG":           "lawful_good", "NG": "neutral_good", "CG": "chaotic_good", "LN": "lawful_neutral",
		"TN": "true_neutral", "N": "true_neutral", "CN": "chaotic_neutral", "LE": "lawful_evil",
		"NE": "neutral_evil", "CE": "chaotic_evil", "L.G.": "lawful_good", "c n": "chaotic_neutral",
		"Loyal bon": "lawful_good", "neutre bon": "neutral_good", "Chaotique bon": "chaotic_good",
		"Loyal neutre": "lawful_neutral", "Neutre": "true_neutral", "neutre strict": "true_neutral",
		"Chaotique neutre": "chaotic_neutral", "Loyal mauvais": "lawful_evil", "Neutre mauvais": "neutral_evil",
		"chaotique mauvais": "chaotic_evil", "  Chaotique   Neutre ": "chaotic_neutral",
		"LB": "lawful_good", "CM": "chaotic_evil",
		// Already a code: a second run changes nothing.
		"lawful_good": "lawful_good", "chaotic_evil": "chaotic_evil",
		// Not recognised.
		"": "", "good": "", "bon": "", "whatever": "", "chaotique": "", "lawful chaotic": "",
	} {
		assert.Equal(t, want, mapAlignment(raw), "%q", raw)
	}
	for _, code := range ruletablesvc.Alignments {
		assert.Equal(t, code, mapAlignment(code), "every alignment code maps to itself")
	}
}

// ADR 0032 step 8.1: sex, en and fr, and the initials M / F / H.
func TestMapSex_Step8_1_EnAndFrSpellings(t *testing.T) {
	for raw, want := range map[string]string{
		"male": "male", "Male": "male", "M": "male", "m.": "male", "H": "male", "Homme": "male",
		"Masculin": "male", "man": "male",
		"female": "female", "FEMALE": "female", "F": "female", "Femme": "female", "Féminin": "female",
		"woman": "female",
		"":      "", "x": "", "autre": "", "?": "", "mâle ou femelle": "",
	} {
		assert.Equal(t, want, mapSex(raw), "%q", raw)
	}
	for _, code := range charactersvc.Sexes {
		assert.Equal(t, code, mapSex(code))
	}
}

// ADR 0032 step 8: on existing data the mapped values become codes, the unrecognised ones are
// cleared (the card then shows them missing) and the rules race is untouched; a second run
// changes nothing, and a character with no value to map is left as it is.
func TestMigrate_Step8_FreeTextBecomesCodesTheRestIsCleared(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	store := newFakeStore()
	for _, c := range []struct {
		name, alignment, sex, race string
	}{
		{"Fr", "Chaotique neutre", "Homme", "dwarf"},
		{"En", "CN", "female", "half_elf"},
		{"Junk", "plutôt gentil", "yes", ""},
		{"Blank", "", "", "human"},
		{"Done", "lawful_good", "male", "elf"}, // a code already: a migrated character
	} {
		id, err := store.InsertCharacter(context.Background(), insertParams(c.name, c.race))
		require.NoError(t, err)
		store.t.Alignments[id], store.t.Sexes[id] = c.alignment, c.sex
	}
	var recorded []string
	for _, m := range embedded {
		if m.name != identityMigration {
			recorded = append(recorded, m.name)
		}
	}
	run := func(recorded []string) {
		f := &fakeMigrations{recorded: recorded, store: store}
		require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error {
			before, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
			return runMigrations(ctx, f, embedded, before, after)
		}))
	}

	run(recorded)
	ids := store.t.Characters
	assert.Equal(t, []string{"chaotic_neutral", "chaotic_neutral", "", "", "lawful_good"},
		[]string{store.t.Alignments[ids[0]], store.t.Alignments[ids[1]], store.t.Alignments[ids[2]], store.t.Alignments[ids[3]], store.t.Alignments[ids[4]]})
	assert.Equal(t, []string{"male", "female", "", "", "male"},
		[]string{store.t.Sexes[ids[0]], store.t.Sexes[ids[1]], store.t.Sexes[ids[2]], store.t.Sexes[ids[3]], store.t.Sexes[ids[4]]})
	assert.Equal(t, []string{"dwarf", "half_elf", "", "human", "elf"},
		[]string{store.t.Races[ids[0]], store.t.Races[ids[1]], store.t.Races[ids[2]], store.t.Races[ids[3]], store.t.Races[ids[4]]},
		"the rules race is untouched")

	// Idempotent: the step run again changes nothing; an applied migration runs no step.
	before := copyTables(store.t)
	require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error { return mapIdentity(ctx, store) }))
	assert.Equal(t, before, store.t)
	store.t.Alignments[ids[2]] = "free text"
	run(append(recorded, identityMigration))
	assert.Equal(t, "free text", store.t.Alignments[ids[2]])
}

// ADR 0032 step 8, the contract release: the alignment and sex the release before wrote as free
// text after migration 013 are mapped to codes, or cleared, when the race column is dropped; the
// ones already codes are kept, and the step runs on no database that applied the migration.
func TestMigrate_Step8_DroppingTheRaceMapsTheFreeTextTheOldReleaseWrote(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	store := newFakeStore()
	for _, c := range []struct{ name, alignment, sex string }{
		{"Late", "Loyal bon", "Femme"},
		{"Junk", "unknown", "?"},
		{"Code", "chaotic_good", "male"},
	} {
		id, err := store.InsertCharacter(context.Background(), insertParams(c.name, ""))
		require.NoError(t, err)
		store.t.Alignments[id], store.t.Sexes[id] = c.alignment, c.sex
	}
	var recorded []string
	for _, m := range embedded {
		if m.name != dropRaceMigration {
			recorded = append(recorded, m.name)
		}
	}
	run := func(recorded []string) {
		f := &fakeMigrations{recorded: recorded, store: store}
		require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error {
			before, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
			return runMigrations(ctx, f, embedded, before, after)
		}))
	}

	run(recorded)
	ids := store.t.Characters
	assert.Equal(t, []string{"lawful_good", "", "chaotic_good"},
		[]string{store.t.Alignments[ids[0]], store.t.Alignments[ids[1]], store.t.Alignments[ids[2]]})
	assert.Equal(t, []string{"female", "", "male"},
		[]string{store.t.Sexes[ids[0]], store.t.Sexes[ids[1]], store.t.Sexes[ids[2]]})

	store.t.Alignments[ids[1]] = "free text"
	run(append(recorded, dropRaceMigration))
	assert.Equal(t, "free text", store.t.Alignments[ids[1]], "an applied migration runs no step")
}

// ADR 0032 step 8 and the seed: the seed character (Kelsen) stored before the migration, holding
// no alignment, gets the alignment and sex his seed names, which pass his class's limit; a value
// set since is kept, another character is not his.
func TestMigrate_Step8_TheSeedCharacterGetsAnAlignmentAndSexThatPassTheCheck(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	store := newFakeStore()
	require.NoError(t, seedOn(store, repoSeeds()))
	kelsen := store.t.Characters[0]
	other, err := store.InsertCharacter(context.Background(), insertParams("Other", ""))
	require.NoError(t, err)
	seed, ok, err := seedfile.ReadCharacter(seedfile.Path(repoSeeds(), seedfile.CharacterFile))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, seed.Alignment, store.t.Alignments[kelsen], "a new database seeds the identity with the character")
	assert.Contains(t, charactersvc.AlignmentsOf(catalogueDefs, seed.Class), seed.Alignment, "his class admits it")
	assert.Contains(t, charactersvc.Sexes, seed.Sex)

	store.t.Alignments[kelsen], store.t.Sexes[kelsen] = "Neutre bon", "" // free text from before the codes
	var recorded []string
	for _, m := range embedded {
		if m.name != identityMigration {
			recorded = append(recorded, m.name)
		}
	}
	f := &fakeMigrations{recorded: recorded, store: store}
	require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error {
		before, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
		return runMigrations(ctx, f, embedded, before, after)
	}))
	assert.Equal(t, "neutral_good", store.t.Alignments[kelsen], "the free text is mapped")
	assert.Equal(t, seed.Sex, store.t.Sexes[kelsen], "the missing sex is the seed's")
	assert.Empty(t, store.t.Alignments[other], "another character is not the seed's")

	store.t.Alignments[kelsen] = "chaotic_neutral" // chosen since
	require.NoError(t, store.InTx(context.Background(), func(ctx context.Context) error {
		return identitySeedCharacter(ctx, store, seedfile.Path(repoSeeds(), seedfile.CharacterFile))
	}))
	assert.Equal(t, "chaotic_neutral", store.t.Alignments[kelsen], "a value set since is kept")
}

// insertParams are the minimal row of a stored character.
func insertParams(name, race string) storeq.InsertCharacterParams {
	return storeq.InsertCharacterParams{Name: name, RaceCode: race, Languages: "[]", SkillPoints: "{}"}
}
