// Package charactersvc owns read access to a character: the entered values
// (abilities, HP max, class, race, starting XP) and the values the rules
// derive from them (level, THAC0, saving throws, spell slots; ADR 0030 steps 3
// to 8), and which characters a user owns and plays (ADR 0029 step 7):
// creating, switching, renaming and retiring them. The seed character is
// written by the storage layer's seeding.
package charactersvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ErrNoCurrentCharacter is a per-character request of a user who has no
// current character (ADR 0029 step 7.2): a new player, before they create
// their first one. Its stable code tells the web to offer that.
var ErrNoCurrentCharacter = fmt.Errorf("no current character: %w", domain.ErrNotFound)

// ErrCharacterChanged is a request for a character that is not the user's
// current one any more (ADR 0029 step 7.2): the user switched elsewhere (the
// other server, another tab). Nothing was read or written; the web reloads to
// show the current character.
var ErrCharacterChanged = fmt.Errorf("current character changed: %w", domain.ErrConflict)

// SVC is the driver port.
type SVC interface {
	domain.Service
	GetCharacter(ctx context.Context, id int64) (*Character, error)
	// List is the owner's characters they have not retired, in creation
	// order (ADR 0029 steps 7.1 and 7.3).
	List(ctx context.Context, owner uuid.UUID) ([]Summary, error)
	// SetCurrent makes the owner's character id their current character
	// (step 7.2). Another user's character, a retired one and an unknown id
	// are all domain.ErrCharacterNotFound (step 7.1: a foreign character is
	// not revealed).
	SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error
	// Create stores a new character of the owner's and makes it their
	// current character (ADR 0029 step 7.2); it starts with nothing (no
	// weapon, item, proficiency, skill or spellbook). A *domain.FieldError
	// for invalid input (see NewCharacter).
	Create(ctx context.Context, owner uuid.UUID, n NewCharacter) (int64, error)
	// Change renames, retires or re-arms (a thief's or a bard's skill armour) the
	// owner's character id (step 7.3).
	// Another user's character, a retired one and an unknown id are all
	// domain.ErrCharacterNotFound; retiring the current character makes the
	// owner's first one left current (none: no current character).
	// It returns the owner's current character after the change (0: none).
	Change(ctx context.Context, owner uuid.UUID, id int64, c Change) (current int64, err error)
	// CreationOptions are the classes a new character may have, the races
	// each admits and the entered values' ranges.
	CreationOptions(ctx context.Context) (CreationOptions, error)
	// Restorers are the state keys Change snapshots (a thief's or bard's armour),
	// with the writers an undo or a redo calls.
	Restorers() map[string]actionlog.Restorer
}

// Reader is the read-side driven port. Per clean_architecture.md §5.4, reads
// don't need the full DAL detour; this port returns domain entities directly.
type Reader interface {
	GetCharacter(ctx context.Context, id int64) (*Character, error)
}

// XPLedger is what the XP ledger's entries add to the starting XP (ADR 0015
// step 1); xpsvc.Totals implements it over the ledger's DAL.
type XPLedger interface {
	EntriesTotal(ctx context.Context, characterID int64) (int, error)
}

// Owners is the driven port over who owns and plays which character.
type Owners interface {
	// CharactersOf is the owner's characters they have not retired, by id.
	CharactersOf(ctx context.Context, owner uuid.UUID) ([]Summary, error)
	// SetCurrentCharacter sets the user's current character when the user
	// owns it and has not retired it, else returns
	// domain.ErrCharacterNotFound and changes nothing (one statement: no
	// check-then-write).
	SetCurrentCharacter(ctx context.Context, owner uuid.UUID, id int64) error
	// CreateCharacter stores c (its ID is ignored) as the owner's, with no
	// spell prepared, no buff and round 0, and returns its id.
	CreateCharacter(ctx context.Context, owner uuid.UUID, c Character) (int64, error)
	// RenameCharacter and RetireCharacter change the owner's character id
	// when the owner has it and has not retired it, else return
	// domain.ErrCharacterNotFound and change nothing (one statement each).
	RenameCharacter(ctx context.Context, owner uuid.UUID, id int64, name string) error
	RetireCharacter(ctx context.Context, owner uuid.UUID, id int64) error
	// RestoreSkillArmor writes a character's skill armour back for an undo
	// or a redo (no owner: the log already ties the entry to the character).
	RestoreSkillArmor(ctx context.Context, id int64, armor string) error
	// SetSkillArmor sets the armour a thief or a bard wears for its skills, with the
	// same ownership rule as RenameCharacter.
	SetSkillArmor(ctx context.Context, owner uuid.UUID, id int64, armor string) error
	// RepairCurrentCharacter makes the owner's first character not retired
	// current unless their current one is such a character (none left: no
	// current character).
	RepairCurrentCharacter(ctx context.Context, owner uuid.UUID) error
	// CurrentCharacter is the owner's current character, read through the
	// ownership: 0 when it is none, retired or not theirs.
	CurrentCharacter(ctx context.Context, owner uuid.UUID) (int64, error)
}

// Sessions is the login's session cache (authsvc.SVC satisfies it): a user
// whose current character changed is read again by their next request.
type Sessions interface {
	ForgetUser(userID uuid.UUID)
}

// Deps are the service's collaborators.
type Deps struct {
	Reader   Reader
	XP       XPLedger
	Rules    Rules
	Owners   Owners
	Tx       domain.TxRunner
	Sessions Sessions
	// Log records a skill armour change (ADR 0030 step 1); Tx must be the
	// action-log transaction runner.
	Log actionlog.Recorder
	// Languages is the language catalogue (ADR 0032 step 4); nil is the empty
	// one of a catalogue before format 8, which knows no language.
	Languages LanguageBook
}

type service struct {
	reader   Reader
	xp       XPLedger
	rules    Rules
	owners   Owners
	tx       domain.TxRunner
	sessions Sessions
	log      actionlog.Recorder
	// languages is never nil: the empty catalogue when Deps holds none.
	languages LanguageBook
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.Reader == nil || d.XP == nil || d.Rules == nil || d.Owners == nil || d.Tx == nil || d.Sessions == nil || d.Log == nil {
		return nil, errors.New("charactersvc: nil dependency")
	}
	if d.Languages == nil {
		d.Languages = (*ruletablesvc.Languages)(nil)
	}
	return &service{reader: d.Reader, xp: d.XP, rules: d.Rules, owners: d.Owners, tx: d.Tx, sessions: d.Sessions, log: d.Log, languages: d.Languages}, nil
}

// NewWithConfig is here for symmetry with the other services. There is no
// configurable surface today.
func NewWithConfig(_ *Config, d Deps) (SVC, error) { return New(d) }

// GetCharacter reads the character and derives its level, THAC0, saves and
// slots from its class, race, abilities and XP total (ADR 0030 steps 3 to 8):
// every reader of a character (attacks, saves, preparation, casting, the XP
// ledger) gets the derived values, so an XP award that crosses a threshold
// changes them all at once.
func (s *service) GetCharacter(ctx context.Context, id int64) (*Character, error) {
	c, err := s.reader.GetCharacter(ctx, id)
	if err != nil {
		return nil, err
	}
	ledger, err := s.xp.EntriesTotal(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("character %d XP: %w", id, err)
	}
	if c.Derived, err = Derive(c, ledger, s.rules); err != nil {
		return nil, fmt.Errorf("character %d: %w", id, err)
	}
	// The native language is always among the languages (ADR 0032 step 4.1),
	// a character stored before it was added included; the warnings follow
	// what is read (steps 4.2 and 8.4).
	native := s.languages.Native(c.Race)
	c.Details = c.Details.Known(s.languages).WithNative(native)
	c.Warnings = c.Details.Warnings(c.Class)
	if w, over := c.Details.LanguageWarning(c.Int, native); over {
		c.Warnings = append(c.Warnings, w)
	}
	return c, nil
}

// List names each character with its derived level (ADR 0030 step 3).
func (s *service) List(ctx context.Context, owner uuid.UUID) ([]Summary, error) {
	list, err := s.owners.CharactersOf(ctx, owner)
	if err != nil {
		return nil, err
	}
	for i := range list {
		c, err := s.GetCharacter(ctx, list[i].ID)
		if err != nil {
			return nil, err
		}
		list[i].Level = c.Derived.RealizedLevel
	}
	return list, nil
}

// SetCurrent writes under the writer lock like every mutation (ADR 0025 step
// 4), outside the action log: choosing a character is no move of the game and
// is not undone. This server forgets the owner's sessions once it committed, so
// their next request plays the new character (ADR 0029 step 4.7).
func (s *service) SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error {
	if err := s.tx.InTx(ctx, func(ctx context.Context) error {
		return s.owners.SetCurrentCharacter(ctx, owner, id)
	}); err != nil {
		return err
	}
	s.sessions.ForgetUser(owner)
	return nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
