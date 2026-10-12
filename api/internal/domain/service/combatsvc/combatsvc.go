// Package combatsvc owns attack resolution and saving throws. It has no DAL of
// its own — it consumes other services and infra adapters through narrow ports
// declared here (clean_architecture.md §4.5).
package combatsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	ResolveAttack(ctx context.Context, characterID int64, weaponID string, targetAC int) (*RollResult, error)
	ResolveSavingThrow(ctx context.Context, characterID int64, category SaveCategory) (*RollResult, error)
	ListWeapons(ctx context.Context) ([]Weapon, error)
}

// Characters is the narrow inter-service port (just the read we need).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Buffs is the narrow inter-service port.
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

type service struct {
	chars   Characters
	buffs   Buffs
	dice    Dice
	weapons Weapons
}

// New constructs the service.
func New(chars Characters, buffs Buffs, dice Dice, weapons Weapons) (SVC, error) {
	if chars == nil || buffs == nil || dice == nil || weapons == nil {
		return nil, fmt.Errorf("combatsvc: nil dependency")
	}
	return &service{chars: chars, buffs: buffs, dice: dice, weapons: weapons}, nil
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(_ *Config, chars Characters, buffs Buffs, dice Dice, weapons Weapons) (SVC, error) {
	return New(chars, buffs, dice, weapons)
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
