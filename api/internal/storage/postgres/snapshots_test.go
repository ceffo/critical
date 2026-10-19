package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/storage/postgres/snapshotfixtures"
)

// ADR 0018 step 9, ADR 0025 step 2.1.1: the data step re-encodes every stored snapshot into the
// action log codec's envelope, one fixture per old encoding (snapshotfixtures.Old). It changes the
// encoding only: the same state, the same absence. composition
// TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage restores the same envelopes.
var oldSnapshots = snapshotfixtures.Old

const snapshotOwner = snapshotfixtures.Owner

func reencodeStore() *fakeStore {
	store := newFakeStore()
	for old, renamed := range snapshotfixtures.OwnerRenames {
		store.t.Renames[rowKey(snapshotOwner, old)] = renamed
	}
	for i, s := range oldSnapshots {
		store.t.Snapshots = append(store.t.Snapshots, snapshotRow{
			ID: int64(i + 1), ActionID: int64(i + 1), Key: s.Key, Before: s.Stored, After: s.Stored, CharacterID: snapshotOwner,
		})
	}
	return store
}

func TestReencodeSnapshots_EveryOldEncodingBecomesTheEnvelope(t *testing.T) {
	store := reencodeStore()
	require.NoError(t, reencodeSnapshots(context.Background(), store))
	for i, s := range oldSnapshots {
		t.Run(s.Name, func(t *testing.T) {
			row := store.t.Snapshots[i]
			assert.JSONEq(t, s.Envelope, row.BeforeEnv)
			assert.JSONEq(t, s.Envelope, row.AfterEnv)
			assert.Equal(t, s.Stored, row.Before, "the old column is left as it was")
		})
	}
}

// The step runs on the migration run's transaction right after the envelopes are added
// (migration 019), on a database that gets 019 in this run; a database that records 019 never
// runs it again.
func TestMigrate_SnapshotsAreReencodedRightAfterTheEnvelopesAreAdded(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	var recorded []string
	for _, m := range embedded {
		if m.name < "019" {
			recorded = append(recorded, m.name)
		}
	}
	store := reencodeStore()
	f := &fakeMigrations{recorded: recorded, store: store}
	missingAtApply := -1
	f.onApply = func(m migration) {
		if m.name == snapshotEnvelopesMigration {
			missing, err := store.SnapshotsToReencode(context.Background())
			require.NoError(t, err)
			missingAtApply = len(missing)
		}
	}
	run := func() error {
		return store.InTx(context.Background(), func(ctx context.Context) error {
			before, after := migrationDataSteps(repoSeeds(), catalogueEquipment)
			return runMigrations(ctx, f, embedded, before, after)
		})
	}
	require.NoError(t, run())
	assert.Equal(t, len(oldSnapshots), missingAtApply, "the step runs after the columns exist")
	missing, err := store.SnapshotsToReencode(context.Background())
	require.NoError(t, err)
	assert.Empty(t, missing, "every snapshot has its envelopes when the run commits")

	// A rerun finds 019 recorded and runs no step: an envelope it wrote is not rewritten.
	store.t.Snapshots[0].BeforeEnv = `{"state":"kept"}`
	require.NoError(t, run())
	assert.JSONEq(t, `{"state":"kept"}`, store.t.Snapshots[0].BeforeEnv)
}

// The step reads only the rows without their envelopes, so it can run again (just before the
// migration that makes the envelopes required): a row the release before the codec recorded
// after the first run, during the rolling deploy, is re-encoded by the next run, and a row that
// already has its envelopes is not read again.
func TestReencodeSnapshots_ARerunEncodesOnlyWhatIsMissing(t *testing.T) {
	store := newFakeStore()
	store.t.Snapshots = []snapshotRow{
		{ID: 1, Key: "round", Before: `"garbage"`, After: `"garbage"`, BeforeEnv: `{"state":1}`, AfterEnv: `{"state":2}`},
		{ID: 2, Key: "round", Before: `2`, After: `3`},
	}
	require.NoError(t, reencodeSnapshots(context.Background(), store))
	assert.JSONEq(t, `{"state":1}`, store.t.Snapshots[0].BeforeEnv, "already re-encoded: not read again")
	assert.JSONEq(t, `{"state":2}`, store.t.Snapshots[1].BeforeEnv)
	assert.JSONEq(t, `{"state":3}`, store.t.Snapshots[1].AfterEnv)
}

// A snapshot no restorer reads, or one that does not decode, fails the step (and so the run,
// which rolls back): nothing is guessed.
func TestReencodeSnapshots_RefusesWhatItCannotRead(t *testing.T) {
	for _, tc := range []struct{ name, key, stored string }{
		{"unknown family", "hit_points", `{"hp":3}`},
		{"items with two counts", "items:a", `[{"id":"a","count":1},{"id":"b","count":2}]`},
		{"items not a list", "items:a", `{"id":"a","count":1}`},
		{"not JSON", "round", `{`},
		{"xp not an object", "xp:1", `[]`},
		{"spellbook with a bad spell id", "spellbook:x", `{"spellId":3}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			store.t.Snapshots = []snapshotRow{{ID: 9, Key: tc.key, Before: tc.stored, After: tc.stored}}
			err := reencodeSnapshots(context.Background(), store)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "action_snapshots 9")
		})
	}
}
