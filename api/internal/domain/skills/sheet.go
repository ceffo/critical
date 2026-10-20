package skills

// Sheet is what the proficiency pools read of a character (ADR 0037 steps 3 to 6): its class,
// realized level, Intelligence as stored (no effect applied), picks and languages (as read:
// known codes, the native one among them), its race's native language ("" without a catalogue)
// and its score in an ability (ok false when not known: a creation draft).
type Sheet struct {
	Class        string
	Level        int
	Intelligence int
	Picks        Picks
	Languages    []string
	Native       string
	Score        func(ability string) (int, bool)
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
	f, ok := ProficiencyFigures(book, s.Class, s.Level, s.Intelligence, s.Spending())
	if !ok {
		return nil, nil
	}
	if s.Score != nil {
		f.SetTargets(s.Score)
	}
	return &f, SlotWarnings(f)
}
