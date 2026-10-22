package ruletablesvc

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
)

// The non-weapon proficiency catalogue and PHB Table 38's crossovers (ADR 0037
// steps 1 and 2): data a campaign extends with a row, never code. A name is an
// i18n key by id, so the entities carry none.

// GeneralProficiencyGroup is PHB Table 37's group open to every class; the
// table's other groups are the class groups of the definitions (ADR 0040
// step 1.2).
const GeneralProficiencyGroup = "general"

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

// SlotRow is one row of PHB Table 34 (ADR 0037 step 1): a class group's
// weapon and non-weapon slots. A pool's slots are Initial plus one at every
// level divisible by its Levels (step 3).
type SlotRow struct {
	Group            string
	WeaponInitial    int
	WeaponLevels     int
	Penalty          int
	NonweaponInitial int
	NonweaponLevels  int
}

// Proficiencies is the validated catalogue. The zero value and nil are the
// empty catalogue.
type Proficiencies struct {
	list       []Proficiency
	crossovers []Crossover
	slots      []SlotRow
	// groupOf is each defined class's group (ADR 0040 step 1.2).
	groupOf map[string]string
}

// NewProficiencies validates the rows: an id once and non-empty, a known
// ability (or none, with a 0 modifier), at least one slot, one or more known
// groups (the general one or a class group) once each; the crossovers cover each of Classes exactly once with
// known groups; the slot rows (Table 34) cover each of Groups exactly once,
// with initial slots of 0 or more, #Levels of 1 or more and a penalty of 0 or
// less.
func NewProficiencies(list []Proficiency, crossovers []Crossover, slots []SlotRow, defs *classes.Registry) (*Proficiencies, error) {
	if defs == nil {
		return nil, errors.New("proficiencies: no class definitions")
	}
	classIDs, groups := defs.ClassIDs(), defs.Groups()
	profGroups := append([]string{GeneralProficiencyGroup}, groups...)
	seen := make(map[string]bool, len(list))
	out := &Proficiencies{list: make([]Proficiency, len(list)), crossovers: make([]Crossover, len(crossovers)), slots: slices.Clone(slots), groupOf: map[string]string{}}
	for _, c := range defs.Classes() {
		out.groupOf[c.ID] = c.Group
	}
	if err := checkSlotRows(slots, groups); err != nil {
		return nil, err
	}
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
		if err := checkGroups("proficiencies: "+p.ID, p.Groups, profGroups); err != nil {
			return nil, err
		}
		p.Groups = slices.Clone(p.Groups)
		out.list[i] = p
	}
	have := map[string]bool{}
	for i, c := range crossovers {
		switch {
		case !slices.Contains(classIDs, c.Class):
			return nil, fmt.Errorf("proficiency crossovers: unknown class %q", c.Class)
		case have[c.Class]:
			return nil, fmt.Errorf("proficiency crossovers: class %q twice", c.Class)
		}
		have[c.Class] = true
		if err := checkGroups("proficiency crossovers: "+c.Class, c.Groups, profGroups); err != nil {
			return nil, err
		}
		c.Groups = slices.Clone(c.Groups)
		out.crossovers[i] = c
	}
	for _, class := range classIDs {
		if len(crossovers) > 0 && !have[class] {
			return nil, fmt.Errorf("proficiency crossovers: class %q is missing", class)
		}
	}
	return out, nil
}

func checkSlotRows(rows []SlotRow, groups []string) error {
	have := map[string]bool{}
	for _, r := range rows {
		switch {
		case !slices.Contains(groups, r.Group):
			return fmt.Errorf("proficiency slots: unknown group %q", r.Group)
		case have[r.Group]:
			return fmt.Errorf("proficiency slots: group %q twice", r.Group)
		case r.WeaponInitial < 0 || r.NonweaponInitial < 0:
			return fmt.Errorf("proficiency slots: %q has negative initial slots", r.Group)
		case r.WeaponLevels < 1 || r.NonweaponLevels < 1:
			return fmt.Errorf("proficiency slots: %q needs #Levels of 1 or more", r.Group)
		case r.Penalty > 0:
			return fmt.Errorf("proficiency slots: %q has a positive penalty", r.Group)
		}
		have[r.Group] = true
	}
	for _, g := range groups {
		if len(rows) > 0 && !have[g] {
			return fmt.Errorf("proficiency slots: group %q is missing", g)
		}
	}
	return nil
}

func checkGroups(where string, groups, known []string) error {
	if len(groups) == 0 {
		return fmt.Errorf("%s: no group", where)
	}
	seen := map[string]bool{}
	for _, g := range groups {
		switch {
		case !slices.Contains(known, g):
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

// SlotRows are Table 34's rows, in the file's order.
func (p *Proficiencies) SlotRows() []SlotRow {
	if p == nil {
		return nil
	}
	return slices.Clone(p.slots)
}

// SlotRowOf is Table 34's row for class's group; false for an unknown class
// or a catalogue without the table.
func (p *Proficiencies) SlotRowOf(class string) (SlotRow, bool) {
	if p == nil {
		return SlotRow{}, false
	}
	group, ok := p.groupOf[class]
	if !ok {
		return SlotRow{}, false
	}
	for _, r := range p.slots {
		if r.Group == group {
			return r, true
		}
	}
	return SlotRow{}, false
}

// Get is the catalogue row of id; false when the catalogue has none.
func (p *Proficiencies) Get(id string) (Proficiency, bool) {
	if p == nil {
		return Proficiency{}, false
	}
	for _, r := range p.list {
		if r.ID == id {
			r.Groups = slices.Clone(r.Groups)
			return r, true
		}
	}
	return Proficiency{}, false
}

// GroupsOf are the groups class learns at Table 37's cost (Table 38); nil for
// a class the catalogue has no row for.
func (p *Proficiencies) GroupsOf(class string) []string {
	if p == nil {
		return nil
	}
	for _, c := range p.crossovers {
		if c.Class == class {
			return slices.Clone(c.Groups)
		}
	}
	return nil
}
