package composition_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
)

// level16XP is the XP that reaches bard level 16 (PHB Table 25, Thief/Bard
// column): the fixture Bard is level 15, so it is the XP ledger's next
// threshold, which shows which rule tables a start took.
const level16XP = 1320000

var (
	fixtureCatalogue = filepath.Join("..", "..", "testdata", "catalogue.json")
	fixtureSeeds     = filepath.Join("..", "..", "testdata", "seeds")
)

// writeArtifact writes the fixture catalogue, changed by mutate, to a temp
// file and returns its path.
func writeArtifact(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	raw, err := os.ReadFile(fixtureCatalogue)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	mutate(doc)
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "catalogue.json")
	require.NoError(t, os.WriteFile(path, out, 0o600))
	return path
}

// asFormat1 turns the artifact into format 1: the same spells, no tables.
func asFormat1(d map[string]any) {
	d["format"] = 1
	delete(d, "rule_tables")
}

// setLevel16XP sets the level-16 XP of the rule tables doc (the artifact's
// rule_tables or the seed file).
func setLevel16XP(tables map[string]any, xp int) {
	rows := tables["bard_experience"].([]any)
	row := rows[15].(map[string]any)
	if row["level"] != float64(16) {
		panic("bard_experience row 15 is not level 16")
	}
	row["xp"] = xp
}

// seedsWithRuleTables copies the testdata seeds into a temp dir and adds a
// rule_tables.json: the fixture's tables, changed by mutate, with a _source
// comment as the campaign seed has.
func seedsWithRuleTables(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, os.DirFS(fixtureSeeds)))
	raw, err := os.ReadFile(fixtureCatalogue)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	tables := doc["rule_tables"].(map[string]any)
	mutate(tables)
	tables["_source"] = "test copy"
	out, err := json.Marshal(tables)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rule_tables.json"), out, 0o600))
	return dir
}

func nextThreshold(t *testing.T, cfg composition.Config) int {
	t.Helper()
	ctx := context.Background()
	bs, err := composition.New(ctx, cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, bs.Close(ctx)) }()
	l, err := bs.XP.Get(ctx, bs.CharacterID)
	require.NoError(t, err)
	require.NotNil(t, l.NextThreshold)
	return *l.NextThreshold
}

// ADR 0021 step 9.6.1: the rule tables' source follows the artifact's format.
// Format 3 takes them from its rule_tables and never reads the seeds' file
// (a broken one is ignored); format 1 takes them from <seeds>/rule_tables.json
// as before, and refuses to start without it, naming the file.
func TestStartup_Step9_6_1_TheRuleTablesSourceFollowsTheFormat(t *testing.T) {
	db := func() string { return filepath.Join(t.TempDir(), "critical.db") }

	t.Run("format 3: the artifact's tables", func(t *testing.T) {
		broken := t.TempDir()
		require.NoError(t, os.CopyFS(broken, os.DirFS(fixtureSeeds)))
		require.NoError(t, os.WriteFile(filepath.Join(broken, "rule_tables.json"), []byte("not json"), 0o600))
		artifact := writeArtifact(t, func(d map[string]any) {
			setLevel16XP(d["rule_tables"].(map[string]any), level16XP+3)
		})
		assert.Equal(t, level16XP+3, nextThreshold(t, composition.Config{DSN: db(), SeedsDir: broken, Catalogue: artifact}))
	})

	t.Run("format 1: the seed's tables", func(t *testing.T) {
		seeds := seedsWithRuleTables(t, func(m map[string]any) { setLevel16XP(m, level16XP+5) })
		artifact := writeArtifact(t, asFormat1)
		assert.Equal(t, level16XP+5, nextThreshold(t, composition.Config{DSN: db(), SeedsDir: seeds, Catalogue: artifact}))
	})

	t.Run("format 1 without the seed file refuses", func(t *testing.T) {
		_, err := composition.New(context.Background(), composition.Config{
			DSN: db(), SeedsDir: fixtureSeeds, Catalogue: writeArtifact(t, asFormat1),
		})
		require.ErrorContains(t, err, "rule_tables.json")
	})
}

// ADR 0021 steps 9.4 and 9.5: the tables are read into memory at every start,
// whatever catalogue version the database recorded. A restart on an artifact
// of the same version (step 5.3: the spells' import does nothing) with a
// corrected table uses the corrected number; a broken table refuses the start
// even then, naming the table.
func TestStartup_Step9_5_TheTablesAreReadAtEveryStart(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "critical.db")
	cfg := composition.Config{DSN: dsn, SeedsDir: fixtureSeeds, Catalogue: fixtureCatalogue}
	assert.Equal(t, level16XP, nextThreshold(t, cfg))

	cfg.Catalogue = writeArtifact(t, func(d map[string]any) {
		setLevel16XP(d["rule_tables"].(map[string]any), level16XP+7)
	})
	assert.Equal(t, level16XP+7, nextThreshold(t, cfg), "same version, corrected table")

	cfg.Catalogue = writeArtifact(t, func(d map[string]any) {
		setLevel16XP(d["rule_tables"].(map[string]any), 1)
	})
	_, err := composition.New(context.Background(), cfg)
	require.ErrorContains(t, err, "rule_tables: experience table")
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
			_, err := composition.New(context.Background(), composition.Config{
				DSN: filepath.Join(t.TempDir(), "critical.db"), SeedsDir: fixtureSeeds, Catalogue: artifact,
			})
			require.ErrorContains(t, err, tc.want)
			require.ErrorContains(t, err, "spell catalogue "+artifact)
		})
	}
}
