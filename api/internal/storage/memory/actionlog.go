package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
)

// ActionLog is actionlogsvc.DAL over the store. It only ever appends: an
// entry with its snapshots, a correction (ADR 0007, 0010). Which entries are
// popped is decided by the domain rules (actionlog.StandingVoids,
// actionlog.LatestPopped, ADR 0019 steps 1 and 2), the HP total by
// actionlog.HPTotal (ADR 0007 step 2), as the database adapter does.
type ActionLog struct{ s *Store }

var _ actionlogsvc.DAL = ActionLog{}

// ActionLog returns the action-log port.
func (s *Store) ActionLog() ActionLog { return ActionLog{s} }

// now is the time a row is stamped with, to the second as the database's
// datetime('now').
func now() time.Time { return time.Now().UTC().Truncate(time.Second) }

// InsertEntry appends e and its snapshots to action-log transaction
// transactionID; 0 starts a new transaction, whose id is the new entry's
// (ADR 0018 step 1).
func (l ActionLog) InsertEntry(ctx context.Context, e actionlog.NewEntry, transactionID int64) (int64, error) {
	var id int64
	err := l.s.write(ctx, func(t *tables) error {
		id = int64(len(t.log)) + 1
		if transactionID == 0 {
			transactionID = id
		}
		payload := clone(e.Payload)
		if len(payload) == 0 {
			payload = []byte("{}")
		}
		t.log = append(t.log, actionlog.Entry{
			ID: id, TransactionID: transactionID, CharacterID: e.CharacterID, Round: e.Round, Kind: e.Kind,
			Summary: e.Summary, HPDelta: e.HPDelta, PSPDelta: e.PSPDelta, Payload: payload, Snapshots: clone(e.Snapshots),
			CreatedAt: now(),
		})
		return nil
	})
	return id, err
}

// InsertCorrection appends an amend, void or redo row to an existing entry.
// An empty payload is stored as none (the entry's payload is unchanged).
func (l ActionLog) InsertCorrection(ctx context.Context, c actionlog.Correction) error {
	return l.s.write(ctx, func(t *tables) error {
		if c.ActionID < 1 || c.ActionID > int64(len(t.log)) {
			return fmt.Errorf("insert action_corrections: no entry %d", c.ActionID)
		}
		row := clone(c)
		row.ID = int64(len(t.corrections)) + 1
		row.StepUndone = false
		row.CreatedAt = now()
		if len(row.Payload) == 0 {
			row.Payload = nil
		}
		t.corrections = append(t.corrections, row)
		return nil
	})
}

// GetEntry returns one entry with its snapshots and corrections, or
// domain.ErrNotFound when the character has no such entry.
func (l ActionLog) GetEntry(ctx context.Context, characterID, id int64) (*actionlog.Entry, error) {
	var out *actionlog.Entry
	err := l.s.read(ctx, func(t *tables) error {
		e, ok := t.entry(characterID, id)
		if !ok {
			return fmt.Errorf("%w: log entry %d", domain.ErrNotFound, id)
		}
		entries := t.withCorrections(characterID, []actionlog.Entry{e})
		out = &entries[0]
		return nil
	})
	return out, err
}

// ListEntries returns the newest limit entries, newest first.
func (l ActionLog) ListEntries(ctx context.Context, characterID int64, limit int) ([]actionlog.Entry, error) {
	return l.list(ctx, characterID, limit, func(actionlog.Entry, map[int64]int64) bool { return true })
}

// ListTransaction returns every entry of transaction transactionID, voided
// ones included, newest first.
func (l ActionLog) ListTransaction(ctx context.Context, characterID, transactionID int64) ([]actionlog.Entry, error) {
	return l.list(ctx, characterID, -1, func(e actionlog.Entry, _ map[int64]int64) bool {
		return e.TransactionID == transactionID
	})
}

// ListActiveEntries returns the newest limit entries of kind that are not
// popped.
func (l ActionLog) ListActiveEntries(ctx context.Context, characterID int64, kind actionlog.Kind, limit int) ([]actionlog.Entry, error) {
	return l.list(ctx, characterID, limit, func(e actionlog.Entry, popped map[int64]int64) bool {
		_, gone := popped[e.ID]
		return e.Kind == kind && !gone
	})
}

// list returns the character's newest entries that keep holds (all of them
// for a negative limit), newest first, with snapshots and corrections.
func (l ActionLog) list(ctx context.Context, characterID int64, limit int, keep func(actionlog.Entry, map[int64]int64) bool) ([]actionlog.Entry, error) {
	out := []actionlog.Entry{}
	err := l.s.read(ctx, func(t *tables) error {
		popped := actionlog.StandingVoids(t.correctionsOf(characterID))
		for i := len(t.log) - 1; i >= 0 && (limit < 0 || len(out) < limit); i-- {
			if e := t.log[i]; e.CharacterID == characterID && keep(e, popped) {
				out = append(out, clone(e))
			}
		}
		out = t.withCorrections(characterID, out)
		return nil
	})
	return out, err
}

// LatestActiveID returns the newest entry that is not popped, any kind (the
// top of the stack).
func (l ActionLog) LatestActiveID(ctx context.Context, characterID int64) (int64, bool, error) {
	var id int64
	var ok bool
	err := l.s.read(ctx, func(t *tables) error {
		popped := actionlog.StandingVoids(t.correctionsOf(characterID))
		for i := len(t.log) - 1; i >= 0; i-- {
			e := t.log[i]
			if _, gone := popped[e.ID]; e.CharacterID == characterID && !gone {
				id, ok = e.ID, true
				return nil
			}
		}
		return nil
	})
	return id, ok, err
}

// LatestPopped returns the entry popped last and not redone since, and its
// transaction (ADR 0019 step 2).
func (l ActionLog) LatestPopped(ctx context.Context, characterID int64) (id, transactionID int64, ok bool, err error) {
	err = l.s.read(ctx, func(t *tables) error {
		id, ok = actionlog.LatestPopped(actionlog.StandingVoids(t.correctionsOf(characterID)))
		if !ok {
			return nil
		}
		e, found := t.entry(characterID, id)
		if !found {
			return fmt.Errorf("latest popped action_log %d: not found", id)
		}
		transactionID = e.TransactionID
		return nil
	})
	if err != nil {
		return 0, 0, false, err
	}
	return id, transactionID, ok, nil
}

// LatestID returns the newest entry of kind, popped or not.
func (l ActionLog) LatestID(ctx context.Context, characterID int64, kind actionlog.Kind) (int64, bool, error) {
	var id int64
	var ok bool
	err := l.s.read(ctx, func(t *tables) error {
		for i := len(t.log) - 1; i >= 0; i-- {
			if e := t.log[i]; e.CharacterID == characterID && e.Kind == kind {
				id, ok = e.ID, true
				return nil
			}
		}
		return nil
	})
	return id, ok, err
}

// HPTotal is the sum of the effective HP deltas of the character's entries
// (actionlog.HPTotal, ADR 0007 step 2); 0 when the log is empty.
func (l ActionLog) HPTotal(ctx context.Context, characterID int64) (int, error) {
	var total int
	err := l.s.read(ctx, func(t *tables) error {
		var entries []actionlog.Entry
		for _, e := range t.log {
			if e.CharacterID == characterID {
				entries = append(entries, actionlog.Entry{ID: e.ID, CharacterID: characterID, HPDelta: e.HPDelta})
			}
		}
		total = actionlog.HPTotal(t.withCorrections(characterID, entries))
		return nil
	})
	return total, err
}

// PSPTotal is the sum of the effective PSP deltas of the character's entries
// (actionlog.PSPTotal, ADR 0043 step 5); 0 when the log is empty.
func (l ActionLog) PSPTotal(ctx context.Context, characterID int64) (int, error) {
	var total int
	err := l.s.read(ctx, func(t *tables) error {
		var entries []actionlog.Entry
		for _, e := range t.log {
			if e.CharacterID == characterID {
				entries = append(entries, actionlog.Entry{ID: e.ID, CharacterID: characterID, PSPDelta: e.PSPDelta})
			}
		}
		total = actionlog.PSPTotal(t.withCorrections(characterID, entries))
		return nil
	})
	return total, err
}

// entry is a copy of the character's entry id.
func (t *tables) entry(characterID, id int64) (actionlog.Entry, bool) {
	if id < 1 || id > int64(len(t.log)) || t.log[id-1].CharacterID != characterID {
		return actionlog.Entry{}, false
	}
	return clone(t.log[id-1]), true
}

// correctionsOf is every correction of the character's entries, oldest
// first.
func (t *tables) correctionsOf(characterID int64) []actionlog.Correction {
	var out []actionlog.Correction
	for _, c := range t.corrections {
		if t.log[c.ActionID-1].CharacterID == characterID {
			out = append(out, c)
		}
	}
	return out
}

// withCorrections attaches to entries (copies) their corrections in id
// order, each correction step marked undone when the step is popped (ADR
// 0019 step 1).
func (t *tables) withCorrections(characterID int64, entries []actionlog.Entry) []actionlog.Entry {
	all := t.correctionsOf(characterID)
	popped := actionlog.StandingVoids(all)
	byID := make(map[int64]*actionlog.Entry, len(entries))
	for i := range entries {
		entries[i].Corrections = nil
		byID[entries[i].ID] = &entries[i]
	}
	for _, c := range all {
		e, ok := byID[c.ActionID]
		if !ok {
			continue
		}
		c = clone(c)
		if c.StepID != nil {
			_, c.StepUndone = popped[*c.StepID]
		}
		e.Corrections = append(e.Corrections, c)
	}
	return entries
}
