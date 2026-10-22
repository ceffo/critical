// Package dualclasssvc owns taking a new class (ADR 0042 step 3): a character
// in play leaves its active class, which is frozen at its level and XP, for a
// new one that starts at level 1 with no XP. The book's conditions (PHB
// multi/dual, Dual-Class Benefits and Restrictions; ADR 0042 step 2.2) are
// checked first and each refusal is a sentinel the REST layer words. The
// switch is one action-log transaction with the whole list of class entries
// as its snapshot, so the journal's Undo gives the previous entries back
// exactly (ADR 0018).
package dualclasssvc

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// The refusals of ADR 0042 step 2.2, each a conflict with its own code in
// internal/render. Nothing is written when one is returned.
var (
	// ErrNotAllowed is a character that may not become dual-class: it is
	// multi-class, or its race is restricted and the ruleset's
	// dualclass_any_race is off.
	ErrNotAllowed = fmt.Errorf("dual-class not allowed: %w", domain.ErrConflict)
	// ErrPrimeRequisiteTooLow is a prime requisite under 15 in the current
	// class or under 17 in the new one.
	ErrPrimeRequisiteTooLow = fmt.Errorf("prime requisite too low: %w", domain.ErrConflict)
	// ErrLevelTooLow is a current class under the level the book requires.
	ErrLevelTooLow = fmt.Errorf("class level too low to leave: %w", domain.ErrConflict)
	// ErrClassAlreadyLeft is a new class the character already left.
	ErrClassAlreadyLeft = fmt.Errorf("class already left: %w", domain.ErrConflict)
	// ErrSameClass is a new class equal to the current one.
	ErrSameClass = fmt.Errorf("already in that class: %w", domain.ErrConflict)
	// ErrMinimumsNotMet is a new class whose ability minimums (ADR 0040 step
	// 1.4) the character does not reach.
	ErrMinimumsNotMet = fmt.Errorf("new class minimums not met: %w", domain.ErrConflict)
	// ErrAlignmentNotAllowed is a new class that does not admit the
	// character's alignment (ADR 0040 step 1.5).
	ErrAlignmentNotAllowed = fmt.Errorf("new class does not admit the alignment: %w", domain.ErrConflict)
)

// Prime requisite scores the book asks (PHB multi/dual, Dual-Class: 15 in the
// current class's, 17 in the new class's).
const (
	MinOldPrime = 15
	MinNewPrime = 17
)

// StateKey is the action-log state key of a character's class entries.
const StateKey = "class_entries"

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Switch takes class newClass. A *domain.FieldError on "class" for an
	// unknown or unplayable class; a refusal above otherwise.
	Switch(ctx context.Context, characterID int64, newClass string) (*Result, error)
}

// Result is what the switch did.
type Result struct {
	// From is the class left, frozen at LeftAtLevel with XP; To the class
	// taken, at level 1.
	From, To    string
	LeftAtLevel int
	XP          int
}

// DAL writes a character's class entries. It joins the ctx transaction.
type DAL interface {
	// ReplaceClassEntries replaces the entries whole and the characters'
	// mirror of the first active one; domain.ErrCharacterNotFound for an
	// unknown character.
	ReplaceClassEntries(ctx context.Context, id int64, entries []classes.Entry) error
}

// Characters reads the character with its derived block (the XP of its
// entries).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Ledger lists the XP awards with their ids: the new entry ignores them all
// (classes.Entry.IgnoredAwardsThrough).
type Ledger interface {
	Awards(ctx context.Context, characterID int64) ([]charactersvc.Award, error)
}

// Catalogue is the class and race definitions.
type Catalogue interface {
	Get(class string) (classes.Class, error)
	Race(id string) (classes.Race, bool)
}

// Log is the slice of the action log a switch is recorded into.
type Log interface {
	actionlog.Recorder
}

// Deps are the service's collaborators.
type Deps struct {
	DAL        DAL
	Characters Characters
	Ledger     Ledger
	Catalogue  Catalogue
	Rulesets   ruleset.Resolver
	Log        Log
	Tx         domain.TxRunner
}

type service struct {
	Deps
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Characters == nil || d.Ledger == nil || d.Catalogue == nil || d.Rulesets == nil || d.Log == nil || d.Tx == nil {
		return nil, errors.New("dualclasssvc: nil dependency")
	}
	return &service{d}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Switch is ADR 0042 step 3. In one transaction: the conditions of step 2.2
// and the new class's own minimums and alignment, then the active entry is
// frozen at its realized level with its XP, and the new class starts at
// level 1 with no XP (its cut-off set at the ledger's last entry, so the
// older awards, edited or undone later, leave it alone). The hit points are
// kept.
func (s *service) Switch(ctx context.Context, characterID int64, newClass string) (*Result, error) {
	var res *Result
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.Characters.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		before := slices.Clone(c.Entries())
		old, xp, err := s.check(ctx, c, before, newClass)
		if err != nil {
			return err
		}
		awards, err := s.Ledger.Awards(ctx, characterID)
		if err != nil {
			return err
		}
		var through int64
		for _, a := range awards {
			through = max(through, a.ID)
		}
		after := make([]classes.Entry, 0, len(before)+1)
		for _, e := range before {
			if e.Class == old.Class {
				e = classes.Entry{
					Class: e.Class, RealizedLevel: e.RealizedLevel, StartingXP: xp,
					State: classes.EntryFrozen, LeftAtLevel: e.RealizedLevel,
				}
			}
			after = append(after, e)
		}
		after = append(after, classes.Entry{
			Class: newClass, RealizedLevel: 1, State: classes.EntryActive, IgnoredAwardsThrough: through,
		})
		if err := classes.ValidateEntries(after); err != nil {
			return err
		}
		if err := s.DAL.ReplaceClassEntries(ctx, characterID, after); err != nil {
			return err
		}
		res = &Result{From: old.Class, To: newClass, LeftAtLevel: old.RealizedLevel, XP: xp}
		return s.record(ctx, characterID, *res, before, after)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// check applies the conditions and returns the entry being left with its
// derived XP.
func (s *service) check(ctx context.Context, c *charactersvc.Character, entries []classes.Entry, newClass string) (classes.Entry, int, error) {
	active := classes.Active(entries)
	if len(active) != 1 {
		return classes.Entry{}, 0, ErrNotAllowed
	}
	old := active[0]
	rs, err := s.Rulesets.ForCharacter(ctx, c.ID)
	if err != nil {
		return classes.Entry{}, 0, err
	}
	if race, ok := s.Catalogue.Race(c.Race); ok && race.Restricted() && !rs.RacialLift.DualclassAnyRace {
		return classes.Entry{}, 0, ErrNotAllowed
	}
	if newClass == old.Class {
		return classes.Entry{}, 0, ErrSameClass
	}
	if slices.ContainsFunc(entries, func(e classes.Entry) bool { return e.Class == newClass }) {
		return classes.Entry{}, 0, ErrClassAlreadyLeft
	}
	target, err := s.Catalogue.Get(newClass)
	if err != nil || !target.Implemented() {
		return classes.Entry{}, 0, &domain.FieldError{Field: "class", Message: "unknown_class"}
	}
	from, err := s.Catalogue.Get(old.Class)
	if err != nil {
		return classes.Entry{}, 0, err
	}
	if old.RealizedLevel < classes.MinLeftAtLevel {
		return classes.Entry{}, 0, ErrLevelTooLow
	}
	if !primesAtLeast(c, from, MinOldPrime) || !primesAtLeast(c, target, MinNewPrime) {
		return classes.Entry{}, 0, ErrPrimeRequisiteTooLow
	}
	scores := map[string]int{}
	for _, a := range charactersvc.Abilities {
		scores[string(a)], _ = c.Score(a)
	}
	if len(target.UnmetMinimums(scores)) > 0 {
		return classes.Entry{}, 0, ErrMinimumsNotMet
	}
	if c.Details.Alignment != "" && !target.AlignmentAllowed(c.Details.Alignment) {
		return classes.Entry{}, 0, ErrAlignmentNotAllowed
	}
	derived, ok := c.Derived.Entry(old.Class)
	if !ok {
		return classes.Entry{}, 0, fmt.Errorf("dualclasssvc: no derived entry for %q", old.Class)
	}
	return old, derived.XP, nil
}

// primesAtLeast reports whether every prime requisite of class is at least min
// on the sheet.
func primesAtLeast(c *charactersvc.Character, class classes.Class, min int) bool {
	for _, p := range class.PrimeRequisites {
		if score, ok := c.Score(charactersvc.Ability(p)); !ok || score < min {
			return false
		}
	}
	return true
}
