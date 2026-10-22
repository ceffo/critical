// Package classes holds the class, group and race definitions of the
// catalogue (format 11, ADR 0040 steps 1, 2 and 5) and the lookups that
// read them: a class's group, prime requisites (the XP bonus reads them, xpsvc), ability minimums, allowed
// alignments and experience thresholds. It is pure: the catalogue adapter
// builds a Registry with New and the composition root hands it to the
// readers. No class, group or race id is written here (ADR 0040 step 10):
// every id comes from the data.
package classes

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

// MaxLevel is the last level a class's experience table covers (ADR 0030
// step 3).
const MaxLevel = 20

// Abilities are the six ability score codes, in the sheet's order.
var Abilities = []string{"str", "dex", "con", "int", "wis", "cha"}

// Alignments are the nine alignment codes of the PHB (chapter 4), in the
// order the book lists them (ADR 0032 step 2).
var Alignments = []string{
	"lawful_good", "neutral_good", "chaotic_good",
	"lawful_neutral", "true_neutral", "chaotic_neutral",
	"lawful_evil", "neutral_evil", "chaotic_evil",
}

// AbilityMinimum is a class's lowest score in one ability (PHB Table 13).
type AbilityMinimum struct {
	Ability string
	Min     int
}

// Class is one class definition (ADR 0040 step 1). The feature list, the
// equipment and the hit dice are read by their own readers as they move
// onto the data (step 11.3); this holds the fields read so far.
type Class struct {
	ID   string
	Name localized.Text
	// Group is the class group code (step 1.2).
	Group string
	// PrimeRequisites are ability codes (step 1.3, step 6).
	PrimeRequisites []string
	// AbilityMinimums are PHB Table 13's, in Abilities order (step 1.4).
	AbilityMinimums []AbilityMinimum
	// Alignments are the alignment codes the class allows, in Alignments
	// order; every alignment when the definition lists none (step 1.5).
	Alignments []string
	// Experience[l-1] is the XP that reaches level l, 1..MaxLevel (step 1.7).
	Experience []int
}

// Race is one race definition (ADR 0040 step 2); its traits and tables
// are read by their own readers (step 11.4).
type Race struct {
	ID   string
	Name localized.Text
}

// Registry is the validated set of definitions, in catalogue order. Built
// by New only.
type Registry struct {
	classes []Class
	groups  []string
	races   []Race
}

// New validates the definitions (ADR 0040 step 8, the rules the schema
// cannot express) and builds the registry: ids unique, every class of a
// known group with known ability codes, minimums and prime requisites, known
// alignments, and its experience rows levels 1..MaxLevel from 0 and strictly
// increasing.
func New(classes []Class, groups []string, races []Race) (*Registry, error) {
	if len(classes) == 0 || len(groups) == 0 || len(races) == 0 {
		return nil, errors.New("classes: the catalogue needs classes, groups and races")
	}
	if err := unique(groups, func(g string) string { return g }); err != nil {
		return nil, fmt.Errorf("groups: %w", err)
	}
	if err := unique(races, func(r Race) string { return r.ID }); err != nil {
		return nil, fmt.Errorf("races: %w", err)
	}
	if err := unique(classes, func(c Class) string { return c.ID }); err != nil {
		return nil, fmt.Errorf("classes: %w", err)
	}
	out := &Registry{groups: slices.Clone(groups), races: slices.Clone(races), classes: make([]Class, 0, len(classes))}
	for _, c := range classes {
		v, err := checkClass(c, groups)
		if err != nil {
			return nil, fmt.Errorf("class %q: %w", c.ID, err)
		}
		out.classes = append(out.classes, v)
	}
	return out, nil
}

func unique[T any](items []T, id func(T) string) error {
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		k := id(it)
		if k == "" {
			return errors.New("an empty id")
		}
		if seen[k] {
			return fmt.Errorf("%q twice", k)
		}
		seen[k] = true
	}
	return nil
}

// checkClass validates c and returns it with its lists copied, the
// minimums in Abilities order and the alignments filled and in Alignments
// order.
func checkClass(c Class, groups []string) (Class, error) {
	if !slices.Contains(groups, c.Group) {
		return Class{}, fmt.Errorf("unknown group %q", c.Group)
	}
	if len(c.PrimeRequisites) == 0 {
		return Class{}, errors.New("no prime requisite")
	}
	if err := unique(c.PrimeRequisites, func(a string) string { return a }); err != nil {
		return Class{}, fmt.Errorf("prime requisites: %w", err)
	}
	for _, a := range c.PrimeRequisites {
		if !slices.Contains(Abilities, a) {
			return Class{}, fmt.Errorf("prime requisite: unknown ability %q", a)
		}
	}
	if err := unique(c.AbilityMinimums, func(m AbilityMinimum) string { return m.Ability }); err != nil {
		return Class{}, fmt.Errorf("ability minimums: %w", err)
	}
	mins := make([]AbilityMinimum, 0, len(c.AbilityMinimums))
	for _, a := range Abilities {
		for _, m := range c.AbilityMinimums {
			if m.Ability == a {
				mins = append(mins, m)
			}
		}
	}
	if len(mins) != len(c.AbilityMinimums) {
		return Class{}, errors.New("ability minimums: an unknown ability")
	}
	aligns := slices.Clone(Alignments)
	if len(c.Alignments) > 0 {
		for _, a := range c.Alignments {
			if !slices.Contains(Alignments, a) {
				return Class{}, fmt.Errorf("unknown alignment %q", a)
			}
		}
		aligns = slices.DeleteFunc(aligns, func(a string) bool { return !slices.Contains(c.Alignments, a) })
	}
	if len(c.Experience) != MaxLevel {
		return Class{}, fmt.Errorf("experience: needs levels 1..%d, got %d rows", MaxLevel, len(c.Experience))
	}
	if c.Experience[0] != 0 {
		return Class{}, errors.New("experience: level 1 must start at 0 XP")
	}
	for i := 1; i < len(c.Experience); i++ {
		if c.Experience[i] <= c.Experience[i-1] {
			return Class{}, fmt.Errorf("experience: level %d: %d XP is not above level %d", i+1, c.Experience[i], i)
		}
	}
	return Class{
		ID: c.ID, Name: c.Name.Clean(), Group: c.Group,
		PrimeRequisites: slices.Clone(c.PrimeRequisites), AbilityMinimums: mins,
		Alignments: aligns, Experience: slices.Clone(c.Experience),
	}, nil
}

// ErrUnknownClass is a lookup of a class the catalogue does not define.
var ErrUnknownClass = errors.New("unknown class")

// Classes are the class definitions, in catalogue order (copies).
func (r *Registry) Classes() []Class {
	out := make([]Class, len(r.classes))
	for i, c := range r.classes {
		out[i] = c.clone()
	}
	return out
}

// ClassIDs are the class ids, in catalogue order.
func (r *Registry) ClassIDs() []string {
	out := make([]string, len(r.classes))
	for i, c := range r.classes {
		out[i] = c.ID
	}
	return out
}

// Class is the definition of id; false for a class the catalogue does not
// define.
func (r *Registry) Class(id string) (Class, bool) {
	for _, c := range r.classes {
		if c.ID == id {
			return c.clone(), true
		}
	}
	return Class{}, false
}

// Get is Class with an error naming the id (ErrUnknownClass).
func (r *Registry) Get(id string) (Class, error) {
	c, ok := r.Class(id)
	if !ok {
		return Class{}, fmt.Errorf("%w %q", ErrUnknownClass, id)
	}
	return c, nil
}

// Group is the class group of class (ADR 0040 step 1.2).
func (r *Registry) Group(class string) (string, error) {
	c, err := r.Get(class)
	return c.Group, err
}

// Groups are the group ids, in catalogue order.
func (r *Registry) Groups() []string { return slices.Clone(r.groups) }

// Races are the race definitions, in catalogue order.
func (r *Registry) Races() []Race {
	out := make([]Race, len(r.races))
	for i, x := range r.races {
		out[i] = Race{ID: x.ID, Name: x.Name.Clean()}
	}
	return out
}

// RaceIDs are the race ids, in catalogue order.
func (r *Registry) RaceIDs() []string {
	out := make([]string, len(r.races))
	for i, x := range r.races {
		out[i] = x.ID
	}
	return out
}

func (c Class) clone() Class {
	c.Name = c.Name.Clean()
	c.PrimeRequisites = slices.Clone(c.PrimeRequisites)
	c.AbilityMinimums = slices.Clone(c.AbilityMinimums)
	c.Alignments = slices.Clone(c.Alignments)
	c.Experience = slices.Clone(c.Experience)
	return c
}

// XPThreshold is the XP that reaches level, an error outside 1..MaxLevel.
func (c Class) XPThreshold(level int) (int, error) {
	if level < 1 || level > len(c.Experience) {
		return 0, fmt.Errorf("%s level %d out of range 1..%d", c.ID, level, len(c.Experience))
	}
	return c.Experience[level-1], nil
}

// LevelFor is ADR 0030 step 3: the highest level whose threshold is at or
// under xp, 1..MaxLevel. Negative XP is level 1, as 0 is.
func (c Class) LevelFor(xp int) int {
	level := 1
	for i, th := range c.Experience {
		if th <= xp {
			level = i + 1
		}
	}
	return level
}

// UnmetMinimums are the abilities whose score in scores (by ability code; an
// ability absent has no score yet and is not unmet) is under the class's
// minimum, in Abilities order.
func (c Class) UnmetMinimums(scores map[string]int) []string {
	out := []string{}
	for _, m := range c.AbilityMinimums {
		if s, ok := scores[m.Ability]; ok && s < m.Min {
			out = append(out, m.Ability)
		}
	}
	return out
}

// AlignmentAllowed reports whether the class admits alignment.
func (c Class) AlignmentAllowed(alignment string) bool {
	return slices.Contains(c.Alignments, alignment)
}
