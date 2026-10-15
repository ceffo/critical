package actionlogsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// maxNoteLen bounds a note (runes).
const maxNoteLen = 500

// Amend sets the note of entry id, any kind (ADR 0007 step 7). User,
// 2026-10-14: "no amending values. just the note". The note is the only thing
// an amend changes: HP, summary, payload and state are untouched. "" clears it.
func (s *service) Amend(ctx context.Context, characterID, id int64, a Amendment) (*actionlog.Entry, error) {
	note := strings.TrimSpace(a.Note)
	if utf8.RuneCountInString(note) > maxNoteLen {
		return nil, fmt.Errorf("%w: a note is at most %d characters", domain.ErrInvalidInput, maxNoteLen)
	}
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		e, err := s.dal.GetEntry(ctx, characterID, id)
		if err != nil {
			return fmt.Errorf("get entry %d: %w", id, err)
		}
		if e.Status() == actionlog.StatusVoided {
			return fmt.Errorf("%w: #%d", ErrAlreadyVoided, id)
		}
		if err := s.dal.InsertCorrection(ctx, actionlog.Correction{
			ActionID: id,
			Kind:     actionlog.CorrectionAmend,
			Note:     &note,
		}); err != nil {
			return fmt.Errorf("insert amend: %w", err)
		}
		out, err = s.dal.GetEntry(ctx, characterID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// AmendLatest corrects entry id after the roll as its own step on the action
// stack (ADR 0010 step 6): it records a KindCorrection entry naming the change
// (actionlog.CorrectionStep, stamped with the current round) and an amend on
// entry id carrying the new payload, linked to that step. Popping the step
// puts the amend out of effect. It is refused when the entry is another kind
// or kind is not a roll (ErrNotAmendable), voided (ErrAlreadyVoided) or not
// the latest roll: a newer entry of any of actionlog.RollKinds exists, voided
// ones included (ErrNotLatest). The checks and both writes share one
// transaction.
func (s *service) AmendLatest(ctx context.Context, characterID, id int64, kind actionlog.Kind, fn actionlog.AmendFunc) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		e, err := s.dal.GetEntry(ctx, characterID, id)
		if err != nil {
			return fmt.Errorf("get entry %d: %w", id, err)
		}
		if e.Kind != kind || !actionlog.IsRoll(kind) {
			return fmt.Errorf("%w: #%d is %s, not %s", ErrNotAmendable, id, e.Kind, kind)
		}
		if e.Status() == actionlog.StatusVoided {
			return fmt.Errorf("%w: #%d", ErrAlreadyVoided, id)
		}
		latest, err := s.latestRoll(ctx, characterID)
		if err != nil {
			return err
		}
		if latest != id {
			return fmt.Errorf("%w: only the latest roll can be adjusted, undo this %s and roll again", ErrNotLatest, kind)
		}
		a, err := fn(*e)
		if err != nil {
			return err
		}
		if len(a.Payload) == 0 || a.Change.Field == "" {
			return fmt.Errorf("%w: amendment needs a payload and the field it changes", domain.ErrInvalidInput)
		}
		stepID, err := s.recordStep(ctx, characterID, e, a.Change)
		if err != nil {
			return err
		}
		if err := s.dal.InsertCorrection(ctx, actionlog.Correction{
			ActionID: id,
			Kind:     actionlog.CorrectionAmend,
			Payload:  a.Payload,
			StepID:   &stepID,
		}); err != nil {
			return fmt.Errorf("insert amend: %w", err)
		}
		out, err = s.dal.GetEntry(ctx, characterID, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// latestRoll is the id of the newest entry of any roll kind (0: none).
func (s *service) latestRoll(ctx context.Context, characterID int64) (int64, error) {
	var latest int64
	for _, k := range actionlog.RollKinds {
		id, ok, err := s.dal.LatestID(ctx, characterID, k)
		if err != nil {
			return 0, fmt.Errorf("latest %s entry: %w", k, err)
		}
		if ok && id > latest {
			latest = id
		}
	}
	return latest, nil
}

// recordStep inserts the KindCorrection entry of a correction on e; it runs
// inside AmendLatest's transaction.
func (s *service) recordStep(ctx context.Context, characterID int64, e *actionlog.Entry, c actionlog.Change) (int64, error) {
	payload, err := json.Marshal(actionlog.CorrectionStep{EntryID: e.ID, EntryKind: e.Kind, Field: c.Field, From: c.From, To: c.To})
	if err != nil {
		return 0, fmt.Errorf("encode correction step: %w", err)
	}
	round, err := s.rounds.GetRound(ctx, characterID)
	if err != nil {
		return 0, fmt.Errorf("current round: %w", err)
	}
	id, err := s.dal.InsertEntry(ctx, actionlog.NewEntry{
		CharacterID: characterID, Round: round, Kind: actionlog.KindCorrection, Payload: payload,
	})
	if err != nil {
		return 0, fmt.Errorf("insert correction step: %w", err)
	}
	return id, nil
}
