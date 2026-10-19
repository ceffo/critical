package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADR 0018 step 9, ADR 0025 step 2.1.1: the data step re-encodes every stored snapshot into the
// action log codec's envelope, one fixture per old encoding (the nine services' snapshot files
// and the AC override). It changes the encoding only: the same state, the same absence. The
// re-encoded fixtures are the ones composition's TestUndo_EveryStateKeyIsRestoredThroughItsServicesStorage
// restores through each service's storage.
var oldSnapshots = []struct {
	name             string
	key              string
	stored, envelope string
}{
	{
		"active buffs (buffsvc)", "buffs",
		`[{"id":"b1","name":"Stoneskin","charges":7,"duration":0,"effects":{},"spell_id":"stoneskin"}]`,
		`{"state":[{"id":"b1","name":"Stoneskin","charges":7,"duration":0,"effects":{},"spell_id":"stoneskin"}]}`,
	},
	{"no buff", "buffs", `[]`, `{"state":[]}`},
	{
		"saved templates (buffsvc)", "saved_buffs",
		`[{"id":1,"name":"Gem","duration":360,"charges":0,"effects":{"weapon_buff":5}}]`,
		`{"state":[{"id":1,"name":"Gem","duration":360,"charges":0,"effects":{"weapon_buff":5}}]}`,
	},
	{"round (buffsvc)", "round", `3`, `{"state":3}`},
	{"round 0 is a state", "round", `0`, `{"state":0}`},
	{"no initiative (buffsvc): null is absent", "initiative", `null`, `{"absent":true}`},
	{"initiative", "initiative", `{"d10":4,"entryId":12}`, `{"state":{"d10":4,"entryId":12}}`},
	{"initiative not yet naming its entry", "initiative", `{"d10":9}`, `{"state":{"d10":9}}`},
	{"spell slots (prepsvc)", "spell_slots", `[{"SpellID":"alarm"}]`, `{"state":[{"SpellID":"alarm"}]}`},
	{
		"realized level (levelupsvc)", "realized_level", `{"realizedLevel":15,"hpMax":85}`,
		`{"state":{"realizedLevel":15,"hpMax":85}}`,
	},
	{"skill points (skillsvc)", "skill_points", `{"points":{"climb_walls":80}}`, `{"state":{"points":{"climb_walls":80}}}`},
	{"skill armour (charactersvc)", "skill_armor", `{"armor":"none"}`, `{"state":{"armor":"none"}}`},
	{"AC override (acsvc)", "ac_override", `{"override":null}`, `{"state":{"override":null}}`},
	{
		"spellbook (learnsvc): the spell goes to the envelope", "spellbook:sleep",
		`{"spellId":"sleep","known":false,"failedAtLevel":0}`, `{"id":"sleep","state":{"known":false,"failedAtLevel":0}}`,
	},
	{
		"spellbook, a failure", "spellbook:light", `{"spellId":"light","known":false,"failedAtLevel":4}`,
		`{"id":"light","state":{"known":false,"failedAtLevel":4}}`,
	},
	{
		"XP entry added (xpsvc): absent, with its zeroed fields", "xp:12",
		`{"id":12,"absent":true,"date":"","session":"","reason":"","base":0,"bonus":false}`, `{"id":"12","absent":true}`,
	},
	{
		"XP entry", "xp:3", `{"id":3,"date":"2026-10-18","session":"S1","reason":"troll","base":2000,"bonus":true}`,
		`{"id":"3","state":{"date":"2026-10-18","session":"S1","reason":"troll","base":2000,"bonus":true}}`,
	},
	{
		"weapon added before the equipment model (weaponsvc): absent", "weapons:dagger2", `{"id":"dagger2","absent":true}`,
		`{"id":"dagger2","absent":true}`,
	},
	{
		"weapon with a name saved before localisation (weaponsvc)", "weapons:longsword",
		`{"id":"longsword","position":1,"name":"Long Sword +1","damageSm":"1d8","damageL":"1d12","magicToHit":1,"magicDamage":1,` +
			`"kind":"melee","twoHanded":false,"ammoKind":"","weaponType":"long_sword"}`,
		`{"id":"longsword","state":{"position":1,"name":{"en":"Long Sword +1"},"namePartial":true,"damageSm":"1d8","damageL":"1d12",` +
			`"magicToHit":1,"magicDamage":1,"kind":"melee","twoHanded":false,"ammoKind":"","weaponType":"long_sword"}}`,
	},
	{
		"item count of the equipment model (itemsvc)", "items:wand_lightning", `[{"id":"wand_lightning","count":4,"model":2}]`,
		`{"id":"wand_lightning","state":{"count":4}}`,
	},
	{
		"item a use removed: absent", "items:wand_lightning", `[{"id":"wand_lightning","count":0,"removed":true,"model":2}]`,
		`{"id":"wand_lightning","absent":true}`,
	},
	{
		"item added by hand: absent", "items:new", `[{"id":"new","count":0,"removed":true,"absent":true,"model":2}]`,
		`{"id":"new","absent":true}`,
	},
	{
		"item edited, a localized name", "items:rope",
		`[{"id":"rope","count":1,"replace":true,"model":2,"item":{"position":2,"name":{"en":"Rope","fr":"Corde"},"powers":[],` +
			`"piece":{"category":"gear","state":"carried"}}}]`,
		`{"id":"rope","state":{"count":1,"replace":true,"item":{"position":2,"name":{"en":"Rope","fr":"Corde"},"powers":[],` +
			`"piece":{"category":"gear","state":"carried"}}}}`,
	},
	{
		"item with a name saved before localisation, before powers and the model", "items:wand_lightning",
		`[{"id":"wand_lightning","count":5,"item":{"position":4,"name":"Wand of lightning","type":"wand","behaviour":"charges"}}]`,
		`{"id":"wand_lightning","state":{"count":5,"item":{"position":4,"name":{"en":"Wand of lightning"},"namePartial":true,` +
			`"type":"wand","behaviour":"charges"}}}`,
	},
	{
		"item count before the model, renamed by the conversion (ADR 0033 step 26.3)", "items:dagger",
		`[{"id":"dagger","count":3}]`, `{"id":"dagger_2","state":{"count":3}}`,
	},
	{
		"item count of the model: never renamed", "items:dagger", `[{"id":"dagger","count":3,"model":2}]`,
		`{"id":"dagger","state":{"count":3}}`,
	},
}

const snapshotOwner = int64(7)

func reencodeStore() *fakeStore {
	store := newFakeStore()
	store.t.Renames[rowKey(snapshotOwner, "dagger")] = "dagger_2"
	for i, s := range oldSnapshots {
		store.t.Snapshots = append(store.t.Snapshots, snapshotRow{
			ID: int64(i + 1), ActionID: int64(i + 1), Key: s.key, Before: s.stored, After: s.stored, CharacterID: snapshotOwner,
		})
	}
	return store
}

func TestReencodeSnapshots_EveryOldEncodingBecomesTheEnvelope(t *testing.T) {
	store := reencodeStore()
	require.NoError(t, reencodeSnapshots(context.Background(), store))
	for i, s := range oldSnapshots {
		t.Run(s.name, func(t *testing.T) {
			row := store.t.Snapshots[i]
			assert.JSONEq(t, s.envelope, row.BeforeEnv)
			assert.JSONEq(t, s.envelope, row.AfterEnv)
			assert.Equal(t, s.stored, row.Before, "the old column is left as it was")
		})
	}
}

// The step runs on the migration run's transaction just before the envelopes become required
// (migration 020), on a database that gets 019 and 020 in one run; a database that records 020
// never runs it again.
func TestMigrate_SnapshotsAreReencodedBeforeTheEnvelopesAreRequired(t *testing.T) {
	embedded, err := embeddedMigrations()
	require.NoError(t, err)
	var recorded []string
	for _, m := range embedded {
		if m.name < "019" {
			recorded = append(recorded, m.name)
		}
	}
	store := reencodeStore()
	f := &fakeMigrations{recorded: recorded, store: store}
	missingAtApply := -1
	f.onApply = func(m migration) {
		if m.name == snapshotEnvelopesMigration {
			missing, err := store.SnapshotsToReencode(context.Background())
			require.NoError(t, err)
			missingAtApply = len(missing)
		}
	}
	run := func() error {
		return store.InTx(context.Background(), func(ctx context.Context) error {
			before, after := migrationDataSteps(repoSeeds(), catalogueWeapons)
			return runMigrations(ctx, f, embedded, before, after)
		})
	}
	require.NoError(t, run())
	assert.Equal(t, 0, missingAtApply, "every snapshot has its envelopes when they become required")
	assert.Contains(t, f.steps, "apply "+snapshotEnvelopesMigration)

	// A rerun finds 020 recorded and runs no step: an envelope it wrote is not rewritten.
	store.t.Snapshots[0].BeforeEnv = `{"state":"kept"}`
	require.NoError(t, run())
	assert.JSONEq(t, `{"state":"kept"}`, store.t.Snapshots[0].BeforeEnv)
}

// Rows that already have their envelopes are not read again (a step run twice changes nothing).
func TestReencodeSnapshots_SkipsWhatIsAlreadyReencoded(t *testing.T) {
	store := newFakeStore()
	store.t.Snapshots = []snapshotRow{{ID: 1, Key: "round", Before: `"garbage"`, After: `"garbage"`, BeforeEnv: `{"state":1}`, AfterEnv: `{"state":2}`}}
	require.NoError(t, reencodeSnapshots(context.Background(), store))
	assert.JSONEq(t, `{"state":1}`, store.t.Snapshots[0].BeforeEnv)
}

// A snapshot no restorer reads, or one that does not decode, fails the step (and so the run,
// which rolls back): nothing is guessed.
func TestReencodeSnapshots_RefusesWhatItCannotRead(t *testing.T) {
	for _, tc := range []struct{ name, key, stored string }{
		{"unknown family", "hit_points", `{"hp":3}`},
		{"items with two counts", "items:a", `[{"id":"a","count":1},{"id":"b","count":2}]`},
		{"items not a list", "items:a", `{"id":"a","count":1}`},
		{"not JSON", "round", `{`},
		{"xp not an object", "xp:1", `[]`},
		{"spellbook with a bad spell id", "spellbook:x", `{"spellId":3}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			store.t.Snapshots = []snapshotRow{{ID: 9, Key: tc.key, Before: tc.stored, After: tc.stored}}
			err := reencodeSnapshots(context.Background(), store)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "action_snapshots 9")
		})
	}
}
