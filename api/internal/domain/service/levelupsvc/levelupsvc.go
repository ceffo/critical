// Package levelupsvc owns the level-up roll (ADR 0030 step 7): when the
// level a character's XP gives is above the level its max HP was last set
// for, each pending level's hit points are rolled on the server (or entered
// from the die the player rolled at the table), lowest level first, one
// request each. A roll raises the max HP by its gain and the max HP's level
// by 1, in one action-log transaction whose snapshot the journal's Undo
// writes back (ADR 0018).
package levelupsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ErrNotPending is a roll for a level that is not the lowest pending one:
// none is pending, or the page was stale (another tab rolled it). Nothing is
// rolled or written.
var ErrNotPending = fmt.Errorf("level-up not pending: %w", domain.ErrConflict)

// Field error codes of a roll's entered die. The web words them.
const (
	// CodeOutOfRange is an entered die outside 1 to the hit die's sides.
	CodeOutOfRange = "out_of_range"
	// CodeFixedGain is an entered die for a level above the name level,
	// whose gain is fixed (step 7.2): there is no die to enter.
	CodeFixedGain = "fixed_gain"
)

// StateKey is the action-log state key of a character's hit points: the max
// HP and the level it is set for.
const StateKey = "hit_points"

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Roll records the hit points of in.Level, which must be the lowest
	// pending level (else ErrNotPending), and returns the breakdown. A
	// *domain.FieldError on "die" for an entered die that is out of range or
	// not wanted (a fixed gain).
	Roll(ctx context.Context, characterID int64, in RollInput) (*Result, error)
	// Restorers writes the hit points back when a roll's entry is popped (and
	// again when it is redone), keyed by StateKey.
	Restorers() map[string]actionlog.Restorer
}

// RollInput is one level-up roll.
type RollInput struct {
	// Level is the level rolled for, as the page last read it: the lowest
	// pending one.
	Level int
	// Die is the die the player rolled at the table (the manual entry);
	// nil rolls it on the server.
	Die *int
}

// Result is the roll's full breakdown (ADR 0030 step 7.3).
type Result struct {
	// Level is the level reached.
	Level int
	// HitDie is the die's sides, 0 for a fixed gain.
	HitDie int
	// Rolled is the die as rolled (or entered), nil for a fixed gain.
	Rolled *int
	// Counted is the die after Table 3's low-die floor (equal to Rolled
	// below Constitution 20), nil for a fixed gain.
	Counted *int
	// ConAdjustment is PHB Table 3's hit point adjustment, 0 for a fixed
	// gain.
	ConAdjustment int
	// Gain is what the max HP rises by.
	Gain int
	// Fixed is a level above the name level (step 7.2).
	Fixed bool
	// Manual is a die the player entered.
	Manual bool
	// HPMaxBefore and HPMaxAfter are the max HP around the roll.
	HPMaxBefore, HPMaxAfter int
	// Pending are the levels still owed after this one.
	Pending []charactersvc.PendingLevel
}

// DAL writes a character's hit points. It joins the ctx transaction.
type DAL interface {
	// SetHitPoints sets the max HP and the level it is set for;
	// domain.ErrCharacterNotFound for an unknown character.
	SetHitPoints(ctx context.Context, id int64, hpMax, hpLevel int) error
}

// Characters reads the character with its derived block (the pending
// levels).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Rules is PHB Table 3.
type Rules interface {
	ConstitutionFor(con int) (ruletablesvc.ConstitutionRow, error)
}

// Dice is the dice port.
type Dice interface {
	RollDie(sides int) int
}

// Log is the slice of the action log a roll is recorded into.
type Log interface {
	actionlog.Recorder
}

// Deps are the service's collaborators.
type Deps struct {
	DAL        DAL
	Characters Characters
	Rules      Rules
	Dice       Dice
	Log        Log
	Tx         domain.TxRunner
}

type service struct {
	dal   DAL
	chars Characters
	rules Rules
	dice  Dice
	log   Log
	tx    domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Characters == nil || d.Rules == nil || d.Dice == nil || d.Log == nil || d.Tx == nil {
		return nil, errors.New("levelupsvc: nil dependency")
	}
	return &service{dal: d.DAL, chars: d.Characters, rules: d.Rules, dice: d.Dice, log: d.Log, tx: d.Tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Roll reads the pending levels, rolls (or takes) the die and records the
// gain, all in one transaction: two requests for the same level cannot both
// roll it.
func (s *service) Roll(ctx context.Context, characterID int64, in RollInput) (*Result, error) {
	var res *Result
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		pending := c.Derived.LevelUp.Pending
		if len(pending) == 0 || pending[0].Level != in.Level {
			return ErrNotPending
		}
		if res, err = s.gain(c, pending[0], in.Die); err != nil {
			return err
		}
		res.Pending = pending[1:]
		res.HPMaxBefore, res.HPMaxAfter = c.HPMax, c.HPMax+res.Gain
		before := hitPoints{HPMax: c.HPMax, HPLevel: c.Derived.LevelUp.HPLevel}
		after := hitPoints{HPMax: res.HPMaxAfter, HPLevel: res.Level}
		if err := s.dal.SetHitPoints(ctx, characterID, after.HPMax, after.HPLevel); err != nil {
			return err
		}
		return s.record(ctx, characterID, *res, before, after)
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// gain is ADR 0030 steps 7.1 and 7.2 for the pending level p of c.
func (s *service) gain(c *charactersvc.Character, p charactersvc.PendingLevel, entered *int) (*Result, error) {
	res := &Result{Level: p.Level, HitDie: p.Die, Manual: entered != nil}
	// Step 7.2: above the name level, a fixed gain: no die, no Constitution.
	if p.Die == 0 {
		if entered != nil {
			return nil, &domain.FieldError{Field: "die", Message: CodeFixedGain}
		}
		res.Gain, res.Fixed = p.FixedGain, true
		return res, nil
	}
	// Step 7.1: one hit die of the group, through the dice port unless the
	// player entered the one rolled at the table.
	rolled := 0
	if entered != nil {
		if *entered < 1 || *entered > p.Die {
			return nil, &domain.FieldError{Field: "die", Message: CodeOutOfRange}
		}
		rolled = *entered
	} else {
		rolled = s.dice.RollDie(p.Die)
	}
	row, err := s.rules.ConstitutionFor(c.Con)
	if err != nil {
		return nil, fmt.Errorf("level-up: %w", err)
	}
	// Table 3's footnotes from Constitution 20: a low die counts as the
	// floor; the die shown is the one rolled.
	counted := max(rolled, row.LowDieMin)
	// Warriors take the bracketed adjustment, every other class the plain
	// one (at most +2).
	res.ConAdjustment = row.HP
	if group, _ := ruletablesvc.ClassGroup(c.Class); group == "warrior" {
		res.ConAdjustment = row.HPWarrior
	}
	res.Rolled, res.Counted = &rolled, &counted
	// The gain is at least 1 (a low die with a negative adjustment never
	// lowers or keeps the max HP).
	res.Gain = max(1, counted+res.ConAdjustment)
	return res, nil
}
