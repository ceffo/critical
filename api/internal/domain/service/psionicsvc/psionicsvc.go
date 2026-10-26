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
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
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
	// StopAndRecover ends every power row that pays PSPs and recovers as
	// Recover does, as one undoable psp_recovery entry (ADR 0047 item 4). It
	// returns the pool after it and the rows ended. The refusals are
	// Recover's, but never ErrRecoveryWhileMaintaining.
	StopAndRecover(ctx context.Context, characterID int64, hours float64, activity string) (StopRecoverResult, error)
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
	// PowerThisRound is the id of the power used in the current round (ADR
	// 0043 step 12), "" when the round is free or out of combat.
	PowerThisRound(ctx context.Context, characterID int64) (string, error)
	// RaiseDefence raises a known defence mode (ADR 0044 steps 4 and 5): no
	// roll, one defence_raised entry paying its initial cost and putting its
	// row. The refusals are RaiseDefence's (defence.go).
	RaiseDefence(ctx context.Context, characterID int64, powerID string) (*RaiseResult, error)
	// Defend is a contest against the character (ADR 0044 step 8): one
	// check of the current defence per incoming roll, one psionic_defence
	// entry. The refusals are Defend's (defence.go).
	Defend(ctx context.Context, characterID int64, req DefendRequest) (*DefendResult, error)
	// Attack uses a known attack mode on the request's opponent (ADR 0044
	// step 7): two checks for one cost, one psionic_attack entry. The
	// refusals are Attack's (attack.go).
	Attack(ctx context.Context, characterID int64, req AttackRequest) (*AttackResult, error)
	// Contact uses the contact power on the request's open mind (ADR 0044
	// step 9): one check, its cost by the target's level, one power_check
	// entry making the contact row on a success. The refusals are
	// Contact's (contact.go).
	Contact(ctx context.Context, characterID int64, req ContactRequest) (*UseResult, error)
}

// StopRecoverResult is a stop-and-recover's outcome.
type StopRecoverResult struct {
	// PSP is the pool after the recovery.
	PSP actionlog.PSPState
	// Ended are the rows it took off (powers by catalogue id, tangent and
	// contact rows by target), as they were.
	Ended []buffsvc.BuffEffect
}

// Buffs is the buff list a maintained power's row lives on (buffsvc, ADR
// 0043 step 13).
type Buffs interface {
	// Add puts the row on inside the ctx transaction, without recording:
	// the power check's entry records the snapshot.
	Add(ctx context.Context, characterID int64, buff buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error)
	ListActive(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, error)
	// EndMaintained takes the paying rows off inside the ctx transaction,
	// without recording: the caller's entry records the snapshot.
	EndMaintained(ctx context.Context, characterID int64) ([]buffsvc.BuffEffect, actionlog.Snapshot, error)
	// GetRound is the round counter (ADR 0005; 0: out of combat).
	GetRound(ctx context.Context, characterID int64) (int, error)
	Dismiss(ctx context.Context, characterID int64, buffID string, cause spelleffectsvc.EndCause) ([]buffsvc.BuffEffect, buffsvc.EndResult, error)
	// AddTangents is an attack's one change to the tangent and contact rows
	// (ADR 0044 step 3), inside the ctx transaction, without recording: the
	// attack's entry records the snapshot (zero when nothing changed).
	AddTangents(ctx context.Context, characterID int64, target string, won int) (buffsvc.TangentsChange, actionlog.Snapshot, error)
	// MakeContact is the contact power's success (ADR 0044 step 9), as
	// AddTangents: inside the ctx transaction, without recording.
	MakeContact(ctx context.Context, characterID int64, target string) (buffsvc.TangentsChange, actionlog.Snapshot, error)
}

// Dice rolls the power check's d20 (dicesvc).
type Dice interface {
	RollDie(sides int) int
}

// Tables are the rule tables the service reads (ruletablesvc.Tables).
type Tables interface {
	// PSPRecoveryRowFor is Table 6's row for activity: the PSPs an hour of it
	// recovers and the proficiency it requires, if any; false when the table
	// has no such row.
	PSPRecoveryRowFor(activity string) (ruletablesvc.PSPRecoveryRow, bool, error)
	// AttackVersusDefence is Table 14's modifier to the attacker's score
	// for an attack mode meeting a defence mode (ADR 0044 step 1.1).
	AttackVersusDefence(attack, defence string) (int, error)
	// ContactCost is Table 29's contact cost for a target's level
	// (ruletablesvc.ErrLevelOutOfTable past it, ADR 0044 Q15).
	ContactCost(level int) (int, error)
	// ContactModifier is the contact power's score modifier for code of
	// kind (ADR 0044 step 1.3).
	ContactModifier(kind, code string) (int, error)
}

// Log is the PSP ledger's writer (actionlogsvc).
type Log interface {
	actionlog.Recorder
	AppendPSP(ctx context.Context, e actionlog.NewEntry) (actionlog.PSPState, error)
	// ActiveEntries are the newest non-voided entries of kind, newest first.
	ActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error)
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
	Tables     Tables
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
	rates Tables
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
	if d.Tables == nil || d.Log == nil || d.Characters == nil || d.Picks == nil || d.Dice == nil || d.Buffs == nil || d.Book == nil || d.Tx == nil {
		return nil, fmt.Errorf("psionicsvc: nil dependency")
	}
	return &service{rates: d.Tables, log: d.Log, chars: d.Characters, picks: d.Picks, dice: d.Dice, buffs: d.Buffs, book: d.Book, tx: d.Tx}, nil
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
	// PowersEnded, PowerNames and TargetsEnded are the rows a stop-and-recover
	// ended (ADR 0047 item 4), worded as the long rest's (ADR 0043 step 7).
	PowersEnded  []string                  `json:"powersEnded,omitempty"`
	PowerNames   map[string]localized.Text `json:"powerNames,omitempty"`
	TargetsEnded []string                  `json:"targetsEnded,omitempty"`
}

// recovery is a checked request: the rate read and the PSPs it is worth.
type recovery struct {
	record recoveryRecord
	delta  int
}

// plan checks a recovery request and prices it. Recover and StopAndRecover
// share it, so both honour the hours bounds, the unknown-activity refusal and
// a row's required proficiency (ADR 0047 item 6) the same way.
func (s *service) plan(ctx context.Context, characterID int64, hours float64, activity string) (recovery, error) {
	if math.IsNaN(hours) || math.IsInf(hours, 0) || hours <= 0 {
		return recovery{}, domain.FieldErrorf("hours", "hours must be more than 0")
	}
	if hours > MaxRecoveryHours {
		return recovery{}, domain.FieldErrorf("hours", "at most %d hours at a time", MaxRecoveryHours)
	}
	row, found, err := s.rates.PSPRecoveryRowFor(activity)
	if err != nil {
		return recovery{}, fmt.Errorf("psp recovery rate: %w", err)
	}
	if !found {
		return recovery{}, domain.FieldErrorf("activity", "unknown activity %q", activity)
	}
	if row.RequiresProficiency != "" {
		// ADR 0047 item 6: a gated row is recovered by only when the character
		// knows the proficiency, whichever class granted the pick; otherwise it
		// is refused like an unknown activity.
		c, err := s.chars.GetCharacter(ctx, characterID)
		if err != nil {
			return recovery{}, err
		}
		if !slices.ContainsFunc(c.Proficiencies.Nonweapon, func(p skills.Pick) bool { return p.ID == row.RequiresProficiency }) {
			return recovery{}, domain.FieldErrorf("activity", "unknown activity %q", activity)
		}
	}
	return recovery{
		record: recoveryRecord{Hours: hours, Activity: activity, PerHour: row.PerHour},
		delta:  int(math.Floor(hours*float64(row.PerHour) + 1e-9)),
	}, nil
}

// Recover is ADR 0043 step 6: hours of one of Table 6's activities recover
// hours × the activity's hourly rate (whole PSPs, rounded down), as one
// psp_recovery entry; the recording path caps the gain at the PSPs spent
// (step 5). An activity whose rate is 0 (exertion) logs the hours with no
// PSP. Refused with ErrRecoveryWhileMaintaining while a power is maintained:
// the book recovers only in hours with no PSP spent.
func (s *service) Recover(ctx context.Context, characterID int64, hours float64, activity string) (actionlog.PSPState, error) {
	r, err := s.plan(ctx, characterID, hours, activity)
	if err != nil {
		return actionlog.PSPState{}, err
	}
	payload, err := json.Marshal(r.record)
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
			PSPDelta:    r.delta,
		})
		return err
	})
	if err != nil {
		return actionlog.PSPState{}, err
	}
	return out, nil
}

// StopAndRecover is ADR 0047 item 4 (amending ADR 0043 steps 6 and 16): it
// ends every row that pays PSPs (BuffEffect.Maintained: a maintenance cost
// above 0; rows that cost nothing stay up and do not block the rest) and
// recovers as Recover does, in one transaction and one psp_recovery entry. The
// entry carries the ended rows as a buffs snapshot, so one undo puts the rows
// back, with their rounds kept, and takes the PSPs off.
func (s *service) StopAndRecover(ctx context.Context, characterID int64, hours float64, activity string) (StopRecoverResult, error) {
	r, err := s.plan(ctx, characterID, hours, activity)
	if err != nil {
		return StopRecoverResult{}, err
	}
	var res StopRecoverResult
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		ended, snap, err := s.buffs.EndMaintained(ctx, characterID)
		if err != nil {
			return fmt.Errorf("end maintained powers: %w", err)
		}
		rec := r.record
		for _, b := range ended {
			switch {
			case b.PowerID != "":
				rec.PowersEnded = append(rec.PowersEnded, b.PowerID)
			case b.Target != "":
				rec.TargetsEnded = append(rec.TargetsEnded, b.Target)
			}
		}
		rec.PowerNames = buffsvc.PowerNames(ended)
		payload, err := json.Marshal(rec)
		if err != nil {
			return fmt.Errorf("psp recovery payload: %w", err)
		}
		e := actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindPSPRecovery,
			Payload:     payload,
			PSPDelta:    r.delta,
		}
		if len(ended) > 0 {
			e.Snapshots = []actionlog.Snapshot{snap}
		}
		state, err := s.log.AppendPSP(ctx, e)
		if err != nil {
			return err
		}
		res = StopRecoverResult{PSP: state, Ended: ended}
		return nil
	})
	if err != nil {
		return StopRecoverResult{}, err
	}
	return res, nil
}
