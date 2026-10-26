// Package diceadpt is the cryptographically-secure DiceRoller implementation.
// It satisfies combatsvc.Dice.
package diceadpt

import (
	"crypto/rand"
	"math/big"
	mrand "math/rand/v2"
	"sync"
)

// Roller satisfies combatsvc.Dice. The zero value rolls from crypto/rand;
// NewSeeded rolls a repeatable sequence (the e2e runs only).
type Roller struct {
	mu     sync.Mutex
	seeded *mrand.Rand
}

// New returns a new Roller.
func New() *Roller { return &Roller{} }

// NewSeeded returns a Roller whose rolls are a pure function of seed: two
// rollers with the same seed give the same sequence for the same calls. It is
// for the e2e runs; the boot refuses the seed anywhere real data lives.
func NewSeeded(seed uint64) *Roller {
	//nolint:gosec // a repeatable sequence is the point; it never decides anything secret.
	return &Roller{seeded: mrand.New(mrand.NewPCG(seed, seed))}
}

// RollDie rolls a die with the given number of sides.
func (r *Roller) RollDie(sides int) int {
	if sides <= 0 {
		return 0
	}
	if r.seeded != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.seeded.IntN(sides) + 1
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
