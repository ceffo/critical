package ruletablesvc

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

// armorPaddedStudded is the skill armour of Table 29's worst column.
const armorPaddedStudded = "padded_studded_leather"

// SkillArmorFor is ADR 0033 step 14: the skill armour (one of ThiefArmors,
// or BardArmors for a bard) the class's skills read when it wears a body
// armour of Table 29 column column ("" when it wears none: the none column,
// PHB p.39, bracers and cloaks included). An armour whose column is
// none_allowed, metal for a thief, or a column this list does not know reads
// the padded or studded column, the worst one (not chosen: PHB chapter 3's
// multi-class reading, every skill but two unusable). A shield is no body
// armour: it changes no skill.
func SkillArmorFor(class, column string) string {
	switch column {
	case "", ColumnNone:
		return ColumnNone
	case ColumnLeather:
		return ArmorLeather
	case ColumnPaddedStudded:
		return armorPaddedStudded
	case ColumnElvenChain:
		return armorElvenChain
	case ColumnMetal:
		if HasBardSkills(class) {
			return ArmorMetal
		}
	}
	return armorPaddedStudded
}
