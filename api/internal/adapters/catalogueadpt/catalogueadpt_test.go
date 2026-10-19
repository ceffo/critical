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
	assert.Equal(t, catalogue.FormatLanguages, c.Format)
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

// classTables are the keys format 4 adds to rule_tables (ADR 0021 step 10).
var classTables = []string{
	"experience", "hit_dice", "constitution", "intelligence", "wisdom", "con_save_bonus", "race_requirements",
	"race_adjustments", "thac0", "saving_throws", "warrior_attacks", "specialist_attacks", "rate_of_fire",
	"wizard_spells", "priest_spells", "priest_spell_wisdom", "thief_skill_base", "thief_skill_race",
	"thief_skill_dexterity", "thief_skill_armor", "bard_armor_penalty", "bard_skill_base", "backstab",
	"turning_undead",
}

// levelLimits is the key format 5 adds to rule_tables (ADR 0021 step 11).
const levelLimits = "race_level_limits"

// sphereAccess is the key format 6 adds to rule_tables, beside each priest
// spell's sphere codes (ADR 0021 step 12).
const sphereAccess = "sphere_access"

// equipmentTables are the keys format 7 adds to rule_tables, beside the
// top-level equipment object (ADR 0021 step 13).
var equipmentTables = []string{"weapon_vs_armor", "initiative_item"}

// asFormat7 turns the format-8 fixture into a format-7 artifact: the same
// spells and tables, without the languages and the race_languages table.
func asFormat7(d map[string]any) {
	d["format"] = 7
	delete(d, "languages")
	delete(d["rule_tables"].(map[string]any), "race_languages")
}

// asFormat6 turns the format-8 fixture into a format-6 artifact: the same
// spells and tables, without the equipment and its two rule tables.
func asFormat6(d map[string]any) {
	asFormat7(d)
	d["format"] = 6
	delete(d, "equipment")
	for _, k := range equipmentTables {
		delete(d["rule_tables"].(map[string]any), k)
	}
}

// asFormat5 turns the format-7 fixture into a format-5 artifact: the same
// spells without their sphere codes, the tables without the sphere access.
func asFormat5(d map[string]any) {
	asFormat6(d)
	d["format"] = 5
	delete(d["rule_tables"].(map[string]any), sphereAccess)
	for _, s := range spells(d) {
		delete(s.(map[string]any), "spheres")
	}
}

// asFormat4 turns the format-7 fixture into a format-4 artifact: the same
// spells, PHB rule tables and class and race tables, without the racial
// level limits.
func asFormat4(d map[string]any) {
	asFormat5(d)
	d["format"] = 4
	delete(d["rule_tables"].(map[string]any), levelLimits)
}

// asFormat3 turns the format-7 fixture into a format-3 artifact: the same
// spells and the four PHB rule tables of step 9.
func asFormat3(d map[string]any) {
	asFormat4(d)
	d["format"] = 3
	for _, k := range classTables {
		delete(d["rule_tables"].(map[string]any), k)
	}
}

// Step 5.2: unknown format, schema violations (the first ones listed) and
// duplicate ids are refused.
func TestParse_Step2_Refusals(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"unknown format names it", func(d map[string]any) { d["format"] = 9 }, []string{"unknown format 9", "formats 3, 4, 5, 6, 7 and 8", "just catalogue"}},
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
	}
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

func TestParse_NotJSON(t *testing.T) {
	_, err := Parse([]byte("[1, 2]"))
	require.Error(t, err)
}

// ADR 0021 step 9.1: format 3 hands on its rule tables as written
// (ruletablesadpt parses them).
func TestParse_Step9_1_ReadsFormat3(t *testing.T) {
	three, err := ParseArtifact(artifact(t, asFormat3))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatRuleTables, three.Catalogue.Format)
	assert.NotEmpty(t, three.Catalogue.Spells)
	var tables map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(three.RuleTables, &tables))
	assert.ElementsMatch(t, []string{"strength", "dexterity", "bard_spells", "bard_experience"}, keys(tables))
}

// ADR 0021 step 10.1: format 4 hands on the four PHB rule tables and the
// class and race tables as written (ruletablesadpt parses them).
func TestParse_Step10_1_ReadsFormat4(t *testing.T) {
	four, err := ParseArtifact(artifact(t, asFormat4))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatClassTables, four.Catalogue.Format)
	var tables map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(four.RuleTables, &tables))
	assert.ElementsMatch(t, append([]string{"strength", "dexterity", "bard_spells", "bard_experience"}, classTables...), keys(tables))
}

// ADR 0021 step 11.1: format 5 hands on the format-4 tables and the racial
// level limits as written (ruletablesadpt parses them).
func TestParse_Step11_1_ReadsFormat5(t *testing.T) {
	five, err := ParseArtifact(artifact(t, asFormat5))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatLevelLimits, five.Catalogue.Format)
	var tables map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(five.RuleTables, &tables))
	assert.ElementsMatch(t,
		append(append([]string{"strength", "dexterity", "bard_spells", "bard_experience"}, classTables...), levelLimits),
		keys(tables))
}

// ADR 0021 step 9.6.3: format 1 is retired. A format-1 artifact (spells
// only, its tables in the seeds) is refused like format 2, the retracted stat
// shape, and any other unknown format: the refusal names the format found,
// the one this API reads and `just catalogue`, before the schema is checked.
func TestParse_Step9_6_3_RefusesTheRetiredFormat1(t *testing.T) {
	for _, format := range []int{1, 2} {
		_, err := Parse(artifact(t, func(d map[string]any) {
			d["format"] = format
			delete(d, "rule_tables")
		}))
		require.ErrorContains(t, err, fmt.Sprintf(
			"unknown format %d: this API reads formats 3, 4, 5, 6, 7 and 8; run `just catalogue` to fetch a current artifact", format))
	}
}

// ADR 0021 steps 9.1, 9.2 and 10.2: format 3 requires the four tables and
// nothing else in them, format 4 every class and race table too; a schema
// refusal names the format found.
func TestParse_Step9_2_TablesBelongToFormat3Only(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"format 3 without tables", func(d map[string]any) { asFormat3(d); delete(d, "rule_tables") }, []string{"format 3 schema", "rule_tables"}},
		{"format 3 missing a table", func(d map[string]any) {
			asFormat3(d)
			delete(d["rule_tables"].(map[string]any), "dexterity")
		}, []string{"format 3 schema", "dexterity"}},
		{"format 3 with a text in the tables", func(d map[string]any) {
			asFormat3(d)
			d["rule_tables"].(map[string]any)["_source"] = "PHB"
		}, []string{"format 3 schema", "_source"}},
		{"format 3 with a class table", func(d map[string]any) {
			asFormat4(d)
			d["format"] = 3
			for _, k := range classTables[1:] {
				delete(d["rule_tables"].(map[string]any), k)
			}
		}, []string{"format 3 schema", "/rule_tables/experience"}},
		{"format 3 with the level limits", func(d map[string]any) {
			limits := d["rule_tables"].(map[string]any)[levelLimits]
			asFormat3(d)
			d["rule_tables"].(map[string]any)[levelLimits] = limits
		}, []string{"format 3 schema", "/rule_tables/race_level_limits"}},
		{"format 4 missing a class table", func(d map[string]any) {
			asFormat4(d)
			delete(d["rule_tables"].(map[string]any), "turning_undead")
		}, []string{"format 4 schema", "turning_undead"}},
		{"format 4 with a class name in a language", func(d map[string]any) {
			asFormat4(d)
			d["rule_tables"].(map[string]any)["hit_dice"].([]any)[0].(map[string]any)["group"] = "guerrier"
		}, []string{"format 4 schema", "hit_dice/0/group"}},
		// ADR 0021 step 11.2: the level limits belong to format 5 only, and
		// format 5 requires them and every format-4 table.
		{
			"format 4 with the level limits", func(d map[string]any) { asFormat5(d); d["format"] = 4 },
			[]string{"format 4 schema", "/rule_tables/race_level_limits"},
		},
		{"format 5 missing the level limits", func(d map[string]any) {
			asFormat5(d)
			delete(d["rule_tables"].(map[string]any), levelLimits)
		}, []string{"format 5 schema", "race_level_limits"}},
		{"format 5 missing a class table", func(d map[string]any) {
			asFormat5(d)
			delete(d["rule_tables"].(map[string]any), "thac0")
		}, []string{"format 5 schema", "thac0"}},
		{"format 5 with a human row", func(d map[string]any) {
			asFormat5(d)
			d["rule_tables"].(map[string]any)[levelLimits].([]any)[0].(map[string]any)["race"] = "human"
		}, []string{"format 5 schema", "race_level_limits/0/race"}},
		{"format 5 with a level above 20", func(d map[string]any) {
			asFormat5(d)
			d["rule_tables"].(map[string]any)[levelLimits].([]any)[0].(map[string]any)["max_level"] = 21
		}, []string{"format 5 schema", "race_level_limits/0/max_level"}},
		{"format 5 with a class name in a language", func(d map[string]any) {
			asFormat5(d)
			d["rule_tables"].(map[string]any)[levelLimits].([]any)[0].(map[string]any)["class"] = "guerrier"
		}, []string{"format 5 schema", "race_level_limits/0/class"}},
		{"format 5 with 24 level limits", func(d map[string]any) {
			asFormat5(d)
			rt := d["rule_tables"].(map[string]any)
			rt[levelLimits] = rt[levelLimits].([]any)[1:]
		}, []string{"format 5 schema", "race_level_limits"}},
	}
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

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ADR 0021 step 12.1: format 6 hands on the format-5 tables and the sphere
// access as written, and every priest spell's sphere codes.
func TestParse_Step12_1_ReadsFormat6(t *testing.T) {
	six, err := ParseArtifact(artifact(t, asFormat6))
	require.NoError(t, err)
	assert.Nil(t, six.Equipment, "a format-6 artifact has no equipment")
	assert.Equal(t, catalogue.FormatSpheres, six.Catalogue.Format)
	var tables map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(six.RuleTables, &tables))
	assert.ElementsMatch(t,
		append(append([]string{"strength", "dexterity", "bard_spells", "bard_experience"}, classTables...), levelLimits, sphereAccess),
		keys(tables))
	assert.Equal(t, []string{"elemental_fire"}, six.Catalogue.ByID()["produce_flame"].Spheres)
}

// ADR 0021 step 12.2: the sphere codes and access belong to format 6 only,
// and format 6 requires both.
func TestParse_Step12_2_SpheresBelongToFormat6(t *testing.T) {
	priest := func(d map[string]any) map[string]any {
		for _, s := range spells(d) {
			if m := s.(map[string]any); m["id"] == "bless" {
				return m
			}
		}
		t.Fatal("the fixture holds bless")
		return nil
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"format 5 with the sphere access", func(d map[string]any) {
			access := d["rule_tables"].(map[string]any)[sphereAccess]
			asFormat5(d)
			d["rule_tables"].(map[string]any)[sphereAccess] = access
		}, []string{"format 5 schema", "/rule_tables/sphere_access"}},
		{"format 5 with sphere codes", func(d map[string]any) {
			asFormat5(d)
			priest(d)["spheres"] = []any{"all"}
		}, []string{"format 5 schema", "spheres"}},
		{"format 6 missing the sphere access", func(d map[string]any) {
			asFormat6(d)
			delete(d["rule_tables"].(map[string]any), sphereAccess)
		}, []string{"format 6 schema", "sphere_access"}},
		{"format 6 with a priest spell without codes", func(d map[string]any) {
			asFormat6(d)
			delete(priest(d), "spheres")
		}, []string{"format 6 schema", "spheres"}},
		{"format 6 with a sphere name in a language", func(d map[string]any) {
			asFormat6(d)
			priest(d)["spheres"] = []any{"Tout"}
		}, []string{"format 6 schema", "spheres/0"}},
	}
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

// ADR 0021 step 13.1: format 7 hands on the format-6 tables with Tables 52
// and 56, and the equipment object, as written (ruletablesadpt parses them).
func TestParse_Step13_1_ReadsFormat7(t *testing.T) {
	seven, err := ParseArtifact(artifact(t, asFormat7))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatEquipment, seven.Catalogue.Format)
	assert.Nil(t, seven.Languages, "a format-7 artifact has no languages")
	var tables map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(seven.RuleTables, &tables))
	assert.ElementsMatch(t,
		append(append([]string{"strength", "dexterity", "bard_spells", "bard_experience"}, classTables...),
			append([]string{levelLimits, sphereAccess}, equipmentTables...)...),
		keys(tables))
	var equipment map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(seven.Equipment, &equipment))
	assert.ElementsMatch(t, []string{"weapons", "ammunition", "armour", "shields"}, keys(equipment))
}

// ADR 0021 step 13.2: the equipment and its two tables belong to format 7
// only, and format 7 requires all three; a row has the codes and shapes of
// ADR 0033 steps 21 and 25.
func TestParse_Step13_2_EquipmentBelongsToFormat7(t *testing.T) {
	row := func(d map[string]any, list string, i int) map[string]any {
		return d["equipment"].(map[string]any)[list].([]any)[i].(map[string]any)
	}
	tables := func(d map[string]any) map[string]any { return d["rule_tables"].(map[string]any) }
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"format 6 with the equipment", func(d map[string]any) {
			equipment := d["equipment"]
			asFormat6(d)
			d["equipment"] = equipment
		}, []string{"format 6 schema", "/equipment"}},
		{"format 6 with Table 52", func(d map[string]any) {
			table := tables(d)["weapon_vs_armor"]
			asFormat6(d)
			tables(d)["weapon_vs_armor"] = table
		}, []string{"format 6 schema", "/rule_tables/weapon_vs_armor"}},
		{"format 7 without the equipment", func(d map[string]any) { delete(d, "equipment") }, []string{"format 7 schema", "equipment"}},
		{"format 7 without Table 52", func(d map[string]any) { delete(tables(d), "weapon_vs_armor") }, []string{"format 7 schema", "weapon_vs_armor"}},
		{"format 7 without Table 56", func(d map[string]any) { delete(tables(d), "initiative_item") }, []string{"format 7 schema", "initiative_item"}},
		{"a code in capitals", func(d map[string]any) { row(d, "weapons", 0)["code"] = "BattleAxe" }, []string{"format 7 schema", "code"}},
		{"a name without English", func(d map[string]any) { row(d, "weapons", 0)["name"] = map[string]any{"fr": "Hache"} }, []string{"format 7 schema", "en"}},
		{"a weapon type outside the list", func(d map[string]any) { row(d, "weapons", 0)["weapon_type"] = "axe" }, []string{"format 7 schema", "weapon_type"}},
		{"three damage types", func(d map[string]any) { row(d, "weapons", 0)["damage_types"] = []any{"b", "p", "s"} }, []string{"format 7 schema", "damage_types"}},
		{"a speed above 20", func(d map[string]any) { row(d, "weapons", 0)["speed"] = 21 }, []string{"format 7 schema", "speed"}},
		{"damage in words", func(d map[string]any) { row(d, "weapons", 0)["damage_sm"] = "one d8" }, []string{"format 7 schema", "damage_sm"}},
		{"damage S-M without damage L", func(d map[string]any) { delete(row(d, "weapons", 0), "damage_l") }, []string{"format 7 schema", "damage_l"}},
		{"a launcher with damage", func(d map[string]any) {
			row(d, "weapons", 2)["damage_sm"], row(d, "weapons", 2)["damage_l"] = "1d6", "1d6"
		}, []string{"format 7 schema", "damage_sm"}},
		{"an unknown ammunition kind", func(d map[string]any) { row(d, "ammunition", 0)["ammo_kind"] = "bolt" }, []string{"format 7 schema", "ammo_kind"}},
		{"ammunition without a damage type", func(d map[string]any) { row(d, "ammunition", 0)["damage_types"] = []any{} }, []string{"format 7 schema", "damage_types"}},
		{"a base AC above 10", func(d map[string]any) { row(d, "armour", 0)["base_ac"] = 11 }, []string{"format 7 schema", "base_ac"}},
		{"an unknown Table 29 column", func(d map[string]any) { row(d, "armour", 0)["skill_column"] = "plate" }, []string{"format 7 schema", "skill_column"}},
		{"an unknown Table 52 row", func(d map[string]any) { row(d, "armour", 0)["armor_row"] = "bronze_plate" }, []string{"format 7 schema", "armor_row"}},
		{"a shield bonus above 5", func(d map[string]any) { row(d, "shields", 0)["ac_melee"] = 6 }, []string{"format 7 schema", "ac_melee"}},
		{"a field the format lacks", func(d map[string]any) { row(d, "armour", 0)["weight"] = 35 }, []string{"format 7 schema", "weight"}},
		{"Table 52 with ten rows", func(d map[string]any) {
			tables(d)["weapon_vs_armor"] = tables(d)["weapon_vs_armor"].([]any)[1:]
		}, []string{"format 7 schema", "weapon_vs_armor"}},
		{"Table 56 with a modifier above 20", func(d map[string]any) {
			tables(d)["initiative_item"].([]any)[0].(map[string]any)["modifier"] = 21
		}, []string{"format 7 schema", "modifier"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(artifact(t, func(d map[string]any) { asFormat7(d); tc.mutate(d) }))
			require.Error(t, err)
			for _, w := range tc.want {
				assert.Contains(t, err.Error(), w)
			}
		})
	}
}

// ADR 0021 step 14.1: format 8 hands on the format-7 tables, the equipment and
// the language catalogue with each race's list as written (ruletablesadpt
// parses them).
func TestParse_Step14_1_ReadsFormat8(t *testing.T) {
	eight, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatLanguages, eight.Catalogue.Format)
	var tables map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(eight.RuleTables, &tables))
	assert.Contains(t, keys(tables), "race_languages")
	var languages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(eight.Languages, &languages))
	assert.NotEmpty(t, languages)
	assert.ElementsMatch(t, []string{"code", "name"}, keys(languages[0]))
}

// ADR 0021 step 14.2: the languages and the race_languages table belong to
// format 8 only, and format 8 requires both; a row has the codes and shapes of
// ADR 0032 step 4.
func TestParse_Step14_2_LanguagesBelongToFormat8(t *testing.T) {
	tables := func(d map[string]any) map[string]any { return d["rule_tables"].(map[string]any) }
	language := func(d map[string]any, i int) map[string]any { return d["languages"].([]any)[i].(map[string]any) }
	tests := []struct {
		name   string
		mutate func(map[string]any)
		want   []string
	}{
		{"format 7 with the languages", func(d map[string]any) {
			languages := d["languages"]
			asFormat7(d)
			d["languages"] = languages
		}, []string{"format 7 schema", "/languages"}},
		{"format 7 with race_languages", func(d map[string]any) {
			rows := tables(d)["race_languages"]
			asFormat7(d)
			tables(d)["race_languages"] = rows
		}, []string{"format 7 schema", "/rule_tables/race_languages"}},
		{"format 8 without the languages", func(d map[string]any) { delete(d, "languages") }, []string{"format 8 schema", "languages"}},
		{"format 8 without race_languages", func(d map[string]any) { delete(tables(d), "race_languages") }, []string{"format 8 schema", "race_languages"}},
		{"a code in capitals", func(d map[string]any) { language(d, 0)["code"] = "Common" }, []string{"format 8 schema", "code"}},
		{"a name without English", func(d map[string]any) { language(d, 0)["name"] = map[string]any{"fr": "Commun"} }, []string{"format 8 schema", "en"}},
		{"a field the format lacks", func(d map[string]any) { language(d, 0)["family"] = "x" }, []string{"format 8 schema", "family"}},
		{"a race outside the six", func(d map[string]any) {
			tables(d)["race_languages"].([]any)[0].(map[string]any)["race"] = "orc"
		}, []string{"format 8 schema", "race"}},
	}
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
