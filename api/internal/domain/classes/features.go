package classes

import (
	"errors"
	"fmt"
	"slices"
)

// The closed list of class feature kinds (ADR 0040 step 3). A class has a
// mechanic only by listing its kind; the code implements each kind once and
// switches on the kind, never on a class id. A new mechanic is a new kind
// here (an amendment of step 3), never a branch on an id.
const (
	// KindExceptionalStrength: an 18/xx at Strength 18 and the warrior's
	// scale of Strength spells above 18 (ADR 0030 steps 9 and 10). No params.
	KindExceptionalStrength = "exceptional_strength"
	// KindMeleeAttacksPerRound: the attacks per round of PHB Table 15 (ADR
	// 0030 step 11.1). Param Table.
	KindMeleeAttacksPerRound = "melee_attacks_per_round"
	// KindWeaponSpecialization: a weapon specialization pick (ADR 0030 steps
	// 11 and 12, ADR 0037 step 9). No params.
	KindWeaponSpecialization = "weapon_specialization"
	// KindWisdomBonusSpells: PHB Table 5's bonus spells on the slots (ADR 0030
	// step 13). No params.
	KindWisdomBonusSpells = "wisdom_bonus_spells"
	// KindSpellFailure: PHB Table 5's chance of spell failure (ADR 0030 step
	// 14). Param Ability.
	KindSpellFailure = "spell_failure"
	// KindSphereAccess: the spheres a priest prepares from (ADR 0030 step 15).
	// Param Spheres.
	KindSphereAccess = "sphere_access"
	// KindTurnUndead: PHB Table 61 (ADR 0030 step 16). Param Table.
	KindTurnUndead = "turn_undead"
	// KindRogueSkills: a skill set with its points, budget and armour (ADR
	// 0030 steps 17 and 18). Param RogueSkills.
	KindRogueSkills = "rogue_skills"
	// KindBackstab: PHB Table 30's multiplier (ADR 0030 step 19). Param Table.
	KindBackstab = "backstab"
	// KindSpellbook: a spellbook (ADR 0030 step 20). Param Spellbook.
	KindSpellbook = "spellbook"
)

// FeatureKinds is the closed list of step 3, in the ADR's order.
var FeatureKinds = []string{
	KindExceptionalStrength, KindMeleeAttacksPerRound, KindWeaponSpecialization,
	KindWisdomBonusSpells, KindSpellFailure, KindSphereAccess, KindTurnUndead,
	KindRogueSkills, KindBackstab, KindSpellbook,
}

// Feature is one entry of a class's feature list (ADR 0040 step 1.11): a kind,
// the class level it starts at and the kind's parameters. Only the fields
// of its kind are set (New refuses the others).
type Feature struct {
	Kind string
	// FromLevel is the class level the feature starts at, 1..MaxLevel (1
	// when the data leaves it out).
	FromLevel int
	// Table names the rule table of melee_attacks_per_round, turn_undead and
	// backstab.
	Table string
	// Ability is spell_failure's ability code.
	Ability string
	// Spheres are sphere_access's params.
	Spheres *SphereAccess
	// RogueSkills are rogue_skills' params.
	RogueSkills *RogueSkills
	// Spellbook is spellbook's params.
	Spellbook *Spellbook
}

// SphereAccess are the params of sphere_access (ADR 0030 step 15): the
// major and minor sphere codes and the highest spell level a minor sphere
// opens.
type SphereAccess struct {
	Major, Minor  []string
	MinorMaxLevel int
}

// Opens is ADR 0030 step 15.2: a spell of spellLevel with the sphere codes
// spheres is open when one of its spheres is a major sphere (any level) or a
// minor one at levels 1 to MinorMaxLevel. A sphere in neither list is closed.
func (a SphereAccess) Opens(spellLevel int, spheres []string) bool {
	for _, s := range spheres {
		if slices.Contains(a.Major, s) {
			return true
		}
		if spellLevel <= a.MinorMaxLevel && slices.Contains(a.Minor, s) {
			return true
		}
	}
	return false
}

// Codes are every sphere code the access names, major then minor.
func (a SphereAccess) Codes() []string {
	return append(slices.Clone(a.Major), a.Minor...)
}

// RogueSkills are the params of rogue_skills (ADR 0030 steps 17 and 18).
type RogueSkills struct {
	// Set is the skill set code: the skill list and the base rows it reads
	// (PHB Table 26 or 33) and the check kind it logs (ADR 0040 step 3.2).
	Set string
	// Budget is the skill point budget.
	Budget SkillBudget
	// Armour maps each body armour skill column (ADR 0033 step 14) to the
	// thief_skill_armor row it reads and the extra adjustment.
	Armour []SkillArmour
}

// SkillBudget is a rogue_skills budget: First points at level 1 and PerLevel
// at each level after, at most PerSkillFirst plus PerSkillPerLevel a level
// after the first in one skill (both 0: no per-skill limit).
type SkillBudget struct {
	First, PerLevel, PerSkillFirst, PerSkillPerLevel int
}

// SkillArmour is one rogue_skills armour entry: Code is a body armour's
// Table 29 skill column (or "none"), Column the thief_skill_armor row it
// reads ("" for leather, the baseline: every adjustment 0) and Extra an
// adjustment added to every skill (the bard's metal armour).
type SkillArmour struct {
	Code, Column string
	Extra        int
}

// Spellbook are the params of spellbook (ADR 0030 step 20).
type Spellbook struct {
	// MaxSpellLevel is the highest spell level the class writes (step 20.1).
	MaxSpellLevel int
	// FreeSpellPerNewSpellLevel: each new spell level the class opens grants
	// a free spell (step 20.8).
	FreeSpellPerNewSpellLevel bool
}

// Feature is ADR 0040 step 5: the feature of kind the class grants at level
// (listed at a from_level at or under level), false when it lists none or
// only above level.
func (c Class) Feature(kind string, level int) (Feature, bool) {
	f, ok := c.Lists(kind)
	if !ok || f.FromLevel > level {
		return Feature{}, false
	}
	return f, true
}

// Has reports whether the class grants kind at level (Feature).
func (c Class) Has(kind string, level int) bool {
	_, ok := c.Feature(kind, level)
	return ok
}

// Lists is the feature of kind the class lists at any level, false for
// none: what the creation options show of a class before a level is known.
func (c Class) Lists(kind string) (Feature, bool) {
	for _, f := range c.Features {
		if f.Kind == kind {
			return f.clone(), true
		}
	}
	return Feature{}, false
}

func (f Feature) clone() Feature {
	if f.Spheres != nil {
		s := *f.Spheres
		s.Major, s.Minor = slices.Clone(s.Major), slices.Clone(s.Minor)
		f.Spheres = &s
	}
	if f.RogueSkills != nil {
		r := *f.RogueSkills
		r.Armour = slices.Clone(r.Armour)
		f.RogueSkills = &r
	}
	if f.Spellbook != nil {
		b := *f.Spellbook
		f.Spellbook = &b
	}
	return f
}

// checkFeatures validates a class's feature list (ADR 0040 step 8): known
// kinds, each once, a from_level in 1..MaxLevel (0 reads as 1), and exactly
// the params of each kind. The list comes back copied with FromLevel set.
func checkFeatures(fs []Feature) ([]Feature, error) {
	if err := unique(fs, func(f Feature) string { return f.Kind }); err != nil {
		return nil, fmt.Errorf("features: %w", err)
	}
	out := make([]Feature, 0, len(fs))
	for _, f := range fs {
		if f.FromLevel == 0 {
			f.FromLevel = 1
		}
		if f.FromLevel < 1 || f.FromLevel > MaxLevel {
			return nil, fmt.Errorf("feature %s: from level %d out of range 1..%d", f.Kind, f.FromLevel, MaxLevel)
		}
		if err := checkParams(f); err != nil {
			return nil, fmt.Errorf("feature %s: %w", f.Kind, err)
		}
		out = append(out, f.clone())
	}
	return out, nil
}

// checkParams refuses a missing param of f's kind or a param of another
// kind.
func checkParams(f Feature) error {
	table, ability, spheres, rogue, book := f.Table != "", f.Ability != "", f.Spheres != nil, f.RogueSkills != nil, f.Spellbook != nil
	want := func(t, a, s, r, b bool) error {
		if table != t || ability != a || spheres != s || rogue != r || book != b {
			return errors.New("a missing or unexpected parameter")
		}
		return nil
	}
	switch f.Kind {
	case KindExceptionalStrength, KindWeaponSpecialization, KindWisdomBonusSpells:
		return want(false, false, false, false, false)
	case KindMeleeAttacksPerRound, KindTurnUndead, KindBackstab:
		return want(true, false, false, false, false)
	case KindSpellFailure:
		if err := want(false, true, false, false, false); err != nil {
			return err
		}
		if !slices.Contains(Abilities, f.Ability) {
			return fmt.Errorf("unknown ability %q", f.Ability)
		}
		return nil
	case KindSphereAccess:
		if err := want(false, false, true, false, false); err != nil {
			return err
		}
		if f.Spheres.MinorMaxLevel < 1 {
			return errors.New("minor_max_level under 1")
		}
		return nil
	case KindRogueSkills:
		if err := want(false, false, false, true, false); err != nil {
			return err
		}
		return checkRogueSkills(*f.RogueSkills)
	case KindSpellbook:
		if err := want(false, false, false, false, true); err != nil {
			return err
		}
		if f.Spellbook.MaxSpellLevel < 1 || f.Spellbook.MaxSpellLevel > 9 {
			return fmt.Errorf("max spell level %d out of range 1..9", f.Spellbook.MaxSpellLevel)
		}
		return nil
	}
	return fmt.Errorf("unknown feature kind %q", f.Kind)
}

func checkRogueSkills(r RogueSkills) error {
	if r.Set == "" {
		return errors.New("no skill set")
	}
	b := r.Budget
	if b.First < 0 || b.PerLevel < 0 || b.PerSkillFirst < 0 || b.PerSkillPerLevel < 0 {
		return errors.New("a negative budget")
	}
	if len(r.Armour) == 0 {
		return errors.New("no armour entry")
	}
	return unique(r.Armour, func(a SkillArmour) string { return a.Code })
}

// SkillArmourFor is the rogue_skills armour entry of a body armour's Table
// 29 skill column code, false when the params list none.
func (r RogueSkills) SkillArmourFor(code string) (SkillArmour, bool) {
	for _, a := range r.Armour {
		if a.Code == code {
			return a, true
		}
	}
	return SkillArmour{}, false
}
