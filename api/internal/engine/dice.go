package engine

import (
	"math/rand"
)

type DiceRoller interface {
	RollDie(sides int) int
	RollD20() int
	RollD100() int
}

type realDiceRoller struct {
	rnd *rand.Rand
}

func NewDiceRoller(src rand.Source) DiceRoller {
	return &realDiceRoller{
		rnd: rand.New(src),
	}
}

func (dr *realDiceRoller) RollDie(sides int) int {
	if sides <= 0 {
		return 0
	}
	return dr.rnd.Intn(sides) + 1
}

func (dr *realDiceRoller) RollD20() int {
	return dr.RollDie(20)
}

func (dr *realDiceRoller) RollD100() int {
	return dr.RollDie(100)
}
