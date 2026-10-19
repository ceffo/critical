// Package creationrollsvc owns the rolls a character's creation offers
// (ADR 0032 steps 6 and 7): the six ability scores by one of DMG chapter 1's
// methods, and the max HP from level 1 to the starting level. A roll goes
// through the dice port, returns its whole breakdown and writes nothing: it
// fills fields the player may still edit, and the creation validates a rolled
// value exactly like a typed one (charactersvc.Create).
package creationrollsvc

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Scores rolls the method's dice (step 6). A *domain.FieldError on
	// "method" (unknown) or "race" (unknown).
	Scores(ctx context.Context, in ScoresInput) (*ScoresResult, error)
	// HP rolls the max HP from level 1 to the level in.XP gives (step 7). A
	// *domain.FieldError on "class" (unknown, not_creatable), "con" or "xp"
	// (out_of_range).
	HP(ctx context.Context, in HPInput) (*HPResult, error)
}

// ScoresInput is a score roll: a method code (ruletablesvc.ScoreMethods)
// and, optionally, the race whose PHB Table 8 adjustments the answer shows.
type ScoresInput struct {
	Method string
	// Race is a charactersvc.Races code, "" for none chosen yet (no
	// adjustments in the answer).
	Race string
}

// ScoreRoll is one total: its dice in the order drawn, the dropped one
// (method V) and the total of the dice kept.
type ScoreRoll struct {
	Dice []int
	// Dropped is the index in Dice of the die dropped, nil for none.
	Dropped *int
	Total   int
	// Ability is the ability the total belongs to for the methods that roll
	// in ability order (I and II), "" when the player assigns it.
	Ability string
}

// ScoresResult is the score roll's breakdown.
type ScoresResult struct {
	Method ruletablesvc.ScoreMethod
	// Rolls are the totals in the order rolled.
	Rolls []ScoreRoll
	// Adjustments are PHB Table 8 for the race by ability code, nil without
	// a race.
	Adjustments map[string]int
	// Scores are method I's finished scores by ability code, the race's
	// adjustment included (the stored score is the adjusted one, ADR 0030
	// step 1); nil for the methods the player arranges.
	Scores map[string]int
}

// HPInput is a max HP roll: the class, the Constitution score as it stands
// on the sheet and the starting XP.
type HPInput struct {
	Class string
	Con   int
	XP    int
}

// HPLevel is one level's gain: one hit die with the Constitution adjustment
// up to the name level (step 7.1), the fixed gain above it (step 7.2).
type HPLevel struct {
	Level int
	// HitDie is the die's sides, 0 for a fixed gain.
	HitDie int
	// Rolled is the die as rolled, Counted its value after Table 3's raised
	// low dice (Constitution 20 and up); nil for a fixed gain.
	Rolled, Counted *int
	// ConAdjustment is PHB Table 3's adjustment, 0 for a fixed gain.
	ConAdjustment int
	Gain          int
	Fixed         bool
}

// HPResult is the max HP roll's breakdown: one row per level from 1 to
// Level, and their sum.
type HPResult struct {
	Class string
	// Level is the starting level (ADR 0030 step 3 at the XP).
	Level  int
	Levels []HPLevel
	Total  int
}

// Rules are the tables the rolls read.
type Rules interface {
	// RaceAdjustmentsFor is PHB Table 8 for a race.
	RaceAdjustmentsFor(race string) (map[string]int, error)
	// LevelFor is the class's level at xp (ADR 0030 step 3).
	LevelFor(class string, xp int) (int, error)
	// HitDiceFor is the group's hit die, name level and fixed gain after it.
	HitDiceFor(group string) (ruletablesvc.HitDiceRow, error)
	// ConstitutionFor is PHB Table 3.
	ConstitutionFor(con int) (ruletablesvc.ConstitutionRow, error)
}

// Dice is the dice port.
type Dice interface {
	RollDie(sides int) int
}

// Deps are the service's collaborators.
type Deps struct {
	Rules Rules
	Dice  Dice
}

type service struct {
	rules Rules
	dice  Dice
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.Rules == nil || d.Dice == nil {
		return nil, errors.New("creationrollsvc: nil dependency")
	}
	return &service{rules: d.Rules, dice: d.Dice}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Scores is ADR 0032 step 6: the method's totals, each its dice drawn in
// order, method V's lowest die dropped; methods I and II give each total its
// ability in order (II two per ability, together).
func (s *service) Scores(_ context.Context, in ScoresInput) (*ScoresResult, error) {
	m, ok := ruletablesvc.ScoreMethodFor(in.Method)
	if !ok {
		return nil, &domain.FieldError{Field: "method", Message: charactersvc.CodeUnknown}
	}
	res := &ScoresResult{Method: m, Rolls: make([]ScoreRoll, 0, m.Rolls)}
	if in.Race != "" {
		if !slices.Contains(charactersvc.Races, in.Race) {
			return nil, &domain.FieldError{Field: "race", Message: charactersvc.CodeUnknown}
		}
		adj, err := s.rules.RaceAdjustmentsFor(in.Race)
		if err != nil {
			return nil, fmt.Errorf("score roll: %w", err)
		}
		res.Adjustments = adj
	}
	perAbility := 0
	if m.Assign == ruletablesvc.AssignInOrder || m.Assign == ruletablesvc.AssignKeepPerAbility {
		perAbility = m.Rolls / len(ruletablesvc.Abilities)
	}
	for i := range m.Rolls {
		r := s.roll(m)
		if perAbility > 0 {
			r.Ability = ruletablesvc.Abilities[i/perAbility]
		}
		res.Rolls = append(res.Rolls, r)
	}
	if m.Assign == ruletablesvc.AssignInOrder {
		res.Scores = make(map[string]int, len(res.Rolls))
		for _, r := range res.Rolls {
			res.Scores[r.Ability] = r.Total + res.Adjustments[r.Ability]
		}
	}
	return res, nil
}

// roll is one total of m: m.Dice d6 drawn in order, the lowest dropped (the
// first of equal lowest) when m.DropLowest.
func (s *service) roll(m ruletablesvc.ScoreMethod) ScoreRoll {
	r := ScoreRoll{Dice: make([]int, m.Dice)}
	for i := range r.Dice {
		r.Dice[i] = s.dice.RollDie(6)
		r.Total += r.Dice[i]
	}
	if m.DropLowest {
		low := 0
		for i, d := range r.Dice {
			if d < r.Dice[low] {
				low = i
			}
		}
		r.Dropped = &low
		r.Total -= r.Dice[low]
	}
	return r
}

// HP is ADR 0032 step 7: for each level from 1 to the starting level, in
// order, one hit die with the Constitution adjustment up to the class
// group's name level (ADR 0030 step 7.1's arithmetic, shared with the
// level-up roll), the group's fixed gain above it (step 7.2).
func (s *service) HP(_ context.Context, in HPInput) (*HPResult, error) {
	if !slices.Contains(ruletablesvc.Classes, in.Class) {
		return nil, &domain.FieldError{Field: "class", Message: charactersvc.CodeUnknown}
	}
	if !slices.Contains(charactersvc.CreatableClasses, in.Class) {
		return nil, &domain.FieldError{Field: "class", Message: charactersvc.CodeNotCreatable}
	}
	if in.Con < charactersvc.MinAbility || in.Con > charactersvc.MaxAbility {
		return nil, &domain.FieldError{Field: "con", Message: charactersvc.CodeOutOfRange}
	}
	if in.XP < charactersvc.MinStartingXP || in.XP > charactersvc.MaxStartingXP {
		return nil, &domain.FieldError{Field: "xp", Message: charactersvc.CodeOutOfRange}
	}
	group, err := ruletablesvc.ClassGroup(in.Class)
	if err != nil {
		return nil, fmt.Errorf("hp roll: %w", err)
	}
	level, err := s.rules.LevelFor(in.Class, in.XP)
	if err != nil {
		return nil, fmt.Errorf("hp roll: %w", err)
	}
	hd, err := s.rules.HitDiceFor(group)
	if err != nil {
		return nil, fmt.Errorf("hp roll: %w", err)
	}
	con, err := s.rules.ConstitutionFor(in.Con)
	if err != nil {
		return nil, fmt.Errorf("hp roll: %w", err)
	}
	res := &HPResult{Class: in.Class, Level: level, Levels: make([]HPLevel, 0, level)}
	for n := 1; n <= level; n++ {
		row := HPLevel{Level: n}
		if n <= hd.NameLevel {
			rolled := s.dice.RollDie(hd.Die)
			counted, adj, gain := ruletablesvc.HitDieGain(group, rolled, con)
			row.HitDie, row.Rolled, row.Counted, row.ConAdjustment, row.Gain = hd.Die, &rolled, &counted, adj, gain
		} else {
			row.Gain, row.Fixed = hd.GainAfter, true
		}
		res.Levels = append(res.Levels, row)
		res.Total += row.Gain
	}
	return res, nil
}
