package learnsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// spellState is the state of a "spellbook:<spell key>" snapshot (the
// envelope's id names the spell, SpellID is not encoded): the spell is in the
// spellbook, or blocked by a failure at a realized level (0: none). Entries
// already in the log must stay decodable: add fields, never rename.
type spellState struct {
	SpellID       string `json:"-"`
	Known         bool   `json:"known"`
	FailedAtLevel int    `json:"failedAtLevel"`
}

// record is the payload of a spell_learn entry, the roll's breakdown in the
// /api/character/spellbook/learn wire names. The UI words it. Add fields,
// never rename.
type record struct {
	SpellID        string `json:"spellId"`
	SpellLevel     int    `json:"spellLevel"`
	Roll           int    `json:"roll"`
	Chance         int    `json:"chance"`
	Learned        bool   `json:"learned"`
	CharacterLevel int    `json:"characterLevel"`
	RetryLevel     int    `json:"retryLevel"`
}

// spellbookSnaps is the snapshotter of a spell's place in the spellbook (ADR
// 0018 step 8); the key's id is the spell.
var spellbookSnaps = actionlog.NewSnapshotter[spellState](StateFamily)

// state reads the spell's place in the character's spellbook.
func (s *service) state(ctx context.Context, characterID int64, spellID string) (spellState, error) {
	keys, err := s.d.Book.Keys(ctx, characterID)
	if err != nil {
		return spellState{}, fmt.Errorf("read spellbook: %w", err)
	}
	st := spellState{SpellID: spellID}
	for _, k := range keys {
		if k == spellID {
			st.Known = true
		}
	}
	level, failed, err := s.d.DAL.LearnFailure(ctx, characterID, spellID)
	if err != nil {
		return spellState{}, fmt.Errorf("read learn failure: %w", err)
	}
	if failed {
		st.FailedAtLevel = level
	}
	return st, nil
}

// write sets the spell's place in the character's spellbook to st.
func (s *service) write(ctx context.Context, characterID int64, spellID string, st spellState) error {
	if st.Known {
		if err := s.d.DAL.AddEntry(ctx, characterID, spellID); err != nil {
			return fmt.Errorf("add spellbook entry: %w", err)
		}
	} else if err := s.d.DAL.RemoveEntry(ctx, characterID, spellID); err != nil {
		return fmt.Errorf("remove spellbook entry: %w", err)
	}
	if st.FailedAtLevel > 0 {
		if err := s.d.DAL.SetLearnFailure(ctx, characterID, spellID, st.FailedAtLevel); err != nil {
			return fmt.Errorf("set learn failure: %w", err)
		}
	} else if err := s.d.DAL.DeleteLearnFailure(ctx, characterID, spellID); err != nil {
		return fmt.Errorf("delete learn failure: %w", err)
	}
	return nil
}

// record writes the spell_learn entry with the spell's snapshot, inside the
// caller's transaction (ADR 0030 step 20.3, ADR 0018 step 1).
func (s *service) record(ctx context.Context, characterID int64, r Result, before, after spellState) error {
	payload, err := json.Marshal(record(r))
	if err != nil {
		return fmt.Errorf("encode spell_learn payload: %w", err)
	}
	snap, err := spellbookSnaps.Snapshot(r.SpellID, &before, &after)
	if err != nil {
		return err
	}
	if _, err := s.d.Log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: actionlog.KindSpellLearn, Payload: payload,
		Snapshots: []actionlog.Snapshot{snap},
	}); err != nil {
		return fmt.Errorf("record spell_learn: %w", err)
	}
	return nil
}

// Module declares the codec that writes a spell's spellbook state back:
// undoing a success takes the spell out of the spellbook, undoing a failure
// lifts its block (step 20.3); a redo writes the roll's "after" (ADR 0019).
// The action log hands the state, which names the spell; it runs inside the
// undo's transaction.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "learnsvc", Codecs: []actionlog.Codec{
		spellbookSnaps.Restorer(func(ctx context.Context, characterID int64, spellID string, st spellState) error {
			if spellID == "" || st.FailedAtLevel < 0 {
				return fmt.Errorf("decode spellbook snapshot: spell %q failed at level %d", spellID, st.FailedAtLevel)
			}
			st.SpellID = spellID
			if err := s.write(ctx, characterID, spellID, st); err != nil {
				return fmt.Errorf("restore spellbook: %w", err)
			}
			return nil
		}, nil),
	}}
}
