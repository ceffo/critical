package spellbook

import "slices"

// Specialization is what the spellbook's rules read of a specialist wizard's
// school_specialization (ADR 0050 step 2): the school codes he specialises in
// and those in opposition. The zero value is a wizard who is no specialist (a
// mage, a bard): every rule below leaves him as he was.
type Specialization struct {
	Schools, OppositionSchools []string
}

// Specialist reports whether s is a specialist's (it names a school).
func (s Specialization) Specialist() bool { return len(s.Schools) > 0 }

// Standing is how a spell stands for a specialist (ADR 0050 step 5).
type Standing string

// The standings of step 5.
const (
	// StandingNone is a spell no specialist rule reads: a priest spell, or
	// any spell for a wizard who is no specialist.
	StandingNone Standing = ""
	// StandingOpposed is a wizard spell one of whose schools is in his
	// opposition: never learned, added or prepared (steps 6 and 11).
	StandingOpposed Standing = "opposed"
	// StandingHisSchool is a wizard spell, not opposed, one of whose schools
	// is his.
	StandingHisSchool Standing = "his_school"
	// StandingOther is any other wizard spell, one with no school code
	// included.
	StandingOther Standing = "other"
)

// School is ADR 0050 step 5: how wizard spell sp stands for the specialist
// s. Opposed wins over his school (Q4: a transmuter may not learn an
// Alteration, Necromancy spell), then his school, then other. A priest spell,
// or any spell for one who is no specialist, is StandingNone.
func School(s Specialization, sp Spell) Standing {
	if !s.Specialist() || !sp.Wizard {
		return StandingNone
	}
	in := func(list []string) bool {
		return slices.ContainsFunc(sp.Schools, func(c string) bool { return slices.Contains(list, c) })
	}
	switch {
	case in(s.OppositionSchools):
		return StandingOpposed
	case in(s.Schools):
		return StandingHisSchool
	}
	return StandingOther
}

// The learn chance's adjustments of ADR 0050 step 7 (PHB p.32), the same for
// every specialist (ADR 0040 step 3.3: a constant of the kind's code).
const (
	// SchoolLearnBonus is added to Table 4's chance for a spell of his school.
	SchoolLearnBonus = 15
	// OtherLearnPenalty is added to Table 4's chance for any other spell.
	OtherLearnPenalty = -15
)

// LearnAdjustment is step 7: what is added to Table 4's chance to learn
// spell sp for the specialist s (SchoolLearnBonus, OtherLearnPenalty), 0 for
// one who is no specialist. An opposed spell is refused before (step 6); it
// reads as other here. The chance is not capped.
func LearnAdjustment(s Specialization, sp Spell) int {
	switch School(s, sp) {
	case StandingHisSchool:
		return SchoolLearnBonus
	case StandingOther, StandingOpposed:
		return OtherLearnPenalty
	}
	return 0
}
