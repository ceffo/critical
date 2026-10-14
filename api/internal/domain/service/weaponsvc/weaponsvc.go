// Package weaponsvc adds, edits and removes weapons by hand (critical-qki.32),
// instead of editing seeds/weapons.json. Each change is one action-log entry
// carrying the weapon's "weapons:<id>" snapshot, so popping the newest entry
// undoes it (an add is deleted, an edit restored, a remove re-inserted with
// its id and place). Attack entries store the weapon's name, so old log lines
// keep reading right after a removal. combatsvc reads the weapons table
// through its own port; this package only writes it.
package weaponsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ErrWeaponNotFound is returned when a weapon id is unknown.
var ErrWeaponNotFound = errors.New("weapon not found")

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Create adds a weapon and logs it with its snapshot, in one
	// transaction. A *domain.FieldError for invalid input.
	Create(ctx context.Context, characterID int64, in WeaponInput) (*ChangeResult, error)
	// Update edits the set fields of a weapon and logs the before/after
	// fields. ErrWeaponNotFound, *domain.FieldError.
	Update(ctx context.Context, characterID int64, id string, p WeaponPatch) (*ChangeResult, error)
	// Delete removes a weapon and logs it; popping the entry brings it back.
	// ErrWeaponNotFound.
	Delete(ctx context.Context, characterID int64, id string) (*ChangeResult, error)
	// Restorers maps the weapons state key to the Restorer that writes a
	// "before" snapshot back when an action-log entry is voided.
	Restorers() map[string]actionlog.Restorer
}

// DAL is the driven port. Every method joins the transaction carried by ctx.
type DAL interface {
	List(ctx context.Context) ([]Weapon, error)
	// Get returns one weapon, or ErrWeaponNotFound.
	Get(ctx context.Context, id string) (Weapon, error)
	Insert(ctx context.Context, w Weapon) error
	// Update overwrites every field (position included); ErrWeaponNotFound.
	Update(ctx context.Context, w Weapon) error
	// Delete removes a weapon; ErrWeaponNotFound.
	Delete(ctx context.Context, id string) error
}

// Log is the slice of the action log weapon changes are recorded into.
type Log interface {
	actionlog.Recorder
}

// Deps are the collaborators besides the DAL.
type Deps struct {
	Log Log
	Tx  domain.TxRunner
}

type service struct {
	dal DAL
	log Log
	tx  domain.TxRunner
}

// New constructs the service.
func New(dal DAL, d Deps) (SVC, error) {
	if dal == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("weaponsvc: nil dependency")
	}
	return &service{dal: dal, log: d.Log, tx: d.Tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
