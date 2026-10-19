// Package dicesvc owns the dice composer: free rolls of a formula such as
// 2d6+d4-1, rolled on the server and recorded as dice entries of the action
// log, and the favourite formulas of a character. A free roll changes no
// state: its entry carries no snapshot, so popping it only removes it.
//
// The formula grammar and its limits are in formula.go and docs/backend.md.
package dicesvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// MaxFavourites is how many formulas a character can keep.
const MaxFavourites = 50

// ErrTooManyFavourites is the conflict returned beyond MaxFavourites; it also
// wraps domain.ErrConflict.
var ErrTooManyFavourites = errors.New("too many favourites")

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Roll rolls the formula on the server and records it in the action
	// log, in one transaction. An invalid formula is a domain.FieldError on
	// "formula" and nothing is rolled.
	Roll(ctx context.Context, characterID int64, formula string) (*Result, error)
	// Favourites returns the character's favourites in the user's order: the
	// order they were saved in until ReorderFavourites sets another.
	Favourites(ctx context.Context, characterID int64) ([]Favourite, error)
	// AddFavourite saves the formula in its normal form. Saving a formula
	// already saved returns the existing favourite. Beyond MaxFavourites it
	// wraps domain.ErrConflict.
	AddFavourite(ctx context.Context, characterID int64, formula string) (*Favourite, error)
	// DeleteFavourite removes a favourite; an unknown id wraps
	// domain.ErrNotFound.
	DeleteFavourite(ctx context.Context, characterID, id int64) error
	// ReorderFavourites sets the full order, in one transaction. ids must be
	// exactly the character's favourite ids, each once: anything else is a
	// domain.FieldError on "ids" and the order is unchanged.
	ReorderFavourites(ctx context.Context, characterID int64, ids []int64) error
}

// DAL is the driven port over the favourites. Every method joins the ctx
// transaction.
type DAL interface {
	List(ctx context.Context, characterID int64) ([]Favourite, error)
	Insert(ctx context.Context, characterID int64, formula string) (int64, error)
	// Delete reports whether a row was removed.
	Delete(ctx context.Context, characterID, id int64) (bool, error)
	// SetPosition places a favourite in the order; it reports whether the row
	// existed. Insert puts a new favourite after the last one.
	SetPosition(ctx context.Context, characterID, id, position int64) (bool, error)
}

// Dice is the dice port.
type Dice interface {
	RollDie(sides int) int
}

// Log records a roll.
type Log interface {
	actionlog.Recorder
}

// Deps are the service's collaborators.
type Deps struct {
	DAL  DAL
	Dice Dice
	Log  Log
	Tx   domain.TxRunner
}

type service struct {
	dal  DAL
	dice Dice
	log  Log
	tx   domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.DAL == nil || d.Dice == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("dicesvc: nil dependency")
	}
	return &service{dal: d.DAL, dice: d.Dice, log: d.Log, tx: d.Tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module declares no codec: free rolls change no state.
func (s *service) Module() domain.Module { return domain.Module{Name: "dicesvc"} }

// Favourite is a saved formula, in normal form.
type Favourite struct {
	ID      int64
	Formula string
}

// RolledGroup is the dice of one group with what they rolled. Subtotal is
// signed: negative for a subtracted group.
type RolledGroup struct {
	Sides    int   `json:"sides"`
	Rolls    []int `json:"rolls"`
	Negative bool  `json:"negative"`
	Subtotal int   `json:"subtotal"`
}

// Result is one free roll: the normal form of the formula, every die rolled,
// the constant and the total.
type Result struct {
	Formula    string
	Groups     []RolledGroup
	Modifier   int
	Total      int
	LogEntryID int64
}

// rollRecord is dicesvc's JSON encoding of a roll inside its dice entry
// (payload). Entries already in the log must stay decodable: add fields,
// never rename them.
type rollRecord struct {
	Formula  string        `json:"formula"`
	Groups   []RolledGroup `json:"groups"`
	Modifier int           `json:"modifier"`
	Total    int           `json:"total"`
}

// Roll parses the formula, then rolls its groups in order of first
// appearance, one RollDie call per die, and records the dice entry.
func (s *service) Roll(ctx context.Context, characterID int64, formula string) (*Result, error) {
	f, err := Parse(formula)
	if err != nil {
		return nil, err
	}
	rec := rollRecord{Formula: f.String(), Groups: make([]RolledGroup, 0, len(f.Groups)), Modifier: f.Modifier}
	rec.Total = f.Modifier
	for _, g := range f.Groups {
		rg := RolledGroup{Sides: g.Sides, Rolls: make([]int, 0, g.Count), Negative: g.Negative}
		for range g.Count {
			v := s.dice.RollDie(g.Sides)
			rg.Rolls = append(rg.Rolls, v)
			rg.Subtotal += v
		}
		if g.Negative {
			rg.Subtotal = -rg.Subtotal
		}
		rec.Total += rg.Subtotal
		rec.Groups = append(rec.Groups, rg)
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("encode dice payload: %w", err)
	}
	var entryID int64
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		id, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindDice, Payload: payload,
		})
		if err != nil {
			return fmt.Errorf("record dice roll: %w", err)
		}
		entryID = id
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Result{
		Formula: rec.Formula, Groups: rec.Groups, Modifier: rec.Modifier, Total: rec.Total, LogEntryID: entryID,
	}, nil
}

func (s *service) Favourites(ctx context.Context, characterID int64) ([]Favourite, error) {
	return s.dal.List(ctx, characterID)
}

func (s *service) AddFavourite(ctx context.Context, characterID int64, formula string) (*Favourite, error) {
	f, err := Parse(formula)
	if err != nil {
		return nil, err
	}
	normal := f.String()
	var out *Favourite
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		all, err := s.dal.List(ctx, characterID)
		if err != nil {
			return err
		}
		for _, fav := range all {
			if fav.Formula == normal {
				out = &fav
				return nil
			}
		}
		if len(all) >= MaxFavourites {
			return fmt.Errorf("%w: %w (at most %d)", domain.ErrConflict, ErrTooManyFavourites, MaxFavourites)
		}
		id, err := s.dal.Insert(ctx, characterID, normal)
		if err != nil {
			return err
		}
		out = &Favourite{ID: id, Formula: normal}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *service) DeleteFavourite(ctx context.Context, characterID, id int64) error {
	removed, err := s.dal.Delete(ctx, characterID, id)
	if err != nil {
		return err
	}
	if !removed {
		return fmt.Errorf("%w: favourite %d", domain.ErrNotFound, id)
	}
	return nil
}

func (s *service) ReorderFavourites(ctx context.Context, characterID int64, ids []int64) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		all, err := s.dal.List(ctx, characterID)
		if err != nil {
			return err
		}
		known := make(map[int64]bool, len(all))
		for _, f := range all {
			known[f.ID] = true
		}
		if len(ids) != len(all) {
			return domain.FieldErrorf("ids", "must list each of the %d favourites once", len(all))
		}
		for _, id := range ids {
			if !known[id] {
				return domain.FieldErrorf("ids", "unknown or repeated favourite %d", id)
			}
			delete(known, id)
		}
		for i, id := range ids {
			if _, err := s.dal.SetPosition(ctx, characterID, id, int64(i+1)); err != nil {
				return err
			}
		}
		return nil
	})
}
