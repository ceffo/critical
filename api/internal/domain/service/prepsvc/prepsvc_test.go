package prepsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spelleffectsadpt"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

// passTx runs fn directly: the transaction itself is storage's job.
type passTx struct{ calls int }

func (p *passTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	p.calls++
	return fn(ctx)
}

type fixture struct {
	svc   prepsvc.SVC
	items *mocks.MockItems
	slots *mocks.MockSlots
	log   *mocks.MockLog
	tx    *passTx
}

// bardTable is PHB Table 32 at level 15.
var bardTable = map[int]int{1: 3, 2: 3, 3: 3, 4: 3, 5: 2}

var book = map[string]prepsvc.SpellbookEntry{
	"magic_missile":           {ID: "magic_missile", Name: "Magic Missile", Level: 1},
	"alarm":                   {ID: "alarm", Name: "Alarm", Level: 1},
	"identify":                {ID: "identify", Name: "Identify", Level: 1},
	"enlarge":                 {ID: "enlarge", Name: "Enlarge", Level: 1},
	"mirror_image":            {ID: "mirror_image", Name: "Mirror Image", Level: 2},
	"fireball":                {ID: "fireball", Name: "Fireball", Level: 3},
	"chromatic_orb":           {ID: "chromatic_orb", Name: "Chromatic Orb", Level: 1},
	"rarys_mnemonic_enhancer": {ID: "rarys_mnemonic_enhancer", Name: "Rary's Mnemonic Enhancer", Level: 4},
	"wall_of_ice":             {ID: "wall_of_ice", Name: "Wall of Ice", Level: 4},
	"cloudkill":               {ID: "cloudkill", Name: "Cloudkill", Level: 5},
	"slow":                    {ID: "slow", Name: "Slow", Level: 3},
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{slots: mocks.NewMockSlots(t), log: mocks.NewMockLog(t), items: mocks.NewMockItems(t), tx: &passTx{}}

	b := mocks.NewMockSpellbook(t)
	b.EXPECT().Get(mock.Anything).RunAndReturn(func(id string) (prepsvc.SpellbookEntry, bool) {
		e, ok := book[id]
		return e, ok
	}).Maybe()
	b.EXPECT().All().RunAndReturn(func() []prepsvc.SpellbookEntry {
		return []prepsvc.SpellbookEntry{book["alarm"], book["fireball"], book["chromatic_orb"], book["rarys_mnemonic_enhancer"], book["wall_of_ice"], book["cloudkill"], book["enlarge"], book["slow"], book["mirror_image"]}
	}).Maybe()

	limits := mocks.NewMockLimits(t)
	limits.EXPECT().BardSpellsPerDay(15).Return(bardTable, nil).Maybe()

	chars := mocks.NewMockCharacters(t)
	chars.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(&charactersvc.Character{Level: 15}, nil).Maybe()

	svc, err := prepsvc.New(prepsvc.Deps{
		Slots: f.slots, Spellbook: b, Effects: catalog(t), Limits: limits, Items: f.items, Characters: chars, Log: f.log, Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

func TestNew_NilDeps(t *testing.T) {
	_, err := prepsvc.New(prepsvc.Deps{})
	require.Error(t, err)
}

func TestInfo(t *testing.T) {
	f := newFixture(t)
	f.items.EXPECT().List(mock.Anything).Return(nil, nil)
	info, err := f.svc.Info(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.LevelLimit{{1, 3}, {2, 3}, {3, 3}, {4, 3}, {5, 2}}, info.Limits)
	require.Len(t, info.Spellbook, 9)
	assert.Equal(t, prepsvc.CastInputs{}, info.Spellbook[0].Cast, "alarm asks nothing")
}

// ADR 0008 step 14: a spell with cast-time modes offers them in the data's
// order (the first is the default), each with the damage a cast in it rolls
// at the character's level; the spell's own damage is the default mode's
// (Wall of Ice's ice plane: none).
func TestInfo_ServesTheModes(t *testing.T) {
	f := newFixture(t)
	f.items.EXPECT().List(mock.Anything).Return(nil, nil)
	info, err := f.svc.Info(context.Background(), 1)
	require.NoError(t, err)
	wall := info.Spellbook[4]
	assert.Nil(t, wall.Damage, "the ice plane rolls no damage")
	assert.Equal(t, []spelleffectsvc.ModeAt{
		{Key: "ice_plane"},
		{Key: "hemisphere"},
		{Key: "ice_sheet", Damage: &spelleffectsvc.DamageAt{
			CasterLevel: 15, Dice: formula.Sum{Dice: []formula.Dice{{Count: 3, Sides: 10}}},
			Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "cold",
		}},
	}, wall.Cast.Modes)
	assert.Nil(t, info.Spellbook[1].Cast.Modes, "fireball has no modes")
}

// ADR 0008 step 16: a spellbook entry carries the exposure damage at the
// character's level (Cloudkill: 1d10 poison a round, and no cast damage);
// none for a spell without one.
func TestInfo_ServesTheExposure(t *testing.T) {
	f := newFixture(t)
	f.items.EXPECT().List(mock.Anything).Return(nil, nil)
	info, err := f.svc.Info(context.Background(), 1)
	require.NoError(t, err)
	cloud := info.Spellbook[5]
	require.Equal(t, "cloudkill", cloud.ID)
	assert.Nil(t, cloud.Damage, "a cast rolls none")
	assert.Equal(t, &spelleffectsvc.ExposureAt{Damage: &spelleffectsvc.DamageAt{
		CasterLevel: 15, Dice: formula.Sum{Dice: []formula.Dice{{Count: 1, Sides: 10}}},
		Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "poison",
	}}, cloud.Exposure)
	assert.Nil(t, info.Spellbook[1].Exposure, "fireball has none")
}

// ADR 0008 steps 9 and 10: what a cast asks of the player comes from the
// spell effects data, never from the spell's id.
func TestInfo_CastInputsFromTheCatalog(t *testing.T) {
	f := newFixture(t)
	f.items.EXPECT().List(mock.Anything).Return(nil, nil)
	info, err := f.svc.Info(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, prepsvc.CastInputs{ConsumeItemType: "gem", AttackRoll: true}, info.Spellbook[2].Cast)
	assert.Equal(t, prepsvc.CastInputs{SlotSplits: [][]int{{1, 1, 1}, {1, 2}, {3}}}, info.Spellbook[3].Cast)
}

// A spellbook entry carries the choice of target its cast offers, from the
// spell effects data: Enlarge (self or other) lands on the Bard, else on an
// ally; Slow (a hostile area) not on him, else on him. A self-only buff
// (Mirror Image) and a spell that adds no buff (Fireball, Alarm) offer none.
func TestInfo_ServesTheTargetChoice(t *testing.T) {
	f := newFixture(t)
	f.items.EXPECT().List(mock.Anything).Return(nil, nil)
	info, err := f.svc.Info(context.Background(), 1)
	require.NoError(t, err)
	byID := map[string]*spelleffectsvc.TargetChoice{}
	for _, e := range info.Spellbook {
		byID[e.ID] = e.Cast.Target
	}
	assert.Equal(t, &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnSelf, Alternate: spelleffectsvc.CastOnOther}, byID["enlarge"])
	assert.Equal(t, &spelleffectsvc.TargetChoice{Default: spelleffectsvc.CastOnOther, Alternate: spelleffectsvc.CastOnSelf}, byID["slow"])
	assert.Nil(t, byID["mirror_image"], "self-only")
	assert.Nil(t, byID["fireball"], "a damage spell")
	assert.Nil(t, byID["alarm"], "a utility spell")
}

// A spellbook entry carries the damage a cast rolls at the character's level
// (15: fireball caps at 10d6), from the spell effects data; none for a spell
// that rolls no damage.
func TestInfo_DamageAtTheCharacterLevel(t *testing.T) {
	f := newFixture(t)
	f.items.EXPECT().List(mock.Anything).Return(nil, nil)
	info, err := f.svc.Info(context.Background(), 1)
	require.NoError(t, err)
	assert.Nil(t, info.Spellbook[0].Damage, "alarm rolls no damage")
	require.NotNil(t, info.Spellbook[1].Damage)
	assert.Equal(t, spelleffectsvc.DamageAt{
		CasterLevel: 15, Dice: formula.Sum{Dice: []formula.Dice{{Count: 10, Sides: 6}}},
		Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "fire",
	}, *info.Spellbook[1].Damage)
}

// A spell that consumes an item lists the inventory items it can spend now:
// of its type, not daily, some left (itemsvc.Item.Spendable). A spell that
// consumes nothing lists none, whatever the inventory holds.
func TestInfo_ServesTheItemsTheSpellCanSpend(t *testing.T) {
	f := newFixture(t)
	f.items.EXPECT().List(mock.Anything).Return([]itemsvc.Item{
		{ID: "ruby", Name: localized.Of("Ruby"), Type: "gem", Behaviour: itemsvc.BehaviourStock, Count: 2},
		{ID: "opal", Name: localized.Of("Opal"), Type: "Gem", Behaviour: itemsvc.BehaviourCharges, Count: 1},
		{ID: "spent", Name: localized.Of("Spent"), Type: "gem", Behaviour: itemsvc.BehaviourStock, Count: 0},
		{ID: "daily", Name: localized.Of("Daily"), Type: "gem", Behaviour: itemsvc.BehaviourDaily, Count: 3},
		{ID: "arrow", Name: localized.Of("Arrow"), Type: "arrow", Behaviour: itemsvc.BehaviourStock, Count: 20},
	}, nil)
	info, err := f.svc.Info(context.Background(), 1)
	require.NoError(t, err)
	assert.Empty(t, info.Spellbook[0].Items, "alarm consumes nothing")
	var ids []string
	for _, it := range info.Spellbook[2].Items {
		ids = append(ids, it.ID)
	}
	assert.Equal(t, []string{"ruby", "opal"}, ids)
}

func TestInfo_ItemsListFailure(t *testing.T) {
	f := newFixture(t)
	f.items.EXPECT().List(mock.Anything).Return(nil, errors.New("boom"))
	_, err := f.svc.Info(context.Background(), 1)
	require.Error(t, err)
}

// catalog is the test seed of spell effects.
func catalog(t *testing.T) spelleffectsvc.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "testdata", "seeds"))
	require.NoError(t, err)
	return c
}

func TestPrepare_Refused(t *testing.T) {
	tests := []struct {
		name    string
		ids     []string
		wantErr error
	}{
		{"4 level-1 spells at level 15", []string{"magic_missile", "alarm", "identify", "enlarge"}, prepsvc.ErrOverLimit},
		{"5 copies of one level-1 spell", []string{"alarm", "alarm", "alarm", "alarm", "alarm"}, prepsvc.ErrOverLimit},
		{"spell outside the spellbook", []string{"shield"}, prepsvc.ErrNotInSpellbook},
		{"valid and invalid mixed", []string{"alarm", "shield"}, prepsvc.ErrNotInSpellbook},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t) // no slot store or log call is expected: nothing is written
			_, err := f.svc.Prepare(context.Background(), 1, tc.ids)
			require.ErrorIs(t, err, tc.wantErr)
			assert.Zero(t, f.tx.calls)
		})
	}
}

func TestPrepare_AcceptsTheLimit_KeepsOrderAndDuplicates(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{}, nil)
	var saved []spellsvc.SpellSlot
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).
		Run(func(_ context.Context, _ int64, s []spellsvc.SpellSlot) { saved = s }).Return(nil)
	var rec actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).
		Run(func(_ context.Context, e actionlog.NewEntry) { rec = e }).Return(int64(7), nil)

	got, err := f.svc.Prepare(context.Background(), 1,
		[]string{"fireball", "magic_missile", "mirror_image", "alarm", "mirror_image", "identify"})
	require.NoError(t, err)

	want := []spellsvc.SpellSlot{
		{Level: 1, SpellID: "magic_missile", PreparedSpellName: "Magic Missile"},
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm"},
		{Level: 1, SpellID: "identify", PreparedSpellName: "Identify"},
		{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image"},
		{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image"},
		{Level: 3, SpellID: "fireball", PreparedSpellName: "Fireball"},
	}
	assert.Equal(t, want, got)
	assert.Equal(t, want, saved)

	// Recorded in the same transaction, with a snapshot under the state key.
	assert.Equal(t, 1, f.tx.calls)
	assert.Equal(t, actionlog.KindSpellPrep, rec.Kind)
	assert.JSONEq(t, `{"spells":[{"id":"magic_missile","name":"Magic Missile","count":1},{"id":"alarm","name":"Alarm","count":1},
		{"id":"identify","name":"Identify","count":1},{"id":"mirror_image","name":"Mirror Image","count":2},{"id":"fireball","name":"Fireball","count":1}]}`,
		string(rec.Payload))
	require.Len(t, rec.Snapshots, 1)
	assert.Equal(t, prepsvc.StateKey, rec.Snapshots[0].Key)
	assert.JSONEq(t, `[]`, string(rec.Snapshots[0].Before))
	var after []spellsvc.SpellSlot
	require.NoError(t, json.Unmarshal(rec.Snapshots[0].After, &after))
	assert.Equal(t, want, after)
}

func TestPrepare_ThreeLevelOneSpellsAccepted(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
	got, err := f.svc.Prepare(context.Background(), 1, []string{"magic_missile", "alarm", "identify"})
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestPrepare_KeepsUsedFlagOfSpellsStillPrepared(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm", Used: true},
		{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image", Used: true},
		{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image"},
	}, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)

	got, err := f.svc.Prepare(context.Background(), 1, []string{"mirror_image", "mirror_image", "mirror_image", "magic_missile"})
	require.NoError(t, err)
	used := []bool{}
	for _, s := range got {
		used = append(used, s.Used)
	}
	// magic_missile (new, unused), then mirror_image: one used cast carried over, two fresh.
	assert.Equal(t, []bool{false, true, false, false}, used)
}

func TestPrepare_RecordFailureFailsTheCall(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return(nil, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	boom := errors.New("boom")
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(0), boom)
	_, err := f.svc.Prepare(context.Background(), 1, []string{"alarm"})
	require.ErrorIs(t, err, boom) // the real TxRunner rolls the slot write back
}

func TestRestore_WritesTheSnapshotBack(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), []spellsvc.SpellSlot{
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm", Used: true},
	}).Return(nil)
	raw, err := json.Marshal([]spellsvc.SpellSlot{{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm", Used: true}})
	require.NoError(t, err)
	require.NoError(t, f.svc.Restore(context.Background(), 1, raw))

	require.Error(t, f.svc.Restore(context.Background(), 1, json.RawMessage(`{`)))
}

// ADR 0008 step 9: extra slots are not part of the preparation, so
// preparing again keeps them as they are.
func TestPrepare_KeepsExtraSlots(t *testing.T) {
	f := newFixture(t)
	extra := spellsvc.SpellSlot{Level: 2, SpellID: "mirror_image", PreparedSpellName: "Mirror Image", Used: true, Extra: true}
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{
		{Level: 1, SpellID: "alarm", PreparedSpellName: "Alarm"}, extra,
	}, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)

	got, err := f.svc.Prepare(context.Background(), 1, []string{"mirror_image"})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.False(t, got[0].Used, "a fresh mirror_image: the extra slot's cast does not count against it")
	assert.Equal(t, extra, got[1])
}

// Preparing nothing is logged with an empty spell list, worded by the UI.
func TestPrepare_NothingLogsAnEmptyList(t *testing.T) {
	f := newFixture(t)
	f.slots.EXPECT().GetSlots(mock.Anything, int64(1)).Return([]spellsvc.SpellSlot{}, nil)
	f.slots.EXPECT().SaveSlots(mock.Anything, int64(1), mock.Anything).Return(nil)
	var rec actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).
		Run(func(_ context.Context, e actionlog.NewEntry) { rec = e }).Return(int64(7), nil)
	_, err := f.svc.Prepare(context.Background(), 1, nil)
	require.NoError(t, err)
	assert.Empty(t, rec.Summary)
	assert.JSONEq(t, `{"spells":[]}`, string(rec.Payload))
}
