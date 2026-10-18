// Package turnsvc rolls turning undead (ADR 0030 step 16): a cleric names an
// undead type (a PHB Table 61 row) and the server reads the cell at the
// cleric's realized level, rolls what it calls for through the dice port and
// logs one roll entry. It changes no state; once per encounter is not
// enforced, and which undead are affected (the lowest Hit Dice first) is the
// DM's.
package turnsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// ErrClassCannotTurn means the character's class does not turn undead.
var ErrClassCannotTurn = errors.New("this class does not turn undead")

// Effects of a successful turn (ADR 0030 step 16.2).
const (
	EffectTurn    = "turn"
	EffectDestroy = "destroy"
)

// The dice of step 16: the d20 against a number, 2d6 undead affected, 2d4
// more for D*.
const (
	turnDie     = 20
	affectedDie = 6
	moreDie     = 4
)

// SVC is the turning service.
type SVC interface {
	domain.Service
	// Turn rolls turning the undead type for the character.
	Turn(ctx context.Context, characterID int64, undead string) (*Result, error)
}

// Characters reads the character with its derived Table 61 row
// (charactersvc.SVC).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Dice is the dice port.
type Dice interface {
	RollDie(sides int) int
}

// Log records the roll entry (actionlogsvc).
type Log interface {
	Record(ctx context.Context, e actionlog.NewEntry) (int64, error)
}

// Deps are the collaborators; all are required.
type Deps struct {
	Characters Characters
	Dice       Dice
	Log        Log
	Tx         domain.TxRunner
}

// Result is the full breakdown of a turn (step 16.5).
type Result struct {
	EntryID int64
	// Undead is the Table 61 row's code, HDMin and HDMax its Hit Dice (0:
	// by type only, and more).
	Undead       string
	HDMin, HDMax int
	// ClericLevel is the realized level whose column was read.
	ClericLevel int
	// Cell is the table's cell: a d20 target or an outcome.
	Cell ruletablesvc.TurningCell
	// D20 is the turning roll (0: the cell needs none).
	D20 int
	// Success: the undead are turned or destroyed.
	Success bool
	// Effect is EffectTurn or EffectDestroy on a success, empty otherwise.
	Effect string
	// Affected are the 2d6 undead affected on a success (nil otherwise);
	// More the 2d4 more of a D* cell (nil otherwise).
	Affected, More []int
}

type service struct{ Deps }

// New returns the service.
func New(d Deps) (SVC, error) {
	if d.Characters == nil || d.Dice == nil || d.Log == nil || d.Tx == nil {
		return nil, errors.New("turnsvc: all dependencies are required")
	}
	return &service{d}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Turn is ADR 0030 step 16: the class must turn undead and the type be a
// Table 61 row (refused, 400, before any die); a dash is refused before any
// die (16.3). A number rolls one d20, a success at or above it (16.1); T, D
// and D* need none (16.2). A success rolls 2d6 affected, D* 2d4 more (16.4).
// One turn_undead roll entry, no snapshot (16.5).
func (s *service) Turn(ctx context.Context, characterID int64, undead string) (*Result, error) {
	c, err := s.Characters.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}
	if c.Derived.Turning == nil {
		return nil, fmt.Errorf("%w: %s", ErrClassCannotTurn, c.Class)
	}
	var row *ruletablesvc.TurningTarget
	for i := range c.Derived.Turning {
		if c.Derived.Turning[i].Undead == undead {
			row = &c.Derived.Turning[i]
			break
		}
	}
	if row == nil {
		return nil, fmt.Errorf("%w: unknown undead type %q", domain.ErrInvalidInput, undead)
	}
	if row.Cell.Outcome == "cannot" {
		return nil, fmt.Errorf("%w: %s at level %d", ruletablesvc.ErrCannotTurn, undead, c.Derived.RealizedLevel)
	}
	res := &Result{
		Undead: row.Undead, HDMin: row.HDMin, HDMax: row.HDMax, ClericLevel: c.Derived.RealizedLevel, Cell: row.Cell,
	}
	switch row.Cell.Outcome {
	case "":
		res.D20 = s.Dice.RollDie(turnDie)
		res.Success = res.D20 >= row.Cell.Roll
		if res.Success {
			res.Effect = EffectTurn
		}
	case "turn":
		res.Success, res.Effect = true, EffectTurn
	case "destroy", "destroy_more":
		res.Success, res.Effect = true, EffectDestroy
	default:
		return nil, fmt.Errorf("turning undead: unknown cell outcome %q", row.Cell.Outcome)
	}
	if res.Success {
		res.Affected = []int{s.Dice.RollDie(affectedDie), s.Dice.RollDie(affectedDie)}
		if row.Cell.Outcome == "destroy_more" {
			res.More = []int{s.Dice.RollDie(moreDie), s.Dice.RollDie(moreDie)}
		}
	}
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		payload, err := json.Marshal(recordOf(res))
		if err != nil {
			return fmt.Errorf("encode turn payload: %w", err)
		}
		res.EntryID, err = s.Log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindTurnUndead, Payload: payload,
		})
		if err != nil {
			return fmt.Errorf("record turn: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// record is the turn_undead entry's payload: the type, the cell, the dice
// and the result (the web words it).
type record struct {
	Undead      string `json:"undead"`
	ClericLevel int    `json:"clericLevel"`
	Target      int    `json:"target,omitempty"`
	Outcome     string `json:"outcome,omitempty"`
	D20         int    `json:"d20,omitempty"`
	Success     bool   `json:"success"`
	Effect      string `json:"effect,omitempty"`
	Affected    []int  `json:"affected,omitempty"`
	More        []int  `json:"more,omitempty"`
}

func recordOf(r *Result) record {
	return record{
		Undead: r.Undead, ClericLevel: r.ClericLevel, Target: r.Cell.Roll, Outcome: r.Cell.Outcome,
		D20: r.D20, Success: r.Success, Effect: r.Effect, Affected: r.Affected, More: r.More,
	}
}
