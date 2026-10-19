package seedcheck_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// fixtureCatalogue carries the books' rule-table numbers (ADR 0021 steps 9.7
// and 10.5), so the comparison runs without the private artifact too.
var fixtureCatalogue = filepath.Join("..", "..", "testdata", "catalogue.json")

// ADR 0030 step 24 for the campaign's character: the level, THAC0 and five
// save targets the sheet in seeds/character.json states, and the spell slots
// the app gave him from the sheet's level (PHB Table 32), are the values the
// rules derive from his class, race and XP; and the bard skill percentages
// of his sheet (seeds/skills.json) are the ones step 18 derives from his
// seeded points (the sheet's percentage minus the computed base, race,
// Dexterity and armour terms, no armour stored being leather), so nothing on
// his sheet changes. A difference is a data or rules
// error for the user's ruling (ADR 0030 Consequences), listed by this test
// with the sheet's and the computed figures. Checked on the fixture's tables
// (the books' numbers) and, when present, on the real artifact's.
func TestKelsen_TheSheetMatchesTheDerivedValues(t *testing.T) {
	seed, ok, err := seedfile.ReadCharacter(filepath.Join(seedsDir, seedfile.CharacterFile))
	require.NoError(t, err)
	require.True(t, ok)
	c := &charactersvc.Character{
		Name: seed.Name, Str: seed.Str, Dex: seed.Dex, Con: seed.Con, Int: seed.Int, Wis: seed.Wis, Cha: seed.Cha,
		HPMax: seed.HPMax, XPTotal: seed.XPTotal, Class: seedfile.DefaultClass, Race: seed.Race,
		SkillPoints: seed.SkillPoints,
		Sheet: charactersvc.Sheet{Level: seed.Level, THAC0: seed.Thac0Base, Saves: charactersvc.Saves{
			ParaPoison: seed.SaveParaPoison, RodStaffWand: seed.SaveRodStaffWand, PetriPoly: seed.SavePetriPoly,
			Breath: seed.SaveBreath, Spell: seed.SaveSpell,
		}},
	}
	skills, err := seedfile.ReadSkills(filepath.Join(seedsDir, seedfile.SkillsFile))
	require.NoError(t, err)
	sheetSkills := map[string]int{}
	for _, b := range skills.BardSkills {
		sheetSkills[b.ID] = b.Percent
	}
	require.Len(t, sheetSkills, len(ruletablesvc.BardSkills), "the sheet states the four bard skills")
	catalogues := map[string]string{"fixture tables": fixtureCatalogue}
	if _, err := os.Stat(catalogueFile); err == nil {
		catalogues["real artifact"] = catalogueFile
	}
	for name, path := range catalogues {
		t.Run(name, func(t *testing.T) {
			tables := parseTables(t, path)
			d, err := charactersvc.Derive(c, 0, tables)
			require.NoError(t, err)
			assert.Empty(t, d.SheetDifferences, "the sheet's figures against the rules")
			sheetSlots, err := tables.BardSpellsPerDay(seed.Level)
			require.NoError(t, err)
			assert.Equal(t, sheetSlots, d.SpellSlots, "the slots of the sheet's level against the derived ones")
			assert.Empty(t, d.RaceSaveBonus, "a half-elf has no racial save bonus")
			derived := map[string]int{}
			for _, b := range d.BardSkills {
				derived[b.Skill] = b.Percent
				assert.Equal(t, sheetSkills[b.Skill]-b.Base-b.Race-b.Dexterity-b.Armor, b.Points,
					"%s: the points are the sheet's percentage minus the computed terms", b.Skill)
			}
			assert.Equal(t, sheetSkills, derived, "the sheet's bard skills against the derived ones (ADR 0030 step 18)")
			assert.Empty(t, d.SkillBudget, "his points are within the bard's budget at his level")
		})
	}
}

func parseTables(t *testing.T, path string) *ruletablesvc.Tables {
	t.Helper()
	artifact, err := catalogueadpt.LoadArtifact(path)
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(artifact.Catalogue.Format, artifact.RuleTables, artifact.Equipment, artifact.Languages)
	require.NoError(t, err)
	_, ok := tables.Classes()
	require.True(t, ok, "%s carries no class tables: run `just catalogue`", path)
	return tables
}
