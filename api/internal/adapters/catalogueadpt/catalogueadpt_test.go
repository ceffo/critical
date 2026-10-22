package catalogueadpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

const fixture = "../../../testdata/catalogue.json"

// The embedded schema is the committed one, byte for byte.
func TestEmbeddedSchemaIsTheCommittedOne(t *testing.T) {
	committed, err := os.ReadFile("../../../../schemas/catalogue.schema.json")
	require.NoError(t, err)
	assert.True(t, bytes.Equal(committed, schemaJSON), "copy schemas/catalogue.schema.json to api/internal/adapters/catalogueadpt/")
}

func TestLoad_Fixture(t *testing.T) {
	a, err := LoadArtifact(fixture)
	require.NoError(t, err)
	c := a.Catalogue
	assert.Equal(t, catalogue.FormatSpellImmunities, c.Format)
	assert.Len(t, c.ProficiencyDescriptions, 65, "the fixture carries the proficiency descriptions")
	assert.Equal(t, "1970-01-01", c.Version)
	assert.NotEmpty(t, a.RuleTables, "the fixture carries the rule tables")
	assert.NotEmpty(t, a.Equipment, "the fixture carries the equipment")
	assert.NotEmpty(t, a.Languages, "the fixture carries the languages")
	mm, ok := c.ByID()["magic_missile"]
	require.True(t, ok)
	assert.Equal(t, localized.Text{"en": "Magic Missile", "fr": "Projectile magique"}, mm.Name)
	assert.NotEmpty(t, mm.Description["fr"])
	assert.Equal(t, "PHB", mm.Source)
	assert.Equal(t, localized.Text{"en": "60 yds. + 10 yds./level"}, mm.Range, "the plain texts are the English books'")
	armor := c.ByID()["armor"]
	assert.Equal(t, localized.Text{"en": "Armor"}, armor.Name, "an omitted language has no entry")
	assert.NotContains(t, armor.Description, "fr")
	assert.Equal(t, localized.Text{"en": "Divination"}, c.ByID()["detect_magic_priest"].Sphere)
	assert.Empty(t, armor.Sphere, "no sphere: an empty text")
	assert.Equal(t, []string{"divination"}, c.ByID()["detect_magic_priest"].Spheres, "format 6: the sphere codes")
	assert.Empty(t, armor.Spheres, "a wizard spell has no sphere code")
}

// Step 5.1: no file -> an error naming the path and `just catalogue`.
func TestLoad_Step1_MissingFileNamesThePathAndTheRecipe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.json")
	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
	assert.Contains(t, err.Error(), "just catalogue")
}

func artifact(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile(fixture)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	mutate(doc)
	out, err := json.Marshal(doc)
	require.NoError(t, err)
	return out
}

func spells(doc map[string]any) []any { return doc["spells"].([]any) }

// refuses runs each mutation of the fixture through Parse and wants an error holding every text.
func refuses(t *testing.T, tests []refusal) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(artifact(t, tc.mutate))
			require.Error(t, err)
			for _, w := range tc.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

type refusal struct {
	name   string
	mutate func(map[string]any)
	want   []string
}

func tables(d map[string]any) map[string]any { return d["rule_tables"].(map[string]any) }

func proficiencyRow(d map[string]any, i int) map[string]any {
	return d["proficiency_descriptions"].([]any)[i].(map[string]any)
}

// Step 5.2: unknown format, schema violations (the first ones listed) and
// duplicate ids are refused.
func TestParse_Step2_Refusals(t *testing.T) {
	refuses(t, []refusal{
		{"unknown format names it", func(d map[string]any) { d["format"] = 13 }, []string{"unknown format 13", "format 12", "just catalogue"}},
		{"missing format", func(d map[string]any) { delete(d, "format") }, []string{"unknown format none"}},
		{"schema violation lists the offence", func(d map[string]any) {
			spells(d)[0].(map[string]any)["level"] = 12
		}, []string{"schema", "/spells/0/level"}},
		{"several offences, capped", func(d map[string]any) {
			for _, s := range spells(d) {
				s.(map[string]any)["class"] = "Bard"
			}
		}, []string{"/spells/0/class", "more"}},
		{"duplicate id", func(d map[string]any) {
			d["spells"] = append(spells(d), spells(d)[0], spells(d)[1])
		}, []string{"duplicate spell ids: alarm, armor"}},
	})
}

func TestParse_NotJSON(t *testing.T) {
	_, err := Parse([]byte("[1, 2]"))
	require.Error(t, err)
}

// ADR 0021 step 16.1: the artifact hands on the proficiency descriptions, both
// languages of every row, with the rule tables, the equipment and the languages
// as written (ruletablesadpt parses them).
func TestParse_Step16_ReadsTheProficiencyDescriptions(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	require.Len(t, a.Catalogue.ProficiencyDescriptions, 65)
	first := a.Catalogue.ProficiencyDescriptions[0]
	assert.Equal(t, "agriculture", first.ID)
	assert.Equal(t, []string{"en", "fr"}, first.Description.Languages())
	assert.Contains(t, first.Description["fr"], "inventé")
	assert.NotEmpty(t, a.RuleTables)
	assert.NotEmpty(t, a.Equipment)
	assert.NotEmpty(t, a.Languages)
}

// ADR 0021 step 17.1: every wisdom row carries the magical defence
// adjustment, an integer from -6 to +4 (the schema refuses a row without it or
// outside the range); later formats keep the column.
func TestParse_Step17_ReadsTheMagicDefence(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatSpellImmunities, a.Catalogue.Format)
	wisdom := func(d map[string]any, i int) map[string]any { return tables(d)["wisdom"].([]any)[i].(map[string]any) }
	refuses(t, []refusal{
		{"a wisdom row without it", func(d map[string]any) { delete(wisdom(d, 2), "magic_defence") }, []string{"format 12 schema", "wisdom/2", "magic_defence"}},
		{"below -6", func(d map[string]any) { wisdom(d, 0)["magic_defence"] = -7 }, []string{"format 12 schema", "wisdom/0/magic_defence"}},
		{"above +4", func(d map[string]any) { wisdom(d, 24)["magic_defence"] = 5 }, []string{"format 12 schema", "wisdom/24/magic_defence"}},
	})
}

// ADR 0021 step 17.3: format 9 (the step 16 shape, no magical defence column)
// is refused as an unknown format that names the format read and `just catalogue`.
func TestParse_Step17_RefusesFormat9WithTheCatalogueAdvice(t *testing.T) {
	_, err := Parse(artifact(t, func(d map[string]any) {
		d["format"] = 9
		for _, r := range tables(d)["wisdom"].([]any) {
			delete(r.(map[string]any), "magic_defence")
		}
	}))
	require.ErrorContains(t, err, "unknown format 9: this API reads format 12; run `just catalogue` to fetch a current artifact")
}

// ADR 0021 step 16.5: format 8, or any earlier format, is refused as an unknown
// format that names the one this API reads and `just catalogue`, before the
// schema is checked.
func TestParse_Step16_RefusesFormat8WithTheCatalogueAdvice(t *testing.T) {
	for _, format := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		_, err := Parse(artifact(t, func(d map[string]any) {
			d["format"] = format
			delete(d, "proficiency_descriptions")
		}))
		require.ErrorContains(t, err, fmt.Sprintf(
			"unknown format %d: this API reads format 12; run `just catalogue` to fetch a current artifact", format))
	}
}

// ADR 0021 step 18.1 and 18.3: the class, group and race definitions are read
// (the schema's own checks are cataloguecheck's); format 10, the step 17 shape
// without the definitions, is refused as an unknown format that names the
// format read and `just catalogue`.
func TestParse_Step18_ReadsTheDefinitionsAndRefusesFormat10(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatSpellImmunities, a.Catalogue.Format)
	refuses(t, []refusal{
		{"a class without its group", func(d map[string]any) { delete(d["classes"].([]any)[0].(map[string]any), "group") }, []string{"format 12 schema", "classes/0", "group"}},
		{"no races", func(d map[string]any) { delete(d, "races") }, []string{"format 12 schema", "races"}},
	})
	_, err = Parse(artifact(t, func(d map[string]any) {
		d["format"] = 10
		for _, k := range []string{"classes", "groups", "races"} {
			delete(d, k)
		}
	}))
	require.ErrorContains(t, err, "unknown format 10: this API reads format 12; run `just catalogue` to fetch a current artifact")
}

// ADR 0021 steps 19.1 and 19.3: format 12 is read; a wisdom row without its
// spell immunities is refused by the schema, and format 11, the step 18 shape
// without the column, is refused as an unknown format that names format 12 and
// `just catalogue`.
func TestParse_Step19_ReadsFormat12AndRefusesFormat11(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, 12, a.Catalogue.Format)
	wisdom := func(d map[string]any, i int) map[string]any { return tables(d)["wisdom"].([]any)[i].(map[string]any) }
	refuses(t, []refusal{
		{"a wisdom row without them", func(d map[string]any) { delete(wisdom(d, 3), "spell_immunities") }, []string{"format 12 schema", "wisdom/3", "spell_immunities"}},
	})
	_, err = Parse(artifact(t, func(d map[string]any) {
		d["format"] = 11
		for _, r := range tables(d)["wisdom"].([]any) {
			delete(r.(map[string]any), "spell_immunities")
		}
	}))
	require.ErrorContains(t, err, "unknown format 11: this API reads format 12; run `just catalogue` to fetch a current artifact")
}

// ADR 0021 step 16.1: the rows are exactly the schema's (65, both languages,
// step 1's subset) and an id appears once.
func TestParse_Step16_RefusesBadProficiencyDescriptions(t *testing.T) {
	refuses(t, []refusal{
		{"no list", func(d map[string]any) { delete(d, "proficiency_descriptions") }, []string{"format 12 schema", "proficiency_descriptions"}},
		{"64 rows", func(d map[string]any) {
			d["proficiency_descriptions"] = d["proficiency_descriptions"].([]any)[1:]
		}, []string{"format 12 schema", "proficiency_descriptions"}},
		{"no French", func(d map[string]any) {
			delete(proficiencyRow(d, 3)["description"].(map[string]any), "fr")
		}, []string{"format 12 schema", "proficiency_descriptions/3/description", "fr"}},
		{"no English", func(d map[string]any) {
			delete(proficiencyRow(d, 3)["description"].(map[string]any), "en")
		}, []string{"format 12 schema", "proficiency_descriptions/3/description", "en"}},
		{"a heading in the text", func(d map[string]any) {
			proficiencyRow(d, 3)["description"].(map[string]any)["en"] = "x\n\n# H"
		}, []string{"format 12 schema", "proficiency_descriptions/3/description/en"}},
		{"a field beside the text", func(d map[string]any) { proficiencyRow(d, 3)["name"] = "x" }, []string{"format 12 schema", "name"}},
		{"an id twice", func(d map[string]any) {
			proficiencyRow(d, 4)["id"] = "agriculture"
		}, []string{"duplicate proficiency description ids: agriculture"}},
	})
}

// ADR 0021 steps 9.2 to 14.2: the rule tables, the equipment and the languages
// are checked by the schema; a refusal names the format found.
func TestParse_Step9_to_14_TablesEquipmentAndLanguagesAreSchemaChecked(t *testing.T) {
	row := func(d map[string]any, list string, i int) map[string]any {
		return d["equipment"].(map[string]any)[list].([]any)[i].(map[string]any)
	}
	language := func(d map[string]any, i int) map[string]any { return d["languages"].([]any)[i].(map[string]any) }
	priest := func(d map[string]any) map[string]any {
		for _, s := range spells(d) {
			if m := s.(map[string]any); m["id"] == "bless" {
				return m
			}
		}
		t.Fatal("the fixture holds bless")
		return nil
	}
	refuses(t, []refusal{
		{"no tables", func(d map[string]any) { delete(d, "rule_tables") }, []string{"format 12 schema", "rule_tables"}},
		{"a missing table", func(d map[string]any) { delete(tables(d), "dexterity") }, []string{"format 12 schema", "dexterity"}},
		{"a text in the tables", func(d map[string]any) { tables(d)["_source"] = "PHB" }, []string{"format 12 schema", "_source"}},
		{"a missing class table", func(d map[string]any) { delete(tables(d), "turning_undead") }, []string{"format 12 schema", "turning_undead"}},
		{"a class name in a language", func(d map[string]any) {
			tables(d)["hit_dice"].([]any)[0].(map[string]any)["group"] = "guerrier"
		}, []string{"format 12 schema", "hit_dice/0/group"}},
		{"no level limits", func(d map[string]any) { delete(tables(d), "race_level_limits") }, []string{"format 12 schema", "race_level_limits"}},
		{"a human level limit", func(d map[string]any) {
			tables(d)["race_level_limits"].([]any)[0].(map[string]any)["race"] = "human"
		}, []string{"format 12 schema", "race_level_limits/0/race"}},
		{"a level limit above 20", func(d map[string]any) {
			tables(d)["race_level_limits"].([]any)[0].(map[string]any)["max_level"] = 21
		}, []string{"format 12 schema", "race_level_limits/0/max_level"}},
		{"24 level limits", func(d map[string]any) {
			tables(d)["race_level_limits"] = tables(d)["race_level_limits"].([]any)[1:]
		}, []string{"format 12 schema", "race_level_limits"}},
		{"no sphere access", func(d map[string]any) { delete(tables(d), "sphere_access") }, []string{"format 12 schema", "sphere_access"}},
		{"a priest spell without codes", func(d map[string]any) { delete(priest(d), "spheres") }, []string{"format 12 schema", "spheres"}},
		{"a sphere name in a language", func(d map[string]any) { priest(d)["spheres"] = []any{"Tout"} }, []string{"format 12 schema", "spheres/0"}},
		{"no equipment", func(d map[string]any) { delete(d, "equipment") }, []string{"format 12 schema", "equipment"}},
		{"no Table 52", func(d map[string]any) { delete(tables(d), "weapon_vs_armor") }, []string{"format 12 schema", "weapon_vs_armor"}},
		{"no Table 56", func(d map[string]any) { delete(tables(d), "initiative_item") }, []string{"format 12 schema", "initiative_item"}},
		{"a code in capitals", func(d map[string]any) { row(d, "weapons", 0)["code"] = "BattleAxe" }, []string{"format 12 schema", "code"}},
		{"a weapon type outside the list", func(d map[string]any) { row(d, "weapons", 0)["weapon_type"] = "axe" }, []string{"format 12 schema", "weapon_type"}},
		{"a speed above 20", func(d map[string]any) { row(d, "weapons", 0)["speed"] = 21 }, []string{"format 12 schema", "speed"}},
		{"damage in words", func(d map[string]any) { row(d, "weapons", 0)["damage_sm"] = "one d8" }, []string{"format 12 schema", "damage_sm"}},
		{"an unknown Table 29 column", func(d map[string]any) { row(d, "armour", 0)["skill_column"] = "plate" }, []string{"format 12 schema", "skill_column"}},
		{"a field the format lacks", func(d map[string]any) { row(d, "armour", 0)["weight"] = 35 }, []string{"format 12 schema", "weight"}},
		{"Table 56 with a modifier above 20", func(d map[string]any) {
			tables(d)["initiative_item"].([]any)[0].(map[string]any)["modifier"] = 21
		}, []string{"format 12 schema", "modifier"}},
		{"no languages", func(d map[string]any) { delete(d, "languages") }, []string{"format 12 schema", "languages"}},
		{"no race_languages", func(d map[string]any) { delete(tables(d), "race_languages") }, []string{"format 12 schema", "race_languages"}},
		{"a language code in capitals", func(d map[string]any) { language(d, 0)["code"] = "Common" }, []string{"format 12 schema", "code"}},
		{"a language name without English", func(d map[string]any) {
			language(d, 0)["name"] = map[string]any{"fr": "Commun"}
		}, []string{"format 12 schema", "en"}},
		{"a race outside the six", func(d map[string]any) {
			tables(d)["race_languages"].([]any)[0].(map[string]any)["race"] = "orc"
		}, []string{"format 12 schema", "race"}},
	})
}

// ADR 0021 steps 12.1, 13.1 and 14.1: the fixture's parts are handed on as
// written, and the sphere codes are read.
func TestParse_Fixture_HandsOnTheTablesEquipmentAndLanguages(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	var rt map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(a.RuleTables, &rt))
	for _, k := range []string{"strength", "thac0", "race_level_limits", "sphere_access", "weapon_vs_armor", "race_languages"} {
		assert.Contains(t, keys(rt), k)
	}
	var equipment map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(a.Equipment, &equipment))
	assert.ElementsMatch(t, []string{"weapons", "ammunition", "armour", "shields"}, keys(equipment))
	var languages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(a.Languages, &languages))
	assert.ElementsMatch(t, []string{"code", "name"}, keys(languages[0]))
	assert.Equal(t, []string{"elemental_fire"}, a.Catalogue.ByID()["produce_flame"].Spheres)
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// buffsvc.ScrollLevel reads a scroll's spell at one caster level, which is
// exact only while no casting time under a round grows with the level. Guards
// that assumption on the fixture: a casting time under a round (10 segments)
// is the same at levels 1 and 20.
func TestFixture_ADR0035Item6_2_NoCastingTimeUnderARoundVariesWithLevel(t *testing.T) {
	a, err := LoadArtifact(fixture)
	require.NoError(t, err)
	for _, s := range a.Catalogue.Spells {
		ct := spellstats.ParseCastingTime(s.CastingTime.Default())
		lo, hi := ct.At(1), ct.At(20)
		if lo.Unit == "segment" && lo.Amount < 10 || hi.Unit == "segment" && hi.Amount < 10 {
			assert.Equal(t, lo, hi, "%s (%q) varies with level below a round", s.ID, ct.Raw)
		}
	}
}
