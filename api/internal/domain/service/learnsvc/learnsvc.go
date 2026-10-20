// Package learnsvc owns writing a spell in the spellbook (ADR 0030 step 20):
// a mage or a bard adds a wizard spell to his spellbook. The class's and PHB
// Table 4's limits refuse it before any die or write. Learn rolls one d100
// through the dice port against Table 4's chance to learn, which writes the
// spell in the spellbook or blocks it until the character's realized level is
// higher; Add (Ajouter, step 20.6) writes it with no roll. Each entry keeps
// its origin (step 20.5). Every write is one action-log transaction whose
// snapshot the journal's Undo writes back (ADR 0018).
package learnsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// The refusals of step 20.1 and 20.2 (all but ErrFailedAtLevel also refuse
// Add, step 20.6): nothing is rolled or written. The web words them with the
// limits of the character's derived spellbook block.
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

// The refusals of a free spell's claim (step 20.8), after those of Add:
// nothing is written.
var (
	// ErrNoFreeSpellOwed is a claim for a realized level that owes no free
	// spell: it opened no spell level, the Intelligence allows none, it was
	// not taken in the app, or its free spell is already in the book.
	ErrNoFreeSpellOwed = fmt.Errorf("no free spell owed for that level: %w", domain.ErrInvalidInput)
	// ErrFreeSpellWrongLevel is a claim with a spell of another spell level
	// than the one the level's free spell must be of.
	ErrFreeSpellWrongLevel = fmt.Errorf("the free spell is of another spell level: %w", domain.ErrInvalidInput)
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
	// Add writes the spell at the end of the character's spellbook with no
	// roll (Ajouter, step 20.6), origin spellbook.OriginGiven. Refused like
	// Learn, but not for a spell failed at this level: the add lifts that
	// block (its undo puts it back). domain.ErrNotFound (wrapped) for a spell
	// the catalogue does not hold.
	Add(ctx context.Context, characterID int64, spellID string) (*Added, error)
	// AddFreeSpell is Add claiming the free spell realized level grantLevel
	// owes (step 20.8): the entry's origin is spellbook.OriginLevelGrant with
	// that level. Refused like Add, then ErrNoFreeSpellOwed when that level
	// owes none, ErrFreeSpellWrongLevel when the spell is not of the spell
	// level it owes.
	AddFreeSpell(ctx context.Context, characterID int64, spellID string, grantLevel int) (*Added, error)
}

// Added is the entry a roll-free add wrote (step 20.6), and for a free
// spell (step 20.8) the realized level that granted it (0 otherwise).
type Added struct {
	SpellID    string
	SpellLevel int
	Origin     spellbook.Origin
	GrantLevel int
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
	// RetryLevel is the realized level from which a failed spell may be
	// tried again, the next one (PHB: "until he advances to the next
	// level"); 0 when the spell was learned.
	RetryLevel int
}

// Spell is what the rules read of a catalogue spell (spellbook.Spell).
type Spell = spellbook.Spell

// Spells reads the spell catalogue.
type Spells interface {
	// Spell returns the catalogue spell id; ok is false when it holds none.
	Spell(id string) (s Spell, ok bool)
}

// Book reads a character's spellbook: the catalogue keys it holds, and its
// entries with their origin (step 20.5).
type Book interface {
	Keys(ctx context.Context, characterID int64) ([]string, error)
	Entries(ctx context.Context, characterID int64) ([]spellbook.Entry, error)
}

// DAL writes a character's spellbook and learn failures. It joins the ctx
// transaction.
type DAL interface {
	// AddEntry writes the entry at the end of the spellbook. A spell already
	// there is left as it is: an add never relabels an entry.
	AddEntry(ctx context.Context, characterID int64, e spellbook.Entry) error
	// RestoreEntry writes the entry as an undo or a redo's snapshot holds
	// it: at the end of the spellbook, or, for a spell already there, in its
	// place with the entry's origin and grant level.
	RestoreEntry(ctx context.Context, characterID int64, e spellbook.Entry) error
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
		c, sp, err := s.writable(ctx, characterID, spellID)
		if err != nil {
			return err
		}
		level := c.Derived.RealizedLevel
		// The failure's block of step 20.2.
		failedAt, failed, err := s.d.DAL.LearnFailure(ctx, c.ID, spellID)
		if err != nil {
			return fmt.Errorf("read learn failure: %w", err)
		}
		if failed && level <= failedAt {
			return ErrFailedAtLevel
		}
		rules := c.Derived.Spellbook
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
		if res.Learned {
			after.Origin = spellbook.OriginLearned // step 20.5
		} else {
			after.FailedAtLevel = level
			res.RetryLevel = level + 1
		}
		if err := s.write(ctx, c.ID, spellID, after, false); err != nil {
			return err
		}
		return s.record(ctx, c.ID, *res, before, after)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Add runs step 20.6 in one transaction: the refusals of step 20.1, then the
// spell written with the origin given and no die. A failure's block is
// lifted (the snapshot's "before" keeps it for the undo).
func (s *service) Add(ctx context.Context, characterID int64, spellID string) (*Added, error) {
	return s.add(ctx, characterID, spellID, 0)
}

// AddFreeSpell runs step 20.8's claim: Add's refusals and write, the entry
// owed by grantLevel, with the origin level_grant.
func (s *service) AddFreeSpell(ctx context.Context, characterID int64, spellID string, grantLevel int) (*Added, error) {
	if grantLevel < 1 {
		return nil, ErrNoFreeSpellOwed
	}
	return s.add(ctx, characterID, spellID, grantLevel)
}

// add is Add (grantLevel 0) and AddFreeSpell, in one transaction.
func (s *service) add(ctx context.Context, characterID int64, spellID string, grantLevel int) (*Added, error) {
	var res *Added
	err := s.d.Tx.InTx(ctx, func(ctx context.Context) error {
		c, sp, err := s.writable(ctx, characterID, spellID)
		if err != nil {
			return err
		}
		res = &Added{SpellID: spellID, SpellLevel: sp.Level, Origin: spellbook.OriginGiven}
		if grantLevel > 0 {
			if err := owes(c.Derived.LevelUp.FreeSpellsOwed, grantLevel, sp.Level); err != nil {
				return err
			}
			res.Origin, res.GrantLevel = spellbook.OriginLevelGrant, grantLevel
		}
		before, err := s.state(ctx, c.ID, spellID)
		if err != nil {
			return err
		}
		after := spellState{SpellID: spellID, Known: true, Origin: res.Origin, GrantLevel: res.GrantLevel}
		if err := s.write(ctx, c.ID, spellID, after, false); err != nil {
			return err
		}
		return s.recordAdd(ctx, c.ID, *res, before, after)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// owes is step 20.8's check of a claim against the free spells owed: one
// granted by grantLevel, of spellLevel.
func owes(owed []spellbook.FreeSpell, grantLevel, spellLevel int) error {
	err := ErrNoFreeSpellOwed
	for _, o := range owed {
		if o.Level != grantLevel {
			continue
		}
		if o.SpellLevel == spellLevel {
			return nil
		}
		err = ErrFreeSpellWrongLevel
	}
	return err
}

// writable reads the character and the spell and returns step 20.1's
// refusal (spellbook.Check, the class's highest spell level as the class's
// table has it), the reason the spell may not be written in his spellbook (by
// a roll or not), nil when it may.
func (s *service) writable(ctx context.Context, characterID int64, spellID string) (*charactersvc.Character, Spell, error) {
	c, err := s.d.Characters.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, Spell{}, err
	}
	rules := c.Derived.Spellbook
	if rules == nil {
		return nil, Spell{}, ErrNoSpellbook
	}
	sp, found := s.d.Spells.Spell(spellID)
	inBook, sameLevel := false, 0
	if found && sp.Wizard {
		keys, err := s.d.Book.Keys(ctx, c.ID)
		if err != nil {
			return nil, Spell{}, fmt.Errorf("read spellbook: %w", err)
		}
		for _, k := range keys {
			if k == spellID {
				inBook = true
			} else if other, ok := s.d.Spells.Spell(k); ok && other.Level == sp.Level {
				sameLevel++
			}
		}
	}
	limits := spellbook.Limits{
		ClassMaxSpellLevel: rules.ClassMaxSpellLevel, IntMaxSpellLevel: rules.IntMaxSpellLevel,
		MaxSpellsPerLevel: rules.MaxSpellsPerLevel, NoMaximum: rules.NoMaximum,
	}
	switch spellbook.Check(limits, sp, found, inBook, sameLevel) {
	case spellbook.Fits:
		return c, sp, nil
	case spellbook.RefusedUnknown:
		return nil, Spell{}, fmt.Errorf("spell %q: %w", spellID, domain.ErrNotFound)
	case spellbook.RefusedNotWizard:
		return nil, Spell{}, ErrNotAWizardSpell
	case spellbook.RefusedAlreadyIn:
		return nil, Spell{}, ErrAlreadyKnown
	case spellbook.RefusedAboveClass:
		return nil, Spell{}, ErrAboveClass
	case spellbook.RefusedAboveIntelligence:
		return nil, Spell{}, ErrAboveIntelligence
	default: // spellbook.RefusedLevelFull
		return nil, Spell{}, ErrLevelFull
	}
}
