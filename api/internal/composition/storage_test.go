package composition

import (
	"context"
	"database/sql"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// ADR 0025 step 5.2: every adapter of the root runs its queries on the one
// connection the root routes to the caller's transaction (sqlite.Routed in
// sqliteStorage). An adapter built over another connection (the pool itself)
// would escape the transaction: its call would not reach conn. Every storage
// port but the store is listed, so a new one fails until it is checked here.
func TestStorageOn_EveryAdapterRunsOnTheRoutedConnection(t *testing.T) {
	ctx := context.Background()
	s := storageOn(nil, conn{})
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
		assert.True(t, reaches(call), "%s does not run on the routed connection", name)
	}
}
