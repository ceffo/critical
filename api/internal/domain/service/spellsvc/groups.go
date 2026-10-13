package spellsvc

import (
	"slices"
	"strings"
)

// School group keys: the eight PHB schools, plus SchoolOther for everything
// else (Tome of Magic's Dimension, Geometry, Alchemy, Elemental..., Force,
// Mentalism, Shadow, Song, Artifice, "Various schools").
const (
	SchoolAbjuration  = "abjuration"
	SchoolAlteration  = "alteration"
	SchoolConjuration = "conjuration" // Conjuration/Summoning
	SchoolDivination  = "divination"
	SchoolEnchantment = "enchantment" // Enchantment/Charm
	SchoolIllusion    = "illusion"    // Illusion/Phantasm
	SchoolEvocation   = "evocation"   // Invocation/Evocation
	SchoolNecromancy  = "necromancy"
	SchoolOther       = "other"
)

// SchoolGroups lists the group keys in the order the lookup offers them.
func SchoolGroups() []string {
	return []string{
		SchoolAbjuration, SchoolAlteration, SchoolConjuration, SchoolDivination,
		SchoolEnchantment, SchoolIllusion, SchoolEvocation, SchoolNecromancy, SchoolOther,
	}
}

// schoolWords maps each word of a school name to its PHB group: the scrape
// writes a school whole ("Invocation/Evocation") or by one half
// ("Evocation", "Summoning", "Phantasm").
var schoolWords = map[string]string{
	"abjuration":  SchoolAbjuration,
	"alteration":  SchoolAlteration,
	"conjuration": SchoolConjuration,
	"summoning":   SchoolConjuration,
	"divination":  SchoolDivination,
	"enchantment": SchoolEnchantment,
	"charm":       SchoolEnchantment,
	"illusion":    SchoolIllusion,
	"phantasm":    SchoolIllusion,
	"invocation":  SchoolEvocation,
	"evocation":   SchoolEvocation,
	"necromancy":  SchoolNecromancy,
	"necromantic": SchoolNecromancy,
}

// SpellSchoolGroups returns the groups of a school text such as
// "Alteration, Elemental Water, Alchemy" (alteration, other). Each
// comma-separated school maps to the PHB groups its words name, or to
// SchoolOther when it names none. An empty text has no group.
func SpellSchoolGroups(school string) []string {
	var out []string
	add := func(g string) {
		if !slices.Contains(out, g) {
			out = append(out, g)
		}
	}
	for _, part := range splitList(school) {
		found := false
		for _, w := range strings.FieldsFunc(strings.ToLower(part), func(r rune) bool { return r == '/' || r == ' ' }) {
			if g, ok := schoolWords[w]; ok {
				add(g)
				found = true
			}
		}
		if !found {
			add(SchoolOther)
		}
	}
	return out
}

// Spheres lists the Priest spheres the lookup offers, alphabetically.
func Spheres() []string {
	return slices.Clone(spheres)
}

var spheres = []string{
	"All", "Animal", "Astral", "Chaos", "Charm", "Combat", "Creation", "Divination",
	"Elemental Air", "Elemental Earth", "Elemental Fire", "Elemental Water",
	"Guardian", "Healing", "Law", "Necromantic", "Numbers", "Plant", "Protection",
	"Summoning", "Sun", "Thought", "Time", "Travelers", "War", "Wards", "Weather",
}

// sphereAliases are the scrape's odd sphere entries: a bare "Elemental" is
// every elemental sphere, a bare "Water" (in "Elemental Air, Water") is
// Elemental Water.
var sphereAliases = map[string][]string{
	"elemental": {"Elemental Air", "Elemental Earth", "Elemental Fire", "Elemental Water"},
	"water":     {"Elemental Water"},
}

// SpellSpheres returns the spheres (names of Spheres) of a sphere text such as
// "Divination (Animal, Plant)". An entry the scrape glued together
// ("Wards Elemental Air", "Elemental Plant") yields every sphere it names.
func SpellSpheres(sphere string) []string {
	var out []string
	add := func(s string) {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	for _, part := range splitList(sphere) {
		lp := strings.ToLower(part)
		if alias, ok := sphereAliases[lp]; ok {
			for _, s := range alias {
				add(s)
			}
			continue
		}
		words := strings.Fields(lp)
		for _, s := range spheres {
			if containsWords(words, strings.Fields(strings.ToLower(s))) {
				add(s)
			}
		}
	}
	slices.Sort(out)
	return out
}

// IsSphere reports whether name is one of Spheres (any case); it returns the
// canonical spelling.
func IsSphere(name string) (string, bool) {
	for _, s := range spheres {
		if strings.EqualFold(s, name) {
			return s, true
		}
	}
	return "", false
}

// splitList splits "A, B (C, D)" into A, B, C, D.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '(' || r == ')' || r == ';' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// containsWords reports whether sub appears in words as consecutive words.
func containsWords(words, sub []string) bool {
	for i := 0; i+len(sub) <= len(words); i++ {
		if slices.Equal(words[i:i+len(sub)], sub) {
			return true
		}
	}
	return false
}
