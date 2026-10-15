package castsvc_test

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
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/pkg/formula"
)

const (
	charID     = int64(1)
	bardLevel  = 15
	buffsAdded = "added-buff"
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool        { return &v }

// catalog is the real seed: the tests check the spells behave per their
// seed entries.
func catalog(t *testing.T) spelleffectsvc.Catalog {
	t.Helper()
	c, err := spelleffectsadpt.New(filepath.Join("..", "..", "..", "..", "testdata", "seeds"))
	require.NoError(t, err)
	return c
}

// seqDice returns the queued values in order and fails the test when a roll
// is not accounted for.
type seqDice struct {
	t     *testing.T
	vals  []int
	sides []int
}

func (d *seqDice) RollDie(sides int) int {
	d.t.Helper()
	if len(d.vals) == 0 {
		d.t.Fatalf("unexpected d%d roll", sides)
	}
	v := d.vals[0]
	d.vals = d.vals[1:]
	d.sides = append(d.sides, sides)
	return v
}

type fakeTx struct{ calls int }

func (f *fakeTx) InTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	return fn(ctx)
}

type fixture struct {
	svc     castsvc.SVC
	slots   *mocks.MockSlots
	texts   *mocks.MockTexts
	buffs   *mocks.MockBuffs
	items   *mocks.MockItems
	attacks *mocks.MockAttacks
	dice    *seqDice
	tx      *fakeTx
	entries []actionlog.NewEntry
	added   []buffsvc.BuffEffect
	saved   [][]spellsvc.SpellSlot
}

func newFixture(t *testing.T, dice ...int) *fixture {
	t.Helper()
	f := &fixture{
		slots:   mocks.NewMockSlots(t),
		texts:   mocks.NewMockTexts(t),
		buffs:   mocks.NewMockBuffs(t),
		items:   mocks.NewMockItems(t),
		attacks: mocks.NewMockAttacks(t),
		dice:    &seqDice{t: t, vals: dice},
		tx:      &fakeTx{},
	}
	chars := mocks.NewMockCharacters(t)
	chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{ID: charID, Level: bardLevel, Str: 12}, nil).Maybe()
	log := mocks.NewMockLog(t)
	log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			f.entries = append(f.entries, e)
			return int64(len(f.entries)), nil
		}).Maybe()
	f.texts.EXPECT().Describe(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (spellsvc.Spell, bool, error) {
			if id == "no_such_spell" {
				return spellsvc.Spell{}, false, nil
			}
			return spellsvc.Spell{Name: "Text of " + id, Description: "desc " + id}, true, nil
		}).Maybe()
	f.buffs.EXPECT().Add(mock.Anything, charID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, b buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error) {
			b.ID = buffsAdded
			f.added = append(f.added, b)
			return b, actionlog.Snapshot{Key: buffsvc.StateKeyBuffs, Before: json.RawMessage(`[]`), After: json.RawMessage(`[1]`)}, nil
		}).Maybe()
	f.slots.EXPECT().SaveSlots(mock.Anything, charID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, s []spellsvc.SpellSlot) error {
			f.saved = append(f.saved, s)
			return nil
		}).Maybe()
	svc, err := castsvc.New(castsvc.Deps{
		Slots: f.slots, Effects: catalog(t), Texts: f.texts, Buffs: f.buffs,
		Items: f.items, Attacks: f.attacks,
		Characters: chars, Dice: f.dice, Log: log, Tx: f.tx,
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

func (f *fixture) prepared(slots ...spellsvc.SpellSlot) {
	f.slots.EXPECT().GetSlots(mock.Anything, charID).Return(slots, nil).Once()
}

func slot(id, name string, level int) spellsvc.SpellSlot {
	return spellsvc.SpellSlot{Level: level, SpellID: id, PreparedSpellName: name}
}

func keys(snaps []actionlog.Snapshot) []string {
	out := []string{}
	for _, s := range snaps {
		out = append(out, s.Key)
	}
	return out
}

// The Bard casts at level 15. Each spell per its seed entry, with injected dice.
func TestCast_PerSeedEntry(t *testing.T) {
	tests := []struct {
		name     string
		slot     spellsvc.SpellSlot
		onSelf   *bool
		dice     []int
		wantBuff *buffsvc.BuffEffect // nil: no buff on the Bard
		wantDmg  *castsvc.DamageResult
		textOnly castsvc.TextOnly
		check    func(t *testing.T, r castsvc.CastResult)
	}{
		{
			name: "stoneskin: 1d4 + level/2 layers, until discharged",
			slot: slot("stoneskin", "Stoneskin", 4), dice: []int{3},
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Stoneskin", Charges: 3 + 7},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Nil(t, r.Buff.Duration)
				assert.Equal(t, "attacks", r.Buff.ChargeUnit)
				assert.Equal(t, []castsvc.DieRoll{{Sides: 4, Value: 3}}, r.Buff.Charges.Dice)
			},
		},
		{
			name: "mirror image: 3 rd/level and 1d4 + level/3 images",
			slot: slot("mirror_image", "Mirror Image", 2), dice: []int{2},
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Mirror Image", Duration: 45, Charges: 2 + 5},
		},
		{
			name: "mirror image caps at 8 images",
			slot: slot("mirror_image", "Mirror Image", 2), dice: []int{4},
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Mirror Image", Duration: 45, Charges: 8},
		},
		{
			name:     "haste: 3 + level rounds; extra attacks, movement and initiative are shown, not tracked",
			slot:     slot("haste", "Haste", 3),
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Haste", Duration: 18},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Equal(t, []castsvc.Untracked{
					{Key: "attacks_pct", Value: 200}, {Key: "initiative", Value: -2}, {Key: "move_pct", Value: 200},
				}, r.Untracked)
			},
		},
		{
			name: "slow (hostile area spell) does not land on the Bard by default",
			slot: slot("slow", "Slow", 3), textOnly: castsvc.TextOnlyNotOnSelf,
		},
		{
			name: "stoneskin cast on an ally: rolled but not on the Bard",
			slot: slot("stoneskin", "Stoneskin", 4), onSelf: boolPtr(false), dice: []int{1},
			textOnly: castsvc.TextOnlyNotOnSelf,
			check: func(t *testing.T, r castsvc.CastResult) {
				require.NotNil(t, r.Buff)
				assert.False(t, r.Buff.Applied)
				assert.Equal(t, 8, r.Buff.Buff.Charges)
			},
		},
		{
			name: "mirror image is self-only: on-self false is ignored",
			slot: slot("mirror_image", "Mirror Image", 2), onSelf: boolPtr(false), dice: []int{1},
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Mirror Image", Duration: 45, Charges: 6},
		},
		{
			name: "enlarge: damage multiplier 100% + 10%/level",
			slot: slot("enlarge", "Enlarge", 1),
			wantBuff: &buffsvc.BuffEffect{
				ID: buffsAdded, Name: "Enlarge", Duration: 75,
				Effects: buffsvc.BuffEffects{SpellsBuff: floatPtr(2.5)},
			},
		},
		{
			name: "fireball: 1d6 per level, max 10d6; save for half noted",
			slot: slot("fireball", "Fireball", 3), dice: []int{1, 2, 3, 4, 5, 6, 1, 2, 3, 4},
			wantDmg: &castsvc.DamageResult{Dice: "(min(level,10))d6", Type: "fire", Instances: 1, Rounds: 1, Total: 31},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Equal(t, &spelleffectsvc.Save{Type: "spell", Effect: spelleffectsvc.SaveHalf}, r.Save)
				require.Len(t, r.Damage.Rolls, 1)
				assert.Len(t, r.Damage.Rolls[0].Dice, 10)
			},
		},
		{
			name: "magic missile: 5 missiles of 1d4+1 at level 9+",
			slot: slot("magic_missile", "Magic Missile", 1), dice: []int{1, 2, 3, 4, 1},
			wantDmg: &castsvc.DamageResult{Dice: "1d4+1", Type: "force", Instances: 5, Rounds: 1, Total: 2 + 3 + 4 + 5 + 2},
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Nil(t, r.Save, "no saving throw")
				require.Len(t, r.Damage.Rolls, 5)
				assert.Equal(t, 5, r.Damage.Rolls[3].Total)
			},
		},
		{
			name: "melf's acid arrow: 2d4 repeated 1 + level/3 rounds",
			slot: slot("melfs_acid_arrow", "Melf's Acid Arrow", 2), dice: []int{1, 1, 2, 2, 3, 3, 4, 4, 1, 2, 1, 1},
			wantDmg: &castsvc.DamageResult{Dice: "2d4", Type: "acid", Instances: 1, Rounds: 6, Total: 2 + 4 + 6 + 8 + 3 + 2},
		},
		{
			name: "fist of stone: STR 18/00 for the fist only, shown not applied",
			slot: slot("fist_of_stone", "Fist of Stone", 1), textOnly: castsvc.TextOnlyByHand,
			check: func(t *testing.T, r castsvc.CastResult) {
				assert.Equal(t, []castsvc.Untracked{{Key: "str_set", Value: 18}}, r.Untracked)
				assert.Nil(t, r.Buff.Buff.Effects.StrBonus, "never raises STR for every attack")
			},
		},
		{
			name: "utility spell shows its text only",
			slot: slot("identify", "Identify", 1), textOnly: castsvc.TextOnlyUtility,
		},
		{
			name: "low-confidence spell shows its text only",
			slot: slot("wall_of_ice", "Wall of Ice", 4), textOnly: castsvc.TextOnlyLowConfidence,
		},
		{
			name: "slot saved before preparation: id from the name",
			slot: spellsvc.SpellSlot{Level: 4, PreparedSpellName: "Stoneskin"}, dice: []int{4},
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Stoneskin", Charges: 11},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.dice...)
			before := []spellsvc.SpellSlot{slot("sleep", "Sleep", 1), tt.slot}
			f.prepared(before...)

			r, err := f.svc.Cast(context.Background(), charID, 1, castsvc.CastRequest{OnSelf: tt.onSelf})
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals, "every queued die is rolled")
			assert.Equal(t, 1, f.tx.calls)

			// The slot is spent and logged.
			require.Len(t, f.saved, 1)
			assert.True(t, f.saved[0][1].Used)
			assert.False(t, f.saved[0][0].Used)
			assert.Equal(t, f.saved[0], r.Slots)
			assert.Equal(t, castsvc.SourceCast, r.Source)
			assert.Equal(t, bardLevel, r.CasterLevel)
			require.Len(t, f.entries, 1)
			e := f.entries[0]
			assert.Equal(t, actionlog.KindCast, e.Kind)
			assert.Equal(t, int64(1), r.EntryID)
			wantSnap, err := prepsvc.SlotsSnapshot(before, f.saved[0])
			require.NoError(t, err)
			assert.Equal(t, wantSnap, e.Snapshots[0])

			assert.Equal(t, tt.textOnly, r.TextOnly)
			if tt.wantBuff != nil {
				require.Len(t, f.added, 1)
				assert.Equal(t, *tt.wantBuff, f.added[0])
				assert.Equal(t, *tt.wantBuff, r.Buff.Buff)
				assert.True(t, r.Buff.Applied)
				assert.Equal(t, []string{prepsvc.StateKey, buffsvc.StateKeyBuffs}, keys(e.Snapshots))
			} else {
				assert.Empty(t, f.added)
				assert.Equal(t, []string{prepsvc.StateKey}, keys(e.Snapshots))
			}
			if tt.wantDmg != nil {
				require.NotNil(t, r.Damage)
				got := *r.Damage
				got.Rolls = nil
				assert.Equal(t, *tt.wantDmg, got)
			} else {
				assert.Nil(t, r.Damage)
			}
			if tt.check != nil {
				tt.check(t, r)
			}
		})
	}
}

// A cast marks the chosen slot used and no other, even when two slots hold
// the same spell, and writes the list once.
func TestCast_ConsumesExactlyTheChosenSlot(t *testing.T) {
	f := newFixture(t)
	f.prepared(
		slot("alarm", "Alarm", 1),
		slot("fireball", "Fireball", 3),
		slot("fireball", "Fireball", 3),
		slot("haste", "Haste", 3),
	)
	// Fireball is 10d6 at level 15: ten distinct rolls.
	f.dice.vals = []int{1, 2, 3, 4, 5, 6, 1, 2, 3, 4}
	res, err := f.svc.Cast(context.Background(), charID, 2, castsvc.CastRequest{})
	require.NoError(t, err)

	require.Len(t, f.saved, 1, "the slot list is written once")
	used := []bool{}
	for _, s := range f.saved[0] {
		used = append(used, s.Used)
	}
	assert.Equal(t, []bool{false, false, true, false}, used)
	assert.Equal(t, used, usedFlags(res.Slots))
	assert.Empty(t, f.dice.vals, "every damage die was rolled once")
	require.Len(t, f.entries, 1, "one entry per cast")
}

func usedFlags(slots []spellsvc.SpellSlot) []bool {
	out := []bool{}
	for _, s := range slots {
		out = append(out, s.Used)
	}
	return out
}

func TestCast_SlotErrors(t *testing.T) {
	tests := []struct {
		name  string
		index int
		want  error
	}{
		{"used slot", 0, spellsvc.ErrSlotAlreadyUsed},
		{"negative", -1, spellsvc.ErrInvalidSlotIndex},
		{"past end", 1, spellsvc.ErrInvalidSlotIndex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			used := slot("fireball", "Fireball", 3)
			used.Used = true
			f.prepared(used)
			_, err := f.svc.Cast(context.Background(), charID, tt.index, castsvc.CastRequest{})
			assert.ErrorIs(t, err, tt.want)
			assert.Empty(t, f.saved)
			assert.Empty(t, f.entries)
		})
	}
}

// A party wizard casts Strength on the Bard: the duration follows their
// level, the gain is the Bard's class group die (rogue 1d6), capped at 18.
func TestReceive(t *testing.T) {
	tests := []struct {
		name     string
		req      castsvc.ReceiveRequest
		dice     []int
		wantBuff *buffsvc.BuffEffect
		textOnly castsvc.TextOnly
		wantErr  error
	}{
		{
			name: "strength from a level 9 wizard",
			req:  castsvc.ReceiveRequest{SpellID: "strength", CasterLevel: 9}, dice: []int{6},
			wantBuff: &buffsvc.BuffEffect{
				ID: buffsAdded, Name: "Strength", Duration: 540,
				Effects: buffsvc.BuffEffects{StrBonus: intPtr(6), StrMax: intPtr(18)},
			},
		},
		{
			name: "improved strength: rogue 1d6+4, max 23",
			req:  castsvc.ReceiveRequest{SpellID: "improved_strength", CasterLevel: 12}, dice: []int{2},
			wantBuff: &buffsvc.BuffEffect{
				ID: buffsAdded, Name: "Improved Strength", Duration: 12,
				Effects: buffsvc.BuffEffects{StrBonus: intPtr(6), StrMax: intPtr(23)},
			},
		},
		{
			name:     "haste received (area) lands on the Bard",
			req:      castsvc.ReceiveRequest{SpellID: "haste", CasterLevel: 7},
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Haste", Duration: 10},
		},
		{
			name: "stoneskin received at the caster's level",
			req:  castsvc.ReceiveRequest{SpellID: "stoneskin", CasterLevel: 10}, dice: []int{2},
			wantBuff: &buffsvc.BuffEffect{ID: buffsAdded, Name: "Stoneskin", Charges: 7},
		},
		{
			name: "a spell outside the effects seed (priest) is text only",
			req:  castsvc.ReceiveRequest{SpellID: "bless", CasterLevel: 5}, textOnly: castsvc.TextOnlyNoEffect,
		},
		{
			// The Priest spell shares its name with the Wizard one but has its own id: it gets no
			// effect (and no buff) from the Wizard spell's entry.
			name: "a Priest spell named like a Wizard spell is text only",
			req:  castsvc.ReceiveRequest{SpellID: "detect_magic_priest", CasterLevel: 9}, textOnly: castsvc.TextOnlyNoEffect,
		},
		{name: "self-only spell cannot be received", req: castsvc.ReceiveRequest{SpellID: "mirror_image", CasterLevel: 9}, wantErr: domain.ErrInvalidInput},
		{name: "fist of stone cannot be received", req: castsvc.ReceiveRequest{SpellID: "fist_of_stone", CasterLevel: 9}, wantErr: domain.ErrInvalidInput},
		{name: "unknown spell", req: castsvc.ReceiveRequest{SpellID: "no_such_spell", CasterLevel: 5}, wantErr: domain.ErrNotFound},
		{name: "no spell id", req: castsvc.ReceiveRequest{CasterLevel: 5}, wantErr: domain.ErrInvalidInput},
		{name: "caster level 0", req: castsvc.ReceiveRequest{SpellID: "strength"}, wantErr: domain.ErrInvalidInput},
		{name: "caster level 41", req: castsvc.ReceiveRequest{SpellID: "strength", CasterLevel: 41}, wantErr: domain.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.dice...)
			r, err := f.svc.Receive(context.Background(), charID, tt.req)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, f.entries)
				return
			}
			require.NoError(t, err)
			assert.Empty(t, f.dice.vals)
			assert.Equal(t, castsvc.SourceReceived, r.Source)
			assert.Equal(t, tt.req.CasterLevel, r.CasterLevel)
			assert.Nil(t, r.SlotIndex)
			assert.Equal(t, tt.textOnly, r.TextOnly)
			require.Len(t, f.entries, 1, "one log entry")
			if tt.wantBuff == nil {
				assert.Empty(t, f.added)
				assert.Empty(t, f.entries[0].Snapshots)
				return
			}
			require.Len(t, f.added, 1)
			assert.Equal(t, *tt.wantBuff, f.added[0])
			assert.Equal(t, []string{buffsvc.StateKeyBuffs}, keys(f.entries[0].Snapshots), "no slot used")
		})
	}
}

func TestReceive_LogPayload(t *testing.T) {
	f := newFixture(t, 4)
	_, err := f.svc.Receive(context.Background(), charID, castsvc.ReceiveRequest{SpellID: "strength", CasterLevel: 9})
	require.NoError(t, err)
	e := f.entries[0]
	assert.Empty(t, e.Summary, "the UI words the cast from its payload")
	var p map[string]any
	require.NoError(t, json.Unmarshal(e.Payload, &p))
	assert.NotContains(t, p, "note", "the catalog's English note stays out of the ledger")
	assert.Equal(t, map[string]any{
		"action": "received", "spellId": "strength", "spell": "Strength", "casterLevel": float64(9),
		"buff": map[string]any{"name": "Strength", "rounds": float64(540), "strGain": float64(4), "applied": true},
	}, p)
}

// The payload records the dice each damage instance threw and the save, so an older cast shows them after a reload.
func TestCast_LogPayloadHoldsTheDiceUsed(t *testing.T) {
	t.Run("fireball: ten d6 and the save", func(t *testing.T) {
		f := newFixture(t, 6, 5, 4, 3, 2, 1, 6, 5, 4, 3)
		f.prepared(slot("fireball", "Fireball", 3))
		_, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
		require.NoError(t, err)
		assert.Empty(t, f.dice.vals, "no die beyond the ten queued")
		var p map[string]any
		require.NoError(t, json.Unmarshal(f.entries[0].Payload, &p))
		assert.Equal(t, map[string]any{"type": "spell", "effect": "half"}, p["save"])
		dmg := p["damage"].(map[string]any)
		assert.Equal(t, "(min(level,10))d6", dmg["dice"])
		rolls := dmg["rolls"].([]any)
		require.Len(t, rolls, 1)
		roll := rolls[0].(map[string]any)
		assert.Equal(t, float64(39), roll["total"])
		var got []float64
		for _, d := range roll["dice"].([]any) {
			dm := d.(map[string]any)
			assert.Equal(t, float64(6), dm["sides"])
			got = append(got, dm["value"].(float64))
		}
		assert.Equal(t, []float64{6, 5, 4, 3, 2, 1, 6, 5, 4, 3}, got, "the values the roll threw, in order")
	})
	t.Run("magic missile: one roll per missile", func(t *testing.T) {
		f := newFixture(t, 1, 2, 3, 4, 1)
		f.prepared(slot("magic_missile", "Magic Missile", 1))
		_, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
		require.NoError(t, err)
		assert.JSONEq(t, `{"total":16,"type":"force","dice":"1d4+1","instances":5,"rounds":1,"rolls":[
			{"formula":"1d4+1","dice":[{"sides":4,"value":1}],"total":2},
			{"formula":"1d4+1","dice":[{"sides":4,"value":2}],"total":3},
			{"formula":"1d4+1","dice":[{"sides":4,"value":3}],"total":4},
			{"formula":"1d4+1","dice":[{"sides":4,"value":4}],"total":5},
			{"formula":"1d4+1","dice":[{"sides":4,"value":1}],"total":2}]}`,
			string(payloadField(t, f.entries[0].Payload, "damage")))
	})
}

func payloadField(t *testing.T, raw json.RawMessage, key string) json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &m))
	require.Contains(t, m, key)
	return m[key]
}

func TestCast_BuffErrorAborts(t *testing.T) {
	f := newFixture(t)
	boom := errors.New("boom")
	buffs := mocks.NewMockBuffs(t)
	buffs.EXPECT().Add(mock.Anything, charID, mock.Anything).Return(buffsvc.BuffEffect{}, actionlog.Snapshot{}, boom)
	chars := mocks.NewMockCharacters(t)
	chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{Level: bardLevel}, nil)
	svc, err := castsvc.New(castsvc.Deps{
		Slots: f.slots, Effects: catalog(t), Texts: f.texts, Buffs: buffs, Items: f.items, Attacks: f.attacks,
		Characters: chars, Dice: f.dice, Log: mocks.NewMockLog(t), Tx: f.tx,
	})
	require.NoError(t, err)
	f.prepared(slot("haste", "Haste", 3))
	_, err = svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
	assert.ErrorIs(t, err, boom, "the transaction rolls the slot back")
}

// The damage served for a spell is what a cast of it rolls: the same number
// of rolls (instances x rounds), each of the same dice, and the same constant.
func TestDamageAt_IsWhatACastRolls(t *testing.T) {
	tests := []struct {
		slot spellsvc.SpellSlot
		want spelleffectsvc.DamageAt
	}{
		{slot("fireball", "Fireball", 3), spelleffectsvc.DamageAt{
			CasterLevel: bardLevel, Dice: formula.Sum{Dice: []formula.Dice{{Count: 10, Sides: 6}}},
			Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "fire",
		}},
		{slot("magic_missile", "Magic Missile", 1), spelleffectsvc.DamageAt{
			CasterLevel: bardLevel, Dice: formula.Sum{Dice: []formula.Dice{{Count: 1, Sides: 4}}, Constant: 1},
			Instances: formula.Sum{Constant: 5}, Rounds: formula.Sum{Constant: 1}, Type: "force",
		}},
		{slot("melfs_acid_arrow", "Melf's Acid Arrow", 2), spelleffectsvc.DamageAt{
			CasterLevel: bardLevel, Dice: formula.Sum{Dice: []formula.Dice{{Count: 2, Sides: 4}}},
			Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 6}, Type: "acid",
		}},
		// ADR 0008 step 13: a damage made of parts serves each part, and a
		// cast rolls each part of each instance.
		{forceMissiles, spelleffectsvc.DamageAt{
			CasterLevel: bardLevel, Dice: formula.Sum{Dice: []formula.Dice{{Count: 2, Sides: 4}}, Constant: bardLevel},
			Instances: formula.Sum{Constant: 3}, Rounds: formula.Sum{Constant: 1}, Type: "force",
			Parts: []spelleffectsvc.DamagePartAt{
				{Dice: formula.Sum{Dice: []formula.Dice{{Count: 2, Sides: 4}}}, Save: spelleffectsvc.SaveNone},
				{Dice: formula.Sum{Constant: bardLevel}, Save: spelleffectsvc.SaveNegates},
			},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.slot.SpellID, func(t *testing.T) {
			f := newFixture(t)
			got, err := f.svc.DamageAt(context.Background(), tt.slot.SpellID, bardLevel)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, *got)

			// Every die shows 1: each roll totals its dice count plus the constant.
			perRoll := got.Dice.Constant
			for _, g := range got.Dice.Dice {
				perRoll += g.Count
			}
			rolls := got.Instances.Constant * got.Rounds.Constant
			f.dice.vals = make([]int, rolls*(perRoll-got.Dice.Constant))
			for i := range f.dice.vals {
				f.dice.vals[i] = 1
			}
			f.prepared(tt.slot)
			res, err := f.svc.Cast(context.Background(), charID, 0, castsvc.CastRequest{})
			require.NoError(t, err)
			require.Len(t, res.Damage.Rolls, rolls)
			for _, r := range res.Damage.Rolls {
				assert.Equal(t, perRoll, r.Total)
				require.Len(t, r.Parts, len(got.Parts), "one roll per served part")
				for i, p := range got.Parts {
					want := p.Dice.Constant
					for _, g := range p.Dice.Dice {
						want += g.Count
					}
					assert.Equal(t, want, r.Parts[i].Total, "part %d", i)
				}
			}
			assert.Empty(t, f.dice.vals, "every served die is rolled")
		})
	}
}

func TestDamageAt_NoneAndBounds(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.DamageAt(context.Background(), "alarm", bardLevel)
	require.NoError(t, err)
	assert.Nil(t, got, "a utility spell rolls no damage")
	got, err = f.svc.DamageAt(context.Background(), "wall_of_ice", bardLevel)
	require.NoError(t, err)
	assert.Nil(t, got, "a low-confidence spell rolls no damage (like cloudkill in the campaign seeds)")
	got, err = f.svc.DamageAt(context.Background(), "no_such_spell", bardLevel)
	require.NoError(t, err)
	assert.Nil(t, got, "no effect entry")
	for _, level := range []int{0, castsvc.MaxCasterLevel + 1} {
		_, err = f.svc.DamageAt(context.Background(), "fireball", level)
		require.ErrorIs(t, err, domain.ErrInvalidInput, "level %d", level)
	}
}
