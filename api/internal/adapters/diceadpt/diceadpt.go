// Package diceadpt is the cryptographically-secure DiceRoller implementation.
// It satisfies combatsvc.Dice.
package diceadpt

import (
	"crypto/rand"
	"math/big"
)

// Roller satisfies combatsvc.Dice.
type Roller struct{}

// New returns a new Roller.
func New() *Roller { return &Roller{} }

// RollDie rolls a die with the given number of sides.
func (r *Roller) RollDie(sides int) int {
	if sides <= 0 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(sides)))
	if err != nil {
		// crypto/rand failure on Linux is essentially impossible; bias to 1.
		return 1
	}
	return int(n.Int64()) + 1
}

// RollD20 rolls a 20-sided die.
func (r *Roller) RollD20() int { return r.RollDie(20) }

// RollD100 rolls a 100-sided die.
func (r *Roller) RollD100() int { return r.RollDie(100) }
