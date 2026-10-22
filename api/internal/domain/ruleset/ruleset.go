// Package ruleset is the entity of ADR 0041: which book data and which house
// rules a game is played with. The house rules are a typed record, one field
// per rule; a rule is added by a code change and a migration, never as an open
// key. The services that apply a rule read it from the ruleset they resolve
// (Resolver), never from a constant.
package ruleset

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
)

// DefaultName is the name of the default ruleset, the one every character
// plays under until games exist (ADR 0041 steps 1 and 9).
const DefaultName = "default"

// CritRow is one inclusive D100 range of the crit table and the multiplier a
// multiplier d100 inside it gives (ADR 0001 step 4, ADR 0041 step 2.1).
type CritRow struct {
	From int
	To   int
	// Multiplier may be a decimal (x1.5); the damage is floored once at the end.
	Multiplier float64
}

// Contains reports whether the D100 roll falls in the range.
func (r CritRow) Contains(roll int) bool { return roll >= r.From && roll <= r.To }

// CritTable is the whole table. Placeholder is true until an admin has saved
// one: the default is a single row 1-100 -> x2.
type CritTable struct {
	Rows        []CritRow
	Placeholder bool
}

// Lookup finds the row holding the multiplier d100 (ADR 0001 step 4). The
// stored table is always valid, so a miss means corrupt data.
func (t CritTable) Lookup(roll int) (CritRow, error) {
	for _, r := range t.Rows {
		if r.Contains(roll) {
			return r, nil
		}
	}
	return CritRow{}, fmt.Errorf("crit table has no row for D100 roll %d", roll)
}

// RacialLift says whether the race's class restriction (the dash of ADR 0030
// step 21) is waived for a multi-class or a dual-class character (ADR 0041
// step 2.2). It does not change a single-class character's check.
type RacialLift struct {
	MulticlassAnyRace bool
	DualclassAnyRace  bool
}

// LevelLimitsMode is how the racial level limits apply (ADR 0041 step 2.3).
type LevelLimitsMode string

// LevelLimitsWarn shows a level above the race's limit as a warning on the
// character, never a refusal and never a cap (ADR 0041 step 5).
const LevelLimitsWarn LevelLimitsMode = "warn"

// SuccessEffect is the buff a successful check of a proficiency puts on the
// character (ADR 0041 step 2.4, ADR 0037 step 20.1): its AC change
// (descending AC, negative betters it, ADR 0004 step 11) and the rounds it
// lasts (ADR 0004 step 2: 1 ends at the next Next Round).
type SuccessEffect struct {
	ProficiencyID string
	AC            int
	Rounds        int
}

// Ruleset is which book data and which house rules a game is played with
// (ADR 0041 step 1).
type Ruleset struct {
	ID   int64
	Name string
	// CatalogueVersion is the catalogue version the ruleset reads; recorded,
	// not enforced (there is one catalogue).
	CatalogueVersion string

	Crit        CritTable
	RacialLift  RacialLift
	LevelLimits LevelLimitsMode
	// SuccessEffects are keyed by proficiency id, sorted by it.
	SuccessEffects []SuccessEffect
}

// SuccessEffectFor is the effect a successful check of proficiency id puts
// on; false when it puts none (ADR 0037 step 20.6).
func (r Ruleset) SuccessEffectFor(id string) (SuccessEffect, bool) {
	for _, e := range r.SuccessEffects {
		if e.ProficiencyID == id {
			return e, true
		}
	}
	return SuccessEffect{}, false
}

// DefaultCritRows is the crit table before anyone edits it.
func DefaultCritRows() []CritRow { return []CritRow{{From: 1, To: 100, Multiplier: 2}} }

// Default is the content of the default ruleset a database starts with (ADR
// 0041 step 10): the placeholder crit table, both racial lifts on (user,
// 2026-10-21), level limits as warnings, and the Tumbling success effect (AC
// -4 for the round; user, triage of 2026-10-20). The ID is the storage's.
func Default() Ruleset {
	return Ruleset{
		Name:           DefaultName,
		Crit:           CritTable{Rows: DefaultCritRows(), Placeholder: true},
		RacialLift:     RacialLift{MulticlassAnyRace: true, DualclassAnyRace: true},
		LevelLimits:    LevelLimitsWarn,
		SuccessEffects: []SuccessEffect{{ProficiencyID: "tumbling", AC: -4, Rounds: 1}},
	}
}

// ValidateCritRows checks that rows (in any order) cover D100 1-100 exactly:
// every range inside 1-100 and not backwards, no overlap, no gap, every
// multiplier at least 1. The error names the offending row (1-based, as the
// editor lists them). It returns the rows sorted by range start.
func ValidateCritRows(rows []CritRow) ([]CritRow, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: the crit table needs at least one row covering 1-100", domain.ErrInvalidInput)
	}
	type indexed struct {
		CritRow
		n int
	}
	list := make([]indexed, len(rows))
	for i, r := range rows {
		n := i + 1
		switch {
		case r.From < 1 || r.From > 100 || r.To < 1 || r.To > 100:
			return nil, fmt.Errorf("%w: row %d (%d-%d) is outside 1-100", domain.ErrInvalidInput, n, r.From, r.To)
		case r.To < r.From:
			return nil, fmt.Errorf("%w: row %d (%d-%d) ends before it starts", domain.ErrInvalidInput, n, r.From, r.To)
		case math.IsNaN(r.Multiplier) || math.IsInf(r.Multiplier, 0) || r.Multiplier < 1:
			return nil, fmt.Errorf("%w: row %d (%d-%d) has multiplier %v, it must be a number of at least 1", domain.ErrInvalidInput, n, r.From, r.To, r.Multiplier)
		}
		list[i] = indexed{r, n}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].From < list[j].From })

	next := 1 // the first D100 value not yet covered
	out := make([]CritRow, len(list))
	for i, r := range list {
		switch {
		case r.From < next:
			return nil, fmt.Errorf("%w: row %d (%d-%d) overlaps the row before it, which reaches %d", domain.ErrInvalidInput, r.n, r.From, r.To, next-1)
		case r.From > next:
			return nil, fmt.Errorf("%w: gap before row %d (%d-%d): %d-%d is not covered", domain.ErrInvalidInput, r.n, r.From, r.To, next, r.From-1)
		}
		next = r.To + 1
		out[i] = r.CritRow
	}
	if next != 101 {
		last := list[len(list)-1]
		return nil, fmt.Errorf("%w: gap after row %d (%d-%d): %d-100 is not covered", domain.ErrInvalidInput, last.n, last.From, last.To, next)
	}
	return out, nil
}

// ValidateSuccessEffects checks the effects: each names a proficiency known
// to the book, lasts at least one round, and no proficiency appears twice
// (ADR 0041 step 2). It returns them sorted by proficiency id.
func ValidateSuccessEffects(effects []SuccessEffect, known func(proficiencyID string) bool) ([]SuccessEffect, error) {
	out := slices.Clone(effects)
	slices.SortFunc(out, func(a, b SuccessEffect) int { return cmp.Compare(a.ProficiencyID, b.ProficiencyID) })
	for i, e := range out {
		switch {
		case !known(e.ProficiencyID):
			return nil, domain.FieldErrorf("successEffects", "%q is not a known proficiency", e.ProficiencyID)
		case e.Rounds < 1:
			return nil, domain.FieldErrorf("successEffects", "the effect of %q must last at least one round", e.ProficiencyID)
		case i > 0 && out[i-1].ProficiencyID == e.ProficiencyID:
			return nil, domain.FieldErrorf("successEffects", "%q appears twice", e.ProficiencyID)
		}
	}
	return out, nil
}

// Change is one line of the ruleset's audit (ADR 0041 step 7): who changed
// which rule, when, and the rule's value before and after as a JSON document.
// It is append-only.
type Change struct {
	ID        int64
	RulesetID int64
	At        time.Time
	By        uuid.UUID
	// Rule is the house rule edited: one of the Rule* constants.
	Rule          string
	Before, After string
}

// The house rules an edit names in a Change.
const (
	RuleCritTable      = "crit_table"
	RuleRacialLift     = "racial_lift"
	RuleSuccessEffects = "success_effects"
)

// Resolver gives a character its ruleset (ADR 0041 step 3). The rules
// services take their ruleset from it, never a constant. Until games exist
// its one implementation returns the default (step 9).
type Resolver interface {
	ForCharacter(ctx context.Context, characterID int64) (Ruleset, error)
}
