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
	// Arrange is the scores a placement of a roll's totals gives (step 6):
	// the method's base, the totals placed and PHB Table 8's adjustment. It
	// rolls nothing. A *domain.FieldError on "method" or "race" (unknown),
	// "placed" (unknown ability, too_many totals) or "placed.<ability>"
	// (too_many, out_of_range, above_max).
	Arrange(ctx context.Context, in ArrangeInput) (*ArrangeResult, error)
	// Identity rolls the figures asked of height, weight and age (step 5.3).
	// A *domain.FieldError on "figures" (empty, unknown, duplicate), "race"
	// (unknown), "sex" (unknown), or the rolled figure itself (race_required,
	// sex_required).
	Identity(ctx context.Context, in IdentityInput) (*IdentityResult, error)
}

// The figures an identity roll rolls (ADR 0032 step 5.3), in the order they
// are drawn whatever the order asked.
const (
	FigureHeight = "height"
	FigureWeight = "weight"
	FigureAge    = "age"
)

// Figures are the identity figures, in the order an identity roll draws them.
var Figures = []string{FigureHeight, FigureWeight, FigureAge}

// The units a figure's table total is in.
const (
	UnitInch  = "in"
	UnitPound = "lb"
	UnitYear  = "year"
)

// IdentityInput is an identity roll: the race (required), the sex (required
// for a height or a weight) and the figures to roll.
type IdentityInput struct {
	Race, Sex string
	Figures   []string
}

// IdentityRoll is one figure's breakdown: the dice in the order drawn (Count
// dice of Sides), the table's Base for the race (and the sex), their Total in
// Unit (inches, pounds or years), and the Value in the stored unit (cm, kg or
// years) that fills the field.
type IdentityRoll struct {
	Figure string
	Dice   []int
	Sides  int
	Base   int
	Total  int
	Unit   string
	Value  float64
}

// IdentityResult is the identity roll's breakdown, one roll per figure asked.
type IdentityResult struct {
	Race, Sex string
	Rolls     []IdentityRoll
}

// The refusals of a placement beyond charactersvc's codes.
const (
	// CodeTooMany: more totals than the method keeps, or than one ability
	// takes.
	CodeTooMany = "too_many"
	// CodeAboveMax: method VI's dice raise an ability above the method's max.
	CodeAboveMax = "above_max"
)

// ArrangeInput is a placement the player chose: the method rolled, the race
// whose Table 8 adjustments apply ("" for none yet) and, by ability code, the
// totals placed there (method VI: the dice added to the base; the others: at
// most one total). The server keeps no roll: it checks each value against
// what the method can make, not against the roll.
type ArrangeInput struct {
	Method string
	Race   string
	Placed map[string][]int
}

// ArrangeResult is the placement's scores by ability code.
type ArrangeResult struct {
	Method ruletablesvc.ScoreMethod
	// Adjustments are PHB Table 8 for the race, nil without a race.
	Adjustments map[string]int
	// Rolled is the method's base plus the totals placed; an ability with
	// nothing placed and no base is absent.
	Rolled map[string]int
	// Scores are Rolled plus the race's adjustment: the score stored (ADR
	// 0030 step 1). Absent where Rolled is.
	Scores map[string]int
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

// Module declares no codec: the creation's rolls write nothing.
func (s *service) Module() domain.Module { return domain.Module{Name: "creationrollsvc"} }

// Scores is ADR 0032 step 6: the method's totals, each its dice drawn in
// order, method V's lowest die dropped; methods I and II give each total its
// ability in order (II two per ability, together).
func (s *service) Scores(_ context.Context, in ScoresInput) (*ScoresResult, error) {
	m, adj, err := s.methodAndRace(in.Method, in.Race)
	if err != nil {
		return nil, err
	}
	res := &ScoresResult{Method: m, Rolls: make([]ScoreRoll, 0, m.Rolls), Adjustments: adj}
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

// methodAndRace reads the method of code and Table 8 for race ("" for none:
// nil adjustments), refusing an unknown one on "method" or "race".
func (s *service) methodAndRace(code, race string) (ruletablesvc.ScoreMethod, map[string]int, error) {
	m, ok := ruletablesvc.ScoreMethodFor(code)
	if !ok {
		return m, nil, &domain.FieldError{Field: "method", Message: charactersvc.CodeUnknown}
	}
	if race == "" {
		return m, nil, nil
	}
	if !slices.Contains(charactersvc.Races, race) {
		return m, nil, &domain.FieldError{Field: "race", Message: charactersvc.CodeUnknown}
	}
	adj, err := s.rules.RaceAdjustmentsFor(race)
	if err != nil {
		return m, nil, fmt.Errorf("score method: %w", err)
	}
	return m, adj, nil
}

// Arrange is ADR 0032 step 6's arithmetic on the player's arrangement: an
// ability's score is the method's base (VI: 8, else 0) plus the totals placed
// on it, never above the method's max (VI: 18), plus PHB Table 8's
// adjustment for the race (the stored score is the adjusted one). Which
// total goes where is the player's choice; each value is checked against
// what one total of the method can be (VI: one d6; V: the three dice kept).
func (s *service) Arrange(_ context.Context, in ArrangeInput) (*ArrangeResult, error) {
	m, adj, err := s.methodAndRace(in.Method, in.Race)
	if err != nil {
		return nil, err
	}
	for a := range in.Placed {
		if !slices.Contains(ruletablesvc.Abilities, a) {
			return nil, &domain.FieldError{Field: "placed", Message: charactersvc.CodeUnknown}
		}
	}
	kept := m.Dice
	if m.DropLowest {
		kept--
	}
	perAbility := 1
	if m.Assign == ruletablesvc.AssignAddToBase {
		perAbility = m.Keep
	}
	res := &ArrangeResult{Method: m, Adjustments: adj, Rolled: map[string]int{}, Scores: map[string]int{}}
	count := 0
	for _, a := range ruletablesvc.Abilities {
		placed := in.Placed[a]
		field := "placed." + a
		if len(placed) > perAbility {
			return nil, &domain.FieldError{Field: field, Message: CodeTooMany}
		}
		count += len(placed)
		if len(placed) == 0 && m.Base == 0 {
			continue
		}
		rolled := m.Base
		for _, v := range placed {
			if v < kept || v > kept*6 {
				return nil, &domain.FieldError{Field: field, Message: charactersvc.CodeOutOfRange}
			}
			rolled += v
		}
		if m.Max > 0 && rolled > m.Max {
			return nil, &domain.FieldError{Field: field, Message: CodeAboveMax}
		}
		res.Rolled[a] = rolled
		res.Scores[a] = rolled + adj[a]
	}
	if count > m.Keep {
		return nil, &domain.FieldError{Field: "placed", Message: CodeTooMany}
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

// Identity is ADR 0032 step 5.3: for each figure asked, in the order height,
// weight, age, the table's dice for the race are drawn in order and added to
// its base (for the sex, in the case of a height and a weight); the answer
// holds the dice, the base, the total in the table's unit and the stored value.
func (s *service) Identity(_ context.Context, in IdentityInput) (*IdentityResult, error) {
	asked := map[string]bool{}
	for _, f := range in.Figures {
		switch {
		case !slices.Contains(Figures, f):
			return nil, &domain.FieldError{Field: "figures", Message: charactersvc.CodeUnknown}
		case asked[f]:
			return nil, &domain.FieldError{Field: "figures", Message: charactersvc.CodeDuplicate}
		}
		asked[f] = true
	}
	if len(asked) == 0 {
		return nil, &domain.FieldError{Field: "figures", Message: charactersvc.CodeEmpty}
	}
	var order []string // the figures asked, in drawing order
	for _, f := range Figures {
		if asked[f] {
			order = append(order, f)
		}
	}
	if in.Race == "" {
		return nil, &domain.FieldError{Field: order[0], Message: charactersvc.CodeRaceRequired}
	}
	row, ok := ruletablesvc.IdentityFor(in.Race)
	if !ok {
		return nil, &domain.FieldError{Field: "race", Message: charactersvc.CodeUnknown}
	}
	if in.Sex != "" && !slices.Contains(charactersvc.Sexes, in.Sex) {
		return nil, &domain.FieldError{Field: "sex", Message: charactersvc.CodeUnknown}
	}
	if in.Sex == "" {
		for _, f := range order {
			if f != FigureAge {
				return nil, &domain.FieldError{Field: f, Message: charactersvc.CodeSexRequired}
			}
		}
	}
	res := &IdentityResult{Race: in.Race, Sex: in.Sex, Rolls: make([]IdentityRoll, 0, len(order))}
	for _, f := range order {
		r := IdentityRoll{Figure: f}
		var dice ruletablesvc.Dice
		switch f {
		case FigureHeight:
			r.Base, _ = row.BaseHeight(in.Sex)
			dice, r.Unit = row.HeightDice, UnitInch
		case FigureWeight:
			r.Base, _ = row.BaseWeight(in.Sex)
			dice, r.Unit = row.WeightDice, UnitPound
		case FigureAge:
			r.Base, dice, r.Unit = row.StartAge, row.StartAgeDice, UnitYear
		}
		r.Sides, r.Dice, r.Total = dice.Sides, make([]int, dice.Count), r.Base
		for i := range r.Dice {
			r.Dice[i] = s.dice.RollDie(dice.Sides)
			r.Total += r.Dice[i]
		}
		switch f {
		case FigureHeight:
			r.Value = ruletablesvc.InchesToCm(float64(r.Total))
		case FigureWeight:
			r.Value = ruletablesvc.PoundsToKg(float64(r.Total))
		default:
			r.Value = float64(r.Total)
		}
		res.Rolls = append(res.Rolls, r)
	}
	return res, nil
}
