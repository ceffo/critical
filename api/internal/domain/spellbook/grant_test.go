package spellbook_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// ADR 0030 step 20.8: "When a mage's realized level opens a new spell level
// (the first realized level at which step 8's slots for that spell level are
// above zero), the level grants one free spell of that new spell level ... No
// grant when the Intelligence's maximum spell level is below the new spell
// level." The slot rows are PHB Table 21's (p.30) at the levels named.
func TestFreeSpellsAt_Step20_8_OneFreeSpellPerSpellLevelOpened(t *testing.T) {
	for _, tc := range []struct {
		name   string
		level  int
		below  map[int]int
		at     map[int]int
		intMax int
		want   []spellbook.FreeSpell
	}{
		{"level 1 opens the 1st", 1, nil, map[int]int{1: 1}, 9, []spellbook.FreeSpell{{Level: 1, SpellLevel: 1}}},
		{"level 2 opens none", 2, map[int]int{1: 1}, map[int]int{1: 2}, 9, nil},
		{"level 3 opens the 2nd", 3, map[int]int{1: 2}, map[int]int{1: 2, 2: 1}, 9, []spellbook.FreeSpell{{Level: 3, SpellLevel: 2}}},
		{"level 12 opens the 6th", 12, map[int]int{1: 4, 2: 4, 3: 4, 4: 3, 5: 3}, map[int]int{1: 4, 2: 4, 3: 4, 4: 4, 5: 4, 6: 1}, 9, []spellbook.FreeSpell{{Level: 12, SpellLevel: 6}}},
		{"a zero count opens nothing", 4, map[int]int{1: 2}, map[int]int{1: 3, 2: 0}, 9, nil},
		{"the 5th at Intelligence 9 (Table 4: 4th) grants none", 9, map[int]int{1: 4, 4: 2}, map[int]int{1: 4, 4: 2, 5: 1}, 4, nil},
		{"the 4th at Intelligence 9 grants it", 7, map[int]int{1: 4, 3: 2}, map[int]int{1: 4, 3: 2, 4: 1}, 4, []spellbook.FreeSpell{{Level: 7, SpellLevel: 4}}},
		{"two spell levels opened, lowest first", 6, map[int]int{1: 1}, map[int]int{1: 1, 3: 1, 2: 1}, 9, []spellbook.FreeSpell{{Level: 6, SpellLevel: 2}, {Level: 6, SpellLevel: 3}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, spellbook.FreeSpellsAt(tc.level, tc.below, tc.at, tc.intMax))
		})
	}
}

// ADR 0030 step 20.8: "Free spells owed = the new spell levels opened by the
// levels taken in the app ... minus the level_grant entries for those
// levels." A claim pays only the grant of its level and spell level, once.
func TestOwed_Step20_8_OwedIsOpenedLevelsMinusGrants(t *testing.T) {
	three := spellbook.FreeSpell{Level: 3, SpellLevel: 2}
	five := spellbook.FreeSpell{Level: 5, SpellLevel: 3}
	for _, tc := range []struct {
		name             string
		granted, claimed []spellbook.FreeSpell
		want             []spellbook.FreeSpell
	}{
		{"nothing granted", nil, nil, nil},
		{"none claimed", []spellbook.FreeSpell{three, five}, nil, []spellbook.FreeSpell{three, five}},
		{"one claimed", []spellbook.FreeSpell{three, five}, []spellbook.FreeSpell{three}, []spellbook.FreeSpell{five}},
		{"all claimed", []spellbook.FreeSpell{three, five}, []spellbook.FreeSpell{five, three}, nil},
		{"a claim of another spell level pays nothing", []spellbook.FreeSpell{three}, []spellbook.FreeSpell{{Level: 3, SpellLevel: 1}}, []spellbook.FreeSpell{three}},
		{"a claim of a level not granted pays nothing", []spellbook.FreeSpell{five}, []spellbook.FreeSpell{three}, []spellbook.FreeSpell{five}},
		{"one claim pays one grant", []spellbook.FreeSpell{three, three}, []spellbook.FreeSpell{three}, []spellbook.FreeSpell{three}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, spellbook.Owed(tc.granted, tc.claimed))
		})
	}
}
