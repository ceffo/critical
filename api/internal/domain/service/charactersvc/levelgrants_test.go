package charactersvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0037 step 15: "the weapon and non-weapon slots gained (available at the level minus
// available at the level below) and, for a rogue, the skill points gained (thief 30, bard 15)",
// over Table 34 as the seed holds it (profBook): warriors gain a weapon slot every 3 levels and a
// non-weapon slot every 3, wizards every 6 and 3, priests every 4 and 3, rogues every 4 and 4.
func TestGrantsAt_Step15_GrantsSlotsAndPoints(t *testing.T) {
	book := profBook(t)
	for _, tc := range []struct {
		class string
		level int
		want  charactersvc.LevelGrants
	}{
		{"thief", 4, charactersvc.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1, SkillPoints: 30}},
		{"thief", 5, charactersvc.LevelGrants{SkillPoints: 30}},
		{"bard", 4, charactersvc.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1, SkillPoints: 15}},
		{"bard", 2, charactersvc.LevelGrants{SkillPoints: 15}},
		{"fighter", 3, charactersvc.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1}},
		{"fighter", 4, charactersvc.LevelGrants{}},
		{"mage", 3, charactersvc.LevelGrants{NonweaponSlots: 1}},
		{"mage", 6, charactersvc.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1}},
		{"cleric", 4, charactersvc.LevelGrants{WeaponSlots: 1}},
		{"cleric", 3, charactersvc.LevelGrants{NonweaponSlots: 1}},
		// Level 1's slots and points are the initial ones: nothing gained.
		{"thief", 1, charactersvc.LevelGrants{}},
		{"fighter", 0, charactersvc.LevelGrants{}},
	} {
		assert.Equal(t, tc.want, charactersvc.GrantsAt(book, tc.class, tc.level), "%s level %d", tc.class, tc.level)
	}
	assert.Equal(t, charactersvc.LevelGrants{SkillPoints: 30}, charactersvc.GrantsAt((*ruletablesvc.Proficiencies)(nil), "thief", 4),
		"without the catalogue's Table 34 only the points")
}

// ADR 0037 step 15's reminder: what is left in each pool and of the skill points, 0 when spent
// or over budget (a negative left is the over-budget warning's, never "to spend").
func TestToSpend_Step15_LeftCountsNeverNegative(t *testing.T) {
	d := charactersvc.Derived{
		Proficiencies: &charactersvc.ProficiencySlots{
			Nonweapon: ruletablesvc.NonweaponPool{Left: 2}, Weapon: ruletablesvc.WeaponPool{Left: -1},
		},
		SkillPoints: &ruletablesvc.SkillPointsBudget{Left: 15},
	}
	assert.Equal(t, charactersvc.LevelGrants{NonweaponSlots: 2, SkillPoints: 15}, charactersvc.ToSpend(d))
	d.Proficiencies.Nonweapon.Left, d.SkillPoints.Left = 0, -5
	assert.Equal(t, charactersvc.LevelGrants{}, charactersvc.ToSpend(d))
	assert.Equal(t, charactersvc.LevelGrants{}, charactersvc.ToSpend(charactersvc.Derived{}), "no pool, no points")
}
