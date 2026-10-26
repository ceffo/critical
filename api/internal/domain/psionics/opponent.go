package psionics

import (
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
)

// The opponent of an attack or a contact (ADR 0044 step 2): the app keeps no
// record of it. The request names it and gives what the DM tells; nothing of
// it is stored but the tangent and contact rows (step 3).

// Mind states of an opponent (step 2): a psionicist or psionic creature is
// closed unless it lets the contact in; a nonpsionicist is open.
const (
	MindOpen   = "open"
	MindClosed = "closed"
)

// Bounds of an opponent (step 2).
const (
	// MaxTargetLen is the longest target label, in characters.
	MaxTargetLen = 60
	// AttackChecks is how many power checks an attack mode's use makes for
	// one cost (step 7.1, the one-two punch): the DM gives one roll per
	// check when the opponent's defence is raised.
	AttackChecks = 2
)

// Opponent is the opponent an attack or a contact names (step 2).
type Opponent struct {
	// Target is the label the player gives the opponent: user data, like a
	// character's name, not a text of the app. It keys the tangent and
	// contact rows.
	Target string
	// Mind is MindOpen or MindClosed, as the DM tells.
	Mind string
	// Level is the opponent's levels or hit dice (the contact cost, step 9);
	// 0 when not given.
	Level int
	// Defence is the defence_mode power the opponent raised, "" for none.
	Defence string
	// Rolls are the DM's rolls for that defence, one per check of the
	// attack (step 7): none without a defence.
	Rolls []ContestSide
}

// NormalizeTarget trims a target label and checks it is 1 to MaxTargetLen
// characters (step 2); a *domain.FieldError on "target" otherwise.
func NormalizeTarget(label string) (string, error) {
	t := strings.TrimSpace(label)
	if t == "" || utf8.RuneCountInString(t) > MaxTargetLen {
		return "", domain.FieldErrorf("target", "a target is 1 to %d characters", MaxTargetLen)
	}
	return t, nil
}

// Validate checks the opponent's shape against book (step 2) and trims its
// target: the label, the mind, a level not below 0, a defence that is a
// defence_mode power, and the DM's rolls (1 to CheckDie, one per check of
// the attack) given only with a defence. Every refusal is a
// *domain.FieldError naming the field. Whether the attack or the contact
// needs the level, or refuses a mind, is the use's rule (steps 7 and 9).
func (o *Opponent) Validate(book Book) error {
	t, err := NormalizeTarget(o.Target)
	if err != nil {
		return err
	}
	o.Target = t
	if o.Mind != MindOpen && o.Mind != MindClosed {
		return domain.FieldErrorf("mind", "the mind is %q or %q", MindOpen, MindClosed)
	}
	if o.Level < 0 {
		return domain.FieldErrorf("level", "a level is 0 or more")
	}
	if o.Defence == "" {
		if len(o.Rolls) > 0 {
			return domain.FieldErrorf("rolls", "no rolls without a defence")
		}
		return nil
	}
	if err := ValidateDefence(book, o.Defence); err != nil {
		return err
	}
	if len(o.Rolls) != AttackChecks {
		return domain.FieldErrorf("rolls", "%d rolls against a defence", AttackChecks)
	}
	for _, r := range o.Rolls {
		if r.Roll < 1 || r.Roll > CheckDie {
			return domain.FieldErrorf("rolls", "a roll is 1 to %d", CheckDie)
		}
	}
	return nil
}

// ValidateDefence checks that id is a defence_mode power of book; a
// *domain.FieldError on "defence" otherwise.
func ValidateDefence(book Book, id string) error {
	if p, ok := book.Power(id); !ok || p.Kind != catalogue.KindDefenceMode {
		return domain.FieldErrorf("defence", "not a defence mode: %q", id)
	}
	return nil
}
