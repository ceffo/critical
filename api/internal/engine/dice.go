// Package engine contains the core game logic for combat, dice rolls, and effects.
package engine

import (
	"crypto/rand"
	"math/big"
)

// DiceRoller defines the interface for rolling dice.
type DiceRoller interface {
	RollDie(sides int) int
	RollD20() int
	RollD100() int
}

type cryptoDiceRoller struct{}

// NewDiceRoller returns a new DiceRoller that uses cryptographically secure
// random numbers.
func NewDiceRoller() DiceRoller {
	return &cryptoDiceRoller{}
}

// RollDie rolls a die with the given number of sides.
func (dr *cryptoDiceRoller) RollDie(sides int) int {
	if sides <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
	if err != nil {
		// This should practically never happen with crypto/rand
		return 1
	}
	return int(n.Int64()) + 1
}

// RollD20 rolls a 20-sided die.
func (dr *cryptoDiceRoller) RollD20() int {
	return dr.RollDie(20)
}

// RollD100 rolls a 100-sided die.
func (dr *cryptoDiceRoller) RollD100() int {
	return dr.RollDie(100)
}
