package saves

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// Inputs are what a character's save is built from, as the view states it
// (ADR 0046 step 3.3): the five targets and what the roll's terms come from.
// The caller (charactersvc.View.SaveInputs) fills it from the derived block.
type Inputs struct {
	// Saves are the five targets (PHB Table 60, the best of the entries
	// that count).
	Saves Saves
	// Race is the race's bonus per category (ADR 0030 step 22.1) and RaceID
	// the race code, the race term's source.
	Race   Saves
	RaceID string
	// MagicDefence is PHB Table 5's adjustment at Wis (ADR 0006 step 9).
	MagicDefence int
	Wis          int
	// Class are the class save_bonus grants, the best per tag (ADR 0043
	// step 3).
	Class []ClassBonus
	// Specialization is the saver's specialist school (ADR 0050 step 12):
	// the zero value for one who is no specialist.
	Specialization spellbook.Specialization
}

// Flags are the request's flags the player sets before the roll.
type Flags struct {
	// MindAffecting marks a save against magic that attacks the mind (ADR
	// 0006 step 9.3, the DM's call).
	MindAffecting bool
	// Spell is the spell the save names (ADR 0006 step 10.2) as the
	// spellbook's rules read it; nil when none is named or the catalogue
	// holds none of that id.
	Spell *spellbook.Spell
	// CasterSpecialist says the spell named was cast by a specialist of its
	// school (ADR 0050 step 13, the player's word: the caster is outside the
	// app).
	CasterSpecialist bool
}

// The specialist's save modifiers (ADR 0050 steps 12 and 13, PHB p.32), the
// same for every specialist (ADR 0040 step 3.3: constants of the kind's
// code, not parameters).
const (
	// SpecialistBonus is a specialist's bonus on his save against a wizard
	// spell of his school.
	SpecialistBonus = 1
	// CasterSpecialistPenalty is anyone's on his save against a specialist's
	// wizard spell of the caster's school.
	CasterSpecialistPenalty = -1
)

// ErrCasterSpecialistNeedsSpell refuses the caster_specialist flag on a save
// that names no wizard spell (ADR 0050 step 13): the -1 is against a
// specialist's spell of his school, so the save must name it.
var ErrCasterSpecialistNeedsSpell = fmt.Errorf("a specialist caster needs the wizard spell the save is against: %w", domain.ErrInvalidInput)

// CheckFlags refuses flags that cannot go together (ADR 0050 step 13): the
// caster_specialist flag without a wizard spell named.
func CheckFlags(fl Flags) error {
	if fl.CasterSpecialist && (fl.Spell == nil || !fl.Spell.Wizard) {
		return ErrCasterSpecialistNeedsSpell
	}
	return nil
}

// CasterModifier is what the targets of the specialist spec's own cast of
// spell sp take on their saves (ADR 0050 step 13): CasterSpecialistPenalty
// for a wizard spell of his school (ADR 0050 step 5's standing: an opposed
// one is not his), 0 otherwise and for one who is no specialist.
func CasterModifier(spec spellbook.Specialization, sp spellbook.Spell) int {
	if spellbook.School(spec, sp) == spellbook.StandingHisSchool {
		return CasterSpecialistPenalty
	}
	return 0
}

// carries says whether a request with these flags carries the tag a class
// save_bonus is against; the one tag is the mind-affecting flag (ADR 0043
// Q12).
func (fl Flags) carries(tag string) bool {
	return tag == classes.SaveAgainstMindAffecting && fl.MindAffecting
}

// Term is one named modifier of a save roll (ADR 0046 step 1.3): its code,
// its source (a race code, the Wisdom score, a class code) and its value on
// the roll. Off marks a term shown that does not add; no save term is Off
// today. The owner never words a term: the web does.
type Term struct {
	Code   string
	Source string
	Value  int
	Off    bool
}

// The term codes, the modifier kinds of the save breakdown.
const (
	TermRace   = "race"
	TermWisdom = "wisdom"
	TermClass  = "class"
	// TermSpecialist is a specialist's +1 against a wizard spell of his
	// school (ADR 0050 step 12); no source.
	TermSpecialist = "specialist"
	// TermCasterSpecialist is the -1 against a specialist caster's spell of
	// his school (ADR 0050 step 13); no source.
	TermCasterSpecialist = "caster_specialist"
)

// Check refuses a code that is none of Categories (invalid input).
func Check(cat string) error {
	if !slices.Contains(Categories, cat) {
		return fmt.Errorf("%w: unknown save category %q", domain.ErrInvalidInput, cat)
	}
	return nil
}

// Target is the save's target in category cat (ADR 0030 step 6, amending ADR
// 0006 step 1): the figure's Table 60 value, never moved by a term. An
// unknown category is refused.
func Target(f Inputs, cat string) (int, error) {
	if err := Check(cat); err != nil {
		return 0, err
	}
	v, _ := f.Saves.Of(cat)
	return v, nil
}

// Terms are the save's named modifiers on the roll in category cat, in order
// (ADR 0046 step 3.3): the race's bonus (ADR 0030 step 22.1), the Wisdom
// adjustment when the request is mind-affecting (ADR 0006 step 9.4, in any
// category), then each class save_bonus whose tag the request carries (ADR
// 0043 step 3), the specialist's +1 against a wizard spell of his school (ADR
// 0050 step 12) and the -1 against a specialist caster (step 13): both may
// stand together and cancel, as PHB p.32 says. A zero race or Wisdom value
// adds no term. Nil for none; the caller appends the buffs' terms. Flags that
// CheckFlags refuses are the caller's to refuse first.
func Terms(f Inputs, cat string, fl Flags) []Term {
	var out []Term
	if b, _ := f.Race.Of(cat); b != 0 {
		out = append(out, Term{Code: TermRace, Source: f.RaceID, Value: b})
	}
	if fl.MindAffecting && f.MagicDefence != 0 {
		out = append(out, Term{Code: TermWisdom, Source: strconv.Itoa(f.Wis), Value: f.MagicDefence})
	}
	for _, b := range f.Class {
		if fl.carries(b.Against) {
			out = append(out, Term{Code: TermClass, Source: b.Class, Value: b.Bonus})
		}
	}
	// ADR 0050 step 12: a save is always against someone else's spell, the
	// book's "cast by other wizards"; the standing of step 5 decides "his
	// school" (an opposed spell is not his). One specialization, so the term
	// counts once.
	if fl.Spell != nil && spellbook.School(f.Specialization, *fl.Spell) == spellbook.StandingHisSchool {
		out = append(out, Term{Code: TermSpecialist, Value: SpecialistBonus})
	}
	// ADR 0050 step 13: the player's word that the wizard spell was cast by a
	// specialist of its school.
	if fl.CasterSpecialist && fl.Spell != nil && fl.Spell.Wizard {
		out = append(out, Term{Code: TermCasterSpecialist, Value: CasterSpecialistPenalty})
	}
	return out
}
