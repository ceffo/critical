// Package acsvc owns the character's armour class (ADR 0033 steps 10 to 13):
// derived on every read from the worn body armour and shield, the sheet's
// Dexterity, the powers of the equipped pieces and the buffs that apply,
// never stored; and the override the player sets for what the model cannot
// hold (the Shield spell), one logged move undone by its snapshot. Nothing
// else in the app reads the character's AC (the attack reads the target's).
package acsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Get is the character's AC with its breakdown and its override.
	Get(ctx context.Context, characterID int64) (*AC, error)
	// SetOverride sets the override (step 13), one ac_override entry; the
	// same override again logs nothing. A *domain.FieldError on "value"
	// (CodeOutOfRange) or "label" (CodeTooLong); nothing is logged then.
	SetOverride(ctx context.Context, characterID int64, o Override) (*AC, error)
	// ClearOverride clears it, one ac_override entry; none set logs nothing.
	ClearOverride(ctx context.Context, characterID int64) (*AC, error)
}

// Items reads the character's pieces (itemsvc).
type Items interface {
	List(ctx context.Context, characterID int64) ([]itemsvc.Item, error)
}

// Effects reads the buffs with whether each applies, and the powers that are
// on (buffsvc: the effective STR gates both).
type Effects interface {
	ListStatus(ctx context.Context, characterID int64) ([]buffsvc.BuffStatus, error)
	ListPowers(ctx context.Context, characterID int64) ([]buffsvc.ItemPower, error)
}

// Characters reads the sheet's Dexterity.
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Tables is PHB Table 2 (ruletablesvc.Lookup's DexMods): the defensive
// adjustment for a Dexterity score.
type Tables interface {
	DexMods(dex int) (missile, ac int, err error)
}

// Overrides stores the override (characters.ac_override and
// ac_override_label). Both join the ctx transaction; nil is none.
// domain.ErrCharacterNotFound for an unknown character.
type Overrides interface {
	GetACOverride(ctx context.Context, characterID int64) (*Override, error)
	SetACOverride(ctx context.Context, characterID int64, o *Override) error
}

// Log records the override's entries.
type Log interface {
	actionlog.Recorder
}

// Deps are the service's collaborators.
type Deps struct {
	Items      Items
	Effects    Effects
	Characters Characters
	Tables     Tables
	Overrides  Overrides
	Log        Log
	Tx         domain.TxRunner
}

type service struct {
	items     Items
	effects   Effects
	chars     Characters
	tables    Tables
	overrides Overrides
	log       Log
	tx        domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.Items == nil || d.Effects == nil || d.Characters == nil || d.Tables == nil || d.Overrides == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("acsvc: nil dependency")
	}
	return &service{
		items: d.Items, effects: d.Effects, chars: d.Characters, tables: d.Tables,
		overrides: d.Overrides, log: d.Log, tx: d.Tx,
	}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Get reads the inputs and derives the AC (ADR 0033 step 10); the override
// rides along (step 13), the derived AC and its breakdown stay beside it.
func (s *service) Get(ctx context.Context, characterID int64) (*AC, error) {
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, err
	}
	_, dexAdj, err := s.tables.DexMods(c.Dex)
	if err != nil {
		return nil, fmt.Errorf("dexterity %d: %w", c.Dex, err)
	}
	items, err := s.items.List(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	buffs, err := s.effects.ListStatus(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list buffs: %w", err)
	}
	powers, err := s.effects.ListPowers(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("list powers: %w", err)
	}
	o, err := s.overrides.GetACOverride(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get ac override: %w", err)
	}
	ac := Derive(Inputs{Items: items, Buffs: buffs, Powers: powers, Dexterity: c.Dex, DexAdjust: dexAdj})
	ac.Override = o
	ac.Warnings = armourWarnings(c.Class, items)
	return &ac, nil
}

// armourWarnings are the class warnings (ADR 0033 step 20) of the worn body
// armour and shield: what the AC card shows beside the breakdown.
func armourWarnings(class string, items []itemsvc.Item) []itemsvc.Warning {
	worn := make(map[string]bool, len(items))
	for _, it := range items {
		worn[it.ID] = it.Category == itemsvc.CategoryArmour || it.Category == itemsvc.CategoryShield
	}
	var out []itemsvc.Warning
	for _, w := range itemsvc.ClassWarnings(class, items) {
		if worn[w.ItemID] {
			out = append(out, w)
		}
	}
	return out
}
