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
	// KindPsionics: the psionicist's PSPs, picks and power checks (ADR 0043
	// steps 2 and 4 to 12). Param Psionics.
	KindPsionics = "psionics"
	// KindSaveBonus: a bonus on a save against a kind of effect (ADR 0043
	// step 3). Param SaveBonus.
	KindSaveBonus = "save_bonus"
)

// SaveAgainstMindAffecting is the one effect tag a save_bonus may name (ADR
// 0043 step 3, Q12): a save the request flags as against magic that attacks
// the mind (ADR 0006 step 9).
const SaveAgainstMindAffecting = "mind_affecting"

// SaveAgainst are the effect tags of a save request a save_bonus may name.
var SaveAgainst = []string{SaveAgainstMindAffecting}

// MaxSaveBonus bounds a save_bonus's bonus (1..MaxSaveBonus), as the
// catalogue schema does.
const MaxSaveBonus = 5

// FeatureKinds is the closed list of step 3, in the ADR's order.
var FeatureKinds = []string{
	KindExceptionalStrength, KindMeleeAttacksPerRound, KindWeaponSpecialization,
	KindWisdomBonusSpells, KindSpellFailure, KindSphereAccess, KindTurnUndead,
	KindRogueSkills, KindBackstab, KindSpellbook, KindPsionics, KindSaveBonus,
}

// notImplemented are the kinds of the closed list the code does not compute
// yet: the catalogue may list them (a class is data), but a class that does
// is not offered at creation until its kinds are built (ADR 0040 step 8).
// A kind leaves this list when its reader lands: every kind of the closed
// list has one now (save_bonus was the last, ADR 0043 step 3).
var notImplemented []string

// Implemented reports whether the code computes every feature kind the class
// lists: the creation gate of ADR 0040 step 8 ("passed the check = fully
// supported"). It reads kinds, never ids.
func (c Class) Implemented() bool {
	for _, f := range c.Features {
		if slices.Contains(notImplemented, f.Kind) {
			return false
		}
	}
	return true
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
	// Psionics is psionics' params.
	Psionics *Psionics
	// SaveBonus is save_bonus's params.
	SaveBonus *SaveBonus
	// Specialization is weapon_specialization's params (ADR 0040 step 13),
	// read from format 19 on; nothing reads them until the specialization
	// rules move to them.
	Specialization *WeaponSpecialization
}

// WeaponSpecialization are the params of weapon_specialization (ADR 0040
// step 13): the slots a specialization costs in all, the Table 35 columns a
// specialist reads in melee and in a missile type no row names, and the
// weapon types that differ.
type WeaponSpecialization struct {
	DefaultSlots  int
	MeleeColumn   string
	MissileColumn string
	Types         []SpecializationType
}

// SpecializationType is one kind of missile weapon that differs: its weapon
// type codes and what it costs in all, and either its own Table 35 Column or
// RateOfFire (the specialist keeps the weapon type's Table 45 rate of fire).
type SpecializationType struct {
	WeaponTypes []string
	Slots       int
	Column      string
	RateOfFire  bool
}

// SaveBonus are the params of save_bonus (ADR 0043 step 3): Bonus is added
// to a save whose request carries the effect tag Against (SaveAgainst), a
// labelled modifier on the roll.
type SaveBonus struct {
	Against string
	Bonus   int
}

// Psionics are the params of psionics (ADR 0043 step 2): the power
// progression table, the PSP rule and the PSP recovery table.
type Psionics struct {
	// Progression names the power progression table (step 2.1).
	Progression string
	// PSP is the PSP maximum's rule (steps 2.2 and 4).
	PSP PSPRule
	// Recovery names the PSP recovery table (steps 2.3 and 6).
	Recovery string
}

// PSPRule is the psp param of psionics (ADR 0043 steps 2.2 and 4): at level
// 1 the Table base score at BaseAbility plus the Table modifiers at
// ModifierAbilities; each later level the Table modifier at PerLevel's
// ability plus PerLevel.Plus.
type PSPRule struct {
	BaseAbility       string
	ModifierAbilities []string
	PerLevel          PSPPerLevel
	// Table names the inherent potential table (Table 5).
	Table string
}

// PSPPerLevel is a psionics level's PSP gain from level 2 (ADR 0043 step
// 4.2): the Table modifier at Ability plus Plus.
type PSPPerLevel struct {
	Ability string
	Plus    int
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
	// Skills are the set's skills in its order and ArmourFree those among them a
	// multi-class character in body armour its class may not wear still uses
	// (ADR 0040 step 12). Not params: the loader fills them from the catalogue's
	// skill set that Set names.
	Skills, ArmourFree []string
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

// FeaturesAt are all the features of kind the class grants at level, in list
// order (a kind may be listed once per Feature.Key, as rogue_skills once per
// skill set): nil for none.
func (c Class) FeaturesAt(kind string, level int) []Feature {
	var out []Feature
	for _, f := range c.Features {
		if f.Kind == kind && f.FromLevel <= level {
			out = append(out, f.clone())
		}
	}
	return out
}

// ListsAll are all the features of kind the class lists at any level, in list
// order: nil for none.
func (c Class) ListsAll(kind string) []Feature {
	var out []Feature
	for _, f := range c.Features {
		if f.Kind == kind {
			out = append(out, f.clone())
		}
	}
	return out
}

// Key tells two features of a class apart: a kind is listed once, except
// rogue_skills, listed once per skill set (a class may grant two skill sets,
// ADR 0040 step 3.5).
func (f Feature) Key() string {
	if f.Kind == KindRogueSkills && f.RogueSkills != nil {
		return f.Kind + ":" + f.RogueSkills.Set
	}
	return f.Kind
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
		r.Skills, r.ArmourFree = slices.Clone(r.Skills), slices.Clone(r.ArmourFree)
		f.RogueSkills = &r
	}
	if f.Spellbook != nil {
		b := *f.Spellbook
		f.Spellbook = &b
	}
	if f.Psionics != nil {
		p := *f.Psionics
		p.PSP.ModifierAbilities = slices.Clone(p.PSP.ModifierAbilities)
		f.Psionics = &p
	}
	if f.SaveBonus != nil {
		b := *f.SaveBonus
		f.SaveBonus = &b
	}
	return f
}

// checkFeatures validates a class's feature list (ADR 0040 step 8): known
// kinds, each once (a rogue_skills once per set), a from_level in 1..MaxLevel (0 reads as 1), and exactly
// the params of each kind. The list comes back copied with FromLevel set.
func checkFeatures(fs []Feature) ([]Feature, error) {
	if err := unique(fs, Feature.Key); err != nil {
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

// checkSaveBonus refuses an unknown effect tag or a bonus outside
// 1..MaxSaveBonus.
func checkSaveBonus(b SaveBonus) error {
	if !slices.Contains(SaveAgainst, b.Against) {
		return fmt.Errorf("unknown save effect tag %q", b.Against)
	}
	if b.Bonus < 1 || b.Bonus > MaxSaveBonus {
		return fmt.Errorf("save bonus %d out of range 1..%d", b.Bonus, MaxSaveBonus)
	}
	return nil
}

// checkParams refuses a missing param of f's kind or a param of another
// kind.
func checkParams(f Feature) error {
	table, ability, spheres, rogue, book := f.Table != "", f.Ability != "", f.Spheres != nil, f.RogueSkills != nil, f.Spellbook != nil
	if (f.Kind != KindPsionics && f.Psionics != nil) || (f.Kind != KindSaveBonus && f.SaveBonus != nil) ||
		(f.Kind != KindWeaponSpecialization && f.Specialization != nil) {
		return errors.New("a missing or unexpected parameter")
	}
	want := func(t, a, s, r, b bool) error {
		if table != t || ability != a || spheres != s || rogue != r || book != b {
			return errors.New("a missing or unexpected parameter")
		}
		return nil
	}
	switch f.Kind {
	case KindExceptionalStrength, KindWeaponSpecialization, KindWisdomBonusSpells:
		return want(false, false, false, false, false)
	case KindSaveBonus:
		if err := want(false, false, false, false, false); err != nil {
			return err
		}
		if f.SaveBonus == nil {
			return errors.New("a missing or unexpected parameter")
		}
		return checkSaveBonus(*f.SaveBonus)
	case KindPsionics:
		if err := want(false, false, false, false, false); err != nil {
			return err
		}
		if f.Psionics == nil {
			return errors.New("a missing or unexpected parameter")
		}
		return checkPsionics(*f.Psionics)
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

// checkPsionics refuses a psionics feature with a table not named or an
// unknown ability (ADR 0040 step 8); that the named tables exist is the rule
// tables' check at start.
func checkPsionics(p Psionics) error {
	if p.Progression == "" || p.Recovery == "" || p.PSP.Table == "" {
		return errors.New("a table not named")
	}
	abilities := append([]string{p.PSP.BaseAbility, p.PSP.PerLevel.Ability}, p.PSP.ModifierAbilities...)
	for _, a := range abilities {
		if !slices.Contains(Abilities, a) {
			return fmt.Errorf("unknown ability %q", a)
		}
	}
	return nil
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
