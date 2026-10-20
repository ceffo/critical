package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// bookCharacter stores a character with the spellbook keys, in that order, each 'starting' (the
// default migration 028 gives the entries stored before it), and returns its id.
func bookCharacter(t *testing.T, f *fakeStore, keys ...string) int64 {
	t.Helper()
	ctx := context.Background()
	id, err := f.InsertCharacter(ctx, storeq.InsertCharacterParams{Name: "mage", ProficiencyPicks: "{}"})
	require.NoError(t, err)
	for i, k := range keys {
		require.NoError(t, f.InsertSpellbookEntry(ctx, storeq.InsertSpellbookEntryParams{
			CharacterID: id, SpellKey: k, Position: int64(i), Origin: "starting",
		}))
	}
	return id
}

// logLearn logs a spell_learn entry of the character for the spell.
func logLearn(f *fakeStore, characterID int64, spellID string, learned bool) {
	payload := `{"spellId":"` + spellID + `","spellLevel":1,"roll":40,"chance":65,"learned":false,"characterLevel":1,"retryLevel":2}`
	if learned {
		payload = `{"spellId":"` + spellID + `","spellLevel":1,"roll":40,"chance":65,"learned":true,"characterLevel":1,"retryLevel":0}`
	}
	f.t.Log = append(f.t.Log, logRow{ID: int64(len(f.t.Log) + 1), CharacterID: characterID, Kind: "spell_learn", Payload: payload})
}

// origins is each character's spellbook as "key:origin", in its order.
func (f *fakeStore) origins(characterID int64) []string {
	out := []string{}
	for _, e := range f.spellbookRows() {
		if e.CharacterID == characterID {
			out = append(out, e.SpellKey+":"+e.Origin)
		}
	}
	return out
}

// runOriginMigration runs every migration with 028 pending on f.
func runOriginMigration(t *testing.T, f *fakeStore) *fakeMigrations {
	t.Helper()
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	before, after := migrationDataSteps(repoSeeds(), catalogueEquipment)
	run := &fakeMigrations{store: f}
	for _, m := range embedded {
		if m.name != spellbookOriginMigration {
			run.recorded = append(run.recorded, m.name)
		}
	}
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Contains(t, run.steps, "apply "+spellbookOriginMigration)
	return run
}

// ADR 0030 step 20.5: "Existing entries, by the migration's data step: one that a successful
// spell_learn journal entry wrote is learned, every other one (Kelsen's imported book included)
// is starting." A failed learn, a success of another character and a success for a spell no
// longer in the book (undone) mark nothing. Running the step twice changes nothing, and a
// given entry is never marked.
func TestMigrate_Step20_5_ExistingEntriesGetTheirOrigin(t *testing.T) {
	f := newFakeStore()
	kelsen := bookCharacter(t, f, "read_magic", "sleep", "charm_person")
	other := bookCharacter(t, f, "charm_person", "alarm")
	logLearn(f, kelsen, "sleep", true)
	logLearn(f, kelsen, "alarm", true)         // undone since: not in his book
	logLearn(f, kelsen, "charm_person", false) // a failure writes nothing
	logLearn(f, other, "alarm", true)

	runOriginMigration(t, f)
	assert.Equal(t, []string{"read_magic:starting", "sleep:learned", "charm_person:starting"}, f.origins(kelsen))
	assert.Equal(t, []string{"charm_person:starting", "alarm:learned"}, f.origins(other))

	// Twice: the step again, on a book that since got a given entry, changes nothing.
	require.NoError(t, f.InsertSpellbookEntry(context.Background(), storeq.InsertSpellbookEntryParams{
		CharacterID: kelsen, SpellKey: "alarm", Position: 3, Origin: "given",
	}))
	snapshot := copyTables(f.t)
	_, after := migrationDataSteps(repoSeeds(), catalogueEquipment)
	require.NoError(t, f.InTx(context.Background(), func(ctx context.Context) error {
		return after[spellbookOriginMigration](ctx, f)
	}))
	assert.Equal(t, snapshot, f.t, "a second run changes nothing")
	assert.Equal(t, []string{"read_magic:starting", "sleep:learned", "charm_person:starting", "alarm:given"}, f.origins(kelsen))
}

// ADR 0030 step 20.9: "No content migration for existing characters ... only the origin of their
// entries is set." The migration run keeps every entry, in its place: same keys, same positions.
func TestMigrate_Step20_9_NoSpellIsAddedOrRemoved(t *testing.T) {
	f := newFakeStore()
	kelsen := bookCharacter(t, f, "read_magic", "sleep", "charm_person")
	logLearn(f, kelsen, "sleep", true)
	logLearn(f, kelsen, "fireball", true) // learned, then undone: not in the book, not added
	type place struct {
		key      string
		position int64
	}
	places := func() []place {
		out := []place{}
		for _, e := range f.spellbookRows() {
			out = append(out, place{e.SpellKey, e.Position})
		}
		return out
	}
	want := places()

	runOriginMigration(t, f)
	assert.Equal(t, want, places())
	assert.Empty(t, f.t.LearnFailures, "no failure written")
}
