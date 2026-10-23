package charactersvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// Draft is the answer to a creation draft (Service.Draft).
type Draft struct {
	// Character is the character Create would store, derived and warned as a read of it; nil
	// while the class, race, scores, skill points or XP are missing or refused. Its picks and
	// languages are read as entered when the skills module refuses them, the native language
	// added as a read adds it.
	Character *Character
	// SkillPoints is a thief's or a bard's skill point budget at the starting level, set as soon
	// as the class and XP read, whatever the race and scores (ADR 0030 steps 17 and 18); nil for
	// another class or while the class or XP is missing or refused. With a Character, it equals
	// the derived block's.
	SkillPoints *ruletablesvc.SkillPointsBudget
	// Spellbook is a mage's or a bard's starting spellbook at the starting level (ADR 0030 step
	// 20.7): its limits and the spells entered by spell level; nil for another class or while
	// Character is nil. Its refusals are among Refusals, by spell.
	Spellbook *StartingSpellbook
	// Refusals are Create's refusals of the draft, in the form's order: each a
	// *domain.FieldError or a *domain.BudgetError.
	Refusals []error
	// Missing are the fields still blank that Create requires, in the form's order.
	Missing []string
	// UnmetMinimums are, for every creatable class, the abilities whose entered score is under
	// the class's minimum (PHB Table 13); empty where the scores meet them or are not entered.
	UnmetMinimums []ClassUnmet
	// ExceptionalStrength is ADR 0030 step 9 on the entries: the class lists exceptional_strength
	// (ADR 0040 step 3), the Strength entered is 18 and the race rolls it. The form offers the
	// 18/xx field on it, and sends one only then.
	ExceptionalStrength bool
	// Alignments are the alignments every active entry's class admits (ADR 0042 step 2), in the
	// first class's order: every one of the nine before a class is entered. The class left by a
	// dual-class character does not limit them.
	Alignments []string
	// SkillsClass is the active class that carries the skill points (the first listing rogue
	// skills), SpellbookClass the one that carries the starting spellbook; "" for none.
	SkillsClass, SpellbookClass string
	// PsionicsClass is the active class that lists psionics (ADR 0043: its known powers and PSP
	// maximum are entered at creation); "" for none.
	PsionicsClass string
	// ComputedPSPMax is ADR 0043 step 4's PSP maximum at the psionics entry's starting level, the
	// one Create stores when none is entered (step 4.4); nil for a character with no psionics
	// entry or while Character is nil. The pool Create would store is Character's Derived.PSP.
	ComputedPSPMax *int
}

// ClassUnmet is a class and the abilities under its minimum.
type ClassUnmet struct {
	Class     string
	Abilities []string
}

// carriers are the classes that carry the character's skill points (ADR 0030 steps 17 and 18) and
// its starting spellbook (step 20.7), and its psionics (ADR 0043 step 9: the known powers and the
// PSP maximum): the first active class listing rogue skills, the first listing a spellbook, the
// first listing psionics, "" for none. The form reads them here instead of choosing.
func carriers(active []classes.Class) (skills, book, psi string) {
	for _, def := range active {
		if _, ok := def.Lists(classes.KindRogueSkills); ok && skills == "" {
			skills = def.ID
		}
		if _, ok := def.Lists(classes.KindSpellbook); ok && book == "" {
			book = def.ID
		}
		if _, ok := def.Lists(classes.KindPsionics); ok && psi == "" {
			psi = def.ID
		}
	}
	return skills, book, psi
}

// skillsLevel is the level the skill point budget reads before a character derives: the skills
// entry's entered level, else the level its XP reaches (ADR 0030 step 3, step 7.5).
func (s *service) skillsLevel(k checked) (int, error) {
	for _, e := range k.ents {
		if e.Class != k.def.ID || e.frozen() {
			continue
		}
		if e.Level > 0 {
			return e.Level, nil
		}
		return k.def.LevelFor(e.XP), nil
	}
	return 0, nil
}

// budgetAt is a draft's skill point budget (of the first skill set) at level (ADR 0030 steps 17 and 18), nil for a class
// without skill points, and adds to out the creation's refusal of points over it (ADR 0037 step
// 10: nothing was spent before).
func budgetAt(out *Draft, def classes.Class, level int, all skills.SetPoints) *ruletablesvc.SkillPointsBudget {
	set := skills.FirstSet(def)
	points := all.Of(set)
	if err := skills.SkillPointsWithinBudget(def, set, level, nil, points); err != nil {
		out.Refusals = append(out.Refusals, err)
	}
	if b, ok := ruletablesvc.SkillPointsFigures(skills.RogueSkillsAt(def, level), level, points); ok {
		return &b
	}
	return nil
}

// unmetMinimums are, for every creatable class, the abilities whose entered score is under the
// class's minimum (PHB Table 13); an ability not entered is not unmet.
func unmetMinimums(defs *classes.Registry, n NewCharacter, blank []string) []ClassUnmet {
	scores := map[string]int{}
	for _, a := range []struct {
		ability Ability
		v       int
	}{{AbilityStr, n.Str}, {AbilityDex, n.Dex}, {AbilityCon, n.Con}, {AbilityInt, n.Int}, {AbilityWis, n.Wis}, {AbilityCha, n.Cha}} {
		if !slices.Contains(blank, string(a.ability)) {
			scores[string(a.ability)] = a.v
		}
	}
	out := make([]ClassUnmet, 0, len(defs.Classes()))
	for _, class := range defs.Classes() {
		if !class.Implemented() {
			continue
		}
		out = append(out, ClassUnmet{Class: class.ID, Abilities: class.UnmetMinimums(scores)})
	}
	return out
}
