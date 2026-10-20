package ruletablesvc

import "slices"

// freeSpellClasses are the classes whose levels grant a free spell of each
// spell level they open (ADR 0030 step 20.8, DMG "Going Up in Levels"): the
// mage. A bard gains no spell by levelling (PHB, the bard).
var freeSpellClasses = []string{"mage"}

// GrantsFreeSpells reports whether class's levels grant free spells (ADR
// 0030 step 20.8).
func GrantsFreeSpells(class string) bool { return slices.Contains(freeSpellClasses, class) }
