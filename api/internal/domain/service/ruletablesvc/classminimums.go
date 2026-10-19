package ruletablesvc

// AbilityMinimum is a class's lowest score in one ability (PHB Table 13).
type AbilityMinimum struct {
	Ability string
	Min     int
}

// classMinimums is PHB Table 13 for the classes built so far, in Abilities
// order. The scores compared are the sheet's (the racial adjustments of Table
// 8 already in, ADR 0030 step 1). A class absent has none listed.
var classMinimums = map[string][]AbilityMinimum{
	"fighter": {{"str", 9}},
	"mage":    {{"int", 9}},
	"cleric":  {{"wis", 9}},
	"thief":   {{"dex", 9}},
	"bard":    {{"dex", 12}, {"int", 13}, {"cha", 15}},
}

// ClassMinimums are the ability minimums of class (PHB Table 13), in
// Abilities order; empty for a class without any.
func ClassMinimums(class string) []AbilityMinimum {
	return append([]AbilityMinimum{}, classMinimums[class]...)
}

// UnmetMinimums are the abilities of class whose score in scores (by
// Abilities code; an ability absent has no score yet) is under its minimum,
// in Abilities order. An ability with no score is not unmet.
func UnmetMinimums(class string, scores map[string]int) []string {
	out := []string{}
	for _, m := range classMinimums[class] {
		if s, ok := scores[m.Ability]; ok && s < m.Min {
			out = append(out, m.Ability)
		}
	}
	return out
}
