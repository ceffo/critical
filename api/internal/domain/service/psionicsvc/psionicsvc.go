// Package psionicsvc owns the psionicist's requests (ADR 0043): recovering
// PSPs over hours of an activity (step 6), saving the list of powers known
// (step 9) and using a power, its check and initial cost (steps 10 and 11). The pool itself is the action log's ledger
// (step 5, actionlogsvc); this service words the recovery and leaves the
// bounds and undo to the log. The known powers are one stored list the
// service checks against Table 4 and logs with a snapshot.
package psionicsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// KnownPowersStateKey is the action-log state key of a character's known
// powers.
const KnownPowersStateKey = "known_powers"

// ErrNotPsionicist is the refusal of known powers for a character whose
// classes list no psionics.
var ErrNotPsionicist = errors.New("the character has no psionics")

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
	// SetKnownPowers saves the whole list of picks (ADR 0043 step 9), checked
	// against Table 4 at the psionics entry's level, as one psionic_pick entry
	// with its snapshot. It returns the known powers as saved. A
	// *psionics.Refusal for a list over the rules, a *domain.FieldError on
	// "picks" for a malformed one, ErrNotPsionicist for a character with no
	// psionics; a refusal writes nothing.
	SetKnownPowers(ctx context.Context, characterID int64, picks []psionics.Pick) (*charactersvc.PowerKnowledge, error)
	// Use checks a known power (ADR 0043 steps 10 and 11): one d20, one
	// power_check entry spending its PSPs. The refusals are Use's (use.go).
	Use(ctx context.Context, characterID int64, powerID string, req UseRequest) (*UseResult, error)
	// Stop ends a maintained power (ADR 0043 step 16): the dismiss of its
	// buff row, free, logged and undoable. ErrNotPsionicist,
	// ErrPowerNotMaintained when no row keeps the power up.
	Stop(ctx context.Context, characterID int64, powerID string) ([]buffsvc.BuffEffect, error)
}

// Buffs is the buff list a maintained power's row lives on (buffsvc, ADR
// 0043 step 13).
type Buffs interface {
	// Add puts the row on inside the ctx transaction, without recording:
	// the power check's entry records the snapshot.
	Add(ctx context.Context, characterID int64, buff buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error)
	ListActive(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, error)
	Dismiss(ctx context.Context, characterID int64, buffID string, cause spelleffectsvc.EndCause) ([]buffsvc.BuffEffect, buffsvc.EndResult, error)
}

// Dice rolls the power check's d20 (dicesvc).
type Dice interface {
	RollDie(sides int) int
}

// RecoveryRates is Table 6 (ruletablesvc.Tables).
type RecoveryRates interface {
	// PSPRecoveryRate is the PSPs an hour of activity recovers.
	PSPRecoveryRate(activity string) (int, error)
}

// Log is the PSP ledger's writer (actionlogsvc).
type Log interface {
	actionlog.Recorder
	AppendPSP(ctx context.Context, e actionlog.NewEntry) (actionlog.PSPState, error)
}

// Characters reads a character with its derived known powers (charactersvc).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Picks writes the character's list of known powers; it joins the ctx
// transaction.
type Picks interface {
	SetPsionicPicks(ctx context.Context, characterID int64, picks []psionics.Pick) error
}

// Deps are the service's collaborators.
type Deps struct {
	Rates      RecoveryRates
	Log        Log
	Characters Characters
	Picks      Picks
	Dice       Dice
	Buffs      Buffs
	// Book is the powers catalogue the picks are checked against.
	Book psionics.Book
	Tx   domain.TxRunner
}

type service struct {
	rates RecoveryRates
	log   Log
	chars Characters
	picks Picks
	dice  Dice
	buffs Buffs
	book  psionics.Book
	tx    domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.Rates == nil || d.Log == nil || d.Characters == nil || d.Picks == nil || d.Dice == nil || d.Buffs == nil || d.Book == nil || d.Tx == nil {
		return nil, fmt.Errorf("psionicsvc: nil dependency")
	}
	return &service{rates: d.Rates, log: d.Log, chars: d.Characters, picks: d.Picks, dice: d.Dice, buffs: d.Buffs, book: d.Book, tx: d.Tx}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module declares the codec of the known powers' snapshot: undo writes the
// save's "before", redo its "after" (ADR 0019), inside the undo's transaction.
// A recovery has no snapshot, its PSP delta is popped with the entry.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "psionicsvc", Codecs: []actionlog.Codec{
		knownSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, st knownState) error {
			if err := s.picks.SetPsionicPicks(ctx, characterID, st.picks()); err != nil {
				return fmt.Errorf("restore known powers: %w", err)
			}
			return nil
		}, nil),
	}}
}

// knownSnaps is the snapshotter of the known powers (ADR 0018 step 8).
var knownSnaps = actionlog.NewSnapshotter[knownState](KnownPowersStateKey)

// knownState is the JSON encoding of a "known_powers" snapshot and the before
// and after of a psionic_pick entry: the picks in order. Entries already in
// the log must stay decodable: add fields, never rename.
type knownState struct {
	Picks []psionics.Pick `json:"picks"`
}

func (k knownState) picks() []psionics.Pick { return slices.Clone(k.Picks) }

func stateOf(picks []psionics.Pick) knownState {
	return knownState{Picks: append([]psionics.Pick{}, picks...)}
}

// knownRecord is the psionic_pick entry's payload. The UI words it. Add
// fields, never rename.
type knownRecord struct {
	Before knownState `json:"before"`
	After  knownState `json:"after"`
}

// SetKnownPowers is ADR 0043 step 9: the player saves the whole list. It is
// checked against Table 4 at the psionics entry's realized level (the
// allowance the read derives) and refused only when it breaks a rule; the
// list is stored and logged as one psionic_pick entry, in one transaction, so
// one undo gives the list before back. The same list as stored records
// nothing.
func (s *service) SetKnownPowers(ctx context.Context, characterID int64, picks []psionics.Pick) (*charactersvc.PowerKnowledge, error) {
	if picks == nil {
		picks = []psionics.Pick{}
	}
	var out *charactersvc.PowerKnowledge
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return err
		}
		k := c.Derived.Powers
		if k == nil {
			return ErrNotPsionicist
		}
		used, err := psionics.Validate(s.book, k.Allowance, picks)
		if err != nil {
			if errors.Is(err, psionics.ErrInvalid) {
				return domain.FieldErrorf("picks", "%s", err.Error())
			}
			return err
		}
		out = &charactersvc.PowerKnowledge{
			Allowance: k.Allowance, Used: used, Primary: psionics.Primary(picks), Picks: slices.Clone(picks),
		}
		if slices.Equal(c.PsionicPicks, picks) {
			return nil
		}
		if err := s.picks.SetPsionicPicks(ctx, characterID, picks); err != nil {
			return fmt.Errorf("set known powers: %w", err)
		}
		b, a := stateOf(c.PsionicPicks), stateOf(picks)
		payload, err := json.Marshal(knownRecord{Before: b, After: a})
		if err != nil {
			return fmt.Errorf("encode known powers payload: %w", err)
		}
		snap, err := knownSnaps.Snapshot("", &b, &a)
		if err != nil {
			return err
		}
		if _, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID, Kind: actionlog.KindPsionicPick, Payload: payload,
			Snapshots: []actionlog.Snapshot{snap},
		}); err != nil {
			return fmt.Errorf("record known powers: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

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
// PSP. Refused with ErrRecoveryWhileMaintaining while a power is maintained:
// the book recovers only in hours with no PSP spent.
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
	var out actionlog.PSPState
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		buffs, err := s.buffs.ListActive(ctx, characterID)
		if err != nil {
			return fmt.Errorf("list active buffs: %w", err)
		}
		if slices.ContainsFunc(buffs, buffsvc.BuffEffect.Maintained) {
			return ErrRecoveryWhileMaintaining
		}
		out, err = s.log.AppendPSP(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindPSPRecovery,
			Payload:     payload,
			PSPDelta:    int(math.Floor(hours*float64(rate) + 1e-9)),
		})
		return err
	})
	if err != nil {
		return actionlog.PSPState{}, err
	}
	return out, nil
}
