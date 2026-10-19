package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// snapshotEnvelopesMigration adds the snapshot envelopes. Right after it the run re-encodes every
// stored snapshot into them (reencodeSnapshots). The envelopes stay nullable in this release (ADR
// 0024 step 5.2: the release before keeps recording snapshots during a rolling deploy); the
// migration that makes them required runs the step again just before it, for the rows that
// release wrote meanwhile.
const snapshotEnvelopesMigration = "019_snapshot_envelopes.sql"

// reencodeSnapshots writes every stored snapshot's "before" and "after" again as the action log
// codec's envelope (ADR 0018 steps 8 and 9): the same state, in the encoding the restorers read.
// It changes an encoding, never a before, an after, a delta, a payload or a correction: each
// family's old encoding (the snapshot encoders of before the codec) is read here and nowhere else.
// The old columns are left as they are, for the release before the codec during a rolling deploy.
// It reads only the rows without their envelopes, so it can run again: a row the release before
// recorded after a first run is re-encoded by the next one, and a re-encoded row is never read
// again.
// A snapshot of a family no restorer knows, or one that does not read, fails the run: the start
// fails, the run rolls back and nothing is half re-encoded.
func reencodeSnapshots(ctx context.Context, q storeq.Querier) error {
	rows, err := q.SnapshotsToReencode(ctx)
	if err != nil {
		return fmt.Errorf("list snapshots: %w", err)
	}
	for _, r := range rows {
		before, err := reencodeSnapshot(ctx, q, r.CharacterID, r.StateKey, r.BeforeState)
		if err != nil {
			return fmt.Errorf("action_snapshots %d before_state: %w", r.ID, err)
		}
		after, err := reencodeSnapshot(ctx, q, r.CharacterID, r.StateKey, r.AfterState)
		if err != nil {
			return fmt.Errorf("action_snapshots %d after_state: %w", r.ID, err)
		}
		if err := q.SetSnapshotEnvelopes(ctx, storeq.SetSnapshotEnvelopesParams{
			ID: r.ID, BeforeEnvelope: string(before), AfterEnvelope: string(after),
		}); err != nil {
			return fmt.Errorf("action_snapshots %d: %w", r.ID, err)
		}
	}
	return nil
}

// reencodeSnapshot is one stored state of key in the codec's envelope.
func reencodeSnapshot(ctx context.Context, q storeq.Querier, characterID int64, key, stored string) (json.RawMessage, error) {
	family, keyID, _ := strings.Cut(key, actionlog.KeySeparator)
	raw := json.RawMessage(stored)
	switch family {
	// The whole state, encoded as the service's state type already was.
	case "buffs", "saved_buffs", "round", "spell_slots", "realized_level", "skill_points", "skill_armor", "ac_override":
		return actionlog.EncodeState(keyID, raw)
	case "initiative":
		// JSON null was "no initiative".
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return actionlog.EncodeAbsent(keyID)
		}
		return actionlog.EncodeState(keyID, raw)
	case "spellbook":
		return reencodeSpellbook(keyID, raw)
	case "xp":
		return reencodeXP(keyID, raw)
	case "weapons":
		return reencodeWeapon(keyID, raw)
	case "items":
		return reencodeItem(ctx, q, characterID, keyID, raw)
	}
	return nil, fmt.Errorf("state key %q: no restorer reads this family", key)
}

// object reads a stored JSON object field by field, keeping every field it does not name.
func object(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var o map[string]json.RawMessage
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, err
	}
	if o == nil {
		return nil, errors.New("not an object")
	}
	return o, nil
}

// field decodes o[name] into v when it is set.
func field(o map[string]json.RawMessage, name string, v any) error {
	if b, ok := o[name]; ok {
		if err := json.Unmarshal(b, v); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// stateOf is o without the fields the envelope now carries.
func stateOf(o map[string]json.RawMessage, drop ...string) (json.RawMessage, error) {
	for _, k := range drop {
		delete(o, k)
	}
	return json.Marshal(o)
}

// reencodeSpellbook: {spellId, known, failedAtLevel}; the spell id goes to the envelope.
func reencodeSpellbook(keyID string, raw json.RawMessage) (json.RawMessage, error) {
	o, err := object(raw)
	if err != nil {
		return nil, fmt.Errorf("spellbook: %w", err)
	}
	id := keyID
	if err := field(o, "spellId", &id); err != nil {
		return nil, fmt.Errorf("spellbook: %w", err)
	}
	state, err := stateOf(o, "spellId")
	if err != nil {
		return nil, err
	}
	return actionlog.EncodeState(id, state)
}

// reencodeXP: {id, absent, date, session, reason, base, bonus}; the id and the absence go to
// the envelope.
func reencodeXP(keyID string, raw json.RawMessage) (json.RawMessage, error) {
	o, err := object(raw)
	if err != nil {
		return nil, fmt.Errorf("xp: %w", err)
	}
	id := keyID
	var num int64
	var absent bool
	if err := errors.Join(field(o, "id", &num), field(o, "absent", &absent)); err != nil {
		return nil, fmt.Errorf("xp: %w", err)
	}
	if num != 0 {
		id = strconv.FormatInt(num, 10)
	}
	if absent {
		return actionlog.EncodeAbsent(id)
	}
	state, err := stateOf(o, "id", "absent")
	if err != nil {
		return nil, err
	}
	return actionlog.EncodeState(id, state)
}

// reencodeWeapon: a weapon of before the one equipment model, {id, absent, position, name, …};
// the id and the absence go to the envelope, a plain-string name becomes a partial one.
func reencodeWeapon(keyID string, raw json.RawMessage) (json.RawMessage, error) {
	o, err := object(raw)
	if err != nil {
		return nil, fmt.Errorf("weapons: %w", err)
	}
	id := keyID
	var absent bool
	if err := errors.Join(field(o, "id", &id), field(o, "absent", &absent)); err != nil {
		return nil, fmt.Errorf("weapons: %w", err)
	}
	if absent {
		return actionlog.EncodeAbsent(id)
	}
	if err := localizeName(o); err != nil {
		return nil, fmt.Errorf("weapons: %w", err)
	}
	state, err := stateOf(o, "id", "absent")
	if err != nil {
		return nil, err
	}
	return actionlog.EncodeState(id, state)
}

// reencodeItem: a list of one count, [{id, count, item, removed, absent, replace, model}]. The
// id goes to the envelope, read through the conversion's renames when the snapshot predates the
// one equipment model (no model marker, ADR 0033 step 26.3); removed or absent is the
// envelope's absence; the item's plain-string name becomes a partial one.
func reencodeItem(ctx context.Context, q storeq.Querier, characterID int64, keyID string, raw json.RawMessage) (json.RawMessage, error) {
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("items: %w", err)
	}
	if len(list) != 1 {
		return nil, fmt.Errorf("items: %d counts in one snapshot, want 1", len(list))
	}
	o, err := object(list[0])
	if err != nil {
		return nil, fmt.Errorf("items: %w", err)
	}
	id := keyID
	var model int
	var absent, removed bool
	if err := errors.Join(field(o, "id", &id), field(o, "model", &model),
		field(o, "absent", &absent), field(o, "removed", &removed)); err != nil {
		return nil, fmt.Errorf("items: %w", err)
	}
	if model == 0 {
		renamed, err := q.RenamedItemID(ctx, storeq.RenamedItemIDParams{CharacterID: characterID, OldID: id})
		switch {
		case err == nil:
			id = renamed
		case !errors.Is(err, sql.ErrNoRows):
			return nil, fmt.Errorf("items: renamed id of %s: %w", id, err)
		}
	}
	if absent || removed {
		return actionlog.EncodeAbsent(id)
	}
	if b, ok := o["item"]; ok && !bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		item, err := object(b)
		if err != nil {
			return nil, fmt.Errorf("items: item: %w", err)
		}
		if err := localizeName(item); err != nil {
			return nil, fmt.Errorf("items: %w", err)
		}
		if o["item"], err = json.Marshal(item); err != nil {
			return nil, err
		}
	}
	state, err := stateOf(o, "id", "model", "absent", "removed")
	if err != nil {
		return nil, err
	}
	return actionlog.EncodeState(id, state)
}

// localizeName turns a name stored before names were localized (a plain string) into its
// fallback-language body, marked namePartial: restoring it replaces that body and keeps the
// other languages (ADR 0023 step 11). A language map is kept as it is.
func localizeName(o map[string]json.RawMessage) error {
	b := bytes.TrimSpace(o["name"])
	if len(b) == 0 || b[0] != '"' {
		return nil
	}
	var plain string
	if err := json.Unmarshal(b, &plain); err != nil {
		return fmt.Errorf("name: %w", err)
	}
	name, err := json.Marshal(localized.Of(plain))
	if err != nil {
		return err
	}
	o["name"], o["namePartial"] = name, json.RawMessage(`true`)
	return nil
}
