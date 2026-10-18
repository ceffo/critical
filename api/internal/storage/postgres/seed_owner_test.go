package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// owners is the owner of every weapon, item, proficiency and bard skill, by table and id.
func (f *fakeStore) owners() map[string]int64 {
	out := map[string]int64{}
	for id, w := range f.t.Weapons {
		out["weapon:"+id] = w.CharacterID
	}
	for id, it := range f.t.Items {
		out["item:"+id] = it.CharacterID
	}
	for id, p := range f.t.Profs {
		out["proficiency:"+id] = p.CharacterID
	}
	for id, b := range f.t.Bard {
		out["bard_skill:"+id] = b.CharacterID
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
// each character owned its equipment.
func (f *fakeStore) disown() {
	for id, w := range f.t.Weapons {
		w.CharacterID = 0
		f.t.Weapons[id] = w
	}
	for id, it := range f.t.Items {
		it.CharacterID = 0
		f.t.Items[id] = it
	}
	for id, p := range f.t.Profs {
		p.CharacterID = 0
		f.t.Profs[id] = p
	}
	for id, b := range f.t.Bard {
		b.CharacterID = 0
		f.t.Bard[id] = b
	}
	clear(f.t.Spellbook)
}

// A fresh database: the starting set (weapons, items, skills, spellbook in the seed's order)
// is the seed character's.
func TestSeed_TheStartingSetIsTheSeedCharacters(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	require.Len(t, f.t.Characters, 1)
	id := f.t.Characters[0]

	require.NotEmpty(t, f.t.Weapons)
	require.NotEmpty(t, f.t.Items)
	require.NotEmpty(t, f.t.Profs)
	require.NotEmpty(t, f.t.Bard)
	assert.Equal(t, f.everyRowOwnedBy(id), f.owners())
	assert.Equal(t, seedBook, f.spellbookOf(id))
}

// The user's existing data: rows stored before each character owned its equipment (no owner)
// all go to the character the seeds name, with every count, position and field as it was, no
// row added or removed, and the spellbook seed becomes its spellbook. Nothing to run by hand.
func TestSeed_ExistingRowsGoToTheSeedCharacterUnchanged(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	id := f.t.Characters[0]
	arrows := f.t.Items["arrows_plus1"]
	arrows.Count = 3 // played: a count the seed does not hold
	f.t.Items["arrows_plus1"] = arrows
	f.disown()
	sheet, profs, bard := f.sheet(), len(f.t.Profs), len(f.t.Bard)

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, f.everyRowOwnedBy(id), f.owners(), "every row is the seed character's")
	assert.Equal(t, sheet, f.sheet(), "no count, position or other field moves, no row added")
	assert.Equal(t, int64(3), f.t.Items["arrows_plus1"].Count)
	assert.Len(t, f.t.Profs, profs)
	assert.Len(t, f.t.Bard, bard)
	assert.Equal(t, seedBook, f.spellbookOf(id))

	again := copyTables(f.t)
	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, again, f.t, "a second start writes nothing")
}

// Rows the release before wrote during a rolling deploy (no owner) go to the seed character on
// the next start, beside its own.
func TestSeed_RowsWithoutOwnerWrittenLaterGoToTheSeedCharacter(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, repoSeeds()))
	id := f.t.Characters[0]
	rope := f.t.Items["arrows_plus1"]
	rope.ID, rope.CharacterID = "rope", 0
	f.t.Items["rope"] = rope

	require.NoError(t, seedOn(f, repoSeeds()))
	assert.Equal(t, id, f.t.Items["rope"].CharacterID)
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
	n, err := f.CountWeapons(t.Context(), other)
	require.NoError(t, err)
	assert.Zero(t, n)
}

// Without the character the seeds name, the starting set has no owner: it is not applied, and
// rows without an owner refuse the start (they would belong to no one) and change nothing.
func TestSeed_NoSeedCharacter(t *testing.T) {
	t.Run("nothing unowned: nothing is seeded", func(t *testing.T) {
		f := newFakeStore()
		require.NoError(t, seedOn(f, repoSeeds()))
		f.t.Names[f.t.Characters[0]] = "Renamed"
		before := copyTables(f.t)
		require.NoError(t, seedOn(f, repoSeeds()))
		assert.Equal(t, before, f.t)
	})
	t.Run("unowned rows refuse the start", func(t *testing.T) {
		f := newFakeStore()
		require.NoError(t, seedOn(f, repoSeeds()))
		f.t.Names[f.t.Characters[0]] = "Renamed"
		f.disown()
		before := copyTables(f.t)
		err := seedOn(f, repoSeeds())
		require.ErrorContains(t, err, "have no owner")
		assert.Equal(t, before, f.t, "the seed is one transaction: nothing changes")
	})
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
