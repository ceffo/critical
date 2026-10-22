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

// RollBody is the body of POST /api/character/levelup/roll: the class entry
// (absent: the first active one, ADR 0042 step 7), the level rolled for (its
// lowest pending one, as the page read it) and, for the manual entry, the die
// the player rolled at the table (absent: the server rolls).
type RollBody struct {
	Class string `json:"class"`
	Level *int   `json:"level"`
	Die   *int   `json:"die"`
}

// ResultDTO is the roll's full breakdown. `hitDie` is the die's sides (0 for
// a fixed gain), `rolled` the die as rolled or entered and `counted` its
// value after the Constitution 20+ floor (both null for a fixed gain),
// `pending` the levels still owed, each with what it grants beyond hit
// points (ADR 0037 step 15: the grants are read from the pending levels).
// A multi-class character's carry `class`, the entry the level was taken in,
// `classes`, the divisor, and `dieShare` / `conShare`, the die's (or fixed
// gain's) and the Constitution adjustment's shares (ADR 0042 step 10.4); a
// single-class character's have none.
type ResultDTO struct {
	Class         string `json:"class,omitempty"`
	Classes       int    `json:"classes,omitempty"`
	DieShare      int    `json:"dieShare,omitempty"`
	ConShare      int    `json:"conShare,omitempty"`
	Level         int    `json:"level"`
	HitDie        int    `json:"hitDie"`
	Rolled        *int   `json:"rolled"`
	Counted       *int   `json:"counted"`
	ConAdjustment int    `json:"conAdjustment"`
	Gain          int    `json:"gain"`
	Fixed         bool   `json:"fixed"`
	Manual        bool   `json:"manual"`
	HPMaxBefore   int    `json:"hpMaxBefore"`
	HPMaxAfter    int    `json:"hpMaxAfter"`
	// Reason is why the gain is 0: dual_class_no_hit_points for a
	// dual-class's level not above its old class's (ADR 0042 step 11).
	Reason  string                 `json:"reason,omitempty"`
	Pending []wire.PendingLevelDTO `json:"pending"`
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
		res, err := svc.Roll(r.Context(), charID(r), levelupsvc.RollInput{Class: b.Class, Level: *b.Level, Die: b.Die})
		if err != nil {
			return err
		}
		dto := ResultDTO{
			Level: res.Level, HitDie: res.HitDie, Rolled: res.Rolled, Counted: res.Counted,
			ConAdjustment: res.ConAdjustment, Gain: res.Gain, Fixed: res.Fixed, Manual: res.Manual,
			HPMaxBefore: res.HPMaxBefore, HPMaxAfter: res.HPMaxAfter, Reason: res.Reason,
			Pending: wire.PendingDTO(res.Pending),
		}
		if res.Classes > 1 {
			dto.Class, dto.Classes, dto.DieShare, dto.ConShare = res.Class, res.Classes, res.DieShare, res.ConShare
		}
		render.OK(r.Context(), w, dto)
		return nil
	}
}
