package ruletablesvc

import "github.com/ceffo/critical/api/internal/domain/classes"

// AbilityMinimum is a class's lowest score in one ability (PHB Table 13).
type AbilityMinimum = classes.AbilityMinimum

// ClassMinimums are the ability minimums of class (PHB Table 13, its
// definition's, ADR 0040 step 1.4), in Abilities order; empty for a class
// without any or the catalogue does not define. The scores compared are the
// sheet's (the racial adjustments of Table 8 already in, ADR 0030 step 1).
func (t *Tables) ClassMinimums(class string) []AbilityMinimum {
	c, err := t.Class(class)
	if err != nil {
		return []AbilityMinimum{}
	}
	return c.AbilityMinimums
}

// UnmetMinimums are the abilities of class whose score in scores (by
// Abilities code; an ability absent has no score yet) is under its minimum,
// in Abilities order. An ability with no score is not unmet.
func (t *Tables) UnmetMinimums(class string, scores map[string]int) []string {
	c, err := t.Class(class)
	if err != nil {
		return []string{}
	}
	return c.UnmetMinimums(scores)
}
