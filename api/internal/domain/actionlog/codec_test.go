package actionlog_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ADR 0018 step 8: the log's codec. A family's things are written back
// through the service's write, absent ones through its remove; the envelope
// names the thing; a redo stamps the replayed entry where the family asks.

type thing struct {
	Count   int   `json:"count"`
	EntryID int64 `json:"entryId,omitempty"`
}

// call is one write or remove a restorer made.
type call struct {
	op    string
	char  int64
	id    string
	state thing
}

func recorder(calls *[]call, fail error) (actionlog.WriteFunc[thing], actionlog.RemoveFunc) {
	write := func(_ context.Context, characterID int64, id string, st thing) error {
		*calls = append(*calls, call{"write", characterID, id, st})
		return fail
	}
	remove := func(_ context.Context, characterID int64, id string) error {
		*calls = append(*calls, call{"remove", characterID, id, thing{}})
		return fail
	}
	return write, remove
}

func TestSnapshotter_EncodesBeforeAndAfterInTheEnvelope(t *testing.T) {
	things := actionlog.NewSnapshotter[thing]("things").MayBeAbsent()

	snap, err := things.Snapshot("arrows", nil, &thing{Count: 3})
	require.NoError(t, err)
	assert.Equal(t, "things:arrows", snap.Key)
	assert.JSONEq(t, `{"id":"arrows","absent":true}`, string(snap.Before), "not created yet")
	assert.JSONEq(t, `{"id":"arrows","state":{"count":3}}`, string(snap.After))

	snap, err = things.Snapshot("arrows", &thing{Count: 1}, nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"arrows","state":{"count":1}}`, string(snap.Before))
	assert.JSONEq(t, `{"id":"arrows","absent":true}`, string(snap.After), "gone")

	round := actionlog.NewSnapshotter[int]("round")
	snap, err = round.Snapshot("", new(0), new(4))
	require.NoError(t, err)
	assert.Equal(t, "round", snap.Key, "a key without an id is the family")
	assert.JSONEq(t, `{"state":0}`, string(snap.Before), "a zero state is a state, not an absence")
	assert.JSONEq(t, `{"state":4}`, string(snap.After))
}

// A family whose things are always present refuses an absent state, both
// ways.
func TestSnapshotter_AbsentOnlyWhereTheFamilyAllowsIt(t *testing.T) {
	round := actionlog.NewSnapshotter[int]("round")
	_, err := round.Snapshot("", nil, new(1))
	require.Error(t, err)

	var calls []call
	write := func(context.Context, int64, string, int) error { calls = append(calls, call{op: "write"}); return nil }
	require.Error(t, round.Restorer(write, nil).Restore(context.Background(), 1, json.RawMessage(`{"absent":true}`)))
	assert.Empty(t, calls, "nothing written")
}

func TestSnapshotter_RestoreWritesTheStateOrRemovesTheThing(t *testing.T) {
	ctx := context.Background()
	var calls []call
	write, remove := recorder(&calls, nil)
	r := actionlog.NewSnapshotter[thing]("things").MayBeAbsent().Restorer(write, remove)

	snap, err := r.Snapshot("arrows", nil, &thing{Count: 3})
	require.NoError(t, err)
	require.NoError(t, r.Restore(ctx, 7, snap.Before), "undo of a creation")
	require.NoError(t, r.Restore(ctx, 7, snap.After), "redo of it")
	assert.Equal(t, []call{
		{"remove", 7, "arrows", thing{}},
		{"write", 7, "arrows", thing{Count: 3}},
	}, calls)
}

// The envelope's id is the thing restored: a thing renamed outside the log
// is restored under the name its envelope carries, whatever its key says.
func TestSnapshotter_TheEnvelopeNamesTheThing(t *testing.T) {
	var calls []call
	write, remove := recorder(&calls, nil)
	r := actionlog.NewSnapshotter[thing]("things").MayBeAbsent().Restorer(write, remove)
	require.NoError(t, r.Restore(context.Background(), 7, json.RawMessage(`{"id":"dagger_2","state":{"count":1}}`)))
	assert.Equal(t, []call{{"write", 7, "dagger_2", thing{Count: 1}}}, calls)
}

func TestSnapshotter_AFailingWriteFailsTheRestore(t *testing.T) {
	boom := errors.New("boom")
	var calls []call
	write, remove := recorder(&calls, boom)
	r := actionlog.NewSnapshotter[thing]("things").MayBeAbsent().Restorer(write, remove)
	err := r.Restore(context.Background(), 7, json.RawMessage(`{"id":"a","state":{"count":1}}`))
	require.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "things:a", "the error names the key")
	require.ErrorIs(t, r.Restore(context.Background(), 7, json.RawMessage(`{"id":"a","absent":true}`)), boom)
}

// ADR 0019 step 1: a redo writes the "after" back through Replay, with the
// replayed entry's id for a family whose "after" cannot name its entry; an
// undo (Restore) never stamps, and a state that already names its entry is
// kept.
func TestSnapshotter_ReplayStampsTheReplayedEntry(t *testing.T) {
	ctx := context.Background()
	var calls []call
	write, remove := recorder(&calls, nil)
	r := actionlog.NewSnapshotter[thing]("initiative").MayBeAbsent().
		WithReplay(func(st *thing, entryID int64) {
			if st.EntryID == 0 {
				st.EntryID = entryID
			}
		}).Restorer(write, remove)
	var _ actionlog.Replayer = r

	require.NoError(t, r.Replay(ctx, 7, 21, json.RawMessage(`{"state":{"count":8}}`)))
	require.NoError(t, r.Replay(ctx, 7, 21, json.RawMessage(`{"state":{"count":8,"entryId":12}}`)))
	require.NoError(t, r.Restore(ctx, 7, json.RawMessage(`{"state":{"count":8}}`)))
	require.NoError(t, r.Replay(ctx, 7, 21, json.RawMessage(`{"absent":true}`)))
	assert.Equal(t, []call{
		{"write", 7, "", thing{Count: 8, EntryID: 21}},
		{"write", 7, "", thing{Count: 8, EntryID: 12}},
		{"write", 7, "", thing{Count: 8}},
		{"remove", 7, "", thing{}},
	}, calls)

	// Without WithReplay, Replay is Restore.
	calls = nil
	plain := actionlog.NewSnapshotter[thing]("things").Restorer(write, nil)
	require.NoError(t, plain.Replay(ctx, 7, 21, json.RawMessage(`{"state":{"count":8}}`)))
	assert.Equal(t, []call{{"write", 7, "", thing{Count: 8}}}, calls)
}

// ADR 0018 step 9: no legacy decoder. A stored state that is not the
// envelope (the encodings of before the codec included) is refused, and
// nothing is written.
func TestDecodeEnvelope_RefusesWhatIsNotAnEnvelope(t *testing.T) {
	var calls []call
	write, remove := recorder(&calls, nil)
	r := actionlog.NewSnapshotter[thing]("things").MayBeAbsent().Restorer(write, remove)
	for _, raw := range []string{
		`[]`, `3`, `null`, `"seven"`, `{`, ``,
		`{"armor":"none"}`, `{"id":"x"}`, `[{"id":"x","count":2}]`,
		`{"id":"x","absent":true,"state":{"count":1}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			_, _, err := actionlog.DecodeEnvelope(json.RawMessage(raw))
			require.ErrorIs(t, err, actionlog.ErrNotAnEnvelope)
			require.ErrorIs(t, r.Restore(context.Background(), 7, json.RawMessage(raw)), actionlog.ErrNotAnEnvelope)
		})
	}
	assert.Empty(t, calls)
	// A state the service's type cannot read is refused too.
	require.Error(t, r.Restore(context.Background(), 7, json.RawMessage(`{"state":{"count":"x"}}`)))
	assert.Empty(t, calls)
}

// A snapshotter declared without a restorer encodes but cannot restore.
func TestSnapshotter_RestoreNeedsTheServicesWrite(t *testing.T) {
	things := actionlog.NewSnapshotter[thing]("things")
	require.Error(t, things.Restore(context.Background(), 7, json.RawMessage(`{"state":{"count":1}}`)))
}

func TestEncodeState_RefusesAnEmptyState(t *testing.T) {
	_, err := actionlog.EncodeState("x", nil)
	require.Error(t, err)
}

// A codec declared to the log must write back every state it decodes: one
// without a write, or whose things may be absent without a remove, is
// refused when it is registered (domain.Module), not at the player's undo.
func TestSnapshotter_CanRestoreOnlyWithItsWriteAndRemove(t *testing.T) {
	var calls []call
	write, remove := recorder(&calls, nil)
	tests := []struct {
		name  string
		codec actionlog.Codec
		ok    bool
	}{
		{"always present, with a write", actionlog.NewSnapshotter[thing]("things").Restorer(write, nil), true},
		{"may be absent, with a write and a remove", actionlog.NewSnapshotter[thing]("things").MayBeAbsent().Restorer(write, remove), true},
		{"no restorer", actionlog.NewSnapshotter[thing]("things"), false},
		{"no write", actionlog.NewSnapshotter[thing]("things").MayBeAbsent().Restorer(nil, remove), false},
		{"may be absent, no remove", actionlog.NewSnapshotter[thing]("things").MayBeAbsent().Restorer(write, nil), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.codec.CanRestore()
			if tc.ok {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, actionlog.ErrNoRestorer)
		})
	}
}
