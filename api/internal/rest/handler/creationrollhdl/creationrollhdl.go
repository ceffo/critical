// Package creationrollhdl exposes the creation's rolls (ADR 0032 steps 6 and
// 7): POST /api/characters/rolls/scores and POST /api/characters/rolls/hp.
// They need a session but no current character (a new player creating their
// first one has none), and write nothing.
package creationrollhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/creationrollsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

type scoresRoller interface {
	Scores(ctx context.Context, in creationrollsvc.ScoresInput) (*creationrollsvc.ScoresResult, error)
}

type hpRoller interface {
	HP(ctx context.Context, in creationrollsvc.HPInput) (*creationrollsvc.HPResult, error)
}

// RegisterRoutes wires the creation's rolls onto r.
func RegisterRoutes(r Router, svc creationrollsvc.SVC) {
	r.Post("/api/characters/rolls/scores", render.Handle(Scores(svc)))
	r.Post("/api/characters/rolls/hp", render.Handle(HP(svc)))
}

// ScoresBody is the body of POST /api/characters/rolls/scores: the method
// code ("I" to "VI") and, optionally, the race code whose PHB Table 8
// adjustments the answer shows.
type ScoresBody struct {
	Method string `json:"method"`
	Race   string `json:"raceCode"`
}

// MethodDTO is the method rolled: `rolls` totals of `dice` d6, the lowest
// die of each dropped with `dropLowest`; `assign` how the totals become
// scores (`in_order`, `keep_per_ability`, `arrange`, `add_to_base`), `keep`
// how many the player keeps, and method VI's `base` and `max` (0 for the
// others).
type MethodDTO struct {
	Code       string `json:"code"`
	Rolls      int    `json:"rolls"`
	Dice       int    `json:"dice"`
	DropLowest bool   `json:"dropLowest"`
	Assign     string `json:"assign"`
	Keep       int    `json:"keep"`
	Base       int    `json:"base"`
	Max        int    `json:"max"`
}

// ScoreRollDTO is one total: its dice in the order drawn, the index of the
// dropped die (null for none), the total kept and, for the methods that roll
// in ability order, its ability code ("" otherwise).
type ScoreRollDTO struct {
	Dice    []int  `json:"dice"`
	Dropped *int   `json:"dropped"`
	Total   int    `json:"total"`
	Ability string `json:"ability"`
}

// ScoresDTO is the score roll's breakdown. `adjustments` (PHB Table 8 by
// ability code) is null without a race; `scores` (method I's finished scores
// by ability code, adjustments included) is null for the arranged methods.
type ScoresDTO struct {
	Method      MethodDTO      `json:"method"`
	Rolls       []ScoreRollDTO `json:"rolls"`
	Adjustments map[string]int `json:"adjustments"`
	Scores      map[string]int `json:"scores"`
}

// HPBody is the body of POST /api/characters/rolls/hp: the class code, the
// Constitution score and the starting XP.
type HPBody struct {
	Class *string `json:"class"`
	Con   *int    `json:"con"`
	XP    *int    `json:"xp"`
}

// HPLevelDTO is one level's gain. `hitDie` is the die's sides (0 for a fixed
// gain), `rolled` the die as rolled and `counted` its value after the
// Constitution 20+ floor (both null for a fixed gain).
type HPLevelDTO struct {
	Level         int  `json:"level"`
	HitDie        int  `json:"hitDie"`
	Rolled        *int `json:"rolled"`
	Counted       *int `json:"counted"`
	ConAdjustment int  `json:"conAdjustment"`
	Gain          int  `json:"gain"`
	Fixed         bool `json:"fixed"`
}

// HPDTO is the max HP roll's breakdown: the starting level, one row per
// level from 1 to it, and the total.
type HPDTO struct {
	Class  string       `json:"class"`
	Level  int          `json:"level"`
	Levels []HPLevelDTO `json:"levels"`
	Total  int          `json:"total"`
}

// decode reads a JSON body into v, refusing unknown fields.
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return render.BadRequestError("invalid JSON body")
	}
	return nil
}

// Scores is POST /api/characters/rolls/scores.
func Scores(svc scoresRoller) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b ScoresBody
		if err := decode(r, &b); err != nil {
			return err
		}
		res, err := svc.Scores(r.Context(), creationrollsvc.ScoresInput{Method: b.Method, Race: b.Race})
		if err != nil {
			return err
		}
		m := res.Method
		out := ScoresDTO{
			Method: MethodDTO{
				Code: m.Code, Rolls: m.Rolls, Dice: m.Dice, DropLowest: m.DropLowest,
				Assign: m.Assign, Keep: m.Keep, Base: m.Base, Max: m.Max,
			},
			Rolls:       make([]ScoreRollDTO, len(res.Rolls)),
			Adjustments: res.Adjustments,
			Scores:      res.Scores,
		}
		for i, roll := range res.Rolls {
			out.Rolls[i] = ScoreRollDTO{Dice: roll.Dice, Dropped: roll.Dropped, Total: roll.Total, Ability: roll.Ability}
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// HP is POST /api/characters/rolls/hp.
func HP(svc hpRoller) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b HPBody
		if err := decode(r, &b); err != nil {
			return err
		}
		if b.Class == nil || b.Con == nil || b.XP == nil {
			return render.BadRequestError("class, con and xp are required")
		}
		res, err := svc.HP(r.Context(), creationrollsvc.HPInput{Class: *b.Class, Con: *b.Con, XP: *b.XP})
		if err != nil {
			return err
		}
		out := HPDTO{Class: res.Class, Level: res.Level, Levels: make([]HPLevelDTO, len(res.Levels)), Total: res.Total}
		for i, l := range res.Levels {
			out.Levels[i] = HPLevelDTO{
				Level: l.Level, HitDie: l.HitDie, Rolled: l.Rolled, Counted: l.Counted,
				ConAdjustment: l.ConAdjustment, Gain: l.Gain, Fixed: l.Fixed,
			}
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}
