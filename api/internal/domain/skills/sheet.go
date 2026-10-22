package skills

// Sheet is what the proficiency pools read of a character (ADR 0037 steps 3 to 6): its class,
// realized level, Intelligence as stored (no effect applied), picks and languages (as read:
// known codes, the native one among them), its race's native language ("" without a catalogue)
// and its score in an ability (ok false when not known: a creation draft).
type Sheet struct {
	Class string
	Level int
	// Classes are a multi-class character's active entries (ADR 0042 step 15); nil reads the
	// one entry Class and Level state.
	Classes      Classes
	Intelligence int
	Picks        Picks
	Languages    []string
	Native       string
	Score        func(ability string) (int, bool)
	// MulticlassAnyWeapon is the ruleset's weapon lift (ADR 0041 step 2.5): on, a multi-class
	// character's blunt_only entry no longer restricts the weapon picks to its own (ADR 0042 step
	// 16.1). The specialization stays refused either way.
	MulticlassAnyWeapon bool
}

// classes are the entries s reads: Classes, else Class at Level.
func (s Sheet) classes() Classes {
	if len(s.Classes) > 0 {
		return s.Classes
	}
	return One(s.Class, s.Level)
}

// Spending is what s spends from its pools: its picks and the languages beyond the native one
// and Common (step 6).
func (s Sheet) Spending() Spending {
	return Spending{Picks: s.Picks, Languages: LanguageSlots(s.Languages, s.Native)}
}

// Read is ADR 0037 steps 3 to 6 and 11 on s, the one place the sheet's proficiency figures are
// assembled (the character's read, the editor's preview and save): both pools at the realized
// level, the languages spending a non-weapon slot each (they replace ADR 0032 step 4.2's
// languages_over_limit: Table 4's count is the pool's Intelligence term, counted once), each
// pick's figures and check target, and the slot warnings. nil and no warning without a Table 34
// row for the class.
func Read(book Book, s Sheet) (*ProficiencySlots, []Warning) {
	f, ok := figuresOf(book, s.classes(), s.MulticlassAnyWeapon, s.Intelligence, s.Spending())
	if !ok {
		return nil, nil
	}
	if s.Score != nil {
		f.SetTargets(s.Score)
	}
	return &f, SlotWarnings(f)
}

// Edit is a change of a sheet's picks and languages (ADR 0037 steps 12, 14 and 18): a nil pool of
// Picks is left as it was (ValidPicks), and nil Languages leave the languages as they were.
type Edit struct {
	Picks     Picks
	Languages []string
}

// Apply is ADR 0037 steps 6, 10, 12, 14 and 18 on before, the one place an edit of the pools is
// judged (the editor's preview and save, and the creation, where nothing was picked before): the
// picks checked by ValidPicks against the sheet's, the languages by ValidLanguages (race's native
// one added), then the budget at the sheet's level, each pool on its own and the languages in the
// non-weapon one. It returns the sheet after the edit, or a refusal: a *domain.FieldError or a
// *domain.BudgetError. Nothing is written.
func Apply(book Book, languages LanguageBook, race string, before Sheet, e Edit) (Sheet, error) {
	picks, err := validPicksOf(book, before.classes(), before.MulticlassAnyWeapon, e.Picks, before.Picks)
	if err != nil {
		return Sheet{}, err
	}
	after := before
	after.Picks = picks
	if e.Languages != nil {
		if after.Languages, err = ValidLanguages(languages, race, before.Languages, e.Languages); err != nil {
			return Sheet{}, err
		}
	}
	if err := picksWithinBudgetOf(book, before.classes(), before.Intelligence, before.Spending(), after.Spending()); err != nil {
		return Sheet{}, err
	}
	return after, nil
}
