// Package psionicsvc owns the psionicist's requests on the PSP pool that are
// not a power (ADR 0043): recovering PSPs over hours of an activity (step 6).
// The pool itself is the action log's ledger (step 5, actionlogsvc); this
// service words the request and leaves the bounds and undo to the log.
package psionicsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// MaxRecoveryHours caps one recovery as passing time caps one fast-forward
// (ADR 0005 step 5): a typo like 8000 hours is refused.
const MaxRecoveryHours = 720

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Recover logs one psp_recovery entry for hours of activity (one of
	// Table 6's rows) and returns the pool after it. A *domain.FieldError for
	// invalid hours or an unknown activity; actionlogsvc.ErrNoPSPPool for a
	// character with no pool.
	Recover(ctx context.Context, characterID int64, hours float64, activity string) (actionlog.PSPState, error)
}

// RecoveryRates is Table 6 (ruletablesvc.Tables).
type RecoveryRates interface {
	// PSPRecoveryRate is the PSPs an hour of activity recovers.
	PSPRecoveryRate(activity string) (int, error)
}

// Log is the PSP ledger's writer (actionlogsvc).
type Log interface {
	AppendPSP(ctx context.Context, e actionlog.NewEntry) (actionlog.PSPState, error)
}

type service struct {
	rates RecoveryRates
	log   Log
}

// New constructs the service.
func New(rates RecoveryRates, log Log) (SVC, error) {
	if rates == nil || log == nil {
		return nil, fmt.Errorf("psionicsvc: nil dependency")
	}
	return &service{rates: rates, log: log}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module declares no codec: a recovery has no snapshot, its PSP delta is
// popped with the entry.
func (s *service) Module() domain.Module { return domain.Module{Name: "psionicsvc"} }

// recoveryRecord is the psp_recovery entry's payload: what was asked, the
// rate it was read at, and (when the gain was capped, step 5) what was
// requested and applied. Add fields, never rename.
type recoveryRecord struct {
	Hours        float64 `json:"hours"`
	Activity     string  `json:"activity"`
	PerHour      int     `json:"perHour"`
	PSPRequested *int    `json:"pspRequested,omitempty"`
	PSPApplied   *int    `json:"pspApplied,omitempty"`
}

// Recover is ADR 0043 step 6: hours of one of Table 6's activities recover
// hours × the activity's hourly rate (whole PSPs, rounded down), as one
// psp_recovery entry; the recording path caps the gain at the PSPs spent
// (step 5). An activity whose rate is 0 (exertion) logs the hours with no
// PSP. Step 6's refusal while a power is maintained waits on maintenance
// (steps 13 to 16), which does not exist yet: nothing can be maintained.
func (s *service) Recover(ctx context.Context, characterID int64, hours float64, activity string) (actionlog.PSPState, error) {
	if math.IsNaN(hours) || math.IsInf(hours, 0) || hours <= 0 {
		return actionlog.PSPState{}, domain.FieldErrorf("hours", "hours must be more than 0")
	}
	if hours > MaxRecoveryHours {
		return actionlog.PSPState{}, domain.FieldErrorf("hours", "at most %d hours at a time", MaxRecoveryHours)
	}
	if !slices.Contains(ruletablesvc.PSPActivities, activity) {
		return actionlog.PSPState{}, domain.FieldErrorf("activity", "unknown activity %q", activity)
	}
	rate, err := s.rates.PSPRecoveryRate(activity)
	if err != nil {
		return actionlog.PSPState{}, fmt.Errorf("psp recovery rate: %w", err)
	}
	payload, err := json.Marshal(recoveryRecord{Hours: hours, Activity: activity, PerHour: rate})
	if err != nil {
		return actionlog.PSPState{}, fmt.Errorf("psp recovery payload: %w", err)
	}
	return s.log.AppendPSP(ctx, actionlog.NewEntry{
		CharacterID: characterID,
		Kind:        actionlog.KindPSPRecovery,
		Payload:     payload,
		PSPDelta:    int(math.Floor(hours*float64(rate) + 1e-9)),
	})
}
