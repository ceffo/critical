package buffsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// logPayload is the structured summary of a buff entry, rendered by the UI
// in the player's language (the Summary string is the English fallback).
type logPayload struct {
	// Action is activate, edit, absorb, dismiss next_round, end_combat, set_round,
	// save_template or delete_template.
	Action      string   `json:"action"`
	Buff        string   `json:"buff,omitempty"`
	Rounds      int      `json:"rounds,omitempty"`
	Charges     int      `json:"charges,omitempty"`
	FromRounds  int      `json:"fromRounds,omitempty"`
	FromCharges int      `json:"fromCharges,omitempty"`
	Ended       bool     `json:"ended,omitempty"`
	FromRound   *int     `json:"fromRound,omitempty"`
	ToRound     *int     `json:"toRound,omitempty"`
	Expired     []string `json:"expired,omitempty"`
	// Note is the buff's note (activate, save_template) or its new note (edit,
	// with NoteChanged); FromNote is the previous one.
	Note        string `json:"note,omitempty"`
	FromNote    string `json:"fromNote,omitempty"`
	NoteChanged bool   `json:"noteChanged,omitempty"`
}

// change is what a use case decided: the new buff list, the new round when
// it changes, and the log entry describing it.
type change struct {
	kind    actionlog.Kind
	summary string
	payload logPayload
	buffs   []BuffEffect
	round   *int
	// roundOnly: the buff list is neither snapshotted nor written (End combat,
	// Set round leave buffs untouched).
	roundOnly bool
}

// apply runs decide on the current state and, in one transaction, records
// the action-log entry with before/after snapshots and writes the new state.
// The entry is recorded before the writes so it is stamped with the round
// the action happened in (Next Round belongs to the round it ends).
func (s *service) apply(ctx context.Context, characterID int64, decide func(buffs []BuffEffect, round int) (change, error)) (change, error) {
	var ch change
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		buffs, err := s.dal.GetActiveBuffs(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get active buffs: %w", err)
		}
		round, err := s.dal.GetRoundNumber(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get round: %w", err)
		}
		ch, err = decide(append([]BuffEffect(nil), buffs...), round)
		if err != nil {
			return err
		}
		snaps, err := snapshots(buffs, ch.buffs, round, ch.round, ch.roundOnly)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(ch.payload)
		if err != nil {
			return fmt.Errorf("encode payload: %w", err)
		}
		if _, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        ch.kind,
			Summary:     ch.summary,
			Payload:     payload,
			Snapshots:   snaps,
		}); err != nil {
			return fmt.Errorf("record %s: %w", ch.kind, err)
		}
		if !ch.roundOnly {
			if err := s.dal.SaveActiveBuffs(ctx, characterID, ch.buffs); err != nil {
				return fmt.Errorf("save active buffs: %w", err)
			}
		}
		if ch.round != nil {
			if err := s.dal.SaveRoundNumber(ctx, characterID, *ch.round); err != nil {
				return fmt.Errorf("save round: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return change{}, err
	}
	return ch, nil
}

func snapshots(before, after []BuffEffect, round int, newRound *int, roundOnly bool) ([]actionlog.Snapshot, error) {
	if roundOnly {
		rb, _ := json.Marshal(round)
		ra, _ := json.Marshal(*newRound)
		return []actionlog.Snapshot{{Key: StateKeyRound, Before: rb, After: ra}}, nil
	}
	b, err := encodeBuffs(before)
	if err != nil {
		return nil, fmt.Errorf("encode buffs: %w", err)
	}
	a, err := encodeBuffs(after)
	if err != nil {
		return nil, fmt.Errorf("encode buffs: %w", err)
	}
	snaps := []actionlog.Snapshot{{Key: StateKeyBuffs, Before: b, After: a}}
	if newRound != nil {
		rb, _ := json.Marshal(round)
		ra, _ := json.Marshal(*newRound)
		snaps = append(snaps, actionlog.Snapshot{Key: StateKeyRound, Before: rb, After: ra})
	}
	return snaps, nil
}

// find returns the index of buffID or ErrNotFound.
func find(buffs []BuffEffect, buffID string) (int, error) {
	i := indexOf(buffs, buffID)
	if i < 0 {
		return -1, fmt.Errorf("%w: buff %q", domain.ErrNotFound, buffID)
	}
	return i, nil
}

// counters renders "5 rd, 3 ch" for summaries.
func counters(duration, charges int) string {
	switch {
	case duration > 0 && charges > 0:
		return fmt.Sprintf("%d rd, %d ch", duration, charges)
	case duration > 0:
		return fmt.Sprintf("%d rd", duration)
	case charges > 0:
		return fmt.Sprintf("%d ch", charges)
	default:
		return "until dismissed"
	}
}
