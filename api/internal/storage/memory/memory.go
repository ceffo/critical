// Package memory is the in-memory storage of the e2e mode (ADR 0025 step 8):
// an implementation of every domain storage port over Go values, with no
// database, file or server. It keeps the contracts of the real adapters:
//
//   - a transaction is all or nothing: InTx snapshots the tables and puts
//     them back when fn fails or panics (step 8.2);
//   - writers are serialised: InTx holds the one write lock for the whole of
//     fn, so a second writer waits, then reads the first one's result (step
//     4.1); a read outside a transaction waits for the writer too, and never
//     sees a write that may still roll back;
//   - the action log is append-only: entries and corrections are only ever
//     appended (a rollback truncates them back), and which entries are popped
//     is decided by the domain rules (actionlog.StandingVoids, LatestPopped,
//     HPTotal), as in the database adapter.
//
// Its data lives as long as the process: every start begins empty and is
// seeded from the seed files and the catalogue, as a new database is. The
// mode is for e2e runs only, never staging or production (step 8.3; the
// boot refuses it there).
package memory

import (
	"context"
	"maps"
	"sync"
)

// Store is the in-memory database: the tables and the lock that serialises
// writers. The zero value is not usable: call New.
type Store struct {
	mu sync.RWMutex
	t  *tables
}

// New returns an empty store.
func New() *Store { return &Store{t: newTables()} }

// txKey marks a context running inside one of this store's transactions.
type txKey struct{ s *Store }

// InTx runs fn inside one transaction: fn holds the store's write lock, and
// every port call made with the context fn receives joins the transaction. A
// nested InTx joins the outer one. When fn returns an error or panics, every
// table is put back as it was before fn ran. Store satisfies domain.TxRunner
// through this method.
func (s *Store) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.inTx(ctx) {
		return fn(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.atomically(func() error {
		return fn(context.WithValue(ctx, txKey{s}, true))
	})
}

func (s *Store) inTx(ctx context.Context) bool {
	in, _ := ctx.Value(txKey{s}).(bool)
	return in
}

// atomically runs fn with the write lock held and restores the tables when
// fn fails or panics.
func (s *Store) atomically(fn func() error) (err error) {
	saved := s.t.save()
	defer func() {
		if p := recover(); p != nil {
			s.t = saved
			panic(p)
		}
		if err != nil {
			s.t = saved
		}
	}()
	return fn()
}

// read runs fn on the tables: inside a transaction of ctx as it is, outside
// under the read lock (a read waits for a running writer).
func (s *Store) read(ctx context.Context, fn func(t *tables) error) error {
	if s.inTx(ctx) {
		return fn(s.t)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fn(s.t)
}

// write runs fn on the tables: inside a transaction of ctx as part of it,
// outside as a transaction of its own (one statement is atomic).
func (s *Store) write(ctx context.Context, fn func(t *tables) error) error {
	if s.inTx(ctx) {
		return fn(s.t)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.atomically(func() error { return fn(s.t) })
}

// Migrate does nothing: the tables are Go values, there is no schema to bring
// up to date, and every row has its owner from the start.
func (s *Store) Migrate(context.Context, string) error { return nil }

// Close does nothing: the data goes with the process.
func (s *Store) Close() error { return nil }

// save returns a copy of t that a rollback puts back. Stored values are never
// changed in place (a write replaces a map entry or appends a row), so
// copying the maps is enough; the append-only tables (the action log) keep
// their length only, and appending after a rollback overwrites what the
// rolled-back transaction appended.
func (t *tables) save() *tables {
	c := *t
	c.characters = maps.Clone(t.characters)
	c.owners = maps.Clone(t.owners)
	c.retired = maps.Clone(t.retired)
	c.current = maps.Clone(t.current)
	c.slots = maps.Clone(t.slots)
	c.buffs = maps.Clone(t.buffs)
	c.rounds = maps.Clone(t.rounds)
	c.savedBuffs = maps.Clone(t.savedBuffs)
	c.spells = maps.Clone(t.spells)
	c.weapons = maps.Clone(t.weapons)
	c.items = maps.Clone(t.items)
	c.proficiencies = maps.Clone(t.proficiencies)
	c.bardSkills = maps.Clone(t.bardSkills)
	c.spellbooks = maps.Clone(t.spellbooks)
	c.learnFailures = maps.Clone(t.learnFailures)
	c.forms = maps.Clone(t.forms)
	c.xp = maps.Clone(t.xp)
	c.dice = maps.Clone(t.dice)
	c.users = maps.Clone(t.users)
	c.invites = maps.Clone(t.invites)
	c.sessions = maps.Clone(t.sessions)
	return &c
}
