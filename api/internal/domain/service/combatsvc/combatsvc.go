// Package combatsvc owns attack resolution and saving throws. It has no DAL of
// its own — it consumes other services and infra adapters through narrow ports
// declared here (clean_architecture.md §4.5).
package combatsvc

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
	// ResolveAttack rolls one attack, spends the ammunition of a missile
	// weapon and records the attack in the action log, all in one
	// transaction.
	ResolveAttack(ctx context.Context, characterID int64, req AttackRequest) (*RollResult, error)
	ResolveSavingThrow(ctx context.Context, characterID int64, category SaveCategory) (*RollResult, error)
	ListWeapons(ctx context.Context) ([]Weapon, error)
	// GetCritTable returns the house crit table (the placeholder default
	// until edited); SetCritTable replaces it after validating it.
	GetCritTable(ctx context.Context) (CritTable, error)
	SetCritTable(ctx context.Context, rows []CritRow) (CritTable, error)
}

// Characters is the narrow inter-service port (just the read we need).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Buffs is the narrow inter-service port. Aggregate takes the sheet's STR
// and returns the buff totals with the effective STR.
type Buffs interface {
	Aggregate(ctx context.Context, characterID int64, str int) (buffsvc.BuffModifiers, error)
}

// Dice is the dice adapter port.
type Dice interface {
	RollDie(sides int) int
	RollD20() int
	RollD100() int
}

// Weapons is the weapons-catalog port.
type Weapons interface {
	// Get returns the weapon, or an error wrapping ErrWeaponNotFound.
	Get(ctx context.Context, id string) (Weapon, error)
	List(ctx context.Context) ([]Weapon, error)
}

// CritStore stores the house crit table. List returns no rows until one was
// saved; Replace swaps the whole table and joins the ctx transaction.
type CritStore interface {
	List(ctx context.Context) ([]CritRow, error)
	Replace(ctx context.Context, rows []CritRow) error
}

// Items spends ammunition (itemsvc). Use must join the ctx transaction.
type Items interface {
	Use(ctx context.Context, itemID string) (itemsvc.Item, error)
}

// Rules is the PHB rule-table lookup (ruletablesvc.Lookup).
type Rules interface {
	StrengthMods(str, exceptional int) (hit, dmg int, err error)
	DexMods(dex int) (missile, ac int, err error)
}

// Log is the slice of the action log attacks are recorded into.
type Log interface {
	actionlog.Recorder
}

// Deps are the service's collaborators.
type Deps struct {
	Chars   Characters
	Buffs   Buffs
	Dice    Dice
	Weapons Weapons
	Crit    CritStore
	Items   Items
	Rules   Rules
	Log     Log
	Tx      domain.TxRunner
}

type service struct {
	chars   Characters
	buffs   Buffs
	dice    Dice
	weapons Weapons
	crit    CritStore
	items   Items
	rules   Rules
	log     Log
	tx      domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	return NewWithConfig(DefaultConfig(), d)
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(cfg *Config, d Deps) (SVC, error) {
	if cfg == nil {
		return nil, fmt.Errorf("combatsvc: nil config")
	}
	if d.Chars == nil || d.Buffs == nil || d.Dice == nil || d.Weapons == nil || d.Crit == nil ||
		d.Items == nil || d.Rules == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("combatsvc: nil dependency")
	}
	return &service{
		chars: d.Chars, buffs: d.Buffs, dice: d.Dice, weapons: d.Weapons, crit: d.Crit,
		items: d.Items, rules: d.Rules, log: d.Log, tx: d.Tx,
	}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
