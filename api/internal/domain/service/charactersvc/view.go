package charactersvc

import (
	"context"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// View is the character read model the services depend on (ADR 0045): plain
// data with no transport tags, built in one place (buildView) from the stored
// character and its derived block, and read-only. It holds what a service
// reads and nothing else: no warnings, details, sheet or stored mirror (Class,
// RealizedLevel, XPTotal). The entries are reached through Primary, Entry,
// Active and SkillClasses only, so "which entry, which level" has one
// implementation.
type View struct {
	// Identity (ADR 0045 step 1.1).
	ID   int64
	Name string
	Race string
	// Languages are the languages the sheet knows, as the read gives them (the
	// skills tab reads them; the rest of Details stays with charactersvc).
	Languages []string

	// Abilities (step 1.2).
	Str            int
	StrExceptional int
	Dex            int
	Con            int
	Int            int
	Wis            int
	Cha            int
	HPMax          int
	PSPMax         int

	// Entries are the class entries in character order (step 1.3); a character
	// built from the mirror alone has its one entry.
	Entries []EntryView

	// The derived figures of the character (step 1.4), as Derived states them.
	THAC0            int
	Saves            Saves
	RaceSaveBonus    Saves
	MagicDefence     int
	ClassSaveBonuses []ClassSaveBonus
	SpellImmunities  []string
	SpellSlots       map[int]int
	Specializes      bool
	Spellbook        *SpellbookRules
	Turning          []ruletablesvc.TurningTarget
	SkillColumn      string
	SkillSets        []skills.SkillSet
	Backstab         int
	// Proficiencies are the derived non-weapon pool and the weapon figures
	// (Derived.Proficiencies); ProficiencyPicks are the picks themselves.
	Proficiencies *skills.ProficiencySlots
	PSP           *PSPPool
	Powers        *PowerKnowledge
	// Features are the grants of the feature kinds (Grant, ActiveGrant,
	// SkillGrants, FeatureLevel, FeatureClass read them).
	Features []FeatureGrant

	// Picks (step 1.5).
	SkillPoints      skills.SetPoints
	ProficiencyPicks skills.Picks
	PsionicPicks     []psionics.Pick
}

// EntryView is one class entry: the stored entry with its derived figures
// (ADR 0045 step 1.3).
type EntryView struct {
	Class       string
	State       classes.EntryState
	StartingXP  int
	LeftAtLevel int
	// Group is the class's group (ADR 0040 step 1.2).
	Group string
	// XP is the entry's starting XP plus its share of the ledger.
	XP int
	// TargetLevel is the class's experience table at XP; 0 for a frozen entry.
	TargetLevel int
	// RealizedLevel is the level taken in the class (a frozen entry's left-at
	// level).
	RealizedLevel int
	// NextLevelXP is the XP that reaches TargetLevel + 1; nil at the last level
	// and for a frozen entry.
	NextLevelXP  *int
	SpellSlots   map[int]int
	Casts        bool
	NoArmour     bool
	SpellFailure int
	LevelUp      LevelUp
	Dormant      bool
}

// Level is the level the entry's features and figures are read at (ADR 0042
// steps 12 and 14): its realized level, a frozen entry's left-at level.
func (e EntryView) Level() int {
	if e.State == classes.EntryFrozen {
		return e.LeftAtLevel
	}
	return e.RealizedLevel
}

// Frozen is a dual-class's entry for a class left (ADR 0042 step 13).
func (e EntryView) Frozen() bool { return e.State == classes.EntryFrozen }

// buildView is the one place that builds a View (ADR 0045 step 2.1): a
// projection of c and the d derived from it, never a second derivation. d's
// Classes are in the order of c.Entries().
func buildView(c Character, d Derived) View {
	stored := c.Entries()
	entries := make([]EntryView, len(d.Classes))
	for i, ed := range d.Classes {
		e := EntryView{
			Class: ed.Class, State: ed.State, Group: ed.Group, XP: ed.XP, TargetLevel: ed.TargetLevel,
			RealizedLevel: ed.RealizedLevel, NextLevelXP: ed.NextLevelXP, SpellSlots: ed.SpellSlots, Casts: ed.Casts,
			NoArmour: ed.NoArmour, SpellFailure: ed.SpellFailure, LevelUp: ed.LevelUp, Dormant: ed.Dormant,
		}
		if i < len(stored) {
			e.StartingXP, e.LeftAtLevel = stored[i].StartingXP, stored[i].LeftAtLevel
		}
		entries[i] = e
	}
	return View{
		ID: c.ID, Name: c.Name, Race: c.Race, Languages: c.Details.Languages,
		Str: c.Str, StrExceptional: c.StrExceptional, Dex: c.Dex, Con: c.Con, Int: c.Int, Wis: c.Wis, Cha: c.Cha,
		HPMax: c.HPMax, PSPMax: c.PSPMax,
		Entries: entries,
		THAC0:   d.THAC0, Saves: d.Saves, RaceSaveBonus: d.RaceSaveBonus, MagicDefence: d.MagicDefence,
		ClassSaveBonuses: d.ClassSaveBonuses, SpellImmunities: d.SpellImmunities, SpellSlots: d.SpellSlots,
		Specializes: d.Specializes, Spellbook: d.Spellbook, Turning: d.Turning, SkillColumn: d.SkillColumn,
		SkillSets: d.SkillSets, Backstab: d.Backstab, Proficiencies: d.Proficiencies, PSP: d.PSP, Powers: d.Powers,
		Features:    d.Features,
		SkillPoints: c.SkillPoints, ProficiencyPicks: c.Proficiencies, PsionicPicks: c.PsionicPicks,
	}
}

// View reads the character and projects it (ADR 0045 step 2.1): read, then
// buildView.
func (s *service) View(ctx context.Context, id int64) (View, error) {
	c, err := s.read(ctx, id)
	if err != nil {
		return View{}, err
	}
	return buildView(*c, c.Derived), nil
}

// Primary is the first active entry (ADR 0045 step 3.1). It cannot fail: a
// view is built from entries classes.ValidateEntries accepted or from the
// mirror's one entry. The first entry stands in for a view with none active.
func (v View) Primary() EntryView {
	if i := slices.IndexFunc(v.Entries, func(e EntryView) bool { return e.State == classes.EntryActive }); i >= 0 {
		return v.Entries[i]
	}
	if len(v.Entries) > 0 {
		return v.Entries[0]
	}
	return EntryView{}
}

// Entry is the entry of class, false when the character has none (step 3.2).
func (v View) Entry(class string) (EntryView, bool) {
	for _, e := range v.Entries {
		if e.Class == class {
			return e, true
		}
	}
	return EntryView{}, false
}

// Level is the level of the entry of class (a frozen entry's left-at level),
// false when the character has none (step 3.2).
func (v View) Level(class string) (int, bool) {
	e, ok := v.Entry(class)
	return e.Level(), ok
}

// Active are the active entries in order (step 3.3).
func (v View) Active() []EntryView {
	var out []EntryView
	for _, e := range v.Entries {
		if e.State == classes.EntryActive {
			out = append(out, e)
		}
	}
	return out
}

// SkillClasses are every entry as the weapon picks and the proficiency pools
// read them (step 3.4): each class at its realized level, a frozen one at its
// left-at level and Frozen (ADR 0042 steps 13 and 15).
func (v View) SkillClasses() skills.Classes {
	out := make(skills.Classes, 0, len(v.Entries))
	for _, e := range v.Entries {
		out = append(out, skills.ClassLevel{Class: e.Class, Level: e.Level(), Frozen: e.Frozen()})
	}
	return out
}

// HasDormant reports a dual-class before the crossover (ADR 0042 step 13).
func (v View) HasDormant() bool {
	return slices.ContainsFunc(v.Entries, func(e EntryView) bool { return e.Dormant })
}

// NonproficientPenalty is ADR 0037 step 8 for a weapon of weaponType, from the
// derived picks: none without them.
func (v View) NonproficientPenalty(weaponType string) int {
	return v.Proficiencies.NonproficientPenalty(weaponType)
}

// Score is the score in ability a; ok is false for an unknown key.
func (v View) Score(a Ability) (score int, ok bool) {
	switch a {
	case AbilityStr:
		return v.Str, true
	case AbilityDex:
		return v.Dex, true
	case AbilityCon:
		return v.Con, true
	case AbilityInt:
		return v.Int, true
	case AbilityWis:
		return v.Wis, true
	case AbilityCha:
		return v.Cha, true
	}
	return 0, false
}

// CasterLevel is ADR 0042 step 14: the realized level of the first caster
// entry (CasterEntry), the primary entry's when none casts.
func (v View) CasterLevel() int {
	if e, ok := v.CasterEntry(""); ok {
		return e.RealizedLevel
	}
	return v.Primary().RealizedLevel
}

// CasterEntry is the entry whose slots, preparation and caster level a spell
// uses (ADR 0042 step 14): the spellcasting entry of class, the first one when
// class is empty (an active one before a frozen one). False when there is none.
func (v View) CasterEntry(class string) (EntryView, bool) {
	for _, e := range v.casterEntries() {
		if class == "" || e.Class == class {
			return e, true
		}
	}
	return EntryView{}, false
}

// Casters is the number of spellcasting entries, a frozen one included.
func (v View) Casters() int { return len(v.casterEntries()) }

func (v View) casterEntries() []EntryView {
	var out []EntryView
	for _, state := range []classes.EntryState{classes.EntryActive, classes.EntryFrozen} {
		for _, e := range v.Entries {
			if e.State == state && e.Casts {
				out = append(out, e)
			}
		}
	}
	return out
}

// Groups are the class groups of the active entries, in order without repeats.
func (v View) Groups() []string {
	var out []string
	for _, e := range v.Active() {
		if e.Group != "" && !slices.Contains(out, e.Group) {
			out = append(out, e.Group)
		}
	}
	if len(out) == 0 {
		if g := v.Primary().Group; g != "" {
			out = []string{g}
		}
	}
	return out
}

// Grant is the first grant of kind (ADR 0042 step 12), false when no entry
// grants it.
func (v View) Grant(kind string) (FeatureGrant, bool) {
	for _, g := range v.Features {
		if g.Kind == kind {
			return g, true
		}
	}
	return FeatureGrant{}, false
}

// ActiveGrant is the first grant of kind that is not Dormant (ADR 0042 step 13).
func (v View) ActiveGrant(kind string) (FeatureGrant, bool) {
	for _, g := range v.Features {
		if g.Kind == kind && !g.Dormant {
			return g, true
		}
	}
	return FeatureGrant{}, false
}

// SkillGrants are the rogue_skills grants in entry order (ADR 0042 step 12);
// with none, the one of class at the primary entry's realized level.
func (v View) SkillGrants(class string) []FeatureGrant {
	var out []FeatureGrant
	for _, g := range v.Features {
		if g.Kind == classes.KindRogueSkills {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		out = []FeatureGrant{{Kind: classes.KindRogueSkills, Class: class, Level: v.Primary().RealizedLevel}}
	}
	return out
}

// FeatureLevel is the level the rule of kind reads (ADR 0042 step 12): the
// granting entry's, the primary entry's realized level when none grants it.
func (v View) FeatureLevel(kind string) int {
	if g, ok := v.Grant(kind); ok {
		return g.Level
	}
	return v.Primary().RealizedLevel
}

// FeatureClass is the class granting kind, fallback when none does.
func (v View) FeatureClass(kind, fallback string) string {
	if g, ok := v.Grant(kind); ok {
		return g.Class
	}
	return fallback
}

// SetSkillFigures replaces the view's skill sets with those of points (ADR
// 0037 step 10's preview): the figures Derived.SetSkillFigures gives the
// entity, from the view's grants, armour column, race and Dexterity.
func (v *View) SetSkillFigures(t skills.SkillTables, def func(string) (classes.Class, error), points skills.SetPoints) error {
	sets, err := skillSetsOf(t, def, v.SkillGrants(v.Primary().Class), v.SkillColumn, v.Race, v.Dex, len(v.Entries) > 1, points)
	if err != nil {
		return err
	}
	v.SkillSets = sets
	return nil
}

// StoredEntries are the entries as classes.Entry values (class, state,
// starting XP, realized and left-at levels): what the XP split and the
// level-from-XP helpers take, without a second read of the stored row.
func (v View) StoredEntries() []classes.Entry {
	out := make([]classes.Entry, len(v.Entries))
	for i, e := range v.Entries {
		out[i] = classes.Entry{
			Class: e.Class, State: e.State, StartingXP: e.StartingXP, RealizedLevel: e.RealizedLevel, LeftAtLevel: e.LeftAtLevel,
		}
	}
	return out
}
