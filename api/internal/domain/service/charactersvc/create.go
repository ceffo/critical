package charactersvc

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// skillsAt is the skills module's verdict on a new character at its starting level (ADR 0037
// steps 6, 10, 14 and 18, skills.Apply with nothing picked before): its picks and its languages
// checked as the editor checks them, then the non-weapon pool. A refusal on the languages names
// the creation form's field, DetailsField + FieldLanguages. The picks are checked under the
// ruleset's weapon lift (ADR 0041 step 2.5).
func (s *service) skillsAt(c Character, n NewCharacter, lift ruleset.WeaponLift) (skills.Sheet, error) {
	before := skills.Sheet{
		Class: c.Class, Level: c.RealizedLevel, Intelligence: c.Int, Native: s.languages.Native(c.Race),
		MulticlassAnyWeapon: lift.MulticlassAnyWeapon,
	}
	if len(c.Classes) > 0 {
		// ADR 0042 step 15: the pools of every entry, a class left at its left-at level.
		before.Classes = c.StoredClasses()
	}
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

// ruleset is the ruleset a new character is created under: the resolver's for no character yet
// (the default until games exist, ADR 0041 step 9).
func (s *service) ruleset(ctx context.Context) (ruleset.Ruleset, error) {
	return s.rulesets.ForCharacter(ctx, 0)
}

// Create validates n, derives its level, THAC0 and saves at the starting XP
// and stores them as the sheet's figures (a character created in the
// app states the rules' own figures), with its
// realized level and max HP at that level (step 7.5: no level-up is pending), then makes
// it current, in one transaction and outside the action log (creating a
// character is no move of the game and is not undone). This server forgets
// the owner's sessions once it committed (ADR 0029 step 4.7).
func (s *service) Create(ctx context.Context, owner uuid.UUID, n NewCharacter) (int64, error) {
	k, err := s.check(ctx, n, nil, false)
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
	if c.PSPMax, err = startingPSPMax(&c, n.PSPMax, s.rules); err != nil {
		return 0, err
	}
	// ADR 0043 step 9: the known powers within Table 4 at the starting level, as the editor
	// checks them, written with the character (no psionic_pick entry: creating is no move).
	if c.PsionicPicks, err = s.startingPicks(c, n.PsionicPicks); err != nil {
		return 0, err
	}
	// ADR 0037 step 10: nothing was spent before, so a new character is
	// never created over its skill point budget.
	if err := skills.SkillPointsWithinBudget(k.def, skills.FirstSet(k.def), entryLevel(&c, k.def.ID), nil, c.SkillPoints.Of(skills.FirstSet(k.def))); err != nil {
		return 0, err
	}
	// ADR 0037 steps 6, 10, 14 and 18: nor with picks or languages the
	// editor refuses, nor over its non-weapon slots.
	rs, err := s.ruleset(ctx)
	if err != nil {
		return 0, fmt.Errorf("new character: %w", err)
	}
	sheet, err := s.skillsAt(c, n, rs.WeaponLift)
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
	d, err := Derive(c, nil, "", rules)
	if err != nil {
		return Derived{}, fmt.Errorf("new character: %w", err)
	}
	c.Sheet = Sheet{Level: d.TargetLevel, THAC0: d.THAC0, Saves: d.Saves}
	if len(c.Classes) == 0 {
		c.RealizedLevel = d.TargetLevel
		return d, nil
	}
	// ADR 0042 step 4: each active entry starts at the level entered for it, else at its target.
	entries := slices.Clone(c.Classes)
	for i := range entries {
		if entries[i].State == classes.EntryActive && entries[i].RealizedLevel == 0 {
			entries[i].RealizedLevel = d.Classes[i].TargetLevel
		}
	}
	c.SetClasses(entries)
	return d, nil
}

// FieldPSPMax is the creation form's field of the PSP maximum entered.
const FieldPSPMax = "pspMax"

// startingPSPMax is ADR 0043 step 4.4: the PSP maximum a new character is stored with, the one
// entered from the sheet when there is one, else step 4's maximum at its psionics entry's starting
// level (a multi-class character's from the psionicist entry alone, its gain not divided, Q2). A
// character with no psionics entry has none, and a maximum entered for it is refused. c is started
// (its entries at their starting levels).
func startingPSPMax(c *Character, entered *int, rules Rules) (int, error) {
	m, ok, err := PSPMaxOf(c, rules)
	switch {
	case err != nil:
		return 0, fmt.Errorf("new character: %w", err)
	case !ok && entered != nil:
		return 0, &domain.FieldError{Field: FieldPSPMax, Message: CodeOutOfRange}
	case !ok:
		return 0, nil
	case entered != nil:
		return *entered, nil
	}
	return m.Max, nil
}

// FieldPsionicPicks is the creation's field of the known powers (ADR 0043 step 9).
const FieldPsionicPicks = "psionicPicks"

// startingPicks is ADR 0043 step 9 at creation: the picks a new character is stored with, checked
// by psionics.Validate against Table 4 at its psionics entry's starting level, as the known-powers
// editor checks a save. It returns the list (nil for none), a *psionics.Refusal for a list over the
// rules, a field error on FieldPsionicPicks for a malformed one (the rule's own message, as the
// editor's) or for picks entered for a character with no psionics entry (CodeNotAllowed). c is
// started (its entries at their starting levels).
func (s *service) startingPicks(c Character, picks []psionics.Pick) ([]psionics.Pick, error) {
	if len(picks) == 0 {
		return nil, nil
	}
	c.PsionicPicks, c.Derived.Powers = nil, nil
	if err := s.setPowers(&c); err != nil {
		return nil, fmt.Errorf("new character: %w", err)
	}
	if c.Derived.Powers == nil {
		return nil, &domain.FieldError{Field: FieldPsionicPicks, Message: CodeNotAllowed}
	}
	if _, err := psionics.Validate(s.powers, c.Derived.Powers.Allowance, picks); err != nil {
		if errors.Is(err, psionics.ErrInvalid) {
			return nil, domain.FieldErrorf(FieldPsionicPicks, "%s", err.Error())
		}
		return nil, err
	}
	return slices.Clone(picks), nil
}

// entryLevel is the level of c's entry of class (the realized level, or the left-at level of a
// class left); c's realized level for a class it has none of.
func entryLevel(c *Character, class string) int {
	if e, ok := classes.Find(c.Entries(), class); ok {
		return e.Level()
	}
	return c.RealizedLevel
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
func (s *service) Draft(ctx context.Context, n NewCharacter, blank []string) (*Draft, error) {
	k, err := s.check(ctx, n, blank, true)
	if err != nil {
		return nil, err
	}
	out := &Draft{Refusals: k.refusals, Missing: k.missing, UnmetMinimums: unmetMinimums(k.defs, n, blank)}
	// ADR 0030 step 9 through the class's features (ADR 0040 step 3): the
	// form offers the 18/xx field when the server says so, never by a rule
	// of its own.
	out.ExceptionalStrength = k.exceptional
	out.Alignments = k.alignments
	out.SkillsClass, out.SpellbookClass, out.PsionicsClass = carriers(k.active)
	if !k.derivable {
		if k.levelled {
			// The starting level is the target level at the entered XP (ADR 0030 step 3, step 7.5).
			level, err := s.skillsLevel(k)
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
	out.SkillPoints = budgetAt(out, k.def, entryLevel(&c, k.def.ID), c.SkillPoints)
	rs, err := s.ruleset(ctx)
	if err != nil {
		return nil, fmt.Errorf("draft: %w", err)
	}
	if sheet, err := s.skillsAt(c, n, rs.WeaponLift); err != nil {
		out.Refusals = append(out.Refusals, err)
		// Refused picks are read as entered, so the step shows their figures beside the refusal.
		c.Proficiencies, c.Details.Languages = n.Proficiencies, n.Languages
	} else {
		c.Proficiencies, c.Details.Languages = sheet.Picks, sheet.Languages
	}
	// Nothing is worn before the character exists (ADR 0033 step 14).
	if c.Derived, err = Derive(&c, nil, "", s.rules); err != nil {
		return nil, fmt.Errorf("draft: %w", err)
	}
	// ADR 0043 step 9: the known powers against the starting level's allowance, read as entered
	// beside their refusal.
	if _, err := s.startingPicks(c, n.PsionicPicks); err != nil {
		out.Refusals = append(out.Refusals, err)
	}
	c.PsionicPicks = slices.Clone(n.PsionicPicks)
	if err := s.setPowers(&c); err != nil {
		return nil, fmt.Errorf("draft: %w", err)
	}
	if err := draftPSP(out, &c, n.PSPMax, s.rules); err != nil {
		return nil, err
	}
	s.readWarnings(&c, k.defs, rs.WeaponLift)
	// ADR 0030 step 20.7: the starting spellbook's limits and refusals.
	book, _, refusals := s.startingBook(&c, n.Spellbook)
	out.Spellbook = book
	out.Refusals = append(out.Refusals, refusals...)
	out.Character = &c
	return out, nil
}

// draftPSP is ADR 0043 step 4.4 on a draft: step 4's maximum at the psionics entry's starting
// level (out.ComputedPSPMax, nil for a character with no psionics entry) and the pool the
// character would be created with, full at the maximum Create stores (the one entered, else the
// computed one; no ledger exists yet). A maximum entered for a character with no psionics entry is
// Create's refusal on pspMax. c is started.
func draftPSP(out *Draft, c *Character, entered *int, rules Rules) error {
	m, ok, err := PSPMaxOf(c, rules)
	if err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	if entered != nil && (*entered < 1 || *entered > MaxPSPMax) {
		entered = nil // already refused out_of_range by check
	}
	stored, err := startingPSPMax(c, entered, rules)
	if fe, isField := errors.AsType[*domain.FieldError](err); isField {
		out.Refusals = append(out.Refusals, fe)
		return nil
	}
	if err != nil {
		return fmt.Errorf("draft: %w", err)
	}
	if ok {
		out.ComputedPSPMax = &m.Max
		c.Derived.PSP = &PSPPool{Current: stored, Max: stored}
	}
	return nil
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





