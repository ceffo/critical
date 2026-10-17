package skillhdl

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// AbilityCheckDTO is one ability check (ADR 0028 step 5): abilityScore is the
// sheet's score, target the score in effect at the roll, adjustedTarget =
// target + adjustment the one the roll is judged against, success the
// verdict in effect and rolledSuccess the verdict as rolled.
type AbilityCheckDTO struct {
	Ability          string `json:"ability"`
	Die              int    `json:"die"`
	Roll             int    `json:"roll"`
	AbilityScore     int    `json:"abilityScore"`
	Target           int    `json:"target"`
	Adjustment       int    `json:"adjustment"`
	AdjustmentReason string `json:"adjustmentReason,omitempty"`
	AdjustedTarget   int    `json:"adjustedTarget"`
	Success          bool   `json:"success"`
	RolledSuccess    bool   `json:"rolledSuccess"`
	LogEntryID       int64  `json:"logEntryId"`
}

// AbilityCheckToDTO maps an ability check to its wire shape.
func AbilityCheckToDTO(c *skillsvc.AbilityCheckResult) AbilityCheckDTO {
	return AbilityCheckDTO{
		Ability: string(c.Ability), Die: c.Die, Roll: c.Roll, AbilityScore: c.AbilityScore, Target: c.Target,
		Adjustment: c.Adjustment, AdjustmentReason: c.AdjustmentReason, AdjustedTarget: c.AdjustedTarget,
		Success: c.Success, RolledSuccess: c.RolledSuccess, LogEntryID: c.LogEntryID,
	}
}

// RollAbility rolls a check of the ability {id} (str … cha; no body); any
// other key is a 400.
func RollAbility(charID CharacterIDFunc, svc skillSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		c, err := svc.RollAbility(r.Context(), charID(r), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, AbilityCheckToDTO(c))
		return nil
	}
}

// AdjustAbility sets the adjustment of the latest ability check (log entry
// {id}); the server judges the same roll against the moved target and
// returns the updated check. 409 when the entry is voided or not the latest
// roll, 400 when it is not an ability check or the adjustment is invalid,
// 404 when it does not exist.
func AdjustAbility(charID CharacterIDFunc, svc skillSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id <= 0 {
			return render.BadRequestError("invalid log entry id")
		}
		var req AdjustmentDTO
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if req.Adjustment == nil {
			return render.BadRequestError("adjustment is required")
		}
		c, err := svc.AdjustAbility(r.Context(), charID(r), id, skillsvc.Adjustment{Value: *req.Adjustment, Reason: req.Reason})
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, AbilityCheckToDTO(c))
		return nil
	}
}
