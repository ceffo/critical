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
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// CreatableClasses are the classes a new character may have (ADR 0030 step
// 1): the bard's and the fighter's rules are built (the fighter enters an
// exceptional strength, step 9), the mage's too (its spellbook starts empty
// and fills by learning, step 20). Each class item adds its class here with
// its own entries. The cleric enters nothing more: its slots, spell failure
// and turning follow its Wisdom and level (steps 13 to 16). The thief enters
// its skill points and the armour it wears for its skills (steps 1 and 17),
// and so does the bard (step 18).
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
	// skill points (steps 17 and 18).
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
	// SkillPoints are a thief's or a bard's (steps 1, 17 and 18): the points
	// by skill code (0 to MaxSkillPoints each, absent is 0). Empty for the
	// other classes. The armour the skills read is the worn one (ADR 0033
	// step 14): nothing is worn at creation.
	SkillPoints map[string]int
	// Details are the details entered with the character, checked as the
	// card's edit checks them (Details.Apply); the alignment and the sex are
	// required, the other nil fields stay unset.
	Details DetailsPatch
}

// Change is a rename (Name) or a retirement (Retired true) of a character;
// nil leaves a field as it is, and Retired false retires nothing (a retired
// character is not found, so none is brought back). The skill armour is no
// entry any more: it is the worn body armour (ADR 0033 step 14).
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
// character it enters. A draft (the preview) may leave the name, the
// alignment and the sex blank; every other refusal is the creation's.
func (s *service) validate(n NewCharacter, draft bool) (Character, error) {
	name, err := cleanName(n.Name)
	if err != nil && (!draft || strings.TrimSpace(n.Name) != "") {
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
	details, err := newDetails(Kind{Class: n.Class, Race: n.Race}, s.languages, n.Details, !draft)
	if err != nil {
		return Character{}, err
	}
	return Character{
		Name: name, Class: n.Class, Race: n.Race,
		Str: n.Str, StrExceptional: n.StrExceptional, Dex: n.Dex, Con: n.Con, Int: n.Int, Wis: n.Wis, Cha: n.Cha,
		HPMax: n.HPMax, XPTotal: n.XP, Details: details,
		SkillPoints: points,
	}, nil
}

// validSkills is ADR 0030 steps 1, 17 and 18: a thief or a bard enters its
// points, each a skill of SkillsOf its class from 0 to MaxSkillPoints; any
// other class enters none. The points come back without the zeros, nil for
// none. A refusal names the field (skillPoints or skillPoints.<skill>). The
// armour is no entry (ADR 0033 step 14).
func validSkills(n NewCharacter) (map[string]int, error) {
	if !ruletablesvc.HasSkillPoints(n.Class) {
		if len(n.SkillPoints) > 0 {
			return nil, &domain.FieldError{Field: "skillPoints", Message: CodeNotAllowed}
		}
		return nil, nil
	}
	return ValidSkillPoints(n.Class, n.SkillPoints)
}

// ValidSkillPoints is ADR 0030 steps 17 and 18's entry check on a thief's or
// a bard's points, at creation and when the player spends them later: each
// skill of SkillsOf the class from 0 to MaxSkillPoints. The budget is no
// refusal, only the derived warning (step 17 as built, step 18). The points
// come back without the zeros, nil for none. A class without skill points
// enters none: any
// point is refused with field skillPoints; another refusal names the field
// skillPoints.<skill>.
func ValidSkillPoints(class string, points map[string]int) (map[string]int, error) {
	if !ruletablesvc.HasSkillPoints(class) {
		if len(points) > 0 {
			return nil, &domain.FieldError{Field: "skillPoints", Message: CodeNotAllowed}
		}
		return nil, nil
	}
	var out map[string]int
	for _, skill := range slices.Sorted(maps.Keys(points)) {
		pts := points[skill]
		switch {
		case !slices.Contains(ruletablesvc.SkillsOf(class), skill):
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
// and stores them as the sheet's figures (a character created in the
// app states the rules' own figures), with its
// realized level and max HP at that level (step 7.5: no level-up is pending), then makes
// it current, in one transaction and outside the action log (creating a
// character is no move of the game and is not undone). This server forgets
// the owner's sessions once it committed (ADR 0029 step 4.7).
func (s *service) Create(ctx context.Context, owner uuid.UUID, n NewCharacter) (int64, error) {
	c, err := s.validate(n, false)
	if err != nil {
		return 0, err
	}
	if err := started(&c, s.rules); err != nil {
		return 0, err
	}
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

// started gives a new character the sheet and the realized level it is
// stored with: its sheet states the rules' own figures and, step 7.5, the max HP entered is the max
// HP at the target level, the level the character starts realized at.
func started(c *Character, rules Rules) error {
	d, err := Derive(c, 0, "", rules)
	if err != nil {
		return fmt.Errorf("new character: %w", err)
	}
	c.Sheet = Sheet{Level: d.TargetLevel, THAC0: d.THAC0, Saves: d.Saves}
	c.RealizedLevel = d.TargetLevel
	return nil
}

// Preview is the character Create would store from n, read as GET
// /api/character reads it once created (its derived block and its warnings),
// without storing anything: the creation's identity step and review show the
// server's figures before the player creates (ADR 0034). A blank name,
// alignment or sex is no refusal here (the caller lists them as missing);
// every other refusal is Create's, field for field.
func (s *service) Preview(_ context.Context, n NewCharacter) (*Character, error) {
	c, err := s.validate(n, true)
	if err != nil {
		return nil, err
	}
	if err := started(&c, s.rules); err != nil {
		return nil, err
	}
	// Nothing is worn before the character exists (ADR 0033 step 14).
	if c.Derived, err = Derive(&c, 0, "", s.rules); err != nil {
		return nil, fmt.Errorf("preview: %w", err)
	}
	s.readWarnings(&c)
	return &c, nil
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

// ArmorStateKey is the action-log state key of a thief's or a bard's skill
// armour as it was entered before ADR 0033 step 14: no move records it any
// more, but the entries already logged still pop and redo (they write the
// stored column back, which nothing reads).
const ArmorStateKey = "skill_armor"

// armorSnaps is the snapshotter of the skill armour (ADR 0018 step 8).
var armorSnaps = actionlog.NewSnapshotter[armorState](ArmorStateKey)

// armorState is the JSON encoding of a "skill_armor" snapshot (the entries'
// payload, {before, after}, is the web's to word). Entries already in the log
// must stay decodable: add fields, never rename.
type armorState struct {
	Armor string `json:"armor"`
}

// Module declares the codec that writes a thief's or a bard's skill armour
// back: undo writes the change's "before", redo its "after" (ADR 0019), inside
// the undo's transaction.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "charactersvc", Codecs: []actionlog.Codec{
		armorSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, st armorState) error {
			if err := s.owners.RestoreSkillArmor(ctx, characterID, st.Armor); err != nil {
				return fmt.Errorf("restore skill armor: %w", err)
			}
			return nil
		}, nil),
	}}
}

// ClassOption is a class a new character may have and the races it admits
// (DMG Table 7), in Races order.
type ClassOption struct {
	Class string
	Races []string
	// ExceptionalStrengthRaces are the races of Races that enter an 18/xx
	// at Strength 18 with the class (ADR 0030 step 9), empty for none.
	ExceptionalStrengthRaces []string
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

	MinAbility, MaxAbility int
	MinHPMax, MaxHPMax     int
	MinXP, MaxXP           int
	MinStrExceptional      int
	MaxStrExceptional      int
	MaxSkillPoints         int
	MaxNameLen             int
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
func raceIdentity(race string) (RaceIdentityOption, bool) {
	row, ok := ruletablesvc.IdentityFor(race)
	if !ok {
		return RaceIdentityOption{}, false
	}
	out := RaceIdentityOption{Race: race}
	for _, sex := range Sexes {
		h, _ := ruletablesvc.HeightRange(race, sex)
		w, _ := ruletablesvc.WeightRange(race, sex)
		out.Height = append(out.Height, FigureOption{Sex: sex, Count: row.HeightDice.Count, Sides: row.HeightDice.Sides, Range: h})
		out.Weight = append(out.Weight, FigureOption{Sex: sex, Count: row.WeightDice.Count, Sides: row.WeightDice.Sides, Range: w})
	}
	age, _ := ruletablesvc.AgeRange(race)
	out.Age = FigureOption{Count: row.StartAgeDice.Count, Sides: row.StartAgeDice.Sides, Range: age}
	out.StartAge, _ = ruletablesvc.StartAgeRange(race)
	return out, true
}

// RaceLanguageOption is a race's own language and its PHB list of languages.
type RaceLanguageOption struct {
	Race   string
	Native string
	List   []string
}

// CreationOptions reads the races of each creatable class from the rules.
func (s *service) CreationOptions(_ context.Context) (CreationOptions, error) {
	out := CreationOptions{
		MinAbility: MinAbility, MaxAbility: MaxAbility, MinHPMax: MinHPMax, MaxHPMax: MaxHPMax,
		MinXP: MinStartingXP, MaxXP: MaxStartingXP,
		MinStrExceptional: MinStrExceptional, MaxStrExceptional: MaxStrExceptional,
		MaxSkillPoints: MaxSkillPoints, MaxNameLen: MaxNameLen, Sexes: slices.Clone(Sexes), Classes: make([]ClassOption, 0, len(CreatableClasses)),
		ScoreMethods: slices.Clone(ruletablesvc.ScoreMethods), RaceAdjustments: make([]RaceAdjustmentOption, 0, len(Races)),
	}
	for _, race := range Races {
		adj, err := s.rules.RaceAdjustmentsFor(race)
		if err != nil {
			return CreationOptions{}, fmt.Errorf("creation options: %w", err)
		}
		out.RaceAdjustments = append(out.RaceAdjustments, RaceAdjustmentOption{Race: race, Adjustments: adj})
		if id, ok := raceIdentity(race); ok {
			out.RaceIdentity = append(out.RaceIdentity, id)
		}
	}
	for _, class := range CreatableClasses {
		group, err := ruletablesvc.ClassGroup(class)
		if err != nil {
			return CreationOptions{}, fmt.Errorf("creation options: %w", err)
		}
		hd, err := s.rules.HitDiceFor(group)
		if err != nil {
			return CreationOptions{}, fmt.Errorf("creation options: %w", err)
		}
		opt := ClassOption{
			Class: class, Alignments: ruletablesvc.AlignmentsOf(class), Races: []string{}, ExceptionalStrengthRaces: []string{}, Skills: []string{},
			HitDie: hd.Die, LevelXP: make([]LevelXP, 0, ruletablesvc.MaxLevel),
			Minimums: ruletablesvc.ClassMinimums(class),
		}
		for level := 1; level <= ruletablesvc.MaxLevel; level++ {
			xp, err := s.rules.XPThreshold(class, level)
			if err != nil {
				return CreationOptions{}, fmt.Errorf("creation options: %w", err)
			}
			opt.LevelXP = append(opt.LevelXP, LevelXP{Level: level, XP: xp})
		}
		if ruletablesvc.HasSkillPoints(class) {
			opt.Skills = slices.Clone(ruletablesvc.SkillsOf(class))
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
	out.Languages = s.languages.All()
	if len(out.Languages) > 0 {
		for _, race := range Races {
			out.RaceLanguages = append(out.RaceLanguages, RaceLanguageOption{
				Race: race, Native: s.languages.Native(race), List: s.languages.RaceList(race),
			})
		}
	}
	return out, nil
}

// DraftInput is a creation draft as far as it is entered: the scores typed
// or rolled so far (an ability absent has none yet), the starting XP when
// readable, and the skill points by skill.
type DraftInput struct {
	Class       string
	Scores      map[string]int
	XP          *int
	SkillPoints map[string]int
}

// SkillBudget is the skill point budget of a thief or a bard at the starting
// level with the terms of its formula (ADR 0030 steps 17 and 18).
type SkillBudget = ruletablesvc.SkillPointsBudget

// DraftChecks are the verdicts on a draft the creation steps show before the
// character can be previewed.
type DraftChecks struct {
	// UnmetMinimums are, for every creatable class, the abilities whose
	// entered score is under the class's minimum (PHB Table 13); empty
	// where the scores meet them or are not entered yet.
	UnmetMinimums []ClassUnmet
	// SkillBudget is the draft class's budget at the level its XP gives;
	// nil for a class without skill points or an XP that gives no level.
	SkillBudget *SkillBudget
}

// ClassUnmet is a class and the abilities under its minimum.
type ClassUnmet struct {
	Class     string
	Abilities []string
}

// CreationChecks reads a draft against the class minimums and the skill
// point budget (ADR 0034, the class and skills steps). Nothing is refused: a
// draft too raw to read (a class that is none of CreatableClasses, an XP
// that gives no level) just has no budget.
func (s *service) CreationChecks(_ context.Context, in DraftInput) (DraftChecks, error) {
	out := DraftChecks{UnmetMinimums: make([]ClassUnmet, 0, len(CreatableClasses))}
	for _, class := range CreatableClasses {
		out.UnmetMinimums = append(out.UnmetMinimums, ClassUnmet{Class: class, Abilities: ruletablesvc.UnmetMinimums(class, in.Scores)})
	}
	if in.XP == nil || *in.XP < MinStartingXP || *in.XP > MaxStartingXP || !slices.Contains(CreatableClasses, in.Class) || !ruletablesvc.HasSkillPoints(in.Class) {
		return out, nil
	}
	level, err := s.rules.LevelFor(in.Class, *in.XP)
	if err != nil {
		return out, nil //nolint:nilerr // an XP outside the table has no level, hence no budget; the creation refuses it
	}
	if b, ok := ruletablesvc.SkillPointsFigures(in.Class, level, in.SkillPoints); ok {
		out.SkillBudget = &b
	}
	return out, nil
}
