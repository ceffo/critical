package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// entriesCharacter stores a character with the stored picks column picks and the class entries
// entries ({class, realized level} active, or frozen with its left-at level).
func entriesCharacter(t *testing.T, f *fakeStore, picks string, entries ...storeq.InsertClassEntryParams) int64 {
	t.Helper()
	id, err := f.InsertCharacter(context.Background(), storeq.InsertCharacterParams{Name: entries[0].Class, ProficiencyPicks: picks})
	require.NoError(t, err)
	f.t.Classes[id] = entries[0].Class
	for i, e := range entries {
		e.CharacterID, e.Position = id, int64(i)
		if e.State == "" {
			e.State = "active"
		}
		require.NoError(t, f.InsertClassEntry(context.Background(), e))
	}
	return id
}

func active(class string, level int64) storeq.InsertClassEntryParams {
	return storeq.InsertClassEntryParams{Class: class, RealizedLevel: level, State: "active"}
}

func frozen(class string, leftAt int64) storeq.InsertClassEntryParams {
	return storeq.InsertClassEntryParams{Class: class, RealizedLevel: leftAt, State: "frozen", LeftAtLevel: leftAt}
}

// returnFree runs the data step after migration 046 on f with the fixture's catalogue.
func returnFree(t *testing.T, f *fakeStore) error {
	t.Helper()
	_, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	return f.InTx(context.Background(), func(ctx context.Context) error {
		return after[freePicksMigration](ctx, f)
	})
}

// ADR 0037 step 22.7: for each stored non-weapon pick whose id a class entry grants free (the
// fixture bard's reading_writing, local_history, singing and musical_instrument at level 1), the
// slots become the stored slots minus the cost the pick paid before (step 4, 1 each for a bard),
// never below 0: a bard's four at 1 become 0, a musical instrument at 3 becomes 2 (its two extra
// slots, so the same check target); his Tumbling stays, as do the weapon picks and every other key.
// A fighter's singing (no grant) and a bard who holds none of the four are unchanged (the latter not
// written at all). Step 22.5 on several entries: a fighter left at 5 now bard 1 gets them free,
// paying before at the active bard's groups (reading_writing 1, so 2 slots become 1); a bard left at
// 5 now fighter 2 keeps them free, paying before at the fighter's groups (reading_writing 2 outside
// them, so 2 slots become 0; singing, general, 1).
func TestMigrate_Step22_7_AFreePickStopsPaying(t *testing.T) {
	f := newFakeStore()
	bard := entriesCharacter(t, f, `{"nonweapon":[{"id":"reading_writing","slots":1},{"id":"local_history","slots":1},
		{"id":"singing","slots":1},{"id":"musical_instrument","slots":3},{"id":"tumbling","slots":1}],
		"weapon":[{"code":"dagger","slots":1}],"weapon_redo":[{"type":"sling","slots":1}]}`, active("bard", 5))
	fighter := entriesCharacter(t, f, `{"nonweapon":[{"id":"singing","slots":1}]}`, active("fighter", 5))
	without := `{"nonweapon": [{"id":"tumbling","slots":2}]}`
	other := entriesCharacter(t, f, without, active("bard", 3))
	toBard := entriesCharacter(t, f, `{"nonweapon":[{"id":"reading_writing","slots":2}]}`, frozen("fighter", 5), active("bard", 1))
	toFighter := entriesCharacter(t, f, `{"nonweapon":[{"id":"reading_writing","slots":2},{"id":"singing","slots":1}]}`,
		frozen("bard", 5), active("fighter", 2))

	require.NoError(t, returnFree(t, f))
	assert.JSONEq(t, `{"nonweapon":[{"id":"reading_writing","slots":0},{"id":"local_history","slots":0},
		{"id":"singing","slots":0},{"id":"musical_instrument","slots":2},{"id":"tumbling","slots":1}],
		"weapon":[{"code":"dagger","slots":1}],"weapon_redo":[{"type":"sling","slots":1}]}`, f.picks(bard))
	assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":1}]}`, f.picks(fighter), "no grant: unchanged")
	assert.Equal(t, without, f.t.Picks[other], "none of the four: not written")
	assert.JSONEq(t, `{"nonweapon":[{"id":"reading_writing","slots":1}]}`, f.picks(toBard), "the active bard grants it")
	assert.JSONEq(t, `{"nonweapon":[{"id":"reading_writing","slots":0},{"id":"singing","slots":0}]}`, f.picks(toFighter),
		"the frozen bard grants it, at the fighter's cost")
}

// ADR 0037 step 22.7 and ADR 0025 step 3: the data step runs once, with migration 046; a later run
// with nothing pending converts nothing again (a second run would take the cost a second time).
func TestMigrate_Step22_7_RunsOnceWithTheMigration(t *testing.T) {
	f := newFakeStore()
	bard := entriesCharacter(t, f, `{"nonweapon":[{"id":"singing","slots":2},{"id":"tumbling","slots":1}]}`, active("bard", 4))
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	before, after := migrationDataSteps(repoSeeds(), catalogueEquipment, catalogueDefs)
	run := &fakeMigrations{store: f}
	for _, m := range embedded {
		if m.name != freePicksMigration {
			run.recorded = append(run.recorded, m.name)
		}
	}
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Contains(t, run.steps, "apply "+freePicksMigration)
	assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":1},{"id":"tumbling","slots":1}]}`, f.picks(bard))

	run.steps = nil
	require.NoError(t, runMigrations(context.Background(), run, embedded, before, after))
	assert.Equal(t, []string{"lock", "ensure table", "applied"}, run.steps, "nothing pending")
	assert.JSONEq(t, `{"nonweapon":[{"id":"singing","slots":1},{"id":"tumbling","slots":1}]}`, f.picks(bard), "never converted twice")
}
