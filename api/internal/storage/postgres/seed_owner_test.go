package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// owners is the owner of every weapon, item, proficiency and bard skill, by table and id.
func (f *fakeStore) owners() map[string]int64 {
	out := map[string]int64{}
	for _, w := range f.t.Weapons {
		out["weapon:"+w.ID] = w.CharacterID
	}
	for _, it := range f.t.Items {
		out["item:"+it.ID] = it.CharacterID
	}
	for _, p := range f.t.Profs {
		out["proficiency:"+p.ID] = p.CharacterID
	}
	for _, b := range f.t.Bard {
		out["bard_skill:"+b.ID] = b.CharacterID
	}
	return out
}

// everyRowOwnedBy is owners with every row owned by id.
func (f *fakeStore) everyRowOwnedBy(id int64) map[string]int64 {
	out := f.owners()
	for k := range out {
		out[k] = id
	}
	return out
}

// disown makes every row ownerless and empties the spellbooks: the database as it was before
// each character owned its equipment (before migration 004 the owner may be NULL).
func (f *fakeStore) disown() {
	f.t.Weapons = rekey(f.t.Weapons, func(w *weaponRow) (*int64, string) { return &w.CharacterID, w.ID })
	f.t.Items = rekey(f.t.Items, func(it *storeq.InsertItemParams) (*int64, string) { return &it.CharacterID, it.ID })
	f.t.Profs = rekey(f.t.Profs, func(p *storeq.UpsertProficiencyParams) (*int64, string) { return &p.CharacterID, p.ID })
	f.t.Bard = rekey(f.t.Bard, func(b *storeq.UpsertBardSkillParams) (*int64, string) { return &b.CharacterID, b.ID })
	clear(f.t.Spellbook)
}

// rekey is table with every row's owner NULL (0), under its new key.
func rekey[R any](table map[string]R, owner func(*R) (*int64, string)) map[string]R {
	out := make(map[string]R, len(table))
	for _, r := range table {
		o, id := owner(&r)
		*o = 0
		out[rowKey(0, id)] = r
	}
	return out
}

// only is the one row of table whose id is id (the zero row when there is none); a second row
// of that id fails: the caller expected one character's rows.
func only[R any](table map[string]R, id string, rowID func(R) string) R {
	var found []R
	for _, r := range table {
		if rowID(r) == id {
			found = append(found, r)
		}
	}
	var zero R
	switch len(found) {
	case 0:
		return zero
	case 1:
		return found[0]
	}
	panic(fmt.Sprintf("%d rows of id %q", len(found), id))
}

func (f *fakeStore) item(id string) storeq.InsertItemParams {
	return only(f.t.Items, id, func(r storeq.InsertItemParams) string { return r.ID })
}

// weapon is a weapon piece (ADR 0033 step 1).
func (f *fakeStore) weapon(id string) storeq.InsertItemParams { return f.item(id) }

func (f *fakeStore) prof(id string) storeq.UpsertProficiencyParams {
	return only(f.t.Profs, id, func(r storeq.UpsertProficiencyParams) string { return r.ID })
}

func (f *fakeStore) bardSkill(id string) storeq.UpsertBardSkillParams {
	return only(f.t.Bard, id, func(r storeq.UpsertBardSkillParams) string { return r.ID })
}

// ownOn runs ownRows as the migration run does: in its one transaction, with the seed character
// named by dir's character.json.
func ownOn(f *fakeStore, dir string) error {
	return f.InTx(context.Background(), func(ctx context.Context) error {
		return ownRows(ctx, f, seedfile.Path(dir, seedfile.CharacterFile))
	})
}

// A fresh database: the starting set (weapons, items, skills, spellbook in the seed's order)
// is the seed character's.
func TestSeed_TheStartingSetIsTheSeedCharacters(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	require.Len(t, f.t.Characters, 1)
	id := f.t.Characters[0]

	require.NotEmpty(t, f.t.Items)
	require.NotEmpty(t, f.t.Profs)
	require.NotEmpty(t, f.t.Bard)
	assert.Equal(t, f.everyRowOwnedBy(id), f.owners())
	assert.Equal(t, seedBook, f.spellbookOf(id))
}

// The user's existing data: rows stored before each character owned its equipment (no owner)
// all go to the character the seeds name before the owner becomes required, with every count,
// position and field as it was, no row added or removed; then the spellbook seed becomes its
// spellbook. Nothing to run by hand.
func TestOwnRows_ExistingRowsGoToTheSeedCharacterUnchanged(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	id := f.t.Characters[0]
	arrows := f.item("arrows_plus1")
	arrows.Count = 3 // played: a count the seed does not hold
	f.t.Items[rowKey(arrows.CharacterID, arrows.ID)] = arrows
	f.disown()
	sheet, profs, bard := f.sheet(), len(f.t.Profs), len(f.t.Bard)

	require.NoError(t, ownOn(f, repoSeeds()))
	assert.Equal(t, f.everyRowOwnedBy(id), f.owners(), "every row is the seed character's")
	assert.Equal(t, sheet, f.sheet(), "no count, position or other field moves, no row added")
	assert.Equal(t, int64(3), f.item("arrows_plus1").Count)
	assert.Len(t, f.t.Profs, profs)
	assert.Len(t, f.t.Bard, bard)

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, sheet, f.sheet(), "the seed adds nothing to the owned rows")
	assert.Equal(t, seedBook, f.spellbookOf(id))

	again := copyTables(f.t)
	require.NoError(t, ownOn(f, repoSeeds()))
	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, again, f.t, "a second start writes nothing")
}

// Rows a release without owners wrote during a rolling deploy (no owner) go to the seed
// character, beside its own.
func TestOwnRows_RowsWithoutOwnerWrittenLaterGoToTheSeedCharacter(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	id := f.t.Characters[0]
	rope := f.item("arrows_plus1")
	rope.ID, rope.CharacterID = "rope", 0
	f.t.Items[rowKey(0, "rope")] = rope

	require.NoError(t, ownOn(f, repoSeeds()))
	assert.Equal(t, id, f.item("rope").CharacterID)
	assert.Equal(t, f.everyRowOwnedBy(id), f.owners())
}

// A character other than the seed character gets nothing from the seeds, on any start.
func TestSeed_AnotherCharacterGetsNothing(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	id := f.t.Characters[0]
	other := id + 1
	f.t.Characters = append(f.t.Characters, other)
	f.t.Names[other] = "Someone else"

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, f.everyRowOwnedBy(id), f.owners(), "nothing is the other character's")
	assert.Empty(t, f.spellbookOf(other))
	n, err := f.CountItems(t.Context(), other)
	require.NoError(t, err)
	assert.Zero(t, n)
}

// Without the character the seeds name, the starting set has no owner: it is not applied, and
// rows without an owner refuse the migration run (they would belong to no one) and change nothing.
func TestOwnRows_NoSeedCharacter(t *testing.T) {
	t.Run("nothing unowned: nothing is seeded", func(t *testing.T) {
		f := newFakeStore()
		require.NoError(t, seedOn(f, repoSeeds()))
		f.t.Names[f.t.Characters[0]] = "Renamed"
		before := copyTables(f.t)
		require.NoError(t, ownOn(f, repoSeeds()))
		require.NoError(t, seedOn(f, repoSeeds()))
		assert.Equal(t, before, f.t)
	})
	t.Run("unowned rows refuse the run", func(t *testing.T) {
		f := newFakeStore()
		require.NoError(t, seedOn(f, repoSeeds()))
		f.t.Names[f.t.Characters[0]] = "Renamed"
		f.disown()
		before := copyTables(f.t)
		err := ownOn(f, repoSeeds())
		require.ErrorContains(t, err, "have no owner")
		assert.Equal(t, before, f.t, "the run is one transaction: nothing changes")
	})
	t.Run("a fresh database: nothing to own", func(t *testing.T) {
		f := newFakeStore()
		require.NoError(t, ownOn(f, repoSeeds()))
		assert.Equal(t, newFakeStore().t, f.t)
	})
}

// Rows are keyed by (character, id): another character may hold the seed character's weapon,
// item and skill ids, and a later start leaves both characters' rows as they are.
func TestSeed_AnotherCharacterMayHoldTheSameIDs(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	seedChar := f.t.Characters[0]
	other := seedChar + 1
	f.t.Characters = append(f.t.Characters, other)
	f.t.Names[other] = "Someone else"
	ctx := t.Context()
	for _, w := range copyTables(f.t).Weapons {
		w.CharacterID = other
		require.NoError(t, f.addWeapon(w))
	}
	for _, it := range copyTables(f.t).Items {
		it.CharacterID, it.Count = other, 0
		require.NoError(t, f.InsertItem(ctx, it))
	}
	for _, p := range copyTables(f.t).Profs {
		p.CharacterID = other
		require.NoError(t, f.UpsertProficiency(ctx, p))
	}
	for _, b := range copyTables(f.t).Bard {
		b.CharacterID, b.Percent = other, 5
		require.NoError(t, f.UpsertBardSkill(ctx, b))
	}
	before := copyTables(f.t)

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, before, f.t, "each character keeps its own rows")
	n, err := f.CountItems(ctx, other)
	require.NoError(t, err)
	assert.Equal(t, int64(13), n, "the twelve seeded pieces and the worn armour (ADR 0033 step 14)")
}

// The spellbook seed is applied once, while the seed character has no spellbook, as its weapons
// and items: a later start writes nothing, whatever the seed now lists (a spell the app removed
// is not added back, the order stays the stored one).
func TestSeed_SpellbookAppliedOnce(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedBookOn(f, repoSeeds(), []string{"alarm", "sleep"}))
	id := f.t.Characters[0]
	require.Equal(t, []string{"alarm", "sleep"}, f.spellbookOf(id))

	delete(f.t.Spellbook, spellbookKey(id, "sleep")) // removed in the app
	before := copyTables(f.t)
	require.NoError(t, seedBookOn(f, repoSeeds(), []string{"web", "sleep", "alarm"}))
	assert.Equal(t, before, f.t, "a second start writes nothing")
	assert.Equal(t, []string{"alarm"}, f.spellbookOf(id))
}
