package charactersvc

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// CreatableClasses are the classes a new character may have (ADR 0030 step
// 1): the bard's and the fighter's rules are built (the fighter enters an
// exceptional strength, step 9), the mage's too (its spellbook starts empty
// and fills by learning, step 20). Each class item adds its class here with
// its own entries. The cleric enters nothing more: its slots, spell failure
// and turning follow its Wisdom and level (steps 13 to 16). The thief enters
// its skill points and the armour it wears for its skills (steps 1 and 17).
var CreatableClasses = []string{"bard", "fighter", "mage", "cleric", "thief"}

// The ranges a new character's entered values must fall in.
const (
	// MinAbility and MaxAbility bound each ability score as it stands on the
	// sheet, after the racial adjustments (ADR 0030 step 1): 3d6, one more
	// or less for a race's adjustment (PHB Table 8) the rolled 18 or 3 takes.
	MinAbility = 3
	MaxAbility = 19
	// MaxNameLen is the longest name, in characters.
	MaxNameLen = 40
	// MinHPMax and MaxHPMax bound the max HP entered.
	MinHPMax = 1
	MaxHPMax = 999
	// MinStartingXP and MaxStartingXP bound the starting XP entered (the
	// highest is more than any class's level 20 threshold).
	MinStartingXP = 0
	MaxStartingXP = 100_000_000
	// MinStrExceptional and MaxStrExceptional bound an exceptional strength
	// (ADR 0030 step 9): 18/01 to 18/00, entered as 1 to 100.
	MinStrExceptional = 1
	MaxStrExceptional = 100
	// MaxSkillPoints bounds a thief's points in one skill (ADR 0030 step 17):
	// more could never count, a skill being capped at 95%. The budget of
	// PHB p.39 is a warning (Derived.SkillBudget), not a bound.
	MaxSkillPoints = ruletablesvc.MaxSkillPercent
)

// Error codes carried by the creation's FieldError messages. The web owns
// the wording.
const (
	CodeEmpty      = "empty"
	CodeTooLong    = "too_long"
	CodeOutOfRange = "out_of_range"
	// CodeNotCreatable is a class no character may be created with yet.
	CodeNotCreatable = "not_creatable"
	// CodeUnknown is a race that is none of Races.
	CodeUnknown = "unknown"
	// CodeRaceNotAllowed is a race the class does not admit (DMG Table 7).
	CodeRaceNotAllowed = "race_not_allowed"
	// CodeNotAllowed is an exceptional strength for a character that has
	// none (ADR 0030 step 9: not a warrior, a Strength other than 18, or a
	// halfling), or skill points or a skill armour for a class without
	// thieving skills (step 17).
	CodeNotAllowed = "not_allowed"
)

// NewCharacter are the values entered for a new character (ADR 0030 step 1):
// never its level, THAC0, saves or slots, which the rules derive.
type NewCharacter struct {
	Name                         string
	Class                        string
	Race                         string
	Str, Dex, Con, Int, Wis, Cha int
	// StrExceptional is a fighter's 18/xx percentile (step 9), 0 for none.
	StrExceptional int
	HPMax          int
	XP             int
	// SkillPoints and SkillArmor are a thief's (steps 1 and 17): the points
	// by skill code (0 to MaxSkillPoints each, absent is 0) and one of
	// ruletablesvc.ThiefArmors. Empty for the other classes.
	SkillPoints map[string]int
	SkillArmor  string
}

// Change is a rename (Name) or a retirement (Retired true) of a character;
// nil leaves a field as it is, and Retired false retires nothing (a retired
// character is not found, so none is brought back).
type Change struct {
	Name    *string
	Retired *bool
}

// cleanName is the trimmed name, or the field error naming what is wrong.
func cleanName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	switch {
	case name == "":
		return "", &domain.FieldError{Field: "name", Message: CodeEmpty}
	case utf8.RuneCountInString(name) > MaxNameLen:
		return "", &domain.FieldError{Field: "name", Message: CodeTooLong}
	}
	return name, nil
}

// validate checks n field by field, in the form's order, and returns the
// character it enters.
func (s *service) validate(n NewCharacter) (Character, error) {
	name, err := cleanName(n.Name)
	if err != nil {
		return Character{}, err
	}
	if !slices.Contains(CreatableClasses, n.Class) {
		return Character{}, &domain.FieldError{Field: "class", Message: CodeNotCreatable}
	}
	switch {
	case n.Race == "":
		return Character{}, &domain.FieldError{Field: "race", Message: CodeEmpty}
	case !slices.Contains(Races, n.Race):
		return Character{}, &domain.FieldError{Field: "race", Message: CodeUnknown}
	}
	allowed, err := s.rules.RaceAllows(n.Race, n.Class)
	if err != nil {
		return Character{}, fmt.Errorf("new character: %w", err)
	}
	if !allowed {
		return Character{}, &domain.FieldError{Field: "race", Message: CodeRaceNotAllowed}
	}
	for _, a := range []struct {
		field string
		score int
	}{
		{string(AbilityStr), n.Str},
		{string(AbilityDex), n.Dex},
		{string(AbilityCon), n.Con},
		{string(AbilityInt), n.Int},
		{string(AbilityWis), n.Wis},
		{string(AbilityCha), n.Cha},
	} {
		if a.score < MinAbility || a.score > MaxAbility {
			return Character{}, &domain.FieldError{Field: a.field, Message: CodeOutOfRange}
		}
	}
	if err := validExceptional(n); err != nil {
		return Character{}, err
	}
	points, err := validSkills(n)
	if err != nil {
		return Character{}, err
	}
	if n.HPMax < MinHPMax || n.HPMax > MaxHPMax {
		return Character{}, &domain.FieldError{Field: "hpMax", Message: CodeOutOfRange}
	}
	if n.XP < MinStartingXP || n.XP > MaxStartingXP {
		return Character{}, &domain.FieldError{Field: "xp", Message: CodeOutOfRange}
	}
	return Character{
		Name: name, Class: n.Class, Race: n.Race,
		Str: n.Str, StrExceptional: n.StrExceptional, Dex: n.Dex, Con: n.Con, Int: n.Int, Wis: n.Wis, Cha: n.Cha,
		HPMax: n.HPMax, XPTotal: n.XP, Details: Details{Languages: []string{}},
		SkillPoints: points, SkillArmor: n.SkillArmor,
	}, nil
}

// validSkills is ADR 0030 steps 1 and 17: a thief enters its armour (one of
// ThiefArmors) and its points, each a skill of ThiefSkills from 0 to
// MaxSkillPoints; any other class enters neither. The points come back
// without the zeros, nil for none. A refusal names the field (skillArmor,
// or skillPoints.<skill>).
func validSkills(n NewCharacter) (map[string]int, error) {
	if !ruletablesvc.HasThiefSkills(n.Class) {
		switch {
		case n.SkillArmor != "":
			return nil, &domain.FieldError{Field: "skillArmor", Message: CodeNotAllowed}
		case len(n.SkillPoints) > 0:
			return nil, &domain.FieldError{Field: "skillPoints", Message: CodeNotAllowed}
		}
		return nil, nil
	}
	switch {
	case n.SkillArmor == "":
		return nil, &domain.FieldError{Field: "skillArmor", Message: CodeEmpty}
	case !slices.Contains(ruletablesvc.ThiefArmors, n.SkillArmor):
		return nil, &domain.FieldError{Field: "skillArmor", Message: CodeUnknown}
	}
	var out map[string]int
	for _, skill := range slices.Sorted(maps.Keys(n.SkillPoints)) {
		pts := n.SkillPoints[skill]
		switch {
		case !slices.Contains(ruletablesvc.ThiefSkills, skill):
			return nil, &domain.FieldError{Field: "skillPoints." + skill, Message: CodeUnknown}
		case pts < 0 || pts > MaxSkillPoints:
			return nil, &domain.FieldError{Field: "skillPoints." + skill, Message: CodeOutOfRange}
		case pts == 0:
			continue
		}
		if out == nil {
			out = map[string]int{}
		}
		out[skill] = pts
	}
	return out, nil
}

// validExceptional is ADR 0030 step 9: an exceptional strength is optional,
// 1 to 100, and only a warrior's with Strength 18 who is not a halfling.
func validExceptional(n NewCharacter) error {
	switch {
	case n.StrExceptional == 0:
		return nil
	case n.StrExceptional < MinStrExceptional || n.StrExceptional > MaxStrExceptional:
		return &domain.FieldError{Field: "strExceptional", Message: CodeOutOfRange}
	case !ruletablesvc.ExceptionalStrengthAllowed(n.Class, n.Race, n.Str):
		return &domain.FieldError{Field: "strExceptional", Message: CodeNotAllowed}
	}
	return nil
}

// Create validates n, derives its level, THAC0 and saves at the starting XP
// and stores them as the sheet's figures (ADR 0030 step 24: a character
// created in the app states no sheet, so it shows no difference), with its
// realized level and max HP at that level (step 7.5: no level-up is pending), then makes
// it current, in one transaction and outside the action log (creating a
// character is no move of the game and is not undone). This server forgets
// the owner's sessions once it committed (ADR 0029 step 4.7).
func (s *service) Create(ctx context.Context, owner uuid.UUID, n NewCharacter) (int64, error) {
	c, err := s.validate(n)
	if err != nil {
		return 0, err
	}
	d, err := Derive(&c, 0, s.rules)
	if err != nil {
		return 0, fmt.Errorf("new character: %w", err)
	}
	c.Sheet = Sheet{Level: d.TargetLevel, THAC0: d.THAC0, Saves: d.Saves}
	// Step 7.5: the max HP entered is the max HP at that level, the level
	// the character starts realized at.
	c.RealizedLevel = d.TargetLevel
	var id int64
	if err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if id, err = s.owners.CreateCharacter(ctx, owner, c); err != nil {
			return err
		}
		return s.owners.SetCurrentCharacter(ctx, owner, id)
	}); err != nil {
		return 0, err
	}
	s.sessions.ForgetUser(owner)
	return id, nil
}

// Change renames and retires in one transaction, outside the action log
// (managing one's characters is no move of the game). Retiring picks the
// owner's next current character in the same transaction (ADR 0029 step 7.2,
// item 6 of its 2026-10-18 amendment).
func (s *service) Change(ctx context.Context, owner uuid.UUID, id int64, c Change) (int64, error) {
	var name string
	if c.Name != nil {
		var err error
		if name, err = cleanName(*c.Name); err != nil {
			return 0, err
		}
	}
	retire := c.Retired != nil && *c.Retired
	var current int64
	if err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if c.Name != nil {
			if err := s.owners.RenameCharacter(ctx, owner, id, name); err != nil {
				return err
			}
		}
		if retire {
			if err := s.owners.RetireCharacter(ctx, owner, id); err != nil {
				return err
			}
			if err := s.owners.RepairCurrentCharacter(ctx, owner); err != nil {
				return err
			}
		}
		var err error
		current, err = s.owners.CurrentCharacter(ctx, owner)
		return err
	}); err != nil {
		return 0, err
	}
	if retire {
		s.sessions.ForgetUser(owner)
	}
	return current, nil
}

// ClassOption is a class a new character may have and the races it admits
// (DMG Table 7), in Races order.
type ClassOption struct {
	Class string
	Races []string
	// ExceptionalStrengthRaces are the races of Races that enter an 18/xx
	// at Strength 18 with the class (ADR 0030 step 9), empty for none.
	ExceptionalStrengthRaces []string
	// Skills are the skills the class enters points in (step 17, in
	// ruletablesvc.ThiefSkills order) and SkillArmors the armours it picks
	// for them; both empty for a class without thieving skills.
	Skills      []string
	SkillArmors []string
}

// CreationOptions are what the creation form offers: the creatable classes
// with their races, and the ranges the entered values must fall in (the
// same ones Create checks).
type CreationOptions struct {
	Classes                []ClassOption
	MinAbility, MaxAbility int
	MinHPMax, MaxHPMax     int
	MinXP, MaxXP           int
	MinStrExceptional      int
	MaxStrExceptional      int
	MaxSkillPoints         int
	MaxNameLen             int
}

// CreationOptions reads the races of each creatable class from the rules.
func (s *service) CreationOptions(_ context.Context) (CreationOptions, error) {
	out := CreationOptions{
		MinAbility: MinAbility, MaxAbility: MaxAbility, MinHPMax: MinHPMax, MaxHPMax: MaxHPMax,
		MinXP: MinStartingXP, MaxXP: MaxStartingXP,
		MinStrExceptional: MinStrExceptional, MaxStrExceptional: MaxStrExceptional,
		MaxSkillPoints: MaxSkillPoints, MaxNameLen: MaxNameLen, Classes: make([]ClassOption, 0, len(CreatableClasses)),
	}
	for _, class := range CreatableClasses {
		if _, err := ruletablesvc.ClassGroup(class); err != nil {
			return CreationOptions{}, fmt.Errorf("creation options: %w", err)
		}
		opt := ClassOption{Class: class, Races: []string{}, ExceptionalStrengthRaces: []string{}, Skills: []string{}, SkillArmors: []string{}}
		if ruletablesvc.HasThiefSkills(class) {
			opt.Skills = slices.Clone(ruletablesvc.ThiefSkills)
			opt.SkillArmors = slices.Clone(ruletablesvc.ThiefArmors)
		}
		for _, race := range Races {
			ok, err := s.rules.RaceAllows(race, class)
			if err != nil {
				return CreationOptions{}, fmt.Errorf("creation options: %w", err)
			}
			if ok {
				opt.Races = append(opt.Races, race)
				if ruletablesvc.ExceptionalStrengthAllowed(class, race, 18) {
					opt.ExceptionalStrengthRaces = append(opt.ExceptionalStrengthRaces, race)
				}
			}
		}
		out.Classes = append(out.Classes, opt)
	}
	return out, nil
}
