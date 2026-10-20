package ruletablesvc

import (
	"fmt"
	"slices"
)

// The non-weapon proficiency catalogue and PHB Table 38's crossovers (ADR 0037
// steps 1 and 2): data a campaign extends with a row, never code. A name is an
// i18n key by id, so the entities carry none.

// ProficiencyGroups are the groups of PHB Table 37: the four class groups and
// the general one.
var ProficiencyGroups = []string{"general", "priest", "rogue", "warrior", "wizard"}

// proficiencyAbilities are the abilities a check can use.
var proficiencyAbilities = []string{"str", "dex", "con", "int", "wis", "cha"}

// Proficiency is one row of Table 37. Ability is "" for the rows the table
// gives no check (Blind-fighting, Mountaineering); Modifier is then 0. Groups
// lists every group the table prints it under.
type Proficiency struct {
	ID       string
	Ability  string
	Modifier int
	Slots    int
	Groups   []string
}

// Crossover is one row of Table 38: the groups a class learns at the table's
// cost (the others cost one more).
type Crossover struct {
	Class  string
	Groups []string
}

// Proficiencies is the validated catalogue. The zero value and nil are the
// empty catalogue.
type Proficiencies struct {
	list       []Proficiency
	crossovers []Crossover
}

// NewProficiencies validates the rows: an id once and non-empty, a known
// ability (or none, with a 0 modifier), at least one slot, one or more known
// groups once each; the crossovers cover each of Classes exactly once with
// known groups.
func NewProficiencies(list []Proficiency, crossovers []Crossover) (*Proficiencies, error) {
	seen := make(map[string]bool, len(list))
	out := &Proficiencies{list: make([]Proficiency, len(list)), crossovers: make([]Crossover, len(crossovers))}
	for i, p := range list {
		switch {
		case p.ID == "":
			return nil, fmt.Errorf("proficiencies: row %d has no id", i)
		case seen[p.ID]:
			return nil, fmt.Errorf("proficiencies: id %q twice", p.ID)
		case p.Ability != "" && !slices.Contains(proficiencyAbilities, p.Ability):
			return nil, fmt.Errorf("proficiencies: %q has the unknown ability %q", p.ID, p.Ability)
		case p.Ability == "" && p.Modifier != 0:
			return nil, fmt.Errorf("proficiencies: %q has a modifier but no ability", p.ID)
		case p.Slots < 1:
			return nil, fmt.Errorf("proficiencies: %q needs at least 1 slot", p.ID)
		}
		seen[p.ID] = true
		if err := checkGroups("proficiencies: "+p.ID, p.Groups); err != nil {
			return nil, err
		}
		p.Groups = slices.Clone(p.Groups)
		out.list[i] = p
	}
	have := map[string]bool{}
	for i, c := range crossovers {
		switch {
		case !slices.Contains(Classes, c.Class):
			return nil, fmt.Errorf("proficiency crossovers: unknown class %q", c.Class)
		case have[c.Class]:
			return nil, fmt.Errorf("proficiency crossovers: class %q twice", c.Class)
		}
		have[c.Class] = true
		if err := checkGroups("proficiency crossovers: "+c.Class, c.Groups); err != nil {
			return nil, err
		}
		c.Groups = slices.Clone(c.Groups)
		out.crossovers[i] = c
	}
	for _, class := range Classes {
		if len(crossovers) > 0 && !have[class] {
			return nil, fmt.Errorf("proficiency crossovers: class %q is missing", class)
		}
	}
	return out, nil
}

func checkGroups(where string, groups []string) error {
	if len(groups) == 0 {
		return fmt.Errorf("%s: no group", where)
	}
	seen := map[string]bool{}
	for _, g := range groups {
		switch {
		case !slices.Contains(ProficiencyGroups, g):
			return fmt.Errorf("%s: unknown group %q", where, g)
		case seen[g]:
			return fmt.Errorf("%s: group %q twice", where, g)
		}
		seen[g] = true
	}
	return nil
}

// All are the catalogue's proficiencies, in the file's order.
func (p *Proficiencies) All() []Proficiency {
	if p == nil {
		return nil
	}
	out := make([]Proficiency, len(p.list))
	for i, r := range p.list {
		r.Groups = slices.Clone(r.Groups)
		out[i] = r
	}
	return out
}

// Crossovers are Table 38's rows, in the file's order.
func (p *Proficiencies) Crossovers() []Crossover {
	if p == nil {
		return nil
	}
	out := make([]Crossover, len(p.crossovers))
	for i, c := range p.crossovers {
		c.Groups = slices.Clone(c.Groups)
		out[i] = c
	}
	return out
}
