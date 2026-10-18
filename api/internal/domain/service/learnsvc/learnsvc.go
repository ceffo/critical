// Package learnsvc owns learning a spell (ADR 0030 step 20): a mage or a
// bard adds a wizard spell to his spellbook. The class's and PHB Table 4's
// limits refuse it before any die; otherwise one d100 through the dice port
// against Table 4's chance to learn writes the spell in the spellbook or
// blocks it until the character's realized level is higher. Success or
// failure is one action-log transaction whose snapshot the journal's Undo
// writes back (ADR 0018).
package learnsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// The refusals of step 20.1 and 20.2: nothing is rolled or written. The web
// words them with the limits of the character's derived spellbook block.
var (
	// ErrNoSpellbook is a class without a spellbook (only mages and bards).
	ErrNoSpellbook = fmt.Errorf("the class has no spellbook: %w", domain.ErrInvalidInput)
	// ErrNotAWizardSpell is a spell of the catalogue that is not a wizard
	// spell.
	ErrNotAWizardSpell = fmt.Errorf("not a wizard spell: %w", domain.ErrInvalidInput)
	// ErrAlreadyKnown is a spell already in the spellbook.
	ErrAlreadyKnown = fmt.Errorf("spell already in the spellbook: %w", domain.ErrConflict)
	// ErrAboveIntelligence is a spell level above Table 4's for the
	// Intelligence.
	ErrAboveIntelligence = fmt.Errorf("spell level above the Intelligence's maximum: %w", domain.ErrInvalidInput)
	// ErrAboveClass is a spell level above the class's highest.
	ErrAboveClass = fmt.Errorf("spell level above the class's maximum: %w", domain.ErrInvalidInput)
	// ErrLevelFull is a spellbook that holds Table 4's maximum of the
	// spell's level.
	ErrLevelFull = fmt.Errorf("spellbook full at the spell's level: %w", domain.ErrInvalidInput)
	// ErrFailedAtLevel is a spell the character failed to learn at his
	// current realized level (or higher, after an undone level-up).
	ErrFailedAtLevel = fmt.Errorf("spell failed at this level: %w", domain.ErrInvalidInput)
)

// StateFamily is the action-log state key family of one spell's place in a
// character's spellbook: "spellbook:<spell key>".
const StateFamily = "spellbook"

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Learn tries to write the spell in the character's spellbook and
	// returns the roll's breakdown (Learned false: the failure is recorded).
	// domain.ErrNotFound (wrapped) for a spell the catalogue does not hold.
	Learn(ctx context.Context, characterID int64, spellID string) (*Result, error)
	// Restorers writes a spell's spellbook state back when a learn entry is
	// popped or redone, keyed by StateFamily.
	Restorers() map[string]actionlog.Restorer
}

// Result is the full breakdown of a learn roll (step 20.2).
type Result struct {
	SpellID    string
	SpellLevel int
	// Roll is the d100, Chance Table 4's chance to learn.
	Roll, Chance int
	// Learned is a roll at or under the chance.
	Learned bool
	// CharacterLevel is the realized level the roll was made at (a failure
	// blocks the spell until a higher one).
	CharacterLevel int
}

// Spell is what the rules read of a catalogue spell.
type Spell struct {
	Level  int
	Wizard bool
}

// Spells reads the spell catalogue.
type Spells interface {
	// Spell returns the catalogue spell id; ok is false when it holds none.
	Spell(id string) (s Spell, ok bool)
}

// Book reads a character's spellbook: the catalogue keys it holds.
type Book interface {
	Keys(ctx context.Context, characterID int64) ([]string, error)
}

// DAL writes a character's spellbook and learn failures. It joins the ctx
// transaction.
type DAL interface {
	AddEntry(ctx context.Context, characterID int64, spellKey string) error
	RemoveEntry(ctx context.Context, characterID int64, spellKey string) error
	// LearnFailure returns the realized level the spell was failed at; ok
	// is false when it was not.
	LearnFailure(ctx context.Context, characterID int64, spellKey string) (level int, ok bool, err error)
	SetLearnFailure(ctx context.Context, characterID int64, spellKey string, level int) error
	DeleteLearnFailure(ctx context.Context, characterID int64, spellKey string) error
}

// Characters reads the character with its derived block (the spellbook
// rules and the realized level).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Dice is the dice port.
type Dice interface {
	RollD100() int
}

// Log is the slice of the action log a roll is recorded into.
type Log interface {
	actionlog.Recorder
}

// Deps are the service's collaborators.
type Deps struct {
	DAL        DAL
	Book       Book
	Spells     Spells
	Characters Characters
	Dice       Dice
	Log        Log
	Tx         domain.TxRunner
}

type service struct{ d Deps }

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Book == nil || d.Spells == nil || d.Characters == nil || d.Dice == nil || d.Log == nil || d.Tx == nil {
		return nil, errors.New("learnsvc: nil dependency")
	}
	return &service{d: d}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Learn runs step 20 in one transaction: two requests for the same spell
// cannot both roll it.
func (s *service) Learn(ctx context.Context, characterID int64, spellID string) (*Result, error) {
	var res *Result
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.d.Characters.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		rules := c.Derived.Spellbook
		if rules == nil {
			return ErrNoSpellbook
		}
		sp, ok := s.d.Spells.Spell(spellID)
		if !ok {
			return fmt.Errorf("spell %q: %w", spellID, domain.ErrNotFound)
		}
		if !sp.Wizard {
			return ErrNotAWizardSpell
		}
		level := c.Derived.RealizedLevel
		if err := s.refusal(ctx, c.ID, spellID, sp.Level, level, rules); err != nil {
			return err
		}
		// Step 20.2: one d100 against Table 4's chance to learn.
		roll := s.d.Dice.RollD100()
		res = &Result{
			SpellID: spellID, SpellLevel: sp.Level, Roll: roll, Chance: rules.LearnChance,
			Learned: roll <= rules.LearnChance, CharacterLevel: level,
		}
		before, err := s.state(ctx, c.ID, spellID)
		if err != nil {
			return err
		}
		after := spellState{SpellID: spellID, Known: res.Learned}
		if !res.Learned {
			after.FailedAtLevel = level
		}
		if err := s.write(ctx, c.ID, spellID, after); err != nil {
			return err
		}
		return s.record(ctx, c.ID, *res, before, after)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// refusal is step 20.1 (and the failure's block of step 20.2): the reason
// the spell may not be rolled for, nil when it may.
func (s *service) refusal(ctx context.Context, characterID int64, spellID string, spellLevel, level int, rules *charactersvc.SpellbookRules) error {
	keys, err := s.d.Book.Keys(ctx, characterID)
	if err != nil {
		return fmt.Errorf("read spellbook: %w", err)
	}
	sameLevel := 0
	for _, k := range keys {
		if k == spellID {
			return ErrAlreadyKnown
		}
		if other, ok := s.d.Spells.Spell(k); ok && other.Level == spellLevel {
			sameLevel++
		}
	}
	switch {
	case spellLevel > rules.ClassMaxSpellLevel:
		return ErrAboveClass
	case spellLevel > rules.IntMaxSpellLevel:
		return ErrAboveIntelligence
	case !rules.NoMaximum && sameLevel >= rules.MaxSpellsPerLevel:
		return ErrLevelFull
	}
	failedAt, failed, err := s.d.DAL.LearnFailure(ctx, characterID, spellID)
	if err != nil {
		return fmt.Errorf("read learn failure: %w", err)
	}
	if failed && level <= failedAt {
		return ErrFailedAtLevel
	}
	return nil
}
