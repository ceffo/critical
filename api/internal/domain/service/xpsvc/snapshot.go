package xpsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// StateKeyXP is the state-key family of the XP ledger. Each entry has its own
// key, StateKey(id) = "xp:<id>", so the undo of one award restores only it
// (ADR 0018 scope, ADR 0015 step 4).
const StateKeyXP = "xp"

// StateKey is the action-log state key of one XP entry.
func StateKey(id int64) string {
	return StateKeyXP + actionlog.KeySeparator + strconv.FormatInt(id, 10)
}

// snapEntry is the state of an "xp:<id>" snapshot (xpSnaps), the envelope's
// id naming the entry. Entries already in the log must stay decodable: add
// fields, never rename. The entry is the whole truth: restoring overwrites
// every field, or writes it back under its id when it is gone; an absent one
// did not exist (an add's "before") or is gone (a remove's "after"), and
// restoring it deletes it.
type snapEntry struct {
	Date    string `json:"date"`
	Session string `json:"session"`
	Reason  string `json:"reason"`
	Base    int    `json:"base"`
	Bonus   bool   `json:"bonus"`
}

// xpSnaps is the snapshotter of the XP ledger's entries (ADR 0018 step 8).
var xpSnaps = actionlog.NewSnapshotter[snapEntry](StateKeyXP).MayBeAbsent()

func toSnap(e *Entry) *snapEntry {
	if e == nil {
		return nil
	}
	return &snapEntry{Date: e.Date, Session: e.Session, Reason: e.Reason, Base: e.Base, Bonus: e.Bonus}
}

func (c snapEntry) entry(id int64) Entry {
	return Entry{ID: id, Date: c.Date, Session: c.Session, Reason: c.Reason, Base: c.Base, Bonus: c.Bonus}
}

// entrySnapshot is the snapshot of an entry going from before to after (nil =
// it did not exist / no longer exists).
func entrySnapshot(id int64, before, after *Entry) (actionlog.Snapshot, error) {
	return xpSnaps.Snapshot(strconv.FormatInt(id, 10), toSnap(before), toSnap(after))
}

// Module declares the codec that writes an XP snapshot back (the StateKeyXP
// family). It runs inside the undo's transaction.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "xpsvc", Codecs: []actionlog.Codec{
		xpSnaps.Restorer(func(ctx context.Context, characterID int64, key string, c snapEntry) error {
			id, err := entryID(key)
			if err != nil {
				return err
			}
			err = s.dal.Update(ctx, characterID, c.entry(id))
			if errors.Is(err, ErrEntryNotFound) {
				err = s.dal.Reinsert(ctx, characterID, c.entry(id))
			}
			return err
		}, func(ctx context.Context, characterID int64, key string) error {
			id, err := entryID(key)
			if err != nil {
				return err
			}
			return s.dal.Delete(ctx, characterID, id)
		}),
	}}
}

// entryID reads an XP snapshot's id.
func entryID(key string) (int64, error) {
	id, err := strconv.ParseInt(key, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("decode xp snapshot: id %q", key)
	}
	return id, nil
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
	// Split is the award's total shared between a multi-class character's
	// classes (ADR 0042 step 5.1); absent for one class.
	Split []classAmountRecord `json:"split,omitempty"`
}

type classAmountRecord struct {
	Class  string `json:"class"`
	Amount int    `json:"amount"`
}

// changeRecord is the payload of an XP log entry: the entry as it is now (as
// it was, for a remove) and, for an edit, the entry before it. The UI words
// it.
type changeRecord struct {
	Entry  entryRecord  `json:"entry"`
	Before *entryRecord `json:"before,omitempty"`
}

func recordOf(e Entry, entries []classes.Entry) entryRecord {
	a := EntryAmount(e.Base, e.Bonus)
	rec := entryRecord{
		ID: e.ID, Date: e.Date, Session: e.Session, Reason: e.Reason,
		Base: e.Base, Bonus: e.Bonus, BonusAmount: a.BonusAmount, Total: a.Total,
	}
	for _, s := range splitAmount(entries, e.ID, a.Total) {
		rec.Split = append(rec.Split, classAmountRecord(s))
	}
	return rec
}

// logChange records one XP change with the entry's snapshot, inside the
// caller's transaction (ADR 0018 step 1).
func (s *service) logChange(ctx context.Context, characterID int64, kind actionlog.Kind, id int64, before, after *Entry) error {
	snap, err := entrySnapshot(id, before, after)
	if err != nil {
		return err
	}
	entries, err := s.chars.ClassEntries(ctx, characterID)
	if err != nil {
		return err
	}
	var rec changeRecord
	switch {
	case after != nil:
		rec.Entry = recordOf(*after, entries)
		if before != nil {
			b := recordOf(*before, entries)
			rec.Before = &b
		}
	case before != nil:
		rec.Entry = recordOf(*before, entries)
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
