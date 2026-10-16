// Package sqlitecopy moves a SQLite database of the app into an empty PostgreSQL database on the
// current baseline (ADR 0025 step 9). It is used twice, for the developer's `.dev/critical.db`
// and to seed production, then retired.
//
// The rules live here, on two ports: Source (the SQLite file, through a private working copy) and
// Target (the PostgreSQL database). The adapters behind them, and the SQLite driver, belong to the
// tool's own binary (`cmd/sqlite-copy`); the server never links them.
package sqlitecopy

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Errors Run refuses with. Each is wrapped with the details (tables, migrations, counts).
var (
	// ErrUnknownSourceMigrations: the SQLite file records a migration the tool does not embed,
	// so its schema is newer than the last one the tool knows.
	ErrUnknownSourceMigrations = errors.New("the SQLite file records migrations the tool does not embed")
	// ErrNotOnBaseline: the target holds a schema that is not the server's current baseline.
	ErrNotOnBaseline = errors.New("the target database is not on the current baseline")
	// ErrSchemaMismatch: a table or column of the SQLite file has no counterpart in the target, or
	// the reverse, so a straight copy would lose or invent data.
	ErrSchemaMismatch = errors.New("the SQLite schema and the target schema differ")
	// ErrTargetNotEmpty: the target already holds rows in an app table.
	ErrTargetNotEmpty = errors.New("the target database already holds rows")
	// ErrCountMismatch: after the copy, a table holds a different number of rows in the target.
	ErrCountMismatch = errors.New("row counts differ after the copy")
)

// migrationsTable records the applied migrations, on both sides. It is bookkeeping, not app data:
// it is never copied (the target records its own baseline).
const migrationsTable = "schema_migrations"

// Migration is one SQLite migration file the tool embeds.
type Migration struct{ Name, Body string }

// Table is an app table of the target, as the copy sees it.
type Table struct {
	Name string
	// Columns in the target's column order; every one is copied.
	Columns []string
	// Identity is the table's identity column, "" when it has none.
	Identity string
}

// ForeignKey says that rows of Table reference rows of References.
type ForeignKey struct{ Table, References string }

// TableCount is the number of rows of one table, equal on both sides after a copy.
type TableCount struct {
	Table string
	Rows  int64
}

// Source is the SQLite database, read through a working copy of the file (the original is only
// ever read), so bringing it to the last SQLite schema writes nothing the operator owns.
type Source interface {
	// Applied lists the migrations the file records; none for a file that has no bookkeeping.
	Applied(ctx context.Context) ([]string, error)
	// Apply runs a migration and records it, all or nothing.
	Apply(ctx context.Context, m Migration) error
	// Tables lists the app tables: neither SQLite's own nor schema_migrations.
	Tables(ctx context.Context) ([]string, error)
	// Columns lists the columns of a table.
	Columns(ctx context.Context, table string) ([]string, error)
	// Count is the number of rows of a table.
	Count(ctx context.Context, table string) (int64, error)
	// Sequence is the highest id SQLite ever handed out in an AUTOINCREMENT table (also the ids of
	// rows deleted since), 0 for a table without one.
	Sequence(ctx context.Context, table string) (int64, error)
	// Rows calls emit with the values of every row, in columns' order.
	Rows(ctx context.Context, table string, columns []string, emit func(row []any) error) error
}

// Target is the PostgreSQL database.
type Target interface {
	// Tables lists every table of the schema, schema_migrations included.
	Tables(ctx context.Context) ([]string, error)
	// Migrate applies the server's migrations, as the server does at boot.
	Migrate(ctx context.Context) error
	// Applied lists the migrations the database records.
	Applied(ctx context.Context) ([]string, error)
	// Schema describes the app tables (not schema_migrations) and their foreign keys.
	Schema(ctx context.Context) ([]Table, []ForeignKey, error)
	// InTx runs fn in one transaction that holds the server's writer lock from its first
	// statement (ADR 0025 step 4): the copy is all or nothing, and a server starting meanwhile
	// waits for it. The calls below run inside it, on the context fn receives.
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
	// Count is the number of rows of a table.
	Count(ctx context.Context, table string) (int64, error)
	// Copy writes the rows rows produces into a table and returns how many it wrote.
	Copy(ctx context.Context, table string, columns []string, rows func(emit func(row []any) error) error) (int64, error)
	// MaxID is the highest value of an identity column, 0 for an empty table.
	MaxID(ctx context.Context, table, column string) (int64, error)
	// SetNextID makes next the value the identity column hands out next.
	SetNextID(ctx context.Context, table, column string, next int64) error
}

// Run copies src into dst (ADR 0025 step 9.1): it brings the SQLite file to the last SQLite
// schema, migrates a brand-new target to the baseline, refuses a target that is not on it or holds
// rows (step 9.2), copies every table in dependency order in one transaction, sets every identity
// past the highest id copied, then compares the row counts of every table and fails on any
// difference, which rolls the whole copy back. baseline names the server's migrations.
func Run(ctx context.Context, src Source, dst Target, sqliteMigrations []Migration, baseline []string) ([]TableCount, error) {
	if err := UpgradeSource(ctx, src, sqliteMigrations); err != nil {
		return nil, err
	}
	if err := PrepareTarget(ctx, dst, baseline); err != nil {
		return nil, err
	}
	tables, fks, err := dst.Schema(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the target schema: %w", err)
	}
	order, err := DependencyOrder(tables, fks)
	if err != nil {
		return nil, err
	}
	if err := CheckSourceSchema(ctx, src, order); err != nil {
		return nil, err
	}
	var counts []TableCount
	err = dst.InTx(ctx, func(ctx context.Context) error {
		if err := RefuseNonEmpty(ctx, dst, order); err != nil {
			return err
		}
		for _, t := range order {
			produce := func(emit func(row []any) error) error { return src.Rows(ctx, t.Name, t.Columns, emit) }
			if _, err := dst.Copy(ctx, t.Name, t.Columns, produce); err != nil {
				return fmt.Errorf("copy %s: %w", t.Name, err)
			}
		}
		if err := ResetIdentities(ctx, src, dst, order); err != nil {
			return err
		}
		counts, err = CompareCounts(ctx, src, dst, order)
		return err
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

// UpgradeSource brings the SQLite file to the last SQLite schema: it applies, in name order, every
// embedded migration the file does not record, as the server did at boot before the engine
// switch. A file that records a migration the tool does not embed is refused before anything is
// applied: its schema is newer than the one the copy is written for.
func UpgradeSource(ctx context.Context, src Source, embedded []Migration) error {
	applied, err := src.Applied(ctx)
	if err != nil {
		return fmt.Errorf("list the SQLite migrations: %w", err)
	}
	known := make(map[string]bool, len(embedded))
	for _, m := range embedded {
		known[m.Name] = true
	}
	done := make(map[string]bool, len(applied))
	var unknown []string
	for _, name := range applied {
		done[name] = true
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("%w: %s", ErrUnknownSourceMigrations, strings.Join(unknown, ", "))
	}
	pending := slices.DeleteFunc(slices.Clone(embedded), func(m Migration) bool { return done[m.Name] })
	slices.SortFunc(pending, func(a, b Migration) int { return strings.Compare(a.Name, b.Name) })
	for _, m := range pending {
		if err := src.Apply(ctx, m); err != nil {
			return fmt.Errorf("apply SQLite migration %s: %w", m.Name, err)
		}
	}
	return nil
}

// PrepareTarget leaves the target on the server's baseline or refuses it. A database without any
// table is migrated, as the server would at its first boot (but without its seeds, so it stays
// empty). A database holding tables must record exactly the baseline's migrations: one without
// bookkeeping is some other database, one recording other migrations has another schema.
func PrepareTarget(ctx context.Context, dst Target, baseline []string) error {
	tables, err := dst.Tables(ctx)
	if err != nil {
		return fmt.Errorf("list the target tables: %w", err)
	}
	if len(tables) == 0 {
		if err := dst.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate the target: %w", err)
		}
	} else if !slices.Contains(tables, migrationsTable) {
		return fmt.Errorf("%w: it holds tables but no %s", ErrNotOnBaseline, migrationsTable)
	}
	applied, err := dst.Applied(ctx)
	if err != nil {
		return fmt.Errorf("list the target migrations: %w", err)
	}
	have, want := sorted(applied), sorted(baseline)
	if !slices.Equal(have, want) {
		return fmt.Errorf("%w: it records [%s], the baseline is [%s]",
			ErrNotOnBaseline, strings.Join(have, ", "), strings.Join(want, ", "))
	}
	return nil
}

// DependencyOrder orders the tables so that a table comes after every table it references, which
// the target's foreign keys require of the inserts. Ties go by name, so the order is the same on
// every run. A table referencing itself (an action's transaction head) is no constraint on the
// order: its rows are copied in one statement, checked at its end.
func DependencyOrder(tables []Table, fks []ForeignKey) ([]Table, error) {
	byName := make(map[string]Table, len(tables))
	for _, t := range tables {
		byName[t.Name] = t
	}
	parents := make(map[string]map[string]bool, len(tables))
	for _, fk := range fks {
		if _, ok := byName[fk.Table]; !ok {
			return nil, fmt.Errorf("%w: foreign key of unknown table %s", ErrSchemaMismatch, fk.Table)
		}
		if _, ok := byName[fk.References]; !ok {
			return nil, fmt.Errorf("%w: %s references unknown table %s", ErrSchemaMismatch, fk.Table, fk.References)
		}
		if fk.Table == fk.References {
			continue
		}
		if parents[fk.Table] == nil {
			parents[fk.Table] = map[string]bool{}
		}
		parents[fk.Table][fk.References] = true
	}
	placed := make(map[string]bool, len(tables))
	order := make([]Table, 0, len(tables))
	for len(order) < len(tables) {
		var ready []string
		for name := range byName {
			if placed[name] {
				continue
			}
			free := true
			for p := range parents[name] {
				if !placed[p] {
					free = false
					break
				}
			}
			if free {
				ready = append(ready, name)
			}
		}
		if len(ready) == 0 {
			return nil, fmt.Errorf("%w: the foreign keys form a cycle", ErrSchemaMismatch)
		}
		sort.Strings(ready)
		placed[ready[0]] = true
		order = append(order, byName[ready[0]])
	}
	return order, nil
}

// CheckSourceSchema refuses a copy that would lose or invent data: every app table of the SQLite
// file must exist in the target with the same columns, and the reverse.
func CheckSourceSchema(ctx context.Context, src Source, tables []Table) error {
	have, err := src.Tables(ctx)
	if err != nil {
		return fmt.Errorf("list the SQLite tables: %w", err)
	}
	inSource := make(map[string]bool, len(have))
	for _, name := range have {
		inSource[name] = true
	}
	inTarget := make(map[string]bool, len(tables))
	var problems []string
	for _, t := range tables {
		inTarget[t.Name] = true
		if !inSource[t.Name] {
			problems = append(problems, "table "+t.Name+" is missing from the SQLite file")
			continue
		}
		cols, err := src.Columns(ctx, t.Name)
		if err != nil {
			return fmt.Errorf("list the SQLite columns of %s: %w", t.Name, err)
		}
		if missing := difference(t.Columns, cols); len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("%s: the SQLite file lacks %s", t.Name, strings.Join(missing, ", ")))
		}
		if extra := difference(cols, t.Columns); len(extra) > 0 {
			problems = append(problems, fmt.Sprintf("%s: the target lacks %s", t.Name, strings.Join(extra, ", ")))
		}
	}
	for _, name := range sorted(have) {
		if !inTarget[name] {
			problems = append(problems, "table "+name+" is missing from the target")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrSchemaMismatch, strings.Join(problems, "; "))
	}
	return nil
}

// RefuseNonEmpty refuses a target holding a row in any app table (ADR 0025 step 9.2): the copy
// never merges into existing data.
func RefuseNonEmpty(ctx context.Context, dst Target, tables []Table) error {
	var full []string
	for _, t := range tables {
		n, err := dst.Count(ctx, t.Name)
		if err != nil {
			return fmt.Errorf("count the target rows of %s: %w", t.Name, err)
		}
		if n > 0 {
			full = append(full, fmt.Sprintf("%s (%d)", t.Name, n))
		}
	}
	if len(full) > 0 {
		return fmt.Errorf("%w: %s", ErrTargetNotEmpty, strings.Join(full, ", "))
	}
	return nil
}

// ResetIdentities sets every identity column past the ids the copy wrote and past the ids SQLite
// had already handed out, so the next insert never reuses one (ADR 0025 step 9.1).
func ResetIdentities(ctx context.Context, src Source, dst Target, tables []Table) error {
	for _, t := range tables {
		if t.Identity == "" {
			continue
		}
		maxID, err := dst.MaxID(ctx, t.Name, t.Identity)
		if err != nil {
			return fmt.Errorf("read the highest id of %s: %w", t.Name, err)
		}
		seq, err := src.Sequence(ctx, t.Name)
		if err != nil {
			return fmt.Errorf("read the SQLite sequence of %s: %w", t.Name, err)
		}
		if err := dst.SetNextID(ctx, t.Name, t.Identity, NextID(maxID, seq)); err != nil {
			return fmt.Errorf("set the next id of %s: %w", t.Name, err)
		}
	}
	return nil
}

// NextID is the first id a table hands out after the copy: one past both the highest id copied
// and the highest id SQLite ever handed out. The second matters for an AUTOINCREMENT table whose
// last rows were deleted: an undo restores a deleted row (a saved buff, an XP entry) under its old
// id, which a new row must not have taken meanwhile.
func NextID(maxID, sqliteSequence int64) int64 {
	return max(maxID, sqliteSequence, 0) + 1
}

// CompareCounts counts the rows of every table on both sides and fails on any difference.
func CompareCounts(ctx context.Context, src Source, dst Target, tables []Table) ([]TableCount, error) {
	counts := make([]TableCount, 0, len(tables))
	var diffs []string
	for _, t := range tables {
		want, err := src.Count(ctx, t.Name)
		if err != nil {
			return nil, fmt.Errorf("count the SQLite rows of %s: %w", t.Name, err)
		}
		got, err := dst.Count(ctx, t.Name)
		if err != nil {
			return nil, fmt.Errorf("count the target rows of %s: %w", t.Name, err)
		}
		if got != want {
			diffs = append(diffs, fmt.Sprintf("%s: SQLite %d, target %d", t.Name, want, got))
		}
		counts = append(counts, TableCount{Table: t.Name, Rows: want})
	}
	if len(diffs) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrCountMismatch, strings.Join(diffs, "; "))
	}
	return counts, nil
}

// difference lists the names of a that b lacks, sorted.
func difference(a, b []string) []string {
	var out []string
	for _, name := range a {
		if !slices.Contains(b, name) {
			out = append(out, name)
		}
	}
	return sorted(out)
}

func sorted(names []string) []string {
	out := slices.Clone(names)
	sort.Strings(out)
	return out
}
