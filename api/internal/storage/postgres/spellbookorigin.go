package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/spellbook"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// spellbookOriginMigration adds spellbook_entries.origin and grant_level, every stored entry
// 'starting'. Right after it the run marks the learned ones (markLearnedSpells, ADR 0030 step
// 20.5).
const spellbookOriginMigration = "028_spellbook_origin.sql"

// markLearnedSpells is ADR 0030 step 20.5's data step, after migration 028: a stored entry that
// a successful spell_learn journal entry wrote (its payload's spellId, learned true, for the same
// character) is 'learned'; every other one stays 'starting' (Kelsen's imported book). Only a
// 'starting' entry is marked, so a rerun, or a run after an entry was given, changes nothing. No
// entry is added or removed (step 20.9). A success later undone still counts: before origins,
// only a Learn success wrote an entry other than the seed's, and the seed writes a book only
// while it is empty, so an entry with a success in the log is that success's (or its redo's).
func markLearnedSpells(ctx context.Context, q storeq.Querier) error {
	logged, err := q.SpellLearnPayloads(ctx)
	if err != nil {
		return fmt.Errorf("list spell_learn entries: %w", err)
	}
	learned := map[int64]map[string]bool{}
	for _, r := range logged {
		var p struct {
			SpellID string `json:"spellId"`
			Learned bool   `json:"learned"`
		}
		if err := json.Unmarshal([]byte(r.Payload), &p); err != nil {
			return fmt.Errorf("spell_learn entry of character %d: %w", r.CharacterID, err)
		}
		if !p.Learned || p.SpellID == "" {
			continue
		}
		if learned[r.CharacterID] == nil {
			learned[r.CharacterID] = map[string]bool{}
		}
		learned[r.CharacterID][p.SpellID] = true
	}
	if len(learned) == 0 {
		return nil
	}
	entries, err := q.SpellbookEntryOrigins(ctx)
	if err != nil {
		return fmt.Errorf("list spellbook entries: %w", err)
	}
	for _, e := range entries {
		if spellbook.Origin(e.Origin) != spellbook.OriginStarting || !learned[e.CharacterID][e.SpellKey] {
			continue
		}
		if err := q.SetSpellbookEntryOrigin(ctx, storeq.SetSpellbookEntryOriginParams{
			CharacterID: e.CharacterID, SpellKey: e.SpellKey, Origin: string(spellbook.OriginLearned),
		}); err != nil {
			return fmt.Errorf("mark %q learned for character %d: %w", e.SpellKey, e.CharacterID, err)
		}
	}
	return nil
}
