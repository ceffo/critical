package ruletablesvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The armour a thief's or a bard's skills read (ADR 0033 step 14, amending
// ADR 0030 steps 1, 17 and 18): the worn body armour's PHB Table 29 column,
// never an entered value.

// The Table 29 columns a body armour carries (ADR 0033 step 5): none,
// leather (the baseline), padded or studded leather, elven chain, metal (a
// bard's non-elven chain up to chain mail) and none allowed (an armour a
// thief or bard may not wear). itemsvc.SkillColumns is this list.
const (
	ColumnNone          = "none"
	ColumnLeather       = "leather"
	ColumnPaddedStudded = "padded_studded"
	ColumnElvenChain    = "elven_chain"
	ColumnMetal         = "metal"
	ColumnNoneAllowed   = "none_allowed"
)

// SkillColumns are the Table 29 columns of a body armour, in display order.
var SkillColumns = []string{ColumnNone, ColumnLeather, ColumnPaddedStudded, ColumnElvenChain, ColumnMetal, ColumnNoneAllowed}

// SkillArmorFor is ADR 0033 step 14 read through the rogue_skills params rs
// (ADR 0040 step 3): the skill armour the skills read when the character wears
// a body armour of Table 29 column column ("" when it wears none: the none
// column, PHB p.39, bracers and cloaks included). The params' entry for the
// column names the Table 29 row it reads (none_allowed, the worst column,
// for an armour the class may not wear: PHB chapter 3's multi-class
// reading, and for a column outside SkillColumns, which an item cannot
// store). Every column has an entry: CheckDefinitions refuses the params at
// start otherwise, so no entry is ever missing. A shield is no body armour: it changes no skill. ""
// without rogue_skills.
func SkillArmorFor(rs *classes.RogueSkills, column string) string {
	if rs == nil {
		return ""
	}
	if column == "" {
		column = ColumnNone
	}
	if !slices.Contains(SkillColumns, column) {
		column = ColumnNoneAllowed // no Table 29 column: the worst one
	}
	a, _ := rs.SkillArmourFor(column)
	return skillArmorOf(a)
}

// skillArmorOf is the skill armour an armour entry reads, the value the
// derived block shows and the skills take: its Table 29 row (leather, the
// baseline, for none), or its own code when it adds an extra adjustment
// (the bard's metal: the elven chain row plus its penalty is not the elven
// chain).
func skillArmorOf(a classes.SkillArmour) string {
	switch {
	case a.Extra != 0:
		return a.Code
	case a.Column == "":
		return ArmorLeather
	}
	return a.Column
}
