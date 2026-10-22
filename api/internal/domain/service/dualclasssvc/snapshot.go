package dualclasssvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
)

// entrySnap is one class entry in the StateKey snapshot. Entries already in
// the log must stay decodable: add fields, never rename.
type entrySnap struct {
	Class                string `json:"class"`
	RealizedLevel        int    `json:"realizedLevel"`
	StartingXP           int    `json:"startingXp"`
	State                string `json:"state"`
	LeftAtLevel          int    `json:"leftAtLevel"`
	IgnoredAwardsThrough int64  `json:"ignoredAwardsThrough"`
}

// entriesState is the state of the StateKey snapshot: every class entry, in order.
type entriesState struct {
	Entries []entrySnap `json:"entries"`
}

// entrySnaps is the snapshotter of the class entries (ADR 0018 step 8).
var entrySnaps = actionlog.NewSnapshotter[entriesState](StateKey)

func toState(entries []classes.Entry) *entriesState {
	out := &entriesState{Entries: make([]entrySnap, len(entries))}
	for i, e := range entries {
		out.Entries[i] = entrySnap{
			Class: e.Class, RealizedLevel: e.RealizedLevel, StartingXP: e.StartingXP,
			State: string(e.State), LeftAtLevel: e.LeftAtLevel, IgnoredAwardsThrough: e.IgnoredAwardsThrough,
		}
	}
	return out
}

func (st entriesState) entries() []classes.Entry {
	out := make([]classes.Entry, len(st.Entries))
	for i, e := range st.Entries {
		out[i] = classes.Entry{
			Class: e.Class, RealizedLevel: e.RealizedLevel, StartingXP: e.StartingXP,
			State: classes.EntryState(e.State), LeftAtLevel: e.LeftAtLevel, IgnoredAwardsThrough: e.IgnoredAwardsThrough,
		}
	}
	return out
}

// record is the payload of a class_switch entry. The UI words it. Add
// fields, never rename.
type record struct {
	From        string `json:"from"`
	To          string `json:"to"`
	LeftAtLevel int    `json:"leftAtLevel"`
	XP          int    `json:"xp"`
}

// record writes the class_switch entry with the entries' snapshot, inside the
// caller's transaction (ADR 0018 step 1).
func (s *service) record(ctx context.Context, characterID int64, r Result, before, after []classes.Entry) error {
	payload, err := json.Marshal(record(r))
	if err != nil {
		return fmt.Errorf("encode class_switch payload: %w", err)
	}
	snap, err := entrySnaps.Snapshot("", toState(before), toState(after))
	if err != nil {
		return err
	}
	if _, err := s.Log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: actionlog.KindClassSwitch, Payload: payload,
		Snapshots: []actionlog.Snapshot{snap},
	}); err != nil {
		return fmt.Errorf("record class_switch: %w", err)
	}
	return nil
}

// Module declares the codec that writes the class entries back: the undo of a
// switch gives the single entry back exactly, a redo takes the class again
// (ADR 0019). It runs inside the undo's transaction.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "dualclasssvc", Codecs: []actionlog.Codec{
		entrySnaps.Restorer(func(ctx context.Context, characterID int64, _ string, st entriesState) error {
			if err := s.DAL.ReplaceClassEntries(ctx, characterID, st.entries()); err != nil {
				return fmt.Errorf("restore class entries: %w", err)
			}
			return nil
		}, nil),
	}}
}
