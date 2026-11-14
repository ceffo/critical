package spellbook_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// ADR 0050 step 10: one school slot "for each spell level whose allotment ...
// is 1 or more" (PHB p.32, 06-mage.md l.56: "one additional spell per spell
// level"); none for one who is no specialist. The allotments are PHB Table 21's
// (p.30) rows at levels 1, 3 and 9.
func TestSchoolSlots_Step10_OnePerSpellLevelWithAnAllotment(t *testing.T) {
	transmuter := spellbook.Specialization{Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"}}
	for _, tc := range []struct {
		name      string
		spec      spellbook.Specialization
		allotment map[int]int
		want      map[int]int
	}{
		{"level 1", transmuter, map[int]int{1: 1}, map[int]int{1: 1}},
		{"level 3", transmuter, map[int]int{1: 2, 2: 1}, map[int]int{1: 1, 2: 1}},
		{"level 9", transmuter, map[int]int{1: 4, 2: 3, 3: 3, 4: 2, 5: 1}, map[int]int{1: 1, 2: 1, 3: 1, 4: 1, 5: 1}},
		{"a zero allotment opens none", transmuter, map[int]int{1: 2, 2: 0}, map[int]int{1: 1}},
		{"no slots", transmuter, nil, nil},
		{"a mage has none", spellbook.Specialization{}, map[int]int{1: 2, 2: 1}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, spellbook.SchoolSlots(tc.spec, tc.allotment))
		})
	}
}
