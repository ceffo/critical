package memory_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

const (
	seedsDir      = "../../../testdata/seeds"
	catalogueFile = "../../../testdata/catalogue.json"
)

var errBoom = errors.New("boom")

// seeded is a store seeded from the test fixture, with its character's id.
func seeded(t *testing.T) (*memory.Store, int64) {
	t.Helper()
	ctx := context.Background()
	s := memory.New()
	require.NoError(t, s.Migrate(ctx, ""))
	artifact, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	book, err := spellbookadpt.ReadSeed(seedsDir, artifact.Catalogue)
	require.NoError(t, err)
	require.NoError(t, s.Seed(ctx, seedsDir, book))
	require.NoError(t, s.ImportCatalogue(ctx, artifact.Catalogue, nil))
	return s, seedCharacter(t, s)
}

// seedCharacter is the seeded character's id, read as its owner reads it: a
// user adopts the characters without an owner and finds it current.
func seedCharacter(t *testing.T, s *memory.Store) int64 {
	t.Helper()
	ctx := context.Background()
	u := authsvc.User{ID: uuid.New(), Email: "seed-owner@example.com", Role: authsvc.RoleAdmin}
	require.NoError(t, s.Auth().InsertUser(ctx, u))
	require.NoError(t, s.Auth().AdoptUnownedCharacters(ctx, u.ID))
	got, err := s.Auth().UserByID(ctx, u.ID)
	require.NoError(t, err)
	require.NotZero(t, got.CurrentCharacterID)
	return got.CurrentCharacterID
}

func entry(characterID int64, kind actionlog.Kind) actionlog.NewEntry {
	return actionlog.NewEntry{CharacterID: characterID, Kind: kind, Summary: string(kind)}
}

// state is what the rollback tests compare: one read of every port a write
// below touches.
type state struct {
	Log       []actionlog.Entry
	Buffs     []buffsvc.BuffEffect
	Round     int
	Slots     []spellsvc.SpellSlot
	Items     []itemsvc.Item
	Weapons   []weaponsvc.Weapon
	XP        []xpsvc.Entry
	Favourite int
}

func read(t *testing.T, s *memory.Store, characterID int64) state {
	t.Helper()
	ctx := context.Background()
	var st state
	var err error
	st.Log, err = s.ActionLog().ListEntries(ctx, characterID, 100)
	require.NoError(t, err)
	st.Buffs, err = s.Buffs().GetActiveBuffs(ctx, characterID)
	require.NoError(t, err)
	st.Round, err = s.Buffs().GetRoundNumber(ctx, characterID)
	require.NoError(t, err)
	st.Slots, err = s.Spells().GetSlots(ctx, characterID)
	require.NoError(t, err)
	st.Items, err = s.Items().List(ctx, characterID)
	require.NoError(t, err)
	st.Weapons, err = s.WeaponEditor().List(ctx, characterID)
	require.NoError(t, err)
	st.XP, err = s.XP().List(ctx, characterID)
	require.NoError(t, err)
	favs, err := s.Dice().List(ctx, characterID)
	require.NoError(t, err)
	st.Favourite = len(favs)
	return st
}

// writeEverything writes through every port that writes, in the transaction
// ctx carries.
func writeEverything(t *testing.T, ctx context.Context, s *memory.Store, characterID int64) {
	t.Helper()
	_, err := s.ActionLog().InsertEntry(ctx, entry(characterID, actionlog.KindHPChange), 0)
	require.NoError(t, err)
	require.NoError(t, s.Buffs().SaveActiveBuffs(ctx, characterID, []buffsvc.BuffEffect{{ID: "b", Name: "Haste"}}))
	require.NoError(t, s.Buffs().SaveRoundNumber(ctx, characterID, 7))
	require.NoError(t, s.Spells().SaveSlots(ctx, characterID, []spellsvc.SpellSlot{{Level: 1, Used: true}}))
	_, err = s.Items().Decrement(ctx, characterID, "arrows_plus1")
	require.NoError(t, err)
	require.NoError(t, s.WeaponEditor().Delete(ctx, characterID, "longsword"))
	_, err = s.XP().Insert(ctx, characterID, xpsvc.Entry{Date: "2026-01-01", Reason: "orc", Base: 10})
	require.NoError(t, err)
	_, err = s.Dice().Insert(ctx, characterID, "1d6")
	require.NoError(t, err)
}

// ADR 0025 step 8.2: a transaction is all or nothing. A failing fn leaves
// nothing of what it wrote, in any table, and the ids it took are free again
// (as a rolled-back AUTOINCREMENT); a successful one keeps everything.
func TestStore_Step8_2_TransactionAllOrNothing(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	before := read(t, s, id)

	err := s.InTx(ctx, func(ctx context.Context) error {
		writeEverything(t, ctx, s, id)
		return errBoom
	})
	require.ErrorIs(t, err, errBoom)
	assert.Equal(t, before, read(t, s, id), "a failed transaction leaves nothing")

	require.NoError(t, s.InTx(ctx, func(ctx context.Context) error {
		writeEverything(t, ctx, s, id)
		return nil
	}))
	after := read(t, s, id)
	assert.NotEqual(t, before, after)
	require.Len(t, after.Log, 1)
	assert.Equal(t, int64(1), after.Log[0].ID, "the rolled-back entry's id is taken again")
	assert.Equal(t, 7, after.Round)
}

// A panic inside a transaction rolls it back too, and is passed on.
func TestStore_Step8_2_PanicRollsBack(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	before := read(t, s, id)
	assert.PanicsWithValue(t, "boom", func() {
		_ = s.InTx(ctx, func(ctx context.Context) error {
			writeEverything(t, ctx, s, id)
			panic("boom")
		})
	})
	assert.Equal(t, before, read(t, s, id))
}

// A nested InTx joins the outer transaction: its writes roll back with the
// outer one, as a nested postgres InTx joins the outer transaction.
func TestStore_Step8_2_NestedTransactionJoinsTheOuter(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	before := read(t, s, id)
	err := s.InTx(ctx, func(ctx context.Context) error {
		require.NoError(t, s.InTx(ctx, func(ctx context.Context) error {
			writeEverything(t, ctx, s, id)
			return nil
		}))
		return errBoom
	})
	require.ErrorIs(t, err, errBoom)
	assert.Equal(t, before, read(t, s, id))
}

// A write outside a transaction is a transaction of its own: a refused
// write (an empty item) changes nothing.
func TestStore_Step8_2_WriteOutsideATransactionIsAtomic(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	require.NoError(t, s.Items().SetCount(ctx, id, "arrows_plus1", 0))
	_, err := s.Items().Decrement(ctx, id, "arrows_plus1")
	require.ErrorIs(t, err, itemsvc.ErrItemEmpty)
	it, err := s.Items().Get(ctx, id, "arrows_plus1")
	require.NoError(t, err)
	assert.Zero(t, it.Count, "the count never goes below 0")
}

// ADR 0025 steps 4.1 and 8.2: writers are serialised. A second transaction
// waits for the first to commit, then reads its result; a read outside a
// transaction waits too, and never sees a write that may still roll back.
func TestStore_Step8_2_WritersSerialised(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	inFirst, release := make(chan struct{}), make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- s.InTx(ctx, func(ctx context.Context) error {
			if err := s.Buffs().SaveRoundNumber(ctx, id, 3); err != nil {
				return err
			}
			close(inFirst)
			<-release
			return nil
		})
	}()
	<-inFirst

	var secondIn, readIn atomic.Bool
	secondSaw, readSaw := make(chan int, 1), make(chan int, 1)
	go func() {
		_ = s.InTx(ctx, func(ctx context.Context) error {
			secondIn.Store(true)
			round, err := s.Buffs().GetRoundNumber(ctx, id)
			secondSaw <- round
			return err
		})
	}()
	go func() {
		readIn.Store(true)
		round, _ := s.Buffs().GetRoundNumber(ctx, id)
		readSaw <- round
	}()
	time.Sleep(50 * time.Millisecond)
	assert.False(t, secondIn.Load(), "the second writer entered while the first held the lock")
	select {
	case <-readSaw:
		t.Fatal("a read outside a transaction saw an uncommitted write")
	default:
	}

	close(release)
	require.NoError(t, <-firstDone)
	assert.Equal(t, 3, <-secondSaw, "the second writer reads the first one's result")
	assert.Equal(t, 3, <-readSaw)
}

// logService is the action log service over the store, as the composition
// root wires it.
func logService(t *testing.T, s *memory.Store) actionlogsvc.SVC {
	t.Helper()
	svc, err := actionlogsvc.New(s.ActionLog(), actionlogsvc.Transactions(s), rounds{s}, s.Characters(), nil)
	require.NoError(t, err)
	return svc
}

type rounds struct{ s *memory.Store }

func (r rounds) GetRound(ctx context.Context, id int64) (int, error) {
	return r.s.Buffs().GetRoundNumber(ctx, id)
}

// record records e in a transaction of its own, as a request does.
func record(t *testing.T, s *memory.Store, svc actionlogsvc.SVC, e actionlog.NewEntry) int64 {
	t.Helper()
	var id int64
	require.NoError(t, actionlogsvc.Transactions(s).InTx(context.Background(), func(ctx context.Context) error {
		var err error
		id, err = svc.Record(ctx, e)
		return err
	}))
	return id
}

// ADR 0025 step 8.2 (ADR 0018, 0019): undo pops the top transaction whole and
// redo replays it, chosen by the domain rules over the store's corrections;
// HP follows the ledger (ADR 0007 step 2).
func TestStore_Step8_2_UndoRedo(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	svc := logService(t, s)

	dmg := entry(id, actionlog.KindHPChange)
	dmg.HPDelta = -10
	first := record(t, s, svc, dmg)
	var second, third int64
	require.NoError(t, actionlogsvc.Transactions(s).InTx(ctx, func(ctx context.Context) error {
		var err error
		if second, err = svc.Record(ctx, dmg); err != nil {
			return err
		}
		third, err = svc.Record(ctx, entry(id, actionlog.KindRoundAdvance))
		return err
	}))
	total, err := s.ActionLog().HPTotal(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, -20, total)

	undone, err := svc.Undo(ctx, id, third)
	require.NoError(t, err)
	assert.Equal(t, second, undone.ID, "the undo pops the whole transaction, from its first entry")
	top, ok, err := svc.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, first, top)
	total, err = s.ActionLog().HPTotal(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, -10, total, "a popped entry adds nothing to HP")
	popped, txID, ok, err := s.ActionLog().LatestPopped(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, second, txID)
	assert.Contains(t, []int64{second, third}, popped)

	redone, err := svc.Redo(ctx, id, second)
	require.NoError(t, err)
	assert.Equal(t, second, redone.ID)
	top, _, err = svc.TopID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, third, top)
	total, err = s.ActionLog().HPTotal(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, -20, total)
	_, _, ok, err = s.ActionLog().LatestPopped(ctx, id)
	require.NoError(t, err)
	assert.False(t, ok, "nothing is popped after the redo")
}

// ADR 0025 step 8.2: a stale stack top is refused (ErrTopChanged, mapped to
// 409), never an error of the store: of two undos expecting the same top,
// the second waits for the first, then sees the new top.
func TestStore_Step8_2_StaleTopRefused(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	svc := logService(t, s)
	record(t, s, svc, entry(id, actionlog.KindHPChange))
	top := record(t, s, svc, entry(id, actionlog.KindHPChange))

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := svc.Undo(ctx, id, top)
			errs <- err
		}()
	}
	first, second := <-errs, <-errs
	if first != nil {
		first, second = second, first
	}
	require.NoError(t, first)
	require.ErrorIs(t, second, actionlogsvc.ErrTopChanged)

	_, err := svc.Redo(ctx, id, top+100)
	require.ErrorIs(t, err, actionlogsvc.ErrRedoChanged)
}

// ADR 0025 step 4.1 (ADR 0019): of two redos expecting the same popped transaction, the second
// waits for the first, then finds nothing left to redo: a stack conflict (409), never an error of
// the store, and the transaction is replayed once.
func TestStore_Step8_2_ConcurrentRedosOneWins(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	svc := logService(t, s)
	top := record(t, s, svc, entry(id, actionlog.KindHPChange))
	_, err := svc.Undo(ctx, id, top)
	require.NoError(t, err)

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := svc.Redo(ctx, id, top)
			errs <- err
		}()
	}
	first, second := <-errs, <-errs
	if first != nil {
		first, second = second, first
	}
	require.NoError(t, first)
	require.Error(t, second)
	assert.True(t, errors.Is(second, actionlogsvc.ErrNothingToRedo) || errors.Is(second, actionlogsvc.ErrRedoChanged),
		"a stack conflict, not a store error: %v", second)
	now, ok, err := svc.TopID(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, top, now, "replayed once")
}

// A restore that fails during an undo leaves no void and no partial state
// (ADR 0018: the restores and the voids share one transaction).
func TestStore_Step8_2_FailedUndoLeavesNothing(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	svc := logService(t, s)
	svc.RegisterRestorer("buffs", failingRestorer{s})
	e := entry(id, actionlog.KindBuffChange)
	e.Snapshots = []actionlog.Snapshot{{Key: "buffs", Before: json.RawMessage(`[]`), After: json.RawMessage(`[]`)}}
	top := record(t, s, svc, e)
	before := read(t, s, id)

	_, err := svc.Undo(ctx, id, top)
	require.ErrorIs(t, err, errBoom)
	assert.Equal(t, before, read(t, s, id))
	got, err := s.ActionLog().GetEntry(ctx, id, top)
	require.NoError(t, err)
	assert.Empty(t, got.Corrections, "no void is left by the failed undo")
}

// failingRestorer writes, then fails.
type failingRestorer struct{ s *memory.Store }

func (f failingRestorer) Restore(ctx context.Context, characterID int64, _ json.RawMessage) error {
	if err := f.s.Buffs().SaveActiveBuffs(ctx, characterID, []buffsvc.BuffEffect{{ID: "x"}}); err != nil {
		return err
	}
	return errBoom
}

// ADR 0025 step 8.2 (ADR 0007, 0010): the action log is append-only. A
// correction is a new row: the entry it corrects reads as it was logged,
// ids grow by one, and a value read or written by a caller is a copy, so
// changing it changes nothing stored.
func TestStore_Step8_2_ActionLogAppendOnly(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	log := s.ActionLog()
	e := entry(id, actionlog.KindAttack)
	e.Payload = json.RawMessage(`{"d20Roll":12}`)
	e.HPDelta = -4
	e.Snapshots = []actionlog.Snapshot{{Key: "k", Before: json.RawMessage(`1`), After: json.RawMessage(`2`)}}
	first, err := log.InsertEntry(ctx, e, 0)
	require.NoError(t, err)
	second, err := log.InsertEntry(ctx, entry(id, actionlog.KindHPChange), first)
	require.NoError(t, err)
	assert.Equal(t, first+1, second)
	e.Payload[2] = 'X' // the caller's buffer, changed after the insert
	logged, err := log.GetEntry(ctx, id, first)
	require.NoError(t, err)
	assert.Equal(t, first, logged.TransactionID, "0 starts a transaction named after its first entry")

	delta := -9
	require.NoError(t, log.InsertCorrection(ctx, actionlog.Correction{ActionID: first, Kind: actionlog.CorrectionAmend, HPDelta: &delta}))
	require.NoError(t, log.InsertCorrection(ctx, actionlog.Correction{ActionID: first, Kind: actionlog.CorrectionVoid}))
	require.Error(t, log.InsertCorrection(ctx, actionlog.Correction{ActionID: 99, Kind: actionlog.CorrectionVoid}),
		"a correction of no entry is refused")

	got, err := log.GetEntry(ctx, id, first)
	require.NoError(t, err)
	assert.JSONEq(t, `{"d20Roll":12}`, string(got.Payload))
	assert.Equal(t, -4, got.HPDelta, "the entry keeps its own delta: the amend is a correction")
	assert.Equal(t, second, got.TransactionID+1)
	require.Len(t, got.Corrections, 2)
	assert.Equal(t, []int64{1, 2}, []int64{got.Corrections[0].ID, got.Corrections[1].ID})
	assert.Equal(t, logged.Snapshots, got.Snapshots)

	got.Payload[2] = 'Y'
	got.Corrections[0].Kind = actionlog.CorrectionVoid
	again, err := log.GetEntry(ctx, id, first)
	require.NoError(t, err)
	assert.JSONEq(t, `{"d20Roll":12}`, string(again.Payload))
	assert.Equal(t, actionlog.CorrectionAmend, again.Corrections[0].Kind)

	trans, err := log.ListTransaction(ctx, id, first)
	require.NoError(t, err)
	assert.Len(t, trans, 2, "a voided entry stays in its transaction")
	active, err := log.ListActiveEntries(ctx, id, actionlog.KindAttack, 10)
	require.NoError(t, err)
	assert.Empty(t, active, "the voided attack is not active")
	latest, ok, err := log.LatestID(ctx, id, actionlog.KindAttack)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, first, latest, "LatestID counts voided entries")

	_, err = log.GetEntry(ctx, id+1, first)
	require.ErrorIs(t, err, domain.ErrNotFound, "an entry of another character is not found")
}

// ADR 0025 step 8.1: the store is seeded from the seeds and the catalogue as
// a new database is; the same catalogue again writes nothing, another one is
// refused (the store imports once).
func TestStore_Step8_1_SeededLikeTheDatabase(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	ch, err := s.Characters().GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Test Bard", ch.Name)
	assert.Equal(t, "bard", ch.Class)
	assert.Equal(t, []string{}, ch.Details.Languages)
	maxHP, err := s.Characters().GetMaxHP(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 85, maxHP)

	weapons, err := s.Weapons().List(ctx, id)
	require.NoError(t, err)
	assert.NotEmpty(t, weapons)
	for i, w := range weapons {
		assert.NotEmpty(t, w.Name, "weapon %d has its name", i)
	}
	items, err := s.Items().List(ctx, id)
	require.NoError(t, err)
	for i, it := range items {
		assert.Equal(t, i, it.Position, "items keep the seed's order")
	}
	profs, err := s.Skills().ListProficiencies(ctx, id)
	require.NoError(t, err)
	assert.NotEmpty(t, profs)

	artifact, err := catalogueadpt.LoadArtifact(catalogueFile)
	require.NoError(t, err)
	all, err := s.Spells().Search(ctx, spellsvc.Filter{})
	require.NoError(t, err)
	assert.Len(t, all, len(artifact.Catalogue.Spells))
	wizard, err := s.Spells().Search(ctx, spellsvc.Filter{Class: "WIZARD", Levels: []int{1}})
	require.NoError(t, err)
	require.NotEmpty(t, wizard)
	for _, sp := range wizard {
		assert.Equal(t, "Wizard", sp.Class, "the class matches in any case")
		assert.Equal(t, 1, sp.Level)
		assert.Nil(t, sp.Description, "a search reads no description")
	}
	full, ok, err := s.Spells().Describe(ctx, wizard[0].ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.NotEmpty(t, full.Description)

	require.NoError(t, s.ImportCatalogue(ctx, artifact.Catalogue, nil), "the same version imports nothing")
	other := artifact.Catalogue
	other.Version = "another"
	require.Error(t, s.ImportCatalogue(ctx, other, nil))
}

// The refusals of the database's keys: an id a table holds is not inserted
// again; ids restored under their own value move the sequence past them.
func TestStore_Step8_2_KeysRefusedLikeTheDatabase(t *testing.T) {
	s, id := seeded(t)
	ctx := context.Background()
	w, err := s.WeaponEditor().Get(ctx, id, "longsword")
	require.NoError(t, err)
	require.Error(t, s.WeaponEditor().Insert(ctx, id, w))
	it, err := s.Items().Get(ctx, id, "arrows_plus1")
	require.NoError(t, err)
	require.Error(t, s.Items().Insert(ctx, id, it))

	require.NoError(t, s.XP().Reinsert(ctx, id, xpsvc.Entry{ID: 40, Date: "2026-01-01", Base: 5}))
	require.Error(t, s.XP().Reinsert(ctx, id, xpsvc.Entry{ID: 40}))
	next, err := s.XP().Insert(ctx, id, xpsvc.Entry{Date: "2026-01-02", Base: 1})
	require.NoError(t, err)
	assert.Equal(t, int64(41), next)
	require.ErrorIs(t, s.XP().Delete(ctx, id+1, 40), xpsvc.ErrEntryNotFound)
}

// Weapons, items, skills and spellbooks belong to a character: the seed gives
// the starting set to the seed character only, a character added later owns
// nothing, and the same id names a different row for each character.
func TestStore_EachCharacterOwnsItsRows(t *testing.T) {
	s, kelsen := seeded(t)
	ctx := context.Background()
	other, err := s.AddCharacter(ctx, charactersvc.Character{Name: "Other", Class: "bard"})
	require.NoError(t, err)
	require.NotEqual(t, kelsen, other)

	book, err := s.Spellbooks().Keys(ctx, kelsen)
	require.NoError(t, err)
	assert.NotEmpty(t, book, "the seed character gets the spellbook seed")
	for _, empty := range []func() (int, error){
		func() (int, error) { l, err := s.Items().List(ctx, other); return len(l), err },
		func() (int, error) { l, err := s.WeaponEditor().List(ctx, other); return len(l), err },
		func() (int, error) { l, err := s.Weapons().List(ctx, other); return len(l), err },
		func() (int, error) { l, err := s.Skills().ListProficiencies(ctx, other); return len(l), err },
		func() (int, error) { l, err := s.Spellbooks().Keys(ctx, other); return len(l), err },
	} {
		n, err := empty()
		require.NoError(t, err)
		assert.Zero(t, n, "a character added later starts with nothing")
	}

	arrows, err := s.Items().Get(ctx, kelsen, "arrows_plus1")
	require.NoError(t, err)
	_, err = s.Items().Get(ctx, other, "arrows_plus1")
	require.ErrorIs(t, err, itemsvc.ErrItemNotFound, "another character's item is unknown")
	arrows.Count = 1
	require.NoError(t, s.Items().Insert(ctx, other, arrows), "the same id for another character")
	_, err = s.Items().Decrement(ctx, other, "arrows_plus1")
	require.NoError(t, err)
	mine, err := s.Items().Get(ctx, kelsen, "arrows_plus1")
	require.NoError(t, err)
	theirs, err := s.Items().Get(ctx, other, "arrows_plus1")
	require.NoError(t, err)
	assert.Zero(t, theirs.Count)
	assert.NotZero(t, mine.Count, "spending the other's arrow leaves the seed character's")

	sword, err := s.WeaponEditor().Get(ctx, kelsen, "longsword")
	require.NoError(t, err)
	require.NoError(t, s.WeaponEditor().Insert(ctx, other, sword))
	require.NoError(t, s.WeaponEditor().Delete(ctx, other, "longsword"))
	_, err = s.Weapons().Get(ctx, kelsen, "longsword")
	require.NoError(t, err, "deleting the other's sword leaves the seed character's")
}
