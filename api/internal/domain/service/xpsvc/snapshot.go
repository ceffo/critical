package xpsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// StateKeyXP is the state-key family of the XP ledger. Each entry has its own
// key, StateKey(id) = "xp:<id>", so the undo of one award restores only it
// (ADR 0018 scope, ADR 0015 step 4).
const StateKeyXP = "xp"

// StateKey is the action-log state key of one XP entry.
func StateKey(id int64) string {
	return StateKeyXP + actionlog.KeySeparator + strconv.FormatInt(id, 10)
}

// snapEntry is the JSON encoding of one entry inside a snapshot. Entries
// already in the log must stay decodable: add fields, never rename. Absent
// marks a "before" in which the entry did not exist (restoring deletes it),
// otherwise the entry is the whole truth: restoring overwrites every field,
// or writes it back under its id when it is gone.
type snapEntry struct {
	ID      int64  `json:"id"`
	Absent  bool   `json:"absent,omitempty"`
	Date    string `json:"date"`
	Session string `json:"session"`
	Reason  string `json:"reason"`
	Base    int    `json:"base"`
	Bonus   bool   `json:"bonus"`
}

func toSnap(id int64, e *Entry) snapEntry {
	if e == nil {
		return snapEntry{ID: id, Absent: true}
	}
	return snapEntry{ID: e.ID, Date: e.Date, Session: e.Session, Reason: e.Reason, Base: e.Base, Bonus: e.Bonus}
}

func (c snapEntry) entry() Entry {
	return Entry{ID: c.ID, Date: c.Date, Session: c.Session, Reason: c.Reason, Base: c.Base, Bonus: c.Bonus}
}

// entrySnapshot is the snapshot of an entry going from before to after (nil =
// it did not exist / no longer exists).
func entrySnapshot(id int64, before, after *Entry) (actionlog.Snapshot, error) {
	b, err := json.Marshal(toSnap(id, before))
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode xp snapshot: %w", err)
	}
	a, err := json.Marshal(toSnap(id, after))
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode xp snapshot: %w", err)
	}
	return actionlog.Snapshot{Key: StateKey(id), Before: b, After: a}, nil
}

type restoreFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restoreFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

// Restorers writes an XP snapshot back (registered for the StateKeyXP
// family). It runs inside the undo's transaction.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKeyXP: restoreFunc(func(ctx context.Context, characterID int64, state json.RawMessage) error {
			var c snapEntry
			if err := json.Unmarshal(state, &c); err != nil {
				return fmt.Errorf("decode xp snapshot: %w", err)
			}
			if c.ID == 0 {
				return fmt.Errorf("decode xp snapshot: missing id")
			}
			var err error
			if c.Absent {
				err = s.dal.Delete(ctx, characterID, c.ID)
			} else {
				err = s.dal.Update(ctx, characterID, c.entry())
				if errors.Is(err, ErrEntryNotFound) {
					err = s.dal.Reinsert(ctx, characterID, c.entry())
				}
			}
			if err != nil {
				return fmt.Errorf("restore xp entry %d: %w", c.ID, err)
			}
			return nil
		}),
	}
}

// entryRecord is one entry in an xp_add / xp_edit / xp_remove log payload,
// in the /api/xp wire names, with its computed bonus and total (ADR 0015
// step 1). Add fields, never rename.
type entryRecord struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"`
	Session     string `json:"session"`
	Reason      string `json:"reason"`
	Base        int    `json:"base"`
	Bonus       bool   `json:"bonus"`
	BonusAmount int    `json:"bonusAmount"`
	Total       int    `json:"total"`
}

// changeRecord is the payload of an XP log entry: the entry as it is now (as
// it was, for a remove) and, for an edit, the entry before it. The UI words
// it.
type changeRecord struct {
	Entry  entryRecord  `json:"entry"`
	Before *entryRecord `json:"before,omitempty"`
}

func recordOf(e Entry) entryRecord {
	a := EntryAmount(e.Base, e.Bonus)
	return entryRecord{
		ID: e.ID, Date: e.Date, Session: e.Session, Reason: e.Reason,
		Base: e.Base, Bonus: e.Bonus, BonusAmount: a.BonusAmount, Total: a.Total,
	}
}

// logChange records one XP change with the entry's snapshot, inside the
// caller's transaction (ADR 0018 step 1).
func (s *service) logChange(ctx context.Context, characterID int64, kind actionlog.Kind, id int64, before, after *Entry) error {
	snap, err := entrySnapshot(id, before, after)
	if err != nil {
		return err
	}
	var rec changeRecord
	switch {
	case after != nil:
		rec.Entry = recordOf(*after)
		if before != nil {
			b := recordOf(*before)
			rec.Before = &b
		}
	case before != nil:
		rec.Entry = recordOf(*before)
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode %s payload: %w", kind, err)
	}
	if _, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: kind, Payload: payload,
		Snapshots: []actionlog.Snapshot{snap},
	}); err != nil {
		return fmt.Errorf("record %s: %w", kind, err)
	}
	return nil
}
