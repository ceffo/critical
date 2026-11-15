package charactersvc

import (
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/localized"
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
	// SkillSets are the skill sets the active classes list (ADR 0040 step 3.5), in the entries'
	// order, each with its own skills and its skill point budget at the starting level (ADR 0030
	// steps 17 and 18), set as soon as the class and XP read, whatever the race and scores; none
	// for a class without skill points. With a Character, a set's budget equals the derived
	// block's.
	SkillSets []DraftSkillSet
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
	// SpellbookClass is the active class that carries the starting spellbook (the first listing
	// one); "" for none.
	SpellbookClass string
	// PsionicsClass is the active class that lists psionics (ADR 0043: its known powers and PSP
	// maximum are entered at creation); "" for none.
	PsionicsClass string
	// ComputedPSPMax is ADR 0043 step 4's PSP maximum at the psionics entry's starting level, the
	// one Create stores when none is entered (step 4.4); nil for a character with no psionics
	// entry or while Character is nil. The pool Create would store is Character's Derived.PSP.
	ComputedPSPMax *int
}

// DraftSkillSet is one skill set of a creation draft: the set a creation enters points in, the
// skills of it and its budget at the starting level of the entry that grants it.
type DraftSkillSet struct {
	Set string
	// Name is the set's heading (ADR 0040 step 15).
	Name   localized.Text
	Skills []string
	// Budget is nil while the set is not granted yet at the starting level (its from_level).
	Budget *ruletablesvc.SkillPointsBudget
}

// ClassUnmet is a class and the abilities under its minimum.
type ClassUnmet struct {
	Class     string
	Abilities []string
}

// carriers are the classes that carry the character's starting spellbook (ADR 0030 step 20.7) and
// its psionics (ADR 0043 step 9: the known powers and the PSP maximum): the first active class
// listing a spellbook, the first listing psionics, "" for none. The form reads them here instead
// of choosing. The skill points are per skill set (entrySets).
func carriers(active []classes.Class) (book, psi string) {
	for _, def := range active {
		if _, ok := def.Lists(classes.KindSpellbook); ok && book == "" {
			book = def.ID
		}
		if _, ok := def.Lists(classes.KindPsionics); ok && psi == "" {
			psi = def.ID
		}
	}
	return book, psi
}

// entrySet is a skill set a creation can enter points in (ADR 0040 step 3.5): the active class
// that lists it and its params.
type entrySet struct {
	def classes.Class
	rs  classes.RogueSkills
}

// entrySets are the skill sets the active classes list at any level, in the entries' order (one
// per rogue_skills feature); a set code listed twice keeps its first class.
func entrySets(active []classes.Class) []entrySet {
	var out []entrySet
	for _, def := range active {
		for _, f := range def.ListsAll(classes.KindRogueSkills) {
			if !slices.ContainsFunc(out, func(o entrySet) bool { return o.rs.Set == f.RogueSkills.Set }) {
				out = append(out, entrySet{def: def, rs: *f.RogueSkills})
			}
		}
	}
	return out
}

// skillsLevel is the level a class's skill points read before a character derives: the active
// entry's entered level, else the level its XP reaches (ADR 0030 step 3, step 7.5).
func skillsLevel(k checked, def classes.Class) int {
	for _, e := range k.ents {
		if e.Class != def.ID || e.frozen() {
			continue
		}
		if e.Level > 0 {
			return e.Level
		}
		return def.LevelFor(e.XP)
	}
	return 0
}

// draftSets are the draft's skill sets with the budget of each at its class's level (levelOf)
// on the entered points, and adds to out the creation's refusal of points over a set's budget
// (ADR 0037 step 10: nothing was spent before), named by its set.
func draftSets(out *Draft, sets []entrySet, levelOf func(classes.Class) int, all skills.SetPoints) []DraftSkillSet {
	var res []DraftSkillSet
	for _, s := range sets {
		level, points := levelOf(s.def), all.Of(s.rs.Set)
		if err := skills.SkillPointsWithinBudget(s.def, s.rs.Set, level, nil, points); err != nil {
			out.Refusals = append(out.Refusals, inSet(err, s.rs.Set))
		}
		ds := DraftSkillSet{Set: s.rs.Set, Name: s.rs.Name, Skills: slices.Clone(ruletablesvc.SkillsOf(&s.rs))}
		if rs := skills.RogueSkillsAtFor(s.def, s.rs.Set, level); rs != nil {
			if b, ok := ruletablesvc.SkillPointsFigures(rs, level, points); ok {
				ds.Budget = &b
			}
		}
		res = append(res, ds)
	}
	return res
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
