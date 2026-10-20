package spellbook_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// ADR 0030 step 20.1: Check refuses a spell for the first reason that applies, in the order
// unknown, not a wizard spell, already in the book, above the class's highest level, above the
// Intelligence's, the level full (Table 4; "All" has none), and lets it in otherwise. Each row
// breaks one limit only, and the "first" rows break two to prove the order.
func TestCheck_Step20_1_RefusesInOrder(t *testing.T) {
	limits := spellbook.Limits{ClassMaxSpellLevel: 5, IntMaxSpellLevel: 4, MaxSpellsPerLevel: 6}
	wiz := func(level int) spellbook.Spell { return spellbook.Spell{Level: level, Wizard: true} }
	for _, tt := range []struct {
		name      string
		l         spellbook.Limits
		sp        spellbook.Spell
		found     bool
		inBook    bool
		sameLevel int
		want      spellbook.Refusal
	}{
		{"fits", limits, wiz(3), true, false, 5, spellbook.Fits},
		{"unknown", limits, spellbook.Spell{}, false, false, 0, spellbook.RefusedUnknown},
		{"a priest spell", limits, spellbook.Spell{Level: 1}, true, false, 0, spellbook.RefusedNotWizard},
		{"already in, first", limits, wiz(9), true, true, 6, spellbook.RefusedAlreadyIn},
		{"above the class, first", limits, wiz(6), true, false, 6, spellbook.RefusedAboveClass},
		{"above the Intelligence", limits, wiz(5), true, false, 0, spellbook.RefusedAboveIntelligence},
		{"level full", limits, wiz(2), true, false, 6, spellbook.RefusedLevelFull},
		{"no maximum", spellbook.Limits{ClassMaxSpellLevel: 9, IntMaxSpellLevel: 9, NoMaximum: true}, wiz(2), true, false, 40, spellbook.Fits},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, spellbook.Check(tt.l, tt.sp, tt.found, tt.inBook, tt.sameLevel))
		})
	}
}

// ADR 0030 steps 20.1 and 20.7: the class's highest spell level is the caller's. The same 2nd-level
// spell fits a book whose class maximum is the class table's (Learn and Ajouter: a mage's 9th) and
// is refused at a starting level whose slots stop at the 1st (a level 1 mage's starting book).
func TestCheck_Step20_7_TheClassMaximumIsTheCallers(t *testing.T) {
	sp := spellbook.Spell{Level: 2, Wizard: true}
	table := spellbook.Limits{ClassMaxSpellLevel: 9, IntMaxSpellLevel: 7, MaxSpellsPerLevel: 9}
	atLevel1 := table
	atLevel1.ClassMaxSpellLevel = 1
	assert.Equal(t, spellbook.Fits, spellbook.Check(table, sp, true, false, 0))
	assert.Equal(t, spellbook.RefusedAboveClass, spellbook.Check(atLevel1, sp, true, false, 0))
}

// The spellbook reads a catalogue spell's level, and a wizard spell by its catalogue class.
func TestFromCatalogue(t *testing.T) {
	assert.Equal(t, spellbook.Spell{Level: 3, Wizard: true}, spellbook.FromCatalogue(catalogue.Spell{Level: 3, Class: "Wizard"}))
	assert.Equal(t, spellbook.Spell{Level: 1}, spellbook.FromCatalogue(catalogue.Spell{Level: 1, Class: "Priest"}))
}
