package ruletablesvc

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// ErrLevelOutOfTable is a contact cost asked for a level Table 29 has no row
// for (ADR 0044 step 1.2, Q15): the DM gives a variable cost instead.
var ErrLevelOutOfTable = errors.New("the level is above the contact cost table")

// MaxContactLevel is the last level of CPsiH Table 29.
const MaxContactLevel = 20

// AttackVersusDefenceRow is CPsiH Table 14 (ADR 0044 step 1.1): the modifier to
// the attacker's power score when the attack mode meets the defence mode.
type AttackVersusDefenceRow struct {
	Attack   string
	Defence  string
	Modifier int
}

// ContactCostRow is CPsiH Table 29 (ADR 0044 step 1.2): the contact power's
// initial cost for a target of MinLevel to MaxLevel levels or hit dice.
type ContactCostRow struct {
	MinLevel int
	MaxLevel int
	Cost     int
}

// Kinds of a contact modifier (ADR 0044 step 1.3).
const (
	ContactDistance   = "distance"
	ContactLifeOrder  = "life_order"
	ContactResistance = "resistance"
)

// ContactModifierRow is a modifier to the contact power's score (ADR 0044
// step 1.3): Table 30 by distance, Table 31 by life order, or the resistance
// of a nonpsionicist. Code is language-neutral; the web words it.
type ContactModifierRow struct {
	Kind     string
	Code     string
	Modifier int
}

// ContactModifierCodes are the codes of each kind of contact modifier.
var ContactModifierCodes = map[string][]string{
	ContactDistance:   {"sight", "mile_1", "miles_10", "miles_100", "miles_1000", "miles_10000"},
	ContactLifeOrder:  {"mammal", "marsupial", "bird", "reptile_amphibian", "fish", "arachnid_insect", "monster", "plant"},
	ContactResistance: {"resisting"},
}

func contactModifierKey(r ContactModifierRow) string { return r.Kind + " " + r.Code }

func contactModifierKeys() []string {
	var out []string
	for _, k := range []string{ContactDistance, ContactLifeOrder, ContactResistance} {
		for _, c := range ContactModifierCodes[k] {
			out = append(out, k+" "+c)
		}
	}
	return out
}

// checkAttackVersusDefence: a full grid, each (attack, defence) pair once. The
// modes themselves are the catalogue's powers, checked by CheckPowerKinds.
func checkAttackVersusDefence(rows []AttackVersusDefenceRow, defs *classes.Registry) error {
	if len(rows) == 0 {
		return readByPsionics(defs)
	}
	var attacks, defences []string
	for _, r := range rows {
		if !slices.Contains(attacks, r.Attack) {
			attacks = append(attacks, r.Attack)
		}
		if !slices.Contains(defences, r.Defence) {
			defences = append(defences, r.Defence)
		}
	}
	return once(rows, func(r AttackVersusDefenceRow) string { return r.Attack + " " + r.Defence }, pairs(attacks, defences))
}

// checkContactCost: bands contiguous from level 1, each at least 1 PSP, none
// past MaxContactLevel.
func checkContactCost(rows []ContactCostRow, defs *classes.Registry) error {
	if len(rows) == 0 {
		return readByPsionics(defs)
	}
	next := 1
	for i, r := range rows {
		if r.MinLevel > r.MaxLevel {
			return fmt.Errorf("row %d: min above max", i)
		}
		if r.MinLevel != next {
			return fmt.Errorf("row %d: level %d expected, got %d", i, next, r.MinLevel)
		}
		if r.Cost < 1 {
			return fmt.Errorf("row %d: cost %d", i, r.Cost)
		}
		next = r.MaxLevel + 1
	}
	if next-1 > MaxContactLevel {
		return fmt.Errorf("bands go past level %d", MaxContactLevel)
	}
	return nil
}

// checkContactModifiers: each code of each kind once, none above 0.
func checkContactModifiers(rows []ContactModifierRow, defs *classes.Registry) error {
	if len(rows) == 0 {
		return readByPsionics(defs)
	}
	for _, r := range rows {
		if r.Modifier > 0 {
			return fmt.Errorf("%s: modifier %d above 0", contactModifierKey(r), r.Modifier)
		}
	}
	return once(rows, contactModifierKey, contactModifierKeys())
}

// AttackVersusDefence is Table 14's modifier for an attack mode meeting a
// defence mode (ADR 0044 step 1.1).
func (t *Tables) AttackVersusDefence(attack, defence string) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	for _, r := range c.AttackVersusDefence {
		if r.Attack == attack && r.Defence == defence {
			return r.Modifier, nil
		}
	}
	return 0, fmt.Errorf("attack versus defence: no Table 14 cell for %q against %q", attack, defence)
}

// ContactCost is Table 29's initial cost of the contact power for a target of
// level (or hit dice) (ADR 0044 step 1.2). A level above MaxContactLevel, or
// under 1, is ErrLevelOutOfTable (Q15).
func (t *Tables) ContactCost(level int) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	if level >= 1 && level <= MaxContactLevel {
		for _, r := range c.ContactCost {
			if level >= r.MinLevel && level <= r.MaxLevel {
				return r.Cost, nil
			}
		}
	}
	return 0, ErrLevelOutOfTable
}

// ContactModifier is the contact power's score modifier for code of kind
// (ADR 0044 step 1.3: Table 30 by distance, Table 31 by life order, or the
// resistance). An unknown kind or code is an error.
func (t *Tables) ContactModifier(kind, code string) (int, error) {
	c, err := t.classTables()
	if err != nil {
		return 0, err
	}
	for _, r := range c.ContactModifiers {
		if r.Kind == kind && r.Code == code {
			return r.Modifier, nil
		}
	}
	return 0, fmt.Errorf("contact modifiers: no row for %s %q", kind, code)
}

// CheckPowerKinds is the start-up check that ties the combat tables to the
// powers (ADR 0044 step 1.1): every attack of Table 14 is an attack_mode
// power, every defence a defence_mode power, and every such mode of the
// catalogue has its cells, so a devotion where a mode belongs is refused.
func (t *Tables) CheckPowerKinds(powers []catalogue.Power) error {
	c, err := t.classTables()
	if err != nil {
		return err
	}
	kind := make(map[string]string, len(powers))
	for _, p := range powers {
		kind[p.ID] = p.Kind
	}
	inGrid := map[string]bool{}
	for _, r := range c.AttackVersusDefence {
		if kind[r.Attack] != catalogue.KindAttackMode {
			return fmt.Errorf("attack_versus_defence table: %q is not an attack mode (kind %q)", r.Attack, kind[r.Attack])
		}
		if kind[r.Defence] != catalogue.KindDefenceMode {
			return fmt.Errorf("attack_versus_defence table: %q is not a defence mode (kind %q)", r.Defence, kind[r.Defence])
		}
		inGrid[r.Attack], inGrid[r.Defence] = true, true
	}
	if len(c.AttackVersusDefence) == 0 {
		return nil
	}
	for _, p := range powers {
		if (p.Kind == catalogue.KindAttackMode || p.Kind == catalogue.KindDefenceMode) && !inGrid[p.ID] {
			return fmt.Errorf("attack_versus_defence table: no cells for the %s %q", p.Kind, p.ID)
		}
	}
	return nil
}
