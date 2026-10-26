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
	assert.Equal(t, catalogue.FormatMeditating, c.Format)
	assert.Len(t, c.ProficiencyDescriptions, 66, "the fixture carries the proficiency descriptions")
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
		{"unknown format names it", func(d map[string]any) { d["format"] = 21 }, []string{"unknown format 21", "format 20", "just catalogue"}},
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
	require.Len(t, a.Catalogue.ProficiencyDescriptions, 66)
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
	assert.Equal(t, catalogue.FormatMeditating, a.Catalogue.Format)
	wisdom := func(d map[string]any, i int) map[string]any { return tables(d)["wisdom"].([]any)[i].(map[string]any) }
	refuses(t, []refusal{
		{"a wisdom row without it", func(d map[string]any) { delete(wisdom(d, 2), "magic_defence") }, []string{"format 20 schema", "wisdom/2", "magic_defence"}},
		{"below -6", func(d map[string]any) { wisdom(d, 0)["magic_defence"] = -7 }, []string{"format 20 schema", "wisdom/0/magic_defence"}},
		{"above +4", func(d map[string]any) { wisdom(d, 24)["magic_defence"] = 5 }, []string{"format 20 schema", "wisdom/24/magic_defence"}},
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
	require.ErrorContains(t, err, "unknown format 9: this API reads format 20; run `just catalogue` to fetch a current artifact")
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
			"unknown format %d: this API reads format 20; run `just catalogue` to fetch a current artifact", format))
	}
}

// ADR 0021 step 18.1 and 18.3: the class, group and race definitions are read
// (the schema's own checks are cataloguecheck's); format 10, the step 17 shape
// without the definitions, is refused as an unknown format that names the
// format read and `just catalogue`.
func TestParse_Step18_ReadsTheDefinitionsAndRefusesFormat10(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatMeditating, a.Catalogue.Format)
	refuses(t, []refusal{
		{"a class without its group", func(d map[string]any) { delete(d["classes"].([]any)[0].(map[string]any), "group") }, []string{"format 20 schema", "classes/0", "group"}},
		{"no races", func(d map[string]any) { delete(d, "races") }, []string{"format 20 schema", "races"}},
	})
	_, err = Parse(artifact(t, func(d map[string]any) {
		d["format"] = 10
		for _, k := range []string{"classes", "groups", "races"} {
			delete(d, k)
		}
	}))
	require.ErrorContains(t, err, "unknown format 10: this API reads format 20; run `just catalogue` to fetch a current artifact")
}

// ADR 0021 steps 20.1 and 20.3: the artifact is read with the psionicist's
// class and group entries, its grid rows, the three psionics tables and the
// powers; a power of an unknown kind is refused by the schema; format 12, the
// step 19 shape, is refused as an unknown format that names format 14 and
// `just catalogue`.
func TestParse_Step20_ReadsPsionicsAndRefusesFormat12(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatMeditating, a.Catalogue.Format)
	refuses(t, []refusal{
		{"a power of an unknown kind", func(d map[string]any) { d["powers"].([]any)[0].(map[string]any)["kind"] = "spell" }, []string{"format 20 schema", "powers/0/kind"}},
		{"a power with a cost that is a word", func(d map[string]any) { d["powers"].([]any)[0].(map[string]any)["initial_cost"] = "many" }, []string{"format 20 schema", "powers/0/initial_cost"}},
		{"no powers", func(d map[string]any) { delete(d, "powers") }, []string{"format 20 schema", "powers"}},
		{"no psionics table", func(d map[string]any) { delete(tables(d), "psp_recovery") }, []string{"format 20 schema", "psp_recovery"}},
	})
	_, err = Parse(artifact(t, func(d map[string]any) {
		d["format"] = 12
		for _, k := range []string{"disciplines", "powers"} {
			delete(d, k)
		}
	}))
	require.ErrorContains(t, err, "unknown format 12: this API reads format 20; run `just catalogue` to fetch a current artifact")
}

// ADR 0021 step 23.1: a power's optional slot reaches the domain power, a slot outside science and
// devotion is refused by the schema. A sphere_access table stays refused (step 21.1).
func TestParse_Step23_APowerReadsItsSlot(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	slots := map[string]string{}
	for _, p := range a.Powers {
		slots[p.ID] = p.Slot
	}
	assert.Equal(t, "science", slots["psionic_blast"], "the fixture's Psionic Blast names its slot")
	assert.Empty(t, slots["mind_thrust"], "an attack mode with no slot leaves it to its kind")
	refuses(t, []refusal{
		{"a slot outside science and devotion", func(d map[string]any) { d["powers"].([]any)[0].(map[string]any)["slot"] = "defence_mode" }, []string{"format 20 schema", "powers/0/slot"}},
		{"a sphere_access table", func(d map[string]any) {
			tables(d)["sphere_access"] = []any{map[string]any{"class": "cleric", "sphere": "sun", "access": "major"}}
		}, []string{"format 20 schema", "sphere_access"}},
	})
}

// ADR 0021 steps 27.5 (and 24.1): format 20 is read, a class's optional multiclass flag is a boolean
// (the definitions read it, ruletablesadpt), and format 16 is refused as an unknown format that
// names format 20 and `just catalogue`.
func TestParse_Step25_ReadsFormat19AndRefusesFormat16(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	assert.Equal(t, catalogue.FormatMeditating, a.Catalogue.Format)
	refuses(t, []refusal{
		{"a multiclass flag that is not a boolean", func(d map[string]any) {
			d["classes"].([]any)[0].(map[string]any)["multiclass"] = "no"
		}, []string{"format 20 schema", "classes/0/multiclass"}},
	})
	_, err = Parse(artifact(t, func(d map[string]any) { d["format"] = 16 }))
	require.ErrorContains(t, err, "unknown format 16: this API reads format 20; run `just catalogue` to fetch a current artifact")
}

// ADR 0021 step 20.3 and ADR 0023 step 3: French is optional on a power's text
// (the book has powers whose OCR text could not be matched) and the artifact is
// accepted without it, so a reader serves the English body for a missing
// language; English is required.
func TestParse_Step20_APowerWithoutFrenchTextFallsBackToEnglish(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(d map[string]any) {
		delete(d["powers"].([]any)[0].(map[string]any)["description"].(map[string]any), "fr")
	}))
	require.NoError(t, err, "no French description is not a refusal")
	assert.Equal(t, catalogue.FormatMeditating, a.Catalogue.Format)

	refuses(t, []refusal{
		{"a power description without English", func(d map[string]any) {
			delete(d["powers"].([]any)[0].(map[string]any)["description"].(map[string]any), "en")
		}, []string{"format 20 schema", "powers/0/description", "en"}},
	})
	missing := localized.Text{"en": "Seen from afar."}
	assert.Equal(t, "Seen from afar.", missing.Any(), "the English body stands in for the language the text lacks")
}

// ADR 0043 step 8: every power of the fixture parses with every field (a
// number, `variable` or `contact` cost, an optional score and maintenance,
// prerequisites, the language maps), the six disciplines come in book order,
// and an unknown discipline or prerequisite is refused at start.
func TestPowerCatalogue_Step8_EveryPowerParses(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	require.Len(t, a.Disciplines, 6)
	assert.Equal(t, "Psychometabolism", a.Disciplines[2].Name["en"])
	require.Len(t, a.Powers, 17)
	byID := map[string]catalogue.Power{}
	for _, p := range a.Powers {
		byID[p.ID] = p
	}

	aura := byID["aura_sight"]
	assert.Equal(t, catalogue.Cost{Amount: 9}, aura.InitialCost)
	assert.Equal(t, &catalogue.PowerScore{Ability: "wis", Modifier: -5}, aura.Score)
	assert.Equal(t, &catalogue.Maintenance{Cost: catalogue.Cost{Amount: 9}, Period: 1}, aura.Maintenance)
	assert.Equal(t, "Vision de l'Aura", aura.Name["fr"])
	assert.NotEmpty(t, aura.Range["fr"])
	assert.Equal(t, "science", aura.Kind)

	assert.Equal(t, catalogue.Cost{Kind: catalogue.CostVariable}, byID["telekinesis"].InitialCost)
	assert.Equal(t, catalogue.Cost{Kind: catalogue.CostContact}, byID["mindlink"].InitialCost)
	// ADR 0044 step 1.4: the contact power names its table (a variable cost, so the entered-cost
	// path is unchanged); Switch Personality is a contact with a plus amount.
	assert.Equal(t, catalogue.Cost{Kind: catalogue.CostVariable, Table: "contact_cost"}, byID["contact"].InitialCost)
	assert.Equal(t, catalogue.Cost{Kind: catalogue.CostContact, Plus: 30}, byID["switch_personality"].InitialCost)
	assert.Equal(t, catalogue.Cost{Kind: catalogue.CostVariable}, byID["telekinesis"].Maintenance.Cost)
	assert.Nil(t, byID["psionic_blast"].Maintenance, "a power that cannot be maintained has none")
	assert.Equal(t, []string{"mindlink", "contact"}, byID["mind_thrust"].Prerequisites)
	assert.Empty(t, byID["telekinesis"].Prerequisites)

	refuses(t, []refusal{
		{"a power of an unknown discipline", func(d map[string]any) {
			d["powers"].([]any)[0].(map[string]any)["discipline"] = "pyromancy"
		}, []string{"power aura_sight", `unknown discipline "pyromancy"`}},
		{"a prerequisite that is no power", func(d map[string]any) {
			d["powers"].([]any)[0].(map[string]any)["prerequisites"] = []any{"levitation"}
		}, []string{"power aura_sight", `unknown prerequisite "levitation"`}},
		{"a power id twice", func(d map[string]any) {
			d["powers"].([]any)[1].(map[string]any)["id"] = "aura_sight"
		}, []string{"power aura_sight: repeated"}},
	})
}

// ADR 0021 step 16.1: the rows are exactly the schema's (65, both languages,
// step 1's subset) and an id appears once.
func TestParse_Step16_RefusesBadProficiencyDescriptions(t *testing.T) {
	refuses(t, []refusal{
		{"no list", func(d map[string]any) { delete(d, "proficiency_descriptions") }, []string{"format 20 schema", "proficiency_descriptions"}},
		{"64 rows", func(d map[string]any) {
			d["proficiency_descriptions"] = d["proficiency_descriptions"].([]any)[1:]
		}, []string{"format 20 schema", "proficiency_descriptions"}},
		{"no French", func(d map[string]any) {
			delete(proficiencyRow(d, 3)["description"].(map[string]any), "fr")
		}, []string{"format 20 schema", "proficiency_descriptions/3/description", "fr"}},
		{"no English", func(d map[string]any) {
			delete(proficiencyRow(d, 3)["description"].(map[string]any), "en")
		}, []string{"format 20 schema", "proficiency_descriptions/3/description", "en"}},
		{"a heading in the text", func(d map[string]any) {
			proficiencyRow(d, 3)["description"].(map[string]any)["en"] = "x\n\n# H"
		}, []string{"format 20 schema", "proficiency_descriptions/3/description/en"}},
		{"a field beside the text", func(d map[string]any) { proficiencyRow(d, 3)["name"] = "x" }, []string{"format 20 schema", "name"}},
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
		{"no tables", func(d map[string]any) { delete(d, "rule_tables") }, []string{"format 20 schema", "rule_tables"}},
		{"a missing table", func(d map[string]any) { delete(tables(d), "dexterity") }, []string{"format 20 schema", "dexterity"}},
		{"a text in the tables", func(d map[string]any) { tables(d)["_source"] = "PHB" }, []string{"format 20 schema", "_source"}},
		{"a missing class table", func(d map[string]any) { delete(tables(d), "turning_undead") }, []string{"format 20 schema", "turning_undead"}},
		{"a class name in a language", func(d map[string]any) {
			tables(d)["hit_dice"].([]any)[0].(map[string]any)["group"] = "guerrier"
		}, []string{"format 20 schema", "hit_dice/0/group"}},
		{"no level limits", func(d map[string]any) { delete(tables(d), "race_level_limits") }, []string{"format 20 schema", "race_level_limits"}},
		{"a human level limit", func(d map[string]any) {
			tables(d)["race_level_limits"].([]any)[0].(map[string]any)["race"] = "human"
		}, []string{"format 20 schema", "race_level_limits/0/race"}},
		{"a level limit above 20", func(d map[string]any) {
			tables(d)["race_level_limits"].([]any)[0].(map[string]any)["max_level"] = 21
		}, []string{"format 20 schema", "race_level_limits/0/max_level"}},
		{"24 level limits", func(d map[string]any) {
			tables(d)["race_level_limits"] = tables(d)["race_level_limits"].([]any)[1:]
		}, []string{"format 20 schema", "race_level_limits"}},
		{"a priest spell without codes", func(d map[string]any) { delete(priest(d), "spheres") }, []string{"format 20 schema", "spheres"}},
		{"a sphere name in a language", func(d map[string]any) { priest(d)["spheres"] = []any{"Tout"} }, []string{"format 20 schema", "spheres/0"}},
		{"no equipment", func(d map[string]any) { delete(d, "equipment") }, []string{"format 20 schema", "equipment"}},
		{"no Table 52", func(d map[string]any) { delete(tables(d), "weapon_vs_armor") }, []string{"format 20 schema", "weapon_vs_armor"}},
		{"no Table 56", func(d map[string]any) { delete(tables(d), "initiative_item") }, []string{"format 20 schema", "initiative_item"}},
		{"a code in capitals", func(d map[string]any) { row(d, "weapons", 0)["code"] = "BattleAxe" }, []string{"format 20 schema", "code"}},
		{"a weapon type outside the list", func(d map[string]any) { row(d, "weapons", 0)["weapon_type"] = "axe" }, []string{"format 20 schema", "weapon_type"}},
		{"a speed above 20", func(d map[string]any) { row(d, "weapons", 0)["speed"] = 21 }, []string{"format 20 schema", "speed"}},
		{"damage in words", func(d map[string]any) { row(d, "weapons", 0)["damage_sm"] = "one d8" }, []string{"format 20 schema", "damage_sm"}},
		{"an unknown Table 29 column", func(d map[string]any) { row(d, "armour", 0)["skill_column"] = "plate" }, []string{"format 20 schema", "skill_column"}},
		{"a field the format lacks", func(d map[string]any) { row(d, "armour", 0)["weight"] = 35 }, []string{"format 20 schema", "weight"}},
		{"Table 56 with a modifier above 20", func(d map[string]any) {
			tables(d)["initiative_item"].([]any)[0].(map[string]any)["modifier"] = 21
		}, []string{"format 20 schema", "modifier"}},
		{"no languages", func(d map[string]any) { delete(d, "languages") }, []string{"format 20 schema", "languages"}},
		{"no race_languages", func(d map[string]any) { delete(tables(d), "race_languages") }, []string{"format 20 schema", "race_languages"}},
		{"a language code in capitals", func(d map[string]any) { language(d, 0)["code"] = "Common" }, []string{"format 20 schema", "code"}},
		{"a language name without English", func(d map[string]any) {
			language(d, 0)["name"] = map[string]any{"fr": "Commun"}
		}, []string{"format 20 schema", "en"}},
		{"a race outside the six", func(d map[string]any) {
			tables(d)["race_languages"].([]any)[0].(map[string]any)["race"] = "orc"
		}, []string{"format 20 schema", "race"}},
	})
}

// ADR 0021 steps 12.1, 13.1 and 14.1: the fixture's parts are handed on as
// written, and the sphere codes are read.
func TestParse_Fixture_HandsOnTheTablesEquipmentAndLanguages(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	var rt map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(a.RuleTables, &rt))
	for _, k := range []string{"strength", "thac0", "race_level_limits", "weapon_vs_armor", "race_languages"} {
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

// ADR 0044 step 1.4: the two new shapes of an initial cost are read, and any other object is
// refused by the schema.
func TestPowerCatalogue_Step25_ReadsTheTableAndPlusCosts(t *testing.T) {
	set := func(cost any) func(map[string]any) {
		return func(d map[string]any) { d["powers"].([]any)[0].(map[string]any)["initial_cost"] = cost }
	}
	for name, tc := range map[string]struct {
		cost any
		want catalogue.Cost
	}{
		"a table":      {map[string]any{"table": "contact_cost"}, catalogue.Cost{Kind: catalogue.CostVariable, Table: "contact_cost"}},
		"contact plus": {map[string]any{"kind": "contact", "plus": 30}, catalogue.Cost{Kind: catalogue.CostContact, Plus: 30}},
	} {
		a, err := ParseArtifact(artifact(t, set(tc.cost)))
		require.NoError(t, err, name)
		assert.Equal(t, tc.want, a.Powers[0].InitialCost, name)
	}
	refuses(t, []refusal{
		{"a table that is unknown", set(map[string]any{"table": "level_cost"}), []string{"format 20 schema", "powers/0/initial_cost"}},
		{"a contact plus below 1", set(map[string]any{"kind": "contact", "plus": 0}), []string{"format 20 schema", "powers/0/initial_cost"}},
		{"an empty object", set(map[string]any{}), []string{"format 20 schema", "powers/0/initial_cost"}},
	})
}

// ADR 0044 steps 1.1 to 1.3: the fixture's three combat tables are required, read as data, and
// refused when one is absent or a row is out of shape.
func TestRuleTables_Step25_ReadsTheCombatTables(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	var rt struct {
		AVD []struct {
			Attack, Defence string
			Modifier        int
		} `json:"attack_versus_defence"`
		Cost []struct {
			MinLevel int `json:"min_level"`
			MaxLevel int `json:"max_level"`
			Cost     int
		} `json:"contact_cost"`
		Mods []struct {
			Kind, Code string
			Modifier   int
		} `json:"contact_modifiers"`
	}
	require.NoError(t, json.Unmarshal(a.RuleTables, &rt))
	assert.Len(t, rt.AVD, 25, "five attack modes against five defence modes")
	assert.Len(t, rt.Cost, 4)
	assert.Equal(t, 1, rt.Cost[0].MinLevel)
	assert.Len(t, rt.Mods, 15, "six distances, eight life orders and the resistance")
	refuses(t, []refusal{
		{"no attack versus defence table", func(d map[string]any) { delete(tables(d), "attack_versus_defence") }, []string{"format 20 schema", "attack_versus_defence"}},
		{"no contact cost table", func(d map[string]any) { delete(tables(d), "contact_cost") }, []string{"format 20 schema", "contact_cost"}},
		{"no contact modifiers table", func(d map[string]any) { delete(tables(d), "contact_modifiers") }, []string{"format 20 schema", "contact_modifiers"}},
		{"a distance code of another kind", func(d map[string]any) { tables(d)["contact_modifiers"].([]any)[0].(map[string]any)["code"] = "mammal" }, []string{"format 20 schema", "contact_modifiers/0"}},
		{"a modifier above 0", func(d map[string]any) { tables(d)["contact_modifiers"].([]any)[0].(map[string]any)["modifier"] = 1 }, []string{"format 20 schema", "contact_modifiers/0/modifier"}},
	})
}

// ADR 0040 step 14 (ADR 0021 step 27): the skill sets are read with their names, minimum percentage and
// skills, a set or a skill twice and a skill without a base are refused, and format 18 is refused as an
// unknown format that names format 20 and `just catalogue`.
func TestParse_Step14_ReadsSkillSetsAndRefusesTheOldFormat(t *testing.T) {
	a, err := ParseArtifact(artifact(t, func(map[string]any) {}))
	require.NoError(t, err)
	require.Len(t, a.SkillSets, 2)
	thief, bard := a.SkillSets[0], a.SkillSets[1]
	assert.Equal(t, "thief", thief.ID)
	assert.Equal(t, localized.Text{"en": "Thief skills", "fr": "Talents de voleur"}, thief.Name)
	assert.Equal(t, 1, thief.MinPercent)
	require.Len(t, thief.Skills, 8)
	assert.Equal(t, catalogue.SkillSetSkill{ID: "open_locks", Base: 10, ArmourFree: true}, thief.Skills[1])
	assert.Equal(t, catalogue.SkillSetSkill{ID: "climb_walls", Base: 60}, thief.Skills[6])
	assert.Equal(t, "bard", bard.ID)
	assert.Equal(t, 0, bard.MinPercent)
	assert.Len(t, bard.Skills, 4)
	assert.Contains(t, string(a.Definitions), `"skill_sets"`, "ruletablesadpt reads the sets from the definitions")

	set := func(d map[string]any, i int) map[string]any { return d["skill_sets"].([]any)[i].(map[string]any) }
	skill := func(d map[string]any, i, j int) map[string]any {
		return set(d, i)["skills"].([]any)[j].(map[string]any)
	}
	refuses(t, []refusal{
		{"no skill sets", func(d map[string]any) { delete(d, "skill_sets") }, []string{"format 20 schema", "skill_sets"}},
		{"a set without a name", func(d map[string]any) { delete(set(d, 0), "name") }, []string{"format 20 schema", "skill_sets/0", "name"}},
		{"a skill without a base", func(d map[string]any) { delete(skill(d, 0, 0), "base") }, []string{"format 20 schema", "skill_sets/0/skills/0", "base"}},
		{"an unknown skill", func(d map[string]any) { skill(d, 0, 0)["id"] = "juggling" }, []string{"format 20 schema", "skill_sets/0/skills/0/id"}},
		{"a set twice", func(d map[string]any) { set(d, 1)["id"] = "thief" }, []string{"set \"thief\" twice"}},
		{"a skill twice", func(d map[string]any) { skill(d, 0, 1)["id"] = "pick_pockets" }, []string{"set \"thief\" has skill \"pick_pockets\" twice"}},
		{"the old per-set base table", func(d map[string]any) { tables(d)["thief_skill_base"] = []any{} }, []string{"format 20 schema", "thief_skill_base"}},
		{"Table 28 as one column per skill", func(d map[string]any) {
			tables(d)["skill_dexterity"].([]any)[0].(map[string]any)["pick_pockets"] = -15
		}, []string{"format 20 schema", "skill_dexterity/0"}},
		{"specialization params missing", func(d map[string]any) {
			delete(feature(d, "fighter", "weapon_specialization"), "params")
		}, []string{"format 20 schema", "params"}},
		{"a specialization type with neither a column nor the rate of fire", func(d map[string]any) {
			p := feature(d, "fighter", "weapon_specialization")["params"].(map[string]any)
			delete(p["types"].([]any)[1].(map[string]any), "column")
		}, []string{"format 20 schema"}},
	})
	_, err = Parse(artifact(t, func(d map[string]any) { d["format"] = 18 }))
	require.ErrorContains(t, err, "unknown format 18: this API reads format 20; run `just catalogue` to fetch a current artifact")
}

// feature is the named class's feature of that kind in a document.
func feature(d map[string]any, class, kind string) map[string]any {
	for _, c := range d["classes"].([]any) {
		if m := c.(map[string]any); m["id"] == class {
			for _, f := range m["features"].([]any) {
				if fm := f.(map[string]any); fm["kind"] == kind {
					return fm
				}
			}
		}
	}
	panic("no feature " + kind + " on " + class)
}
