// Package leveluphdl exposes POST /api/character/levelup/roll, the
// level-up roll of the current character (ADR 0030 step 7).
package leveluphdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

// Router is the chi subset we need.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type roller interface {
	Roll(ctx context.Context, characterID int64, in levelupsvc.RollInput) (*levelupsvc.Result, error)
}

// RegisterRoutes wires the level-up roll onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc levelupsvc.SVC) {
	r.Post("/api/character/levelup/roll", render.Handle(Roll(charID, svc)))
}

// RollBody is the body of POST /api/character/levelup/roll: the level rolled
// for (the lowest pending one, as the page read it) and, for the manual
// entry, the die the player rolled at the table (absent: the server rolls).
type RollBody struct {
	Level *int `json:"level"`
	Die   *int `json:"die"`
}

// ResultDTO is the roll's full breakdown. `hitDie` is the die's sides (0 for
// a fixed gain), `rolled` the die as rolled or entered and `counted` its
// value after the Constitution 20+ floor (both null for a fixed gain),
// `pending` the levels still owed.
type ResultDTO struct {
	Level         int                    `json:"level"`
	HitDie        int                    `json:"hitDie"`
	Rolled        *int                   `json:"rolled"`
	Counted       *int                   `json:"counted"`
	ConAdjustment int                    `json:"conAdjustment"`
	Gain          int                    `json:"gain"`
	Fixed         bool                   `json:"fixed"`
	Manual        bool                   `json:"manual"`
	HPMaxBefore   int                    `json:"hpMaxBefore"`
	HPMaxAfter    int                    `json:"hpMaxAfter"`
	Pending       []wire.PendingLevelDTO `json:"pending"`
}

// Roll is POST /api/character/levelup/roll.
func Roll(charID CharacterIDFunc, svc roller) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b RollBody
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		if b.Level == nil {
			return render.BadRequestError("level is required")
		}
		res, err := svc.Roll(r.Context(), charID(r), levelupsvc.RollInput{Level: *b.Level, Die: b.Die})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ResultDTO{
			Level: res.Level, HitDie: res.HitDie, Rolled: res.Rolled, Counted: res.Counted,
			ConAdjustment: res.ConAdjustment, Gain: res.Gain, Fixed: res.Fixed, Manual: res.Manual,
			HPMaxBefore: res.HPMaxBefore, HPMaxAfter: res.HPMaxAfter,
			Pending: wire.PendingDTO(res.Pending),
		})
		return nil
	}
}
