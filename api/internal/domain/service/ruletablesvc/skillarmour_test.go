package ruletablesvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ADR 0033 step 14: a thief's or a bard's skills read the worn body armour's Table 29 column:
// none worn is the none column (bracers and cloaks included); metal is the bard's own column and a
// thief's worst one; an armour neither may wear (and a column this code does not know) reads the
// padded or studded column, the worst one.
func TestSkillArmorFor_Step14_TheWornArmoursColumn(t *testing.T) {
	for _, tc := range []struct{ class, column, want string }{
		{"thief", "", "none"},
		{"thief", "none", "none"},
		{"thief", "leather", "leather"},
		{"thief", "padded_studded", "padded_studded_leather"},
		{"thief", "elven_chain", "elven_chain"},
		{"thief", "metal", "padded_studded_leather"},
		{"thief", "none_allowed", "padded_studded_leather"},
		{"thief", "mithral", "padded_studded_leather"},
		{"bard", "", "none"},
		{"bard", "leather", "leather"},
		{"bard", "metal", "metal"},
		{"bard", "none_allowed", "padded_studded_leather"},
	} {
		got := ruletablesvc.SkillArmorFor(tc.class, tc.column)
		assert.Equal(t, tc.want, got, "%s in %q", tc.class, tc.column)
		armours := ruletablesvc.ThiefArmors
		if tc.class == "bard" {
			armours = ruletablesvc.BardArmors
		}
		assert.Contains(t, armours, got, "an armour the class's skills read")
	}
}
