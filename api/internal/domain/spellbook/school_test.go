package spellbook_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// PHB Table 22 (ADR 0050, the table of Context): a transmuter's school is
// alteration, his opposition abjuration and necromancy; a diviner's (Q3) is
// greater and lesser divination, his opposition conjuration/summoning.
var (
	transmuter = spellbook.Specialization{Schools: []string{"alteration"}, OppositionSchools: []string{"abjuration", "necromancy"}}
	diviner    = spellbook.Specialization{
		Schools: []string{"greater_divination", "lesser_divination"}, OppositionSchools: []string{"conjuration_summoning"},
	}
)

func wizard(schools ...string) spellbook.Spell {
	return spellbook.Spell{Level: 2, Wizard: true, Schools: schools}
}

// ADR 0050 step 5: for a specialist a wizard spell is opposed when one of its
// schools is in his opposition, otherwise of his school when one of its
// schools is his, otherwise other (a spell with no code included). Opposed
// wins over his school (Q4). A priest spell is none of these, and no spell is
// anything for a wizard who is no specialist.
func TestSchool_Step5_OpposedWinsThenHisThenOther(t *testing.T) {
	for _, tt := range []struct {
		name string
		s    spellbook.Specialization
		sp   spellbook.Spell
		want spellbook.Standing
	}{
		{"his school", transmuter, wizard("alteration"), spellbook.StandingHisSchool},
		{"opposed", transmuter, wizard("necromancy"), spellbook.StandingOpposed},
		{"another school", transmuter, wizard("illusion"), spellbook.StandingOther},
		{"no school code", transmuter, wizard(), spellbook.StandingOther},
		{"two schools, one his and one opposed: opposed wins", transmuter, wizard("alteration", "necromancy"), spellbook.StandingOpposed},
		{"two schools, the other order", transmuter, wizard("necromancy", "alteration"), spellbook.StandingOpposed},
		{"two schools, one his and one other", transmuter, wizard("illusion", "alteration"), spellbook.StandingHisSchool},
		{"a diviner's lesser divination", diviner, wizard("lesser_divination"), spellbook.StandingHisSchool},
		{"a priest spell", transmuter, spellbook.Spell{Level: 2, Schools: []string{"necromancy"}}, spellbook.StandingNone},
		{"no specialist", spellbook.Specialization{}, wizard("necromancy"), spellbook.StandingNone},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, spellbook.School(tt.s, tt.sp))
		})
	}
}

// ADR 0050 step 6: Check refuses an opposed spell with spell_school_opposed,
// after already_in_spellbook and before the level checks; the same spell
// fits a mage's book (no specialization) and a specialist's own school's
// spell is not refused.
func TestCheck_Step6_OpposedRefusedInOrder(t *testing.T) {
	limits := spellbook.Limits{ClassMaxSpellLevel: 5, IntMaxSpellLevel: 4, MaxSpellsPerLevel: 6, Specialization: transmuter}
	mage := limits
	mage.Specialization = spellbook.Specialization{}
	opposed := func(level int) spellbook.Spell {
		return spellbook.Spell{Level: level, Wizard: true, Schools: []string{"abjuration"}}
	}
	for _, tt := range []struct {
		name      string
		l         spellbook.Limits
		sp        spellbook.Spell
		inBook    bool
		sameLevel int
		want      spellbook.Refusal
	}{
		{"opposed", limits, opposed(2), false, 0, spellbook.RefusedOpposed},
		{"already in comes first", limits, opposed(2), true, 0, spellbook.RefusedAlreadyIn},
		{"opposed before above the class", limits, opposed(6), false, 0, spellbook.RefusedOpposed},
		{"opposed before above the Intelligence", limits, opposed(5), false, 0, spellbook.RefusedOpposed},
		{"opposed before the level full", limits, opposed(2), false, 6, spellbook.RefusedOpposed},
		{"his school fits", limits, wizard("alteration"), false, 0, spellbook.Fits},
		{"a mage may", mage, opposed(2), false, 0, spellbook.Fits},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, spellbook.Check(tt.l, tt.sp, true, tt.inBook, tt.sameLevel))
		})
	}
}

// ADR 0050 step 7 (PHB p.32): +15 to Table 4's chance for a spell of his
// school, -15 for any other, 0 for a mage.
func TestLearnAdjustment_Step7(t *testing.T) {
	assert.Equal(t, 15, spellbook.LearnAdjustment(transmuter, wizard("alteration")))
	assert.Equal(t, -15, spellbook.LearnAdjustment(transmuter, wizard("illusion")))
	assert.Equal(t, -15, spellbook.LearnAdjustment(transmuter, wizard()))
	assert.Equal(t, 0, spellbook.LearnAdjustment(spellbook.Specialization{}, wizard("alteration")))
}
