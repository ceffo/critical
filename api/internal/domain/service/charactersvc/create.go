package charactersvc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// Every class the catalogue defines is creatable (ADR 0040 step 8): a class
// that passed the start-up check is fully supported, and the races are the
// catalogue's too.

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
	MaxSkillPoints = skills.MaxSkillPoints
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
	// Proficiencies are the picks (ADR 0037 step 14), checked as the
	// editor checks them with nothing spent before.
	Proficiencies skills.Picks
	// Details are the details entered with the character, checked as the
	// card's edit checks them (Details.Apply); the alignment and the sex are
	// required, the other nil fields stay unset.
	Details DetailsPatch
	// Languages are the languages entered with the character (nil: none),
	// checked as the Skills tab checks them (skills.ValidLanguages, ADR 0037
	// step 18); its native language is always added.
	Languages []string
	// Spellbook is a mage's or a bard's starting spellbook (ADR 0030 step
	// 20.7): the catalogue ids of the spells the DM gave, in order, written
	// without a roll; nil for none. Another class enters none.
	Spellbook []string
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

// checked is a new character's entries read field by field (check): the character they enter,
// the refusals in the form's order (Create answers the first), the fields still blank (a draft's;
// Create refuses them), whether the class and XP read (levelled: the starting level and the skill
// point budget need nothing else) and whether every entry the derivation reads does (derivable:
// the class, race, scores, skill points and XP). The picks and the languages are judged later, at
// the starting level (skillsAt).
type checked struct {
	c         Character
	refusals  []error
	missing   []string
	levelled  bool
	derivable bool
	// defs are the class and race definitions the check read.
	defs *classes.Registry
	// def is the entered class's definition (zero for a class missing or
	// unknown).
	def classes.Class
	// strOK: the Strength is entered and in range.
	strOK bool
}

// check reads n field by field, in the form's order. The creation (draft false) stops at the
// first refusal. A draft lists a blank name, class, race, alignment or sex and the numbers named
// in blank as missing instead of refusing them, and goes on past a refusal so that every field's
// verdict is read; every refusal it gives is the creation's, field and code alike.
func (s *service) check(n NewCharacter, blank []string, draft bool) (checked, error) {
	var out checked
	defs, err := s.rules.Definitions()
	if err != nil {
		return checked{}, fmt.Errorf("new character: %w", err)
	}
	out.defs = defs
	stop := func() bool { return !draft && len(out.refusals) > 0 }
	refuse := func(err error) { out.refusals = append(out.refusals, err) }
	empty := func(field string) {
		if draft {
			out.missing = append(out.missing, field)
		} else {
			refuse(&domain.FieldError{Field: field, Message: CodeEmpty})
		}
	}
	name, err := cleanName(n.Name)
	switch {
	case err == nil:
	case strings.TrimSpace(n.Name) == "":
		empty("name")
	default:
		refuse(err)
	}
	if stop() {
		return out, nil
	}
	class, classOK := defs.Class(n.Class)
	// ADR 0040 step 8: a class listing a kind the code does not compute yet
	// is not creatable.
	classOK = classOK && class.Implemented()
	out.def = class
	switch {
	case classOK:
	case draft && n.Class == "":
		empty("class")
	default:
		refuse(&domain.FieldError{Field: "class", Message: CodeNotCreatable})
	}
	if stop() {
		return out, nil
	}
	raceOK := false
	switch {
	case n.Race == "":
		empty("race")
	case !slices.Contains(defs.RaceIDs(), n.Race):
		refuse(&domain.FieldError{Field: "race", Message: CodeUnknown})
	case classOK:
		allowed, err := s.rules.RaceAllows(n.Race, n.Class)
		if err != nil {
			return checked{}, fmt.Errorf("new character: %w", err)
		}
		if !allowed {
			refuse(&domain.FieldError{Field: "race", Message: CodeRaceNotAllowed})
		}
		raceOK = allowed
	}
	if stop() {
		return out, nil
	}
	scoresOK := true
	for _, a := range []struct {
		field     string
		v, lo, hi int
	}{
		{string(AbilityStr), n.Str, MinAbility, MaxAbility},
		{string(AbilityDex), n.Dex, MinAbility, MaxAbility},
		{string(AbilityCon), n.Con, MinAbility, MaxAbility},
		{string(AbilityInt), n.Int, MinAbility, MaxAbility},
		{string(AbilityWis), n.Wis, MinAbility, MaxAbility},
		{string(AbilityCha), n.Cha, MinAbility, MaxAbility},
	} {
		switch {
		case slices.Contains(blank, a.field):
			out.missing, scoresOK = append(out.missing, a.field), false
		case a.v < a.lo || a.v > a.hi:
			refuse(&domain.FieldError{Field: a.field, Message: CodeOutOfRange})
			scoresOK = false
		case a.field == string(AbilityStr):
			out.strOK = true
		}
		if stop() {
			return out, nil
		}
	}
	if err := validExceptional(n, class, raceOf(defs, n.Race)); err != nil {
		refuse(err)
	}
	if stop() {
		return out, nil
	}
	points, err := validSkills(n, class)
	pointsOK := err == nil
	if err != nil {
		refuse(err)
	}
	if stop() {
		return out, nil
	}
	switch {
	case slices.Contains(blank, "hpMax"):
		out.missing = append(out.missing, "hpMax")
	case n.HPMax < MinHPMax || n.HPMax > MaxHPMax:
		refuse(&domain.FieldError{Field: "hpMax", Message: CodeOutOfRange})
	}
	if stop() {
		return out, nil
	}
	xpOK := true
	switch {
	case slices.Contains(blank, "xp"):
		out.missing, xpOK = append(out.missing, "xp"), false
	case n.XP < MinStartingXP || n.XP > MaxStartingXP:
		refuse(&domain.FieldError{Field: "xp", Message: CodeOutOfRange})
		xpOK = false
	}
	details, missing, err := newDetails(AlignmentsOf(defs, n.Class), n.Details, !draft)
	if err != nil {
		refuse(err)
	}
	out.missing = append(out.missing, missing...)
	out.levelled = classOK && xpOK
	out.derivable = out.levelled && raceOK && scoresOK && pointsOK
	out.c = Character{
		Name: name, Class: n.Class, Race: n.Race,
		Str: n.Str, StrExceptional: n.StrExceptional, Dex: n.Dex, Con: n.Con, Int: n.Int, Wis: n.Wis, Cha: n.Cha,
		HPMax: n.HPMax, XPTotal: n.XP, Details: details, SkillPoints: points,
	}
	return out, nil
}

// skillsAt is the skills module's verdict on a new character at its starting level (ADR 0037
// steps 6, 10, 14 and 18, skills.Apply with nothing picked before): its picks and its languages
// checked as the editor checks them, then the non-weapon pool. A refusal on the languages names
// the creation form's field, DetailsField + FieldLanguages.
func (s *service) skillsAt(c Character, n NewCharacter) (skills.Sheet, error) {
	before := skills.Sheet{Class: c.Class, Level: c.RealizedLevel, Intelligence: c.Int, Native: s.languages.Native(c.Race)}
	languages := n.Languages
	if languages == nil {
		languages = []string{}
	}
	after, err := skills.Apply(s.profs, s.languages, c.Race, before, skills.Edit{Picks: n.Proficiencies, Languages: languages})
	if fe, ok := errors.AsType[*domain.FieldError](err); ok && fe.Field == FieldLanguages {
		return skills.Sheet{}, &domain.FieldError{Field: DetailsField + FieldLanguages, Message: fe.Message}
	}
	if be, ok := errors.AsType[*domain.BudgetError](err); ok && be.Field == FieldLanguages {
		return skills.Sheet{}, &domain.BudgetError{Field: DetailsField + FieldLanguages, Spent: be.Spent, Limit: be.Limit}
	}
	return after, err
}

// validSkills is ADR 0030 steps 1, 17 and 18: a thief or a bard enters its
// points (skills.ValidSkillPoints); any other class enters none. The armour is
// no entry (ADR 0033 step 14).
func validSkills(n NewCharacter, def classes.Class) (map[string]int, error) {
	return skills.ValidSkillPoints(def, n.SkillPoints)
}

// validExceptional is ADR 0030 step 9: an exceptional strength is optional,
// 1 to 100, and only with Strength 18 for a class with exceptional_strength
// (ADR 0040 step 3) of a race that rolls it.
func validExceptional(n NewCharacter, def classes.Class, race classes.Race) error {
	switch {
	case n.StrExceptional == 0:
		return nil
	case n.StrExceptional < MinStrExceptional || n.StrExceptional > MaxStrExceptional:
		return &domain.FieldError{Field: "strExceptional", Message: CodeOutOfRange}
	case !ruletablesvc.ExceptionalStrengthAllowed(def, race, n.Str):
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
	k, err := s.check(n, nil, false)
	if err != nil {
		return 0, err
	}
	if len(k.refusals) > 0 {
		return 0, k.refusals[0]
	}
	c := k.c
	derived, err := started(&c, s.rules)
	if err != nil {
		return 0, err
	}
	// ADR 0037 step 10: nothing was spent before, so a new character is
	// never created over its skill point budget.
	if err := skills.SkillPointsWithinBudget(k.def, c.RealizedLevel, nil, c.SkillPoints); err != nil {
		return 0, err
	}
	// ADR 0037 steps 6, 10, 14 and 18: nor with picks or languages the
	// editor refuses, nor over its non-weapon slots.
	sheet, err := s.skillsAt(c, n)
	if err != nil {
		return 0, err
	}
	c.Proficiencies, c.Details.Languages = sheet.Picks, sheet.Languages
	// ADR 0030 step 20.7: the starting spellbook within the limits of step
	// 20.1 at the starting level, no die rolled.
	entries, err := s.startingSpellbook(c, derived, n.Spellbook)
	if err != nil {
		return 0, err
	}
	var id int64
	if err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		if id, err = s.owners.CreateCharacter(ctx, owner, c); err != nil {
			return err
		}
		for _, e := range entries {
			if err := s.book.AddEntry(ctx, id, e); err != nil {
				return fmt.Errorf("new character spellbook: %w", err)
			}
		}
		return s.owners.SetCurrentCharacter(ctx, owner, id)
	}); err != nil {
		return 0, err
	}
	s.sessions.ForgetUser(owner)
	return id, nil
}

// startingSpellbook is Create's step 20.7: the entries of the starting
// spellbook ids (origin starting), or its first refusal; d is c's derivation
// at its starting level.
func (s *service) startingSpellbook(c Character, d Derived, ids []string) ([]spellbook.Entry, error) {
	c.Derived = d
	_, kept, refusals := s.startingBook(&c, ids)
	if len(refusals) > 0 {
		return nil, refusals[0]
	}
	return startingEntries(kept), nil
}

// started gives a new character the sheet and the realized level it is
// stored with: its sheet states the rules' own figures and, step 7.5, the max HP entered is the max
// HP at the target level, the level the character starts realized at. It
// returns that derivation (before c's level was set: the realized level is the
// target one).
func started(c *Character, rules Rules) (Derived, error) {
	d, err := Derive(c, 0, "", rules)
	if err != nil {
		return Derived{}, fmt.Errorf("new character: %w", err)
	}
	c.Sheet = Sheet{Level: d.TargetLevel, THAC0: d.THAC0, Saves: d.Saves}
	c.RealizedLevel = d.TargetLevel
	return d, nil
}

// Draft is a creation draft as far as it is entered (ADR 0034, amendment of 2026-10-20: one
// draft endpoint): the character Create would store, read as GET /api/character reads it (its
// derived block, warnings and languages; nil while an entry the derivation reads, the class,
// race, scores, skill points or XP, is missing or refused), a thief's or a bard's skill point
// budget as soon as the class and XP read (ADR 0030 steps 17 and 18: the budget reads the class
// and level only, so a score out of range does not hide it), every refusal Create would give with
// its field and code, the fields still blank and the class minimums the entered scores miss. blank
// names the numbers not entered. Nothing is refused (a refusal is an answer here) and nothing is
// stored.
func (s *service) Draft(_ context.Context, n NewCharacter, blank []string) (*Draft, error) {
	k, err := s.check(n, blank, true)
	if err != nil {
		return nil, err
	}
	out := &Draft{Refusals: k.refusals, Missing: k.missing, UnmetMinimums: unmetMinimums(k.defs, n, blank)}
	// ADR 0030 step 9 through the class's features (ADR 0040 step 3): the
	// form offers the 18/xx field when the server says so, never by a rule
	// of its own.
	out.ExceptionalStrength = k.strOK && ruletablesvc.ExceptionalStrengthAllowed(k.def, raceOf(k.defs, n.Race), n.Str)
	if !k.derivable {
		if k.levelled {
			// The starting level is the target level at the entered XP (ADR 0030 step 3, step 7.5).
			level, err := s.rules.LevelFor(n.Class, n.XP)
			if err != nil {
				return nil, fmt.Errorf("draft: %w", err)
			}
			out.SkillPoints = budgetAt(out, k.def, level, k.c.SkillPoints)
		}
		return out, nil
	}
	c := k.c
	if _, err := started(&c, s.rules); err != nil {
		return nil, err
	}
	out.SkillPoints = budgetAt(out, k.def, c.RealizedLevel, c.SkillPoints)
	if sheet, err := s.skillsAt(c, n); err != nil {
		out.Refusals = append(out.Refusals, err)
		// Refused picks are read as entered, so the step shows their figures beside the refusal.
		c.Proficiencies, c.Details.Languages = n.Proficiencies, n.Languages
	} else {
		c.Proficiencies, c.Details.Languages = sheet.Picks, sheet.Languages
	}
	// Nothing is worn before the character exists (ADR 0033 step 14).
	if c.Derived, err = Derive(&c, 0, "", s.rules); err != nil {
		return nil, fmt.Errorf("draft: %w", err)
	}
	s.readWarnings(&c, k.defs)
	// ADR 0030 step 20.7: the starting spellbook's limits and refusals.
	book, _, refusals := s.startingBook(&c, n.Spellbook)
	out.Spellbook = book
	out.Refusals = append(out.Refusals, refusals...)
	out.Character = &c
	return out, nil
}

// budgetAt is a draft's skill point budget at level (ADR 0030 steps 17 and 18), nil for a class
// without skill points, and adds to out the creation's refusal of points over it (ADR 0037 step
// 10: nothing was spent before).
func budgetAt(out *Draft, def classes.Class, level int, points map[string]int) *ruletablesvc.SkillPointsBudget {
	if err := skills.SkillPointsWithinBudget(def, level, nil, points); err != nil {
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
			o.SpecializationSlots = ruletablesvc.SpecializationSlots(t)
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
func (s *service) CreationOptions(_ context.Context) (CreationOptions, error) {
	defs, err := s.rules.Definitions()
	if err != nil {
		return CreationOptions{}, fmt.Errorf("creation options: %w", err)
	}
	races := defs.RaceIDs()
	out := CreationOptions{
		MinAbility: MinAbility, MaxAbility: MaxAbility, MinHPMax: MinHPMax, MaxHPMax: MaxHPMax,
		MinXP: MinStartingXP, MaxXP: MaxStartingXP,
		MinStrExceptional: MinStrExceptional, MaxStrExceptional: MaxStrExceptional,
		MaxSkillPoints: MaxSkillPoints, MaxNameLen: MaxNameLen, Sexes: slices.Clone(Sexes), Classes: make([]ClassOption, 0, len(defs.Classes())),
		ScoreMethods: slices.Clone(ruletablesvc.ScoreMethods), RaceAdjustments: make([]RaceAdjustmentOption, 0, len(races)),
	}
	for _, race := range races {
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
}

// ClassUnmet is a class and the abilities under its minimum.
type ClassUnmet struct {
	Class     string
	Abilities []string
}

// raceOf is the race definition of id, the zero Race (no trait) for an id the
// catalogue does not define: a race refusal is another check's.
func raceOf(defs *classes.Registry, id string) classes.Race {
	r, _ := defs.Race(id)
	return r
}
