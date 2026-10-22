package catalogue

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

// Power kinds (ADR 0043 step 8): what a psionicist's pick takes a slot of.
const (
	KindScience     = "science"
	KindDevotion    = "devotion"
	KindDefenceMode = "defence_mode"
	KindAttackMode  = "attack_mode"
)

// PowerKinds lists the kinds a power may have.
var PowerKinds = []string{KindScience, KindDevotion, KindDefenceMode, KindAttackMode}

// Cost kinds of a power that has no printed number.
const (
	// CostVariable: the power's text or a table gives the amount.
	CostVariable = "variable"
	// CostContact: the cost is paid on contact (telepathic combat, layer 2).
	CostContact = "contact"
)

// Cost is a power's initial or maintenance cost in PSPs: a number, or a word
// when the book gives none (CostVariable, CostContact). Kind is empty for a
// number.
type Cost struct {
	Amount int
	Kind   string
}

// Discipline is one of the six psionic disciplines (ADR 0043 step 8); the
// name is a language map (ADR 0023).
type Discipline struct {
	ID   string
	Name localized.Text
}

// PowerScore is a power's score: an ability plus or minus a number (ADR 0043
// step 8). A power whose score is a formula in its text has none.
type PowerScore struct {
	Ability  string
	Modifier int
}

// Maintenance is what a maintained power costs every Period rounds.
type Maintenance struct {
	Cost   Cost
	Period int
}

// Power is one psionic power of the catalogue (format 13, ADR 0021 step 20.3,
// ADR 0043 step 8). Every human-language text is a language map (ADR 0023);
// an absent language has no entry, and the reader falls back to English.
type Power struct {
	ID         string
	Name       localized.Text
	Discipline string
	Kind       string
	// Slot is the slot kind the pick takes where Kind does not say it (an
	// attack mode takes a devotion's slot unless the catalogue says science);
	// empty when the kind says it (ADR 0021 step 23).
	Slot        string
	Score       *PowerScore
	InitialCost Cost
	Maintenance *Maintenance
	// Prerequisites are the ids of the powers a psionicist must know first.
	Prerequisites []string
	// MinLevel is a prerequisite stated as a psionicist level; 0 when none.
	MinLevel        int
	Range           localized.Text
	PreparationTime localized.Text
	Area            localized.Text
	Description     localized.Text
	// PowerScoreResult and Natural20Result are the optional outcome texts.
	PowerScoreResult localized.Text
	Natural20Result  localized.Text
}

// CheckPowers is the import's check of ADR 0043 step 8: ids are unique, every
// power names a known discipline and a known kind, and every prerequisite is
// a power of the catalogue. Each refusal names the power.
func CheckPowers(disciplines []Discipline, powers []Power) error {
	known := make(map[string]bool, len(disciplines))
	var problems []error
	for _, d := range disciplines {
		if known[d.ID] {
			problems = append(problems, fmt.Errorf("discipline %s: repeated", d.ID))
		}
		known[d.ID] = true
	}
	ids := make(map[string]bool, len(powers))
	for _, p := range powers {
		if ids[p.ID] {
			problems = append(problems, fmt.Errorf("power %s: repeated", p.ID))
		}
		ids[p.ID] = true
	}
	for _, p := range powers {
		if !known[p.Discipline] {
			problems = append(problems, fmt.Errorf("power %s: unknown discipline %q", p.ID, p.Discipline))
		}
		if p.Slot != "" && p.Slot != KindScience && p.Slot != KindDevotion {
			problems = append(problems, fmt.Errorf("power %s: unknown slot %q", p.ID, p.Slot))
		}
		if !slices.Contains(PowerKinds, p.Kind) {
			problems = append(problems, fmt.Errorf("power %s: unknown kind %q", p.ID, p.Kind))
		}
		for _, pre := range p.Prerequisites {
			if !ids[pre] {
				problems = append(problems, fmt.Errorf("power %s: unknown prerequisite %q", p.ID, pre))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("psionic powers: %w", errors.Join(problems...))
	}
	return nil
}
