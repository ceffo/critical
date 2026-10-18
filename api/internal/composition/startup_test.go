package composition_test

// The startup sequence of the composition root on mocked storage (ADR 0025
// steps 3 and 6.1). The unknown-migration guard itself belongs to the storage
// engine; here the root's reaction to a failing step is tested.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// The steps run in the order migrate, seed, import the catalogue, read the
// spell catalogue once, give the characters without an owner to the root
// admin (ADR 0029 step 5.3); the import is told the fixture's catalogue and every spell the
// seeds refer to, so it cannot remove one (ADR 0021 step 5.4.3). Closing the
// services closes the store once.
func TestBuild_StartupStepsRunInOrder(t *testing.T) {
	h := newHarness(t)
	// The seeds' character.json names the owner of rows a migration finds
	// without one.
	h.Store.EXPECT().Migrate(mock.Anything, fixtureSeeds).Return(nil).Once()
	var book []string
	h.Store.EXPECT().Seed(mock.Anything, fixtureSeeds, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, b []string) error {
			book = b
			return nil
		}).Once()
	var cat catalogue.Catalogue
	var refs []catalogue.Ref
	h.Store.EXPECT().ImportCatalogue(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, c catalogue.Catalogue, r []catalogue.Ref) error {
			cat, refs = c, r
			return nil
		}).Once()
	search := h.Spell.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(nil, nil).Once()
	h.Spell.EXPECT().Details(mock.Anything, []string{}).Return(nil, nil).Once().NotBefore(search)
	root := &authsvc.User{ID: uuid.New(), Email: "root@example.com", Role: authsvc.RoleAdmin, Root: true}
	h.Auth.EXPECT().RootUser(mock.Anything).Return(root, nil).Once().NotBefore(search)
	adopt := h.Auth.EXPECT().AdoptUnownedCharacters(mock.Anything, root.ID).Return(nil).Once()
	// Then the max HP levels not set yet, after the seed (ADR 0030 step 7.5).
	h.Character.EXPECT().CharactersWithoutHPLevel(mock.Anything).Return(nil, nil).Once().NotBefore(adopt)
	h.Store.EXPECT().Close().Return(nil).Once()

	bs, err := h.build(t, fixtureConfig())
	require.NoError(t, err)
	assert.NotEmpty(t, cat.Spells)
	assert.Contains(t, book, "alarm", "the seed gets the spellbook seed, checked against the catalogue")
	assert.Contains(t, refs, catalogue.Ref{ID: "alarm", Where: "spellbook seed"})
	assert.Contains(t, refs, catalogue.Ref{ID: "alarm", Where: "spell effects seed"})
	assert.Nil(t, bs.DB, "a root built over mocks holds no SQLite handle")
	require.NoError(t, bs.Close(context.Background()))
}

// Closing returns the store's close error.
func TestBuild_CloseReportsTheStoresError(t *testing.T) {
	h := newHarness(t)
	h.expectStartup(fixtureSeeds)
	boom := errors.New("pool already closed")
	h.Store.EXPECT().Close().Return(boom).Once()

	bs, err := h.build(t, fixtureConfig())
	require.NoError(t, err)
	assert.ErrorIs(t, bs.Close(context.Background()), boom)
}

// A failing step refuses the start, names the step, runs none after it and
// closes the store.
func TestBuild_AFailingStepRefusesTheStartAndClosesTheStore(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		prepare func(h *harness)
		want    string
	}{
		{"migrate (the unknown-migration guard's refusal travels this way)", func(h *harness) {
			h.Store.EXPECT().Migrate(mock.Anything, mock.Anything).Return(boom).Once()
		}, "migrate: boom"},
		{"seed", func(h *harness) {
			h.Store.EXPECT().Migrate(mock.Anything, mock.Anything).Return(nil).Once()
			h.Store.EXPECT().Seed(mock.Anything, fixtureSeeds, mock.Anything).Return(boom).Once()
		}, "seed: boom"},
		{"adopting the characters without an owner", func(h *harness) {
			h.Store.EXPECT().Migrate(mock.Anything, mock.Anything).Return(nil).Once()
			h.Store.EXPECT().Seed(mock.Anything, fixtureSeeds, mock.Anything).Return(nil).Once()
			h.Store.EXPECT().ImportCatalogue(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			h.Spell.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(nil, nil).Once()
			h.Spell.EXPECT().Details(mock.Anything, []string{}).Return(nil, nil).Once()
			h.Auth.EXPECT().RootUser(mock.Anything).Return(nil, boom).Once()
		}, "give the characters without an owner to the root admin: boom"},
		{"spell catalogue read", func(h *harness) {
			h.Store.EXPECT().Migrate(mock.Anything, mock.Anything).Return(nil).Once()
			h.Store.EXPECT().Seed(mock.Anything, fixtureSeeds, mock.Anything).Return(nil).Once()
			h.Store.EXPECT().ImportCatalogue(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			h.Spell.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return(nil, boom).Once()
		}, "load the spell catalogue: boom"},
		{"catalogue import", func(h *harness) {
			h.Store.EXPECT().Migrate(mock.Anything, mock.Anything).Return(nil).Once()
			h.Store.EXPECT().Seed(mock.Anything, fixtureSeeds, mock.Anything).Return(nil).Once()
			h.Store.EXPECT().ImportCatalogue(mock.Anything, mock.Anything, mock.Anything).Return(boom).Once()
		}, "boom"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.prepare(h)
			h.Store.EXPECT().Close().Return(nil).Once()
			_, err := h.build(t, fixtureConfig())
			require.ErrorContains(t, err, tc.want)
		})
	}
}

// A spell effect whose id the catalogue does not hold refuses the start,
// naming every such id, before anything is seeded or imported.
func TestStartup_RefusesSpellEffectsTheCatalogueDoesNotHold(t *testing.T) {
	seeds := t.TempDir()
	require.NoError(t, os.CopyFS(seeds, os.DirFS(fixtureSeeds)))
	path := filepath.Join(seeds, "spell_effects.json")
	raw, err := os.ReadFile(path) //nolint:gosec // test temp dir
	require.NoError(t, err)
	var entries []json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &entries))
	for id, name := range map[string]string{"no_such_spell": "No Such Spell", "magic_rocket": "Magic Rocket"} {
		entries = append(entries, json.RawMessage(`{"id":"`+id+`","name":"`+name+`","level":1,"kind":"utility",`+
			`"confidence":"high","target":"none","duration":{"kind":"instant"}}`))
	}
	raw, err = json.Marshal(entries)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	h := newHarness(t)
	h.Store.EXPECT().Migrate(mock.Anything, mock.Anything).Return(nil).Once()
	h.Store.EXPECT().Close().Return(nil).Once() // no Seed or ImportCatalogue expectation: a call fails the test

	_, err = h.build(t, composition.Config{SeedsDir: seeds, Catalogue: fixtureCatalogue})
	require.Error(t, err)
	assert.ErrorContains(t, err, "spell effects: ids not in the spell catalogue: ")
	assert.ErrorContains(t, err, "no_such_spell")
	assert.ErrorContains(t, err, "magic_rocket")
}

// level16XP is the XP that reaches bard level 16 (PHB Table 25, Thief/Bard
// column): the fixture Bard is level 15, so it is the XP ledger's next
// threshold, which shows which rule tables a start took.
const level16XP = 1320000

// setLevel16XP sets the bard level-16 XP of the artifact's class experience
// table (rule_tables.experience), the one the ledger and the level read.
func setLevel16XP(tables map[string]any, xp int) {
	rows := tables["experience"].([]any)
	row := rows[15].(map[string]any)
	if row["level"] != float64(16) {
		panic("experience row 15 is not level 16")
	}
	row["bard"] = xp
}

// nextThreshold starts the root on the artifact at cfg.Catalogue and reads the
// XP ledger of a Bard at 1,293,418 XP (level 15) with no XP entry.
func nextThreshold(t *testing.T, cfg composition.Config) int {
	t.Helper()
	ctx := context.Background()
	h := newHarness(t)
	h.expectStartup(cfg.SeedsDir)
	h.Store.EXPECT().Close().Return(nil).Once()
	h.Character.EXPECT().GetCharacter(mock.Anything, characterID).Return(&charactersvc.Character{
		ID: characterID, Class: "bard", XPTotal: 1_293_418, Dex: 10, Cha: 10,
	}, nil).Once()
	// Read twice: by the character's derivation (ADR 0030 step 3), then by the ledger.
	h.XP.EXPECT().List(mock.Anything, characterID).Return(nil, nil).Twice()

	bs, err := h.build(t, cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, bs.Close(ctx)) }()
	l, err := bs.XP.Get(ctx, characterID)
	require.NoError(t, err)
	require.NotNil(t, l.NextThreshold)
	return *l.NextThreshold
}

// refusedStart builds the root on cfg, which must refuse the start with the
// store closed and nothing seeded or imported.
func refusedStart(t *testing.T, cfg composition.Config) error {
	t.Helper()
	h := newHarness(t)
	h.Store.EXPECT().Migrate(mock.Anything, mock.Anything).Return(nil).Once()
	h.Store.EXPECT().Close().Return(nil).Once()
	_, err := h.build(t, cfg)
	require.Error(t, err)
	return err
}

// ADR 0021 step 9.6.3: format 1 is retired. The tables are the artifact's
// and a rule_tables.json left in the seeds is never read (a broken one is
// ignored). A format-1 artifact, which has no tables, refuses the start even
// with a valid seed file beside it, naming the format and `just catalogue`.
func TestStartup_Step9_6_3_TheTablesAreTheArtifactsAndFormat1IsRefused(t *testing.T) {
	t.Run("the artifact's tables, a seed file ignored", func(t *testing.T) {
		broken := t.TempDir()
		require.NoError(t, os.CopyFS(broken, os.DirFS(fixtureSeeds)))
		require.NoError(t, os.WriteFile(filepath.Join(broken, "rule_tables.json"), []byte("not json"), 0o600))
		artifact := writeArtifact(t, func(d map[string]any) {
			setLevel16XP(d["rule_tables"].(map[string]any), level16XP+3)
		})
		assert.Equal(t, level16XP+3, nextThreshold(t, composition.Config{SeedsDir: broken, Catalogue: artifact}))
	})

	t.Run("format 1 refuses, even with the seed file", func(t *testing.T) {
		raw, err := os.ReadFile(fixtureCatalogue)
		require.NoError(t, err)
		var doc map[string]any
		require.NoError(t, json.Unmarshal(raw, &doc))
		tables, err := json.Marshal(doc["rule_tables"])
		require.NoError(t, err)
		seeds := t.TempDir()
		require.NoError(t, os.CopyFS(seeds, os.DirFS(fixtureSeeds)))
		require.NoError(t, os.WriteFile(filepath.Join(seeds, "rule_tables.json"), tables, 0o600))
		artifact := writeArtifact(t, func(d map[string]any) {
			d["format"] = 1
			delete(d, "rule_tables")
		})

		err = refusedStart(t, composition.Config{SeedsDir: seeds, Catalogue: artifact})
		require.ErrorContains(t, err, "spell catalogue "+artifact+": unknown format 1: this API reads formats 3, 4 and 5")
		require.ErrorContains(t, err, "just catalogue")
	})
}

// ADR 0021 steps 9.4 and 9.5: the tables are read into memory at every start.
// A restart with a corrected table uses the corrected number (the store's
// import of an artifact of the same version does nothing, as the mocked one
// here); a broken table refuses the start even then, naming the table.
func TestStartup_Step9_5_TheTablesAreReadAtEveryStart(t *testing.T) {
	cfg := fixtureConfig()
	assert.Equal(t, level16XP, nextThreshold(t, cfg))

	cfg.Catalogue = writeArtifact(t, func(d map[string]any) {
		setLevel16XP(d["rule_tables"].(map[string]any), level16XP+7)
	})
	assert.Equal(t, level16XP+7, nextThreshold(t, cfg), "same version, corrected table")

	cfg.Catalogue = writeArtifact(t, func(d map[string]any) {
		setLevel16XP(d["rule_tables"].(map[string]any), 1)
	})
	require.ErrorContains(t, refusedStart(t, cfg), "rule_tables: experience table")
}

// ADR 0021 step 9.3: what the schema cannot check in a format-3 artifact's
// tables (scores covered in order, no gap, no overlap, levels once, XP
// increasing) refuses the start, naming the artifact and the table.
func TestStartup_Step9_3_RefusesTablesThatBreakTheRulesNamingTheTable(t *testing.T) {
	rows := func(d map[string]any, table string) []any {
		return d["rule_tables"].(map[string]any)[table].([]any)
	}
	row := func(d map[string]any, table string, i int) map[string]any {
		return rows(d, table)[i].(map[string]any)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a gap in strength", func(d map[string]any) {
			r := rows(d, "strength")
			d["rule_tables"].(map[string]any)["strength"] = append(r[:1:1], r[2:]...)
		}, "rule_tables: strength table: row 1: score 2 expected, got 3"},
		{"an overlap in dexterity", func(d map[string]any) { row(d, "dexterity", 1)["dex_min"] = 1 }, "rule_tables: dexterity table"},
		{"a bard level twice", func(d map[string]any) { row(d, "bard_spells", 1)["level"] = 1 }, "rule_tables: bard table"},
		{"XP not increasing", func(d map[string]any) { row(d, "bard_experience", 2)["xp"] = 1250 }, "rule_tables: experience table"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			artifact := writeArtifact(t, tc.mutate)
			err := refusedStart(t, composition.Config{SeedsDir: fixtureSeeds, Catalogue: artifact})
			require.ErrorContains(t, err, tc.want)
			require.ErrorContains(t, err, "spell catalogue "+artifact)
		})
	}
}

// ADR 0021 step 10.3: a format-4 or -5 artifact missing a class or race table, or
// with one that breaks the rules the schema cannot check (a gap in a level
// band or in the scores, XP not increasing, a code twice), refuses the start,
// naming the artifact and the table. A format-3 artifact still starts (step
// 10.4), without class tables.
func TestStartup_Step10_3_RefusesAMissingOrGappedClassTable(t *testing.T) {
	tables := func(d map[string]any) map[string]any { return d["rule_tables"].(map[string]any) }
	row := func(d map[string]any, table string, i int) map[string]any {
		return tables(d)[table].([]any)[i].(map[string]any)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"a missing table", func(d map[string]any) { delete(tables(d), "turning_undead") }, "turning_undead"},
		{"a gap in the saving throws", func(d map[string]any) {
			row(d, "saving_throws", 8)["level_min"] = 6
		}, "rule_tables: saving_throws table: rogue: row 1: level 5 expected, got 6"},
		{"a gap in the Constitution scores", func(d map[string]any) {
			rows := tables(d)["constitution"].([]any)
			tables(d)["constitution"] = append(rows[:9:9], rows[10:]...)
		}, "rule_tables: constitution table: row 9: score 10 expected, got 11"},
		{"XP not increasing", func(d map[string]any) {
			row(d, "experience", 9)["mage"] = 2500
		}, "rule_tables: experience table: mage level 10"},
		{"a specialist band missing", func(d map[string]any) {
			rows := tables(d)["specialist_attacks"].([]any)
			tables(d)["specialist_attacks"] = rows[1:]
		}, "rule_tables: specialist_attacks table: melee: row 0: level 1 expected, got 7"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			artifact := writeArtifact(t, tc.mutate)
			err := refusedStart(t, composition.Config{SeedsDir: fixtureSeeds, Catalogue: artifact})
			require.ErrorContains(t, err, tc.want)
			require.ErrorContains(t, err, "spell catalogue "+artifact)
		})
	}

	// Every character's level, THAC0 and saves are derived from the class
	// tables (ADR 0030 steps 3 to 6): a format-3 artifact, which has none,
	// refuses the start, naming `just catalogue`.
	t.Run("format 3 is refused", func(t *testing.T) {
		artifact := writeArtifact(t, func(d map[string]any) {
			d["format"] = 3
			for k := range tables(d) {
				if k != "strength" && k != "dexterity" && k != "bard_spells" && k != "bard_experience" {
					delete(tables(d), k)
				}
			}
		})
		err := refusedStart(t, composition.Config{SeedsDir: fixtureSeeds, Catalogue: artifact})
		require.ErrorIs(t, err, ruletablesvc.ErrNoClassTables)
		require.ErrorContains(t, err, "just catalogue")
		require.ErrorContains(t, err, "spell catalogue "+artifact)
	})
}

// ADR 0021 steps 11.3 and 11.4: a format-5 artifact without its racial level
// limits, or with limits that break the rules the schema cannot check (a race
// and class twice), refuses the start, naming the artifact and the table. A
// format-4 artifact, without them, still starts.
func TestStartup_Step11_3_RefusesMissingOrBrokenLevelLimits(t *testing.T) {
	tables := func(d map[string]any) map[string]any { return d["rule_tables"].(map[string]any) }
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{"the table missing", func(d map[string]any) { delete(tables(d), "race_level_limits") }, "race_level_limits"},
		{"a race and class twice", func(d map[string]any) {
			tables(d)["race_level_limits"].([]any)[1].(map[string]any)["class"] = "fighter"
		}, "rule_tables: race_level_limits table: \"dwarf fighter\" twice"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			artifact := writeArtifact(t, tc.mutate)
			err := refusedStart(t, composition.Config{SeedsDir: fixtureSeeds, Catalogue: artifact})
			require.ErrorContains(t, err, tc.want)
			require.ErrorContains(t, err, "spell catalogue "+artifact)
		})
	}

	t.Run("format 4 still starts", func(t *testing.T) {
		artifact := writeArtifact(t, func(d map[string]any) {
			d["format"] = 4
			delete(tables(d), "race_level_limits")
		})
		assert.Equal(t, level16XP, nextThreshold(t, composition.Config{SeedsDir: fixtureSeeds, Catalogue: artifact}))
	})
}

// The spellbook seed is checked against the catalogue before anything is
// written: a spell the catalogue does not hold refuses the start with the seed
// not run, so no character ever stores it.
func TestBuild_ASpellbookSeedTheCatalogueRefusesIsRefusedBeforeTheSeed(t *testing.T) {
	seeds := t.TempDir()
	entries, err := os.ReadDir(fixtureSeeds)
	require.NoError(t, err)
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(fixtureSeeds, e.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(seeds, e.Name()), data, 0o600)) //nolint:gosec // test temp dir, fixture file names
	}
	require.NoError(t, os.WriteFile(filepath.Join(seeds, "spellbook.json"),
		[]byte(`[{"id":"magic_rocket","name":"Magic Rocket","level":1}]`), 0o600))

	h := newHarness(t)
	h.Store.EXPECT().Migrate(mock.Anything, mock.Anything).Return(nil).Once()
	h.Store.EXPECT().Close().Return(nil).Once()
	cfg := fixtureConfig()
	cfg.SeedsDir = seeds
	_, err = h.build(t, cfg)
	require.ErrorContains(t, err, `"magic_rocket" is not a Wizard spell of the catalogue`)
}

// The spell catalogue is read from the store once, at startup: the lookup,
// its filters and the names are served from memory afterwards, however many
// requests ask (the mock refuses a second Search or Details).
func TestBuild_SpellCatalogueReadOnceAtStartup(t *testing.T) {
	h := newHarness(t)
	h.Store.EXPECT().Migrate(mock.Anything, fixtureSeeds).Return(nil).Once()
	h.Store.EXPECT().Seed(mock.Anything, fixtureSeeds, mock.Anything).Return(nil).Once()
	h.Store.EXPECT().ImportCatalogue(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	fire := spellsvc.Spell{ID: "fireball", Class: "Wizard", Level: 3}
	h.Spell.EXPECT().Search(mock.Anything, spellsvc.Filter{}).Return([]spellsvc.Spell{fire}, nil).Once()
	h.Spell.EXPECT().Details(mock.Anything, []string{"fireball"}).Return([]spellsvc.Spell{fire}, nil).Once()
	h.Auth.EXPECT().RootUser(mock.Anything).Return(nil, domain.ErrNotFound).Once()
	h.Character.EXPECT().CharactersWithoutHPLevel(mock.Anything).Return(nil, nil).Once()

	bs, err := h.build(t, fixtureConfig())
	require.NoError(t, err)
	ctx := context.Background()
	for range 3 {
		page, err := bs.Spells.Search(ctx, characterID, spellsvc.Filter{Class: "wizard"}, spellsvc.PageRequest{})
		require.NoError(t, err)
		assert.Equal(t, 1, page.Total)
		names, err := bs.Spells.Names(ctx)
		require.NoError(t, err)
		assert.Len(t, names, 1)
		got, err := bs.Spells.Get(ctx, "fireball")
		require.NoError(t, err)
		assert.Equal(t, fire, got)
	}
}
