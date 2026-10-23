package charactersvc

import (
	"context"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// ClassOption is a class a new character may have and the races it admits
// (DMG Table 7), in Races order.
type ClassOption struct {
	Class string
	Races []string
	// Skills are the skills the class enters points in (steps 17 and 18, in
	// ruletablesvc.SkillsOf order); empty for a class without skill
	// points.
	Skills []string
	// Alignments are the alignments the class admits (ADR 0032 step 2), in
	// ruletablesvc.Alignments order.
	Alignments []string
	// HitDie is the class group's hit die (ADR 0030 step 7), the sides of
	// the die the creation's class card shows.
	HitDie int
	// LevelXP is ADR 0034 step 4.1: for levels 1 to MaxLevel, the lowest XP
	// of the level in the class's table (ADR 0030 step 3). A level the
	// player picks is sent as that XP; the web computes no threshold.
	LevelXP []LevelXP
	// Minimums are the class's ability minimums (PHB Table 13), in ability
	// order; empty for none. Shown on the class card, not enforced (ADR 0030
	// step 21's stance on requirements).
	Minimums []ruletablesvc.AbilityMinimum
	// WeaponTypes are the weapon types the class may pick a proficiency in
	// (ADR 0037 step 7), in ruletablesvc.WeaponTypes order, each with what
	// its specialization spends in all (step 9), 0 for a class that does not
	// specialize: the weapon editor shows and sends the server's cost.
	WeaponTypes []WeaponTypeOption
	// Spellbook is the class's starting spellbook (ADR 0030 step 20.7), nil
	// for a class without a spellbook.
	Spellbook *SpellbookOption
}

// SpellbookOption is a class's starting spellbook in the creation options
// (ADR 0030 step 20.7): the catalogue spells it is offered with, pre-ticked
// and removable (the starting spellbook seed's, SpellCatalogue.StartingSpells;
// none for a bard).
type SpellbookOption struct {
	StartingSpells []string
}

// WeaponTypeOption is one weapon type a class may pick (ADR 0037 step 7) and
// the slots its specialization spends (step 9), 0 when the class does not
// specialize.
type WeaponTypeOption struct {
	Type                string
	SpecializationSlots int
}

// ClassWeaponTypeOptions is ADR 0037 steps 7 and 9 for the editor: every weapon
// type class def may pick, in display order, with its specialization's cost when the
// class specializes (ruletablesvc.Specializes), else 0.
func ClassWeaponTypeOptions(def classes.Class) []WeaponTypeOption {
	out := make([]WeaponTypeOption, 0, len(ruletablesvc.WeaponTypes))
	for _, t := range ruletablesvc.WeaponTypes {
		if !def.Equipment.WeaponTypeAllowed(t) {
			continue
		}
		o := WeaponTypeOption{Type: t}
		if ruletablesvc.Specializes(def) {
			o.SpecializationSlots = ruletablesvc.SpecializationSlots(def, t)
		}
		out = append(out, o)
	}
	return out
}

// LevelXP is a level and the XP that reaches it.
type LevelXP struct {
	Level, XP int
}

// RaceAdjustmentOption is a race and PHB Table 8's adjustment of each
// ability (0 for none), which the creation's race cards show.
type RaceAdjustmentOption struct {
	Race        string
	Adjustments map[string]int
}

// CreationOptions are what the creation form offers: the creatable classes
// with their races, and the ranges the entered values must fall in (the
// same ones Create checks).
type CreationOptions struct {
	Classes []ClassOption
	// Sexes are the sex codes (ADR 0032 step 3).
	Sexes []string
	// Languages are the catalogue languages with their names, RaceLanguages
	// each race's own language and PHB list (ADR 0032 step 4.3: the form
	// offers the whole catalogue, the race's list marked). Both are empty
	// before catalogue format 8. The Table 4 count stays in Go: the server
	// warns (step 4.2), the form computes nothing.
	Languages     []ruletablesvc.Language
	RaceLanguages []RaceLanguageOption
	// Proficiencies is the non-weapon catalogue (PHB Table 37) and
	// ProficiencyGroups the groups each class learns at the table's cost
	// (Table 38), by class (ADR 0037 steps 1 and 2). The names are the web's
	// i18n keys by id. Both are empty without the seed.
	Proficiencies     []ruletablesvc.Proficiency
	ProficiencyGroups map[string][]string
	// ProficiencyCosts is each pickable catalogue proficiency's cost by class
	// (ADR 0037 step 4: Table 37's slots, one more outside the class's Table 38
	// groups), so the editor shows and sends the server's cost. A proficiency
	// never picked on its own (skills.LanguagesModernID, step 6) has no cost: the
	// editor offers only what has one. Empty without the seed.
	ProficiencyCosts map[string]map[string]int
	// ScoreMethods are ADR 0032 step 6's methods, in the book's order: the
	// creation's score step offers them and rolls one through the roll
	// endpoint.
	ScoreMethods []ruletablesvc.ScoreMethod
	// RaceAdjustments are PHB Table 8 for every race, in Races order.
	RaceAdjustments []RaceAdjustmentOption
	// RaceIdentity are ADR 0032 step 5's dice and ranges for every race, in
	// Races order: the identity step shows them beside each roll button and
	// the race cards the height and starting age (ADR 0034).
	RaceIdentity []RaceIdentityOption
	// WizardSpells are the catalogue's wizard spells, by spell level then id:
	// the starting spellbook's choices (ADR 0030 step 20.7; the limits are
	// the draft's, by the starting level and the Intelligence).
	WizardSpells []SpellOption

	MinAbility, MaxAbility int
	MinHPMax, MaxHPMax     int
	MinXP, MaxXP           int
	MinStrExceptional      int
	MaxStrExceptional      int
	MaxSkillPoints         int
	MaxNameLen             int
	// MaxClasses is the most active classes of a multi-class character and MinLeftAtLevel the lowest
	// level a class can be left at (ADR 0042 step 1).
	MaxClasses, MinLeftAtLevel int
	// MulticlassAnyRace and DualclassAnyRace are the ruleset's racial lift (ADR 0041 step 2.2, ADR
	// 0042 step 2): any race may be multi-class or dual-class, so the form offers every race for such a
	// character instead of the class's own. RestrictedRaces are the races with ability requirements,
	// the only multi-class ones with the lift off.
	MulticlassAnyRace, DualclassAnyRace bool
	RestrictedRaces                     []string
	// NoMultiClass are the classes never part of a multi-class character (ADR 0042 step 2.1, the
	// catalogue's `multiclass: false`), in catalogue order: the form leaves them out of a second or
	// third class.
	NoMultiClass []string
	// DualClassOldPrime and DualClassNewPrime are the prime requisite scores of a dual-class
	// character in the class it left and in its new class (step 2.2).
	DualClassOldPrime, DualClassNewPrime int
}

// FigureOption is one identity figure's roll dice (Count dice of Sides) and
// its range in the stored unit (cm, kg or years), outside which a value warns
// (ADR 0032 step 5.1). Sex is the sex of a height or weight row.
type FigureOption struct {
	Sex          string
	Count, Sides int
	Range        ruletablesvc.Range
}

// RaceIdentityOption is a race's height and weight per sex (male, then
// female), its age, and the starting ages a roll gives (within Age.Range).
type RaceIdentityOption struct {
	Race           string
	Height, Weight []FigureOption
	Age            FigureOption
	StartAge       ruletablesvc.Range
}

// raceIdentity is the race's RaceIdentityOption, false for a race with no
// Table 10 row.
func raceIdentity(defs *classes.Registry, race string) (RaceIdentityOption, bool) {
	row, ok := ruletablesvc.IdentityFor(defs, race)
	if !ok {
		return RaceIdentityOption{}, false
	}
	out := RaceIdentityOption{Race: race}
	for _, sex := range Sexes {
		h, w := row.HeightRange(sex), row.WeightRange(sex)
		out.Height = append(out.Height, FigureOption{Sex: sex, Count: row.HeightDice.Count, Sides: row.HeightDice.Sides, Range: h})
		out.Weight = append(out.Weight, FigureOption{Sex: sex, Count: row.WeightDice.Count, Sides: row.WeightDice.Sides, Range: w})
	}
	age := row.AgeRange()
	out.Age = FigureOption{Count: row.StartAgeDice.Count, Sides: row.StartAgeDice.Sides, Range: age}
	out.StartAge = row.StartAgeRange()
	return out, true
}

// RaceLanguageOption is a race's own language and its PHB list of languages.
type RaceLanguageOption struct {
	Race   string
	Native string
	List   []string
}

// CreationOptions reads the races of each creatable class from the rules.
func (s *service) CreationOptions(ctx context.Context) (CreationOptions, error) {
	defs, err := s.rules.Definitions()
	if err != nil {
		return CreationOptions{}, fmt.Errorf("creation options: %w", err)
	}
	races := defs.RaceIDs()
	rs, err := s.ruleset(ctx)
	if err != nil {
		return CreationOptions{}, fmt.Errorf("creation options: %w", err)
	}
	out := CreationOptions{
		MaxClasses: classes.MaxMultiClasses, MinLeftAtLevel: classes.MinLeftAtLevel,
		MulticlassAnyRace: rs.RacialLift.MulticlassAnyRace, DualclassAnyRace: rs.RacialLift.DualclassAnyRace,
		RestrictedRaces: []string{}, NoMultiClass: []string{}, DualClassOldPrime: DualClassOldPrime, DualClassNewPrime: DualClassNewPrime,
		MinAbility: MinAbility, MaxAbility: MaxAbility, MinHPMax: MinHPMax, MaxHPMax: MaxHPMax,
		MinXP: MinStartingXP, MaxXP: MaxStartingXP,
		MinStrExceptional: MinStrExceptional, MaxStrExceptional: MaxStrExceptional,
		MaxSkillPoints: MaxSkillPoints, MaxNameLen: MaxNameLen, Sexes: slices.Clone(Sexes), Classes: make([]ClassOption, 0, len(defs.Classes())),
		ScoreMethods: slices.Clone(ruletablesvc.ScoreMethods), RaceAdjustments: make([]RaceAdjustmentOption, 0, len(races)),
	}
	for _, def := range defs.Classes() {
		if def.NoMultiClass {
			out.NoMultiClass = append(out.NoMultiClass, def.ID)
		}
	}
	for _, race := range races {
		if raceOf(defs, race).Restricted() {
			out.RestrictedRaces = append(out.RestrictedRaces, race)
		}
		adj, err := s.rules.RaceAdjustmentsFor(race)
		if err != nil {
			return CreationOptions{}, fmt.Errorf("creation options: %w", err)
		}
		out.RaceAdjustments = append(out.RaceAdjustments, RaceAdjustmentOption{Race: race, Adjustments: adj})
		if id, ok := raceIdentity(defs, race); ok {
			out.RaceIdentity = append(out.RaceIdentity, id)
		}
	}
	for _, def := range defs.Classes() {
		if !def.Implemented() {
			continue
		}
		class := def.ID
		hd, err := s.rules.HitDiceFor(def.Group)
		if err != nil {
			return CreationOptions{}, fmt.Errorf("creation options: %w", err)
		}
		opt := ClassOption{
			Class: class, Alignments: slices.Clone(def.Alignments), Races: []string{}, Skills: []string{},
			HitDie: hd.Die, LevelXP: make([]LevelXP, 0, ruletablesvc.MaxLevel),
			Minimums: slices.Clone(def.AbilityMinimums), WeaponTypes: ClassWeaponTypeOptions(def),
		}
		for level := 1; level <= ruletablesvc.MaxLevel; level++ {
			xp, err := s.rules.XPThreshold(class, level)
			if err != nil {
				return CreationOptions{}, fmt.Errorf("creation options: %w", err)
			}
			opt.LevelXP = append(opt.LevelXP, LevelXP{Level: level, XP: xp})
		}
		if f, ok := def.Lists(classes.KindRogueSkills); ok {
			opt.Skills = slices.Clone(ruletablesvc.SkillsOf(f.RogueSkills))
		}
		if _, ok := def.Lists(classes.KindSpellbook); ok {
			opt.Spellbook = s.spellbookOption(class)
		}
		for _, race := range races {
			ok, err := s.rules.RaceAllows(race, class)
			if err != nil {
				return CreationOptions{}, fmt.Errorf("creation options: %w", err)
			}
			if ok {
				opt.Races = append(opt.Races, race)
			}
		}
		out.Classes = append(out.Classes, opt)
	}
	out.WizardSpells = s.spells.WizardSpells()
	out.Proficiencies = s.profs.All()
	for _, c := range s.profs.Crossovers() {
		if out.ProficiencyGroups == nil {
			out.ProficiencyGroups = map[string][]string{}
		}
		out.ProficiencyGroups[c.Class] = c.Groups
		costs := make(map[string]int, len(out.Proficiencies))
		for _, p := range out.Proficiencies {
			if p.ID == skills.LanguagesModernID {
				continue // step 6: a language is a pick of the language list
			}
			costs[p.ID] = ruletablesvc.NonweaponCost(p, c.Groups)
		}
		if out.ProficiencyCosts == nil {
			out.ProficiencyCosts = map[string]map[string]int{}
		}
		out.ProficiencyCosts[c.Class] = costs
	}
	out.Languages = s.languages.All()
	if len(out.Languages) > 0 {
		for _, race := range races {
			out.RaceLanguages = append(out.RaceLanguages, RaceLanguageOption{
				Race: race, Native: s.languages.Native(race), List: s.languages.RaceList(race),
			})
		}
	}
	return out, nil
}

// spellbookOption is class's starting spellbook option: its pre-ticked
// spells, read from the data (the seed is checked against the catalogue at
// start-up, so each is a wizard spell Create takes).
func (s *service) spellbookOption(class string) *SpellbookOption {
	return &SpellbookOption{StartingSpells: append([]string{}, s.spells.StartingSpells(class)...)}
}
