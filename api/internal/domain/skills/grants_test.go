package skills_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
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
		want  skills.LevelGrants
	}{
		{"thief", 4, skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1, SkillPoints: 30}},
		{"thief", 5, skills.LevelGrants{SkillPoints: 30}},
		{"bard", 4, skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1, SkillPoints: 15}},
		{"bard", 2, skills.LevelGrants{SkillPoints: 15}},
		{"fighter", 3, skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1}},
		{"fighter", 4, skills.LevelGrants{}},
		{"mage", 3, skills.LevelGrants{NonweaponSlots: 1}},
		{"mage", 6, skills.LevelGrants{WeaponSlots: 1, NonweaponSlots: 1}},
		{"cleric", 4, skills.LevelGrants{WeaponSlots: 1}},
		{"cleric", 3, skills.LevelGrants{NonweaponSlots: 1}},
		// Level 1's slots and points are the initial ones: nothing gained.
		{"thief", 1, skills.LevelGrants{}},
		{"fighter", 0, skills.LevelGrants{}},
	} {
		assert.Equal(t, tc.want, skills.GrantsAt(book, tc.class, tc.level), "%s level %d", tc.class, tc.level)
	}
	assert.Equal(t, skills.LevelGrants{SkillPoints: 30}, skills.GrantsAt((*ruletablesvc.Proficiencies)(nil), "thief", 4),
		"without the catalogue's Table 34 only the points")
}

// ADR 0037 step 15's reminder: what is left in each pool and of the skill points, 0 when spent
// or over budget (a negative left is the over-budget warning's, never "to spend").
func TestToSpend_Step15_LeftCountsNeverNegative(t *testing.T) {
	f := &skills.ProficiencySlots{
		Nonweapon: ruletablesvc.NonweaponPool{Left: 2}, Weapon: ruletablesvc.WeaponPool{Left: -1},
	}
	b := &ruletablesvc.SkillPointsBudget{Left: 15}
	assert.Equal(t, skills.LevelGrants{NonweaponSlots: 2, SkillPoints: 15}, skills.ToSpend(f, b))
	f.Nonweapon.Left, b.Left = 0, -5
	assert.Equal(t, skills.LevelGrants{}, skills.ToSpend(f, b))
	assert.Equal(t, skills.LevelGrants{}, skills.ToSpend(nil, nil), "no pool, no points")
}
