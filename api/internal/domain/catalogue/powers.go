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
	// Table names the rule table that gives the amount (format 18, ADR 0044
	// step 1.4): "contact_cost" for the contact power, whose Kind is then
	// CostVariable so the entered-cost path is unchanged. Empty otherwise.
	Table string
	// Plus is the amount a CostContact cost adds once contact is made (Switch
	// Personality's 30); 0 when there is none.
	Plus int
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

// Weapon kinds a power effect may be limited to.
const (
	WeaponKindMelee   = "melee"
	WeaponKindMissile = "missile"
)

// PowerEffect is what a power does to the psionicist who uses it (format 21,
// ADR 0043 step 17), in the keys of a buff's effects: a zero number is an
// absent key. AC is signed (negative is better). Rounds is how long a power
// without maintenance lasts; a maintained power's row is untimed.
type PowerEffect struct {
	AC         int
	ToHit      int
	Damage     int
	SaveBonus  int
	StrBonus   int
	Initiative int
	// WeaponKind limits ToHit and Damage to melee or missile attacks; empty
	// for either.
	WeaponKind string
	Rounds     int
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
	// Effect is the optional effect on the psionicist (format 21); nil when
	// the power's result stays text.
	Effect *PowerEffect
}

// CheckPowers is the import's check of ADR 0043 step 8: ids are unique, every
// power names a known discipline and a known kind, and every prerequisite is
// a power of the catalogue; and (step 17) an effect is coherent with its
// power. Each refusal names the power.
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
	for _, p := range powers {
		problems = append(problems, checkEffect(p)...)
	}
	if len(problems) > 0 {
		return fmt.Errorf("psionic powers: %w", errors.Join(problems...))
	}
	return nil
}

// checkEffect is ADR 0043 step 17's start-up rule for a power's effect: it
// has at least one effect key; rounds (>= 1) is on a power without
// maintenance only, and a power without maintenance needs it (its effect would
// never apply); str_bonus is >= 1 (the buff rule); and a contact power has
// none (its row sits on a mind, so the effect would not land on the
// psionicist).
func checkEffect(p Power) []error {
	e := p.Effect
	if e == nil {
		return nil
	}
	var out []error
	if e.AC == 0 && e.ToHit == 0 && e.Damage == 0 && e.SaveBonus == 0 && e.StrBonus == 0 && e.Initiative == 0 {
		out = append(out, fmt.Errorf("power %s: effect has no effect key", p.ID))
	}
	if e.StrBonus < 0 {
		out = append(out, fmt.Errorf("power %s: effect str_bonus %d must be at least 1", p.ID, e.StrBonus))
	}
	if e.Rounds < 0 {
		out = append(out, fmt.Errorf("power %s: effect rounds %d must be at least 1", p.ID, e.Rounds))
	}
	if e.WeaponKind != "" && e.WeaponKind != WeaponKindMelee && e.WeaponKind != WeaponKindMissile {
		out = append(out, fmt.Errorf("power %s: effect weapon_kind %q is not melee or missile", p.ID, e.WeaponKind))
	}
	switch {
	case p.Maintenance != nil && e.Rounds > 0:
		out = append(out, fmt.Errorf("power %s: effect has rounds but the power is maintained (its row is untimed)", p.ID))
	case p.Maintenance == nil && e.Rounds == 0:
		out = append(out, fmt.Errorf("power %s: effect without rounds on a power without maintenance would never apply", p.ID))
	}
	if p.InitialCost.Kind == CostContact {
		out = append(out, fmt.Errorf("power %s: a contact power has no effect on the psionicist", p.ID))
	}
	return out
}
