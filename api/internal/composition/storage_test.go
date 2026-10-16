package composition

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/storage/memory"
	"github.com/ceffo/critical/api/internal/storage/postgres"
)

// errReached is what conn panics with: the statement reached the connection
// the root routes.
type errReached struct{}

// conn is a connection on which every statement panics with errReached, so
// a test learns which connection an adapter's query ran on without running
// SQL.
type conn struct{}

func (conn) ExecContext(context.Context, string, ...any) (sql.Result, error) { panic(errReached{}) }
func (conn) PrepareContext(context.Context, string) (*sql.Stmt, error)       { panic(errReached{}) }
func (conn) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	panic(errReached{})
}
func (conn) QueryRowContext(context.Context, string, ...any) *sql.Row { panic(errReached{}) }

// reaches reports whether call ran a statement on conn.
func reaches(call func()) (reached bool) {
	defer func() {
		_, reached = recover().(errReached)
	}()
	call()
	return false
}

// ADR 0025 step 5.2: every adapter postgresStorage builds runs its queries on
// the transaction the caller's context carries (postgres.Routed over the pool).
// The context carries conn as its transaction and the pool is an unopened
// DB, so an adapter built over the pool itself (or any connection but the
// routed one) escapes the transaction: its call does not reach conn. Every
// storage port but the store is listed, so a new one fails until it is
// checked here.
func TestPostgresStorage_EveryAdapterRunsOnTheCallersTransaction(t *testing.T) {
	ctx := postgres.WithTx(context.Background(), conn{})
	s := postgresStorage(&postgres.DB{})
	calls := map[string]func(){
		"Character":    func() { _, _ = s.Character.FirstID(ctx) },
		"HP":           func() { _, _ = s.HP.GetMaxHP(ctx, 1) },
		"Log":          func() { _, _ = s.Log.HPTotal(ctx, 1) },
		"Buff":         func() { _, _ = s.Buff.GetActiveBuffs(ctx, 1) },
		"Spell":        func() { _, _ = s.Spell.GetSlots(ctx, 1) },
		"Weapons":      func() { _, _ = s.Weapons.List(ctx) },
		"WeaponEditor": func() { _, _ = s.WeaponEditor.List(ctx) },
		"Crit":         func() { _, _ = s.Crit.List(ctx) },
		"Items":        func() { _, _ = s.Items.List(ctx) },
		"Forms":        func() { _, _, _ = s.Forms.Get(ctx, 1) },
		"XP":           func() { _, _ = s.XP.List(ctx, 1) },
		"Skill":        func() { _, _ = s.Skill.ListProficiencies(ctx) },
		"Dice":         func() { _, _ = s.Dice.List(ctx, 1) },
	}

	fields := reflect.TypeFor[Storage]()
	for i := range fields.NumField() {
		name := fields.Field(i).Name
		if name == "Store" {
			continue
		}
		call, ok := calls[name]
		require.True(t, ok, "storage port %s has no wiring check", name)
		assert.True(t, reaches(call), "%s does not run on the caller's transaction", name)
	}
}

// ADR 0025 step 8.1: memoryStorage wires an in-memory implementation of
// every storage port, all over the store that runs the transactions: a port
// written inside a failed transaction of the store is rolled back with it.
func TestMemoryStorage_EveryPortOnTheOneStore(t *testing.T) {
	m := memory.New()
	s := memoryStorage(m)
	v := reflect.ValueOf(s)
	for i := range v.NumField() {
		assert.False(t, v.Field(i).IsNil(), "storage port %s is not wired", v.Type().Field(i).Name)
	}
	assert.Same(t, m, s.Store)

	ctx := context.Background()
	_ = s.Store.InTx(ctx, func(ctx context.Context) error {
		require.NoError(t, s.Buff.SaveRoundNumber(ctx, 1, 4))
		return assert.AnError
	})
	round, err := s.Buff.GetRoundNumber(ctx, 1)
	require.NoError(t, err)
	assert.Zero(t, round)
}
