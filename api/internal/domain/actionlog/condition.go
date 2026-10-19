package actionlog

// Condition is the character's state given its current HP, ADR 0011 step 2.
// The values are codes for the client, which words them (i18n).
type Condition string

// Conditions, from best to worst.
const (
	ConditionOK           Condition = "ok"
	ConditionBadlyWounded Condition = "badlyWounded"
	ConditionUnconscious  Condition = "unconscious"
	ConditionDead         Condition = "dead"
)

// ConditionOf returns the condition for current HP out of maxHP (ADR 0011
// step 2): dead at or below HPFloor, unconscious at or below 0,
// badly wounded at or below a quarter of max (compared as 4*current <= max, so
// no rounding), ok above.
func ConditionOf(current, maxHP int) Condition {
	switch {
	case current <= HPFloor:
		return ConditionDead
	case current <= 0:
		return ConditionUnconscious
	case 4*current <= maxHP:
		return ConditionBadlyWounded
	default:
		return ConditionOK
	}
}
