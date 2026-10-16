package sqlitecopy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/sqlitecopy"
	"github.com/ceffo/critical/api/internal/sqlitecopy/mocks"
)

var anyArg = mock.Anything

type txKey struct{}

// inTx matches a context InTx handed to its function: what must happen inside the one transaction.
var inTx = mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txKey{}) != nil })

// outsideTx matches a context no transaction carries.
var outsideTx = mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txKey{}) == nil })

// runInTx makes the target's InTx call fn with a context marked as the transaction's, and
// return what fn returns (the real one commits on nil and rolls back otherwise).
func runInTx(dst *mocks.MockTarget) {
	dst.EXPECT().InTx(anyArg, anyArg).RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		return fn(context.WithValue(ctx, txKey{}, true))
	}).Once()
}

// Three tables: characters, action_log referencing characters and itself, and a table of its own.
var (
	characters = sqlitecopy.Table{Name: "characters", Columns: []string{"id", "name"}, Identity: "id"}
	actionLog  = sqlitecopy.Table{Name: "action_log", Columns: []string{"id", "character_id", "transaction_id"}, Identity: "id"}
	weapons    = sqlitecopy.Table{Name: "weapons", Columns: []string{"id", "position"}}
	schema     = []sqlitecopy.Table{actionLog, characters, weapons}
	keys       = []sqlitecopy.ForeignKey{
		{Table: "action_log", References: "characters"},
		{Table: "action_log", References: "action_log"},
	}
	baseline = []string{"001_baseline.sql"}
)

func names(tables []sqlitecopy.Table) []string {
	out := make([]string, len(tables))
	for i, t := range tables {
		out[i] = t.Name
	}
	return out
}

// ADR 0025 step 9.1: every table comes after the tables it references; a self-reference does not
// constrain the order; ties go by name, so every run copies in the same order.
func TestDependencyOrder_Step9_1_ReferencedTablesFirst(t *testing.T) {
	tests := []struct {
		name   string
		tables []sqlitecopy.Table
		fks    []sqlitecopy.ForeignKey
		want   []string
	}{
		{
			"no keys: by name",
			[]sqlitecopy.Table{weapons, characters, actionLog},
			nil,
			[]string{"action_log", "characters", "weapons"},
		},
		{
			"child after parent, self-reference ignored", schema, keys,
			[]string{"characters", "action_log", "weapons"},
		},
		{
			"a chain",
			[]sqlitecopy.Table{{Name: "a"}, {Name: "b"}, {Name: "c"}},
			[]sqlitecopy.ForeignKey{{Table: "a", References: "b"}, {Table: "b", References: "c"}},
			[]string{"c", "b", "a"},
		},
		{
			"two parents",
			[]sqlitecopy.Table{{Name: "a"}, {Name: "z"}, {Name: "m"}},
			[]sqlitecopy.ForeignKey{{Table: "a", References: "z"}, {Table: "a", References: "m"}},
			[]string{"m", "z", "a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sqlitecopy.DependencyOrder(tt.tables, tt.fks)
			require.NoError(t, err)
			assert.Equal(t, tt.want, names(got))
		})
	}
}

func TestDependencyOrder_RefusesACycleOrAnUnknownTable(t *testing.T) {
	_, err := sqlitecopy.DependencyOrder([]sqlitecopy.Table{{Name: "a"}, {Name: "b"}},
		[]sqlitecopy.ForeignKey{{Table: "a", References: "b"}, {Table: "b", References: "a"}})
	assert.ErrorIs(t, err, sqlitecopy.ErrSchemaMismatch)
	_, err = sqlitecopy.DependencyOrder([]sqlitecopy.Table{{Name: "a"}},
		[]sqlitecopy.ForeignKey{{Table: "a", References: "gone"}})
	assert.ErrorIs(t, err, sqlitecopy.ErrSchemaMismatch)
}

// ADR 0025 step 9.1: the next id is past the highest id copied and past every id SQLite handed
// out, deleted rows included (an undo restores a deleted row under its old id).
func TestNextID_Step9_1_PastTheHighestCopiedAndHandedOutID(t *testing.T) {
	tests := []struct {
		name       string
		maxID, seq int64
		want       int64
	}{
		{"empty table, no sequence", 0, 0, 1},
		{"rows, no sequence (not AUTOINCREMENT)", 6, 0, 7},
		{"sequence equals the highest id", 889, 889, 890},
		{"the last rows were deleted", 0, 1, 2},
		{"sequence past the highest id", 886, 9745, 9746},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sqlitecopy.NextID(tt.maxID, tt.seq))
		})
	}
}

// ADR 0025 step 9.1: every identity column, and only those, is set from the target's highest id
// and the SQLite sequence of its table.
func TestResetIdentities_Step9_1_SetsEveryIdentityColumn(t *testing.T) {
	ctx := context.Background()
	src := mocks.NewMockSource(t)
	dst := mocks.NewMockTarget(t)
	dst.EXPECT().MaxID(ctx, "characters", "id").Return(1, nil).Once()
	src.EXPECT().Sequence(ctx, "characters").Return(1, nil).Once()
	dst.EXPECT().SetNextID(ctx, "characters", "id", int64(2)).Return(nil).Once()
	dst.EXPECT().MaxID(ctx, "action_log", "id").Return(0, nil).Once()
	src.EXPECT().Sequence(ctx, "action_log").Return(12, nil).Once()
	dst.EXPECT().SetNextID(ctx, "action_log", "id", int64(13)).Return(nil).Once()

	require.NoError(t, sqlitecopy.ResetIdentities(ctx, src, dst, []sqlitecopy.Table{characters, actionLog, weapons}))
}

// ADR 0025 step 9.1: the counts of every table are compared, and any difference fails, naming
// each table that differs.
func TestCompareCounts_Step9_1_FailsOnAnyDifference(t *testing.T) {
	ctx := context.Background()
	t.Run("equal", func(t *testing.T) {
		src := mocks.NewMockSource(t)
		dst := mocks.NewMockTarget(t)
		for table, n := range map[string]int64{"characters": 1, "action_log": 889, "weapons": 0} {
			src.EXPECT().Count(ctx, table).Return(n, nil).Once()
			dst.EXPECT().Count(ctx, table).Return(n, nil).Once()
		}
		got, err := sqlitecopy.CompareCounts(ctx, src, dst, []sqlitecopy.Table{characters, actionLog, weapons})
		require.NoError(t, err)
		assert.Equal(t, []sqlitecopy.TableCount{{"characters", 1}, {"action_log", 889}, {"weapons", 0}}, got)
	})
	t.Run("one differs", func(t *testing.T) {
		src := mocks.NewMockSource(t)
		dst := mocks.NewMockTarget(t)
		src.EXPECT().Count(ctx, "characters").Return(1, nil).Once()
		dst.EXPECT().Count(ctx, "characters").Return(1, nil).Once()
		src.EXPECT().Count(ctx, "action_log").Return(889, nil).Once()
		dst.EXPECT().Count(ctx, "action_log").Return(888, nil).Once()
		src.EXPECT().Count(ctx, "weapons").Return(6, nil).Once()
		dst.EXPECT().Count(ctx, "weapons").Return(7, nil).Once()
		_, err := sqlitecopy.CompareCounts(ctx, src, dst, []sqlitecopy.Table{characters, actionLog, weapons})
		require.ErrorIs(t, err, sqlitecopy.ErrCountMismatch)
		assert.ErrorContains(t, err, "action_log: SQLite 889, target 888; weapons: SQLite 6, target 7")
	})
}

// ADR 0025 step 9.2: a target holding a row in any app table is refused, naming the tables.
func TestRefuseNonEmpty_Step9_2_RefusesARowInAnyTable(t *testing.T) {
	ctx := context.Background()
	dst := mocks.NewMockTarget(t)
	dst.EXPECT().Count(ctx, "characters").Return(0, nil).Once()
	dst.EXPECT().Count(ctx, "action_log").Return(0, nil).Once()
	dst.EXPECT().Count(ctx, "weapons").Return(6, nil).Once()
	err := sqlitecopy.RefuseNonEmpty(ctx, dst, []sqlitecopy.Table{characters, actionLog, weapons})
	require.ErrorIs(t, err, sqlitecopy.ErrTargetNotEmpty)
	assert.ErrorContains(t, err, "weapons (6)")

	empty := mocks.NewMockTarget(t)
	empty.EXPECT().Count(ctx, anyArg).Return(0, nil).Times(3)
	assert.NoError(t, sqlitecopy.RefuseNonEmpty(ctx, empty, []sqlitecopy.Table{characters, actionLog, weapons}))
}

// ADR 0025 step 9.2: a new database (no table) is migrated to the baseline; a database on the
// baseline is accepted as it is; anything else is refused before it is changed.
func TestPrepareTarget_Step9_2_OnlyANewOrBaselineDatabase(t *testing.T) {
	ctx := context.Background()
	t.Run("new: migrated", func(t *testing.T) {
		dst := mocks.NewMockTarget(t)
		dst.EXPECT().Tables(ctx).Return(nil, nil).Once()
		dst.EXPECT().Migrate(ctx).Return(nil).Once()
		dst.EXPECT().Applied(ctx).Return(baseline, nil).Once()
		assert.NoError(t, sqlitecopy.PrepareTarget(ctx, dst, baseline))
	})
	t.Run("on the baseline: not migrated", func(t *testing.T) {
		dst := mocks.NewMockTarget(t)
		dst.EXPECT().Tables(ctx).Return([]string{"characters", "schema_migrations"}, nil).Once()
		dst.EXPECT().Applied(ctx).Return(baseline, nil).Once()
		assert.NoError(t, sqlitecopy.PrepareTarget(ctx, dst, baseline))
	})
	t.Run("tables without bookkeeping: refused, not migrated", func(t *testing.T) {
		dst := mocks.NewMockTarget(t)
		dst.EXPECT().Tables(ctx).Return([]string{"foo"}, nil).Once()
		assert.ErrorIs(t, sqlitecopy.PrepareTarget(ctx, dst, baseline), sqlitecopy.ErrNotOnBaseline)
	})
	t.Run("other migrations: refused", func(t *testing.T) {
		dst := mocks.NewMockTarget(t)
		dst.EXPECT().Tables(ctx).Return([]string{"schema_migrations"}, nil).Once()
		dst.EXPECT().Applied(ctx).Return([]string{"001_baseline.sql", "002_newer.sql"}, nil).Once()
		err := sqlitecopy.PrepareTarget(ctx, dst, baseline)
		require.ErrorIs(t, err, sqlitecopy.ErrNotOnBaseline)
		assert.ErrorContains(t, err, "002_newer.sql")
	})
}

// ADR 0025 step 9.1: the SQLite file is brought to the last SQLite schema: the migrations it does
// not record are applied in name order; one it records that the tool lacks is refused first.
func TestUpgradeSource_Step9_1_AppliesThePendingInOrder(t *testing.T) {
	ctx := context.Background()
	embedded := []sqlitecopy.Migration{{Name: "003_c.sql"}, {Name: "001_a.sql"}, {Name: "002_b.sql"}}
	t.Run("pending in order", func(t *testing.T) {
		src := mocks.NewMockSource(t)
		src.EXPECT().Applied(ctx).Return([]string{"001_a.sql"}, nil).Once()
		var applied []string
		src.EXPECT().Apply(ctx, anyArg).RunAndReturn(func(_ context.Context, m sqlitecopy.Migration) error {
			applied = append(applied, m.Name)
			return nil
		}).Times(2)
		require.NoError(t, sqlitecopy.UpgradeSource(ctx, src, embedded))
		assert.Equal(t, []string{"002_b.sql", "003_c.sql"}, applied)
	})
	t.Run("up to date: nothing applied", func(t *testing.T) {
		src := mocks.NewMockSource(t)
		src.EXPECT().Applied(ctx).Return([]string{"001_a.sql", "002_b.sql", "003_c.sql"}, nil).Once()
		require.NoError(t, sqlitecopy.UpgradeSource(ctx, src, embedded))
	})
	t.Run("newer file: refused, nothing applied", func(t *testing.T) {
		src := mocks.NewMockSource(t)
		src.EXPECT().Applied(ctx).Return([]string{"001_a.sql", "004_d.sql"}, nil).Once()
		err := sqlitecopy.UpgradeSource(ctx, src, embedded)
		require.ErrorIs(t, err, sqlitecopy.ErrUnknownSourceMigrations)
		assert.ErrorContains(t, err, "004_d.sql")
	})
}

// A table or column on one side only would lose or invent data: refused.
func TestCheckSourceSchema_RefusesATableOrColumnOnOneSideOnly(t *testing.T) {
	ctx := context.Background()
	t.Run("same", func(t *testing.T) {
		src := mocks.NewMockSource(t)
		src.EXPECT().Tables(ctx).Return([]string{"characters", "weapons"}, nil).Once()
		src.EXPECT().Columns(ctx, "characters").Return([]string{"name", "id"}, nil).Once()
		src.EXPECT().Columns(ctx, "weapons").Return([]string{"id", "position"}, nil).Once()
		assert.NoError(t, sqlitecopy.CheckSourceSchema(ctx, src, []sqlitecopy.Table{characters, weapons}))
	})
	t.Run("differs", func(t *testing.T) {
		src := mocks.NewMockSource(t)
		src.EXPECT().Tables(ctx).Return([]string{"characters", "legacy"}, nil).Once()
		src.EXPECT().Columns(ctx, "characters").Return([]string{"id", "nickname"}, nil).Once()
		err := sqlitecopy.CheckSourceSchema(ctx, src, []sqlitecopy.Table{characters, weapons})
		require.ErrorIs(t, err, sqlitecopy.ErrSchemaMismatch)
		for _, want := range []string{
			"characters: the SQLite file lacks name", "characters: the target lacks nickname",
			"table weapons is missing from the SQLite file", "table legacy is missing from the target",
		} {
			assert.ErrorContains(t, err, want)
		}
	})
}

// expectReadySource sets up a SQLite file on the last schema with the three tables.
func expectReadySource(src *mocks.MockSource) {
	src.EXPECT().Applied(anyArg).Return(baseline, nil).Once()
	src.EXPECT().Tables(anyArg).Return([]string{"action_log", "characters", "weapons"}, nil).Once()
	for _, tb := range schema {
		src.EXPECT().Columns(anyArg, tb.Name).Return(tb.Columns, nil).Once()
	}
}

// ADR 0025 step 9.1: the copy runs in one transaction: emptiness check, every table in dependency
// order with the source's rows, the identities, then the counts, all inside it.
func TestRun_Step9_1_CopiesEveryTableInOrderInOneTransaction(t *testing.T) {
	ctx := context.Background()
	src := mocks.NewMockSource(t)
	dst := mocks.NewMockTarget(t)
	embedded := []sqlitecopy.Migration{{Name: "001_baseline.sql"}}
	expectReadySource(src)
	dst.EXPECT().Tables(outsideTx).Return([]string{"schema_migrations"}, nil).Once()
	dst.EXPECT().Applied(outsideTx).Return(baseline, nil).Once()
	dst.EXPECT().Schema(outsideTx).Return(schema, keys, nil).Once()
	runInTx(dst)

	var steps []string
	rows := map[string][][]any{
		"characters": {{int64(1), "x"}},
		"action_log": {{int64(1), int64(1), int64(1)}, {int64(2), int64(1), int64(1)}},
		"weapons":    {},
	}
	counts := map[string]int64{"characters": 1, "action_log": 2, "weapons": 0}
	copied := map[string]int64{}
	dst.EXPECT().Count(inTx, anyArg).RunAndReturn(func(_ context.Context, table string) (int64, error) {
		steps = append(steps, "count "+table)
		return copied[table], nil
	}).Times(6)
	src.EXPECT().Rows(inTx, anyArg, anyArg, anyArg).RunAndReturn(
		func(_ context.Context, table string, _ []string, emit func([]any) error) error {
			for _, r := range rows[table] {
				if err := emit(r); err != nil {
					return err
				}
			}
			return nil
		}).Times(3)
	dst.EXPECT().Copy(inTx, anyArg, anyArg, anyArg).RunAndReturn(
		func(_ context.Context, table string, cols []string, produce func(func([]any) error) error) (int64, error) {
			steps = append(steps, "copy "+table)
			var got [][]any
			err := produce(func(r []any) error { got = append(got, r); return nil })
			if len(got) > 0 {
				assert.Len(t, cols, len(got[0]))
			}
			assert.Equal(t, len(rows[table]), len(got), table)
			copied[table] = int64(len(got))
			return int64(len(got)), err
		}).Times(3)
	dst.EXPECT().MaxID(inTx, anyArg, "id").RunAndReturn(func(_ context.Context, table, _ string) (int64, error) {
		return copied[table], nil
	}).Times(2)
	src.EXPECT().Sequence(inTx, anyArg).Return(0, nil).Times(2)
	dst.EXPECT().SetNextID(inTx, "characters", "id", int64(2)).RunAndReturn(func(context.Context, string, string, int64) error {
		steps = append(steps, "ids characters")
		return nil
	}).Once()
	dst.EXPECT().SetNextID(inTx, "action_log", "id", int64(3)).RunAndReturn(func(context.Context, string, string, int64) error {
		steps = append(steps, "ids action_log")
		return nil
	}).Once()
	src.EXPECT().Count(inTx, anyArg).RunAndReturn(func(_ context.Context, table string) (int64, error) {
		return counts[table], nil
	}).Times(3)

	got, err := sqlitecopy.Run(ctx, src, dst, embedded, baseline)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"count characters", "count action_log", "count weapons",
		"copy characters", "copy action_log", "copy weapons",
		"ids characters", "ids action_log",
		"count characters", "count action_log", "count weapons",
	}, steps)
	assert.Equal(t, []sqlitecopy.TableCount{{"characters", 1}, {"action_log", 2}, {"weapons", 0}}, got)
}

// ADR 0025 step 9.2: a target that holds rows is refused inside the transaction, before anything
// is copied, and the transaction returns the refusal (so it rolls back).
func TestRun_Step9_2_ANonEmptyTargetCopiesNothing(t *testing.T) {
	ctx := context.Background()
	src := mocks.NewMockSource(t)
	dst := mocks.NewMockTarget(t)
	expectReadySource(src)
	dst.EXPECT().Tables(anyArg).Return([]string{"schema_migrations"}, nil).Once()
	dst.EXPECT().Applied(anyArg).Return(baseline, nil).Once()
	dst.EXPECT().Schema(anyArg).Return(schema, keys, nil).Once()
	var txErr error
	dst.EXPECT().InTx(anyArg, anyArg).RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		txErr = fn(context.WithValue(ctx, txKey{}, true))
		return txErr
	}).Once()
	dst.EXPECT().Count(inTx, "characters").Return(1, nil).Once()
	dst.EXPECT().Count(inTx, "action_log").Return(0, nil).Once()
	dst.EXPECT().Count(inTx, "weapons").Return(0, nil).Once()

	got, err := sqlitecopy.Run(ctx, src, dst, []sqlitecopy.Migration{{Name: "001_baseline.sql"}}, baseline)
	require.ErrorIs(t, err, sqlitecopy.ErrTargetNotEmpty)
	assert.ErrorIs(t, txErr, sqlitecopy.ErrTargetNotEmpty, "the transaction itself fails, so it rolls back")
	assert.Nil(t, got)
	dst.AssertNotCalled(t, "Copy", anyArg, anyArg, anyArg, anyArg)
}

// ADR 0025 step 9.1: a count difference fails the transaction, so nothing of the copy stays.
func TestRun_Step9_1_ACountDifferenceFailsTheTransaction(t *testing.T) {
	ctx := context.Background()
	src := mocks.NewMockSource(t)
	dst := mocks.NewMockTarget(t)
	expectReadySource(src)
	dst.EXPECT().Tables(anyArg).Return([]string{"schema_migrations"}, nil).Once()
	dst.EXPECT().Applied(anyArg).Return(baseline, nil).Once()
	dst.EXPECT().Schema(anyArg).Return(schema, keys, nil).Once()
	var txErr error
	dst.EXPECT().InTx(anyArg, anyArg).RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
		txErr = fn(context.WithValue(ctx, txKey{}, true))
		return txErr
	}).Once()
	targetCounts := []int64{0, 0, 0, 1, 1, 0} // empty before; action_log lost a row
	dst.EXPECT().Count(inTx, anyArg).RunAndReturn(func(context.Context, string) (int64, error) {
		n := targetCounts[0]
		targetCounts = targetCounts[1:]
		return n, nil
	}).Times(6)
	dst.EXPECT().Copy(inTx, anyArg, anyArg, anyArg).Return(0, nil).Times(3)
	dst.EXPECT().MaxID(inTx, anyArg, anyArg).Return(1, nil).Times(2)
	src.EXPECT().Sequence(inTx, anyArg).Return(0, nil).Times(2)
	dst.EXPECT().SetNextID(inTx, anyArg, anyArg, int64(2)).Return(nil).Times(2)
	src.EXPECT().Count(inTx, "characters").Return(1, nil).Once()
	src.EXPECT().Count(inTx, "action_log").Return(2, nil).Once()
	src.EXPECT().Count(inTx, "weapons").Return(0, nil).Once()

	_, err := sqlitecopy.Run(ctx, src, dst, []sqlitecopy.Migration{{Name: "001_baseline.sql"}}, baseline)
	require.ErrorIs(t, err, sqlitecopy.ErrCountMismatch)
	assert.ErrorIs(t, txErr, sqlitecopy.ErrCountMismatch)
}

// A refused SQLite file stops the run before the target is looked at.
func TestRun_ANewerSQLiteFileTouchesNoTarget(t *testing.T) {
	ctx := context.Background()
	src := mocks.NewMockSource(t)
	dst := mocks.NewMockTarget(t)
	src.EXPECT().Applied(ctx).Return([]string{"034_future.sql"}, nil).Once()
	_, err := sqlitecopy.Run(ctx, src, dst, []sqlitecopy.Migration{{Name: "001_a.sql"}}, baseline)
	assert.ErrorIs(t, err, sqlitecopy.ErrUnknownSourceMigrations)
}

// An error inside the transaction is returned as it is, so the transaction rolls back.
func TestRun_ACopyErrorFailsTheTransaction(t *testing.T) {
	ctx := context.Background()
	src := mocks.NewMockSource(t)
	dst := mocks.NewMockTarget(t)
	expectReadySource(src)
	dst.EXPECT().Tables(anyArg).Return([]string{"schema_migrations"}, nil).Once()
	dst.EXPECT().Applied(anyArg).Return(baseline, nil).Once()
	dst.EXPECT().Schema(anyArg).Return(schema, keys, nil).Once()
	runInTx(dst)
	dst.EXPECT().Count(inTx, anyArg).Return(0, nil).Times(3)
	boom := errors.New("duplicate key")
	dst.EXPECT().Copy(inTx, "characters", anyArg, anyArg).Return(0, boom).Once()
	_, err := sqlitecopy.Run(ctx, src, dst, []sqlitecopy.Migration{{Name: "001_baseline.sql"}}, baseline)
	require.ErrorIs(t, err, boom)
	assert.ErrorContains(t, err, "copy characters")
}
