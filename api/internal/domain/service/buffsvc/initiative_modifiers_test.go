package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// ADR 0035 item 6 (with ADR 0033 step 15.1's weapon rule, DMG "Magical
// Weapon Speeds"): the speed factor lowered by the weapon's plus, the lesser
// of its to-hit and damage magic, or by the "weapon becomes +N" when higher;
// never below 0; a cursed weapon is not slowed.
func TestWeaponSpeed_ADR0035Item6_LoweredByThePlusNeverBelowZero(t *testing.T) {
	tests := []struct {
		name       string
		w          buffsvc.HeldWeapon
		weaponBuff int
		want       int
	}{
		{"plain long sword", buffsvc.HeldWeapon{Speed: 5}, 0, 5},
		{"+1/+2: the lesser plus", buffsvc.HeldWeapon{Speed: 5, MagicToHit: 1, MagicDamage: 2}, 0, 4},
		{"+2/+1: the lesser plus", buffsvc.HeldWeapon{Speed: 5, MagicToHit: 2, MagicDamage: 1}, 0, 4},
		{"to hit only: no plus on damage, the lesser is 0", buffsvc.HeldWeapon{Speed: 5, MagicToHit: 3}, 0, 5},
		{"the weapon becomes +3, higher than its +1", buffsvc.HeldWeapon{Speed: 5, MagicToHit: 1, MagicDamage: 1}, 3, 2},
		{"its own +2 beats a +1 buff", buffsvc.HeldWeapon{Speed: 5, MagicToHit: 2, MagicDamage: 2}, 1, 3},
		{"never below 0", buffsvc.HeldWeapon{Speed: 2, MagicToHit: 3, MagicDamage: 3}, 0, 0},
		{"a cursed -1 does not slow it", buffsvc.HeldWeapon{Speed: 5, MagicToHit: -1, MagicDamage: -1}, 0, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buffsvc.WeaponSpeed(tt.w, tt.weaponBuff))
		})
	}
}

// ADR 0035 item 6.2 with ADR 0033 step 15.1: a spell's casting time in
// segments at the caster level; a round or more adds nothing and reads end
// of round; a special one adds nothing.
func TestSpellInitiative_ADR0035Item6_2_CastingTimeInSegments(t *testing.T) {
	tests := []struct {
		raw    string
		level  int
		value  int
		timing buffsvc.CastTiming
	}{
		{"1", 5, 1, buffsvc.TimingSegments},
		{"3", 5, 3, buffsvc.TimingSegments},
		{"9", 5, 9, buffsvc.TimingSegments},
		{"10", 5, 0, buffsvc.TimingEndOfRound},
		{"1 rd.", 5, 0, buffsvc.TimingEndOfRound},
		{"1 turn", 5, 0, buffsvc.TimingEndOfRound},
		{"Special", 5, 0, buffsvc.TimingSpecial},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got := buffsvc.SpellInitiative("x", "X", spellstats.ParseCastingTime(tt.raw), tt.level)
			assert.Equal(t, buffsvc.InitiativeModifier{
				Source: buffsvc.InitiativeSpell, Value: tt.value, SpellID: "x", Name: "X", Timing: tt.timing,
			}, got)
		})
	}
}

// advanceWith sets up a Next Round from round 5 to 6 with buffs (each counted
// down by one), the character's powers, the weapon in hand (nil: none) and
// a d10 showing d10.
func advanceWith(f *fixture, buffs []buffsvc.BuffEffect, powers []buffsvc.ItemPower, held *buffsvc.HeldWeapon, d10 int) {
	f.state(buffs, 5)
	f.records()
	f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).Return(nil).Once()
	f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 6).Return(nil).Once()
	f.initiativeIs(nil) // the advance: no initiative to end
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{ID: charID, Str: 12}, nil).Once()
	f.items.EXPECT().ItemPowers(mock.Anything, charID).Return(powers, nil).Once()
	if held != nil {
		f.weapons.EXPECT().WeaponInHand(mock.Anything, charID).Return(*held, true, nil).Once()
	} else {
		f.weapons.EXPECT().WeaponInHand(mock.Anything, charID).Return(buffsvc.HeldWeapon{}, false, nil).Once()
	}
	f.initiativeIs(nil)
	f.dice.EXPECT().RollDie(10).Return(d10).Once()
	f.records()
	f.savesInitiative()
}

// ADR 0035 item 6 (amending ADR 0013 step 3): Next Round's initiative is the
// one d10 plus the weapon in hand's speed and the initiative effects of the
// buffs that apply and the powers that are on, each a named modifier in the
// entry's payload and the stored initiative. An effect conditioned on a
// weapon kind, or gated by a Strength the character lacks, does not count.
func TestInitiative_ADR0035Item6_NextRoundAddsTheWeaponSpeedAndEffects(t *testing.T) {
	haste := buffsvc.BuffEffect{ID: "h", Name: "Haste", SpellID: "haste", Duration: 9, Effects: buffsvc.BuffEffects{Initiative: intPtr(-2)}}
	meleeOnly := buffsvc.BuffEffect{ID: "m", Name: "Melee drill", Duration: 9, Effects: buffsvc.BuffEffects{Initiative: intPtr(-1), WeaponKind: buffsvc.WeaponMelee}}
	strong := buffsvc.BuffEffect{ID: "s", Name: "Needs 18", Duration: 9, Effects: buffsvc.BuffEffects{Initiative: intPtr(-4), MinStr: intPtr(18)}}
	weaponPlus := buffsvc.BuffEffect{ID: "w", Name: "Gem", Duration: 9, Effects: buffsvc.BuffEffects{WeaponBuff: intPtr(2), WeaponKind: buffsvc.WeaponMelee}}
	boots := buffsvc.ItemPower{ItemID: "boots", ItemName: "Boots of speed", PowerID: "p", Effects: buffsvc.BuffEffects{Initiative: intPtr(-1)}}
	sword := buffsvc.HeldWeapon{ItemID: "sword", Name: "Long sword", Kind: buffsvc.WeaponMelee, Speed: 5}

	t.Run("weapon, buff and power", func(t *testing.T) {
		f := newFixture(t)
		advanceWith(f, []buffsvc.BuffEffect{haste, meleeOnly, strong, weaponPlus}, []buffsvc.ItemPower{boots}, &sword, 6)

		_, _, _, got, err := f.svc.AdvanceRound(context.Background(), charID)
		require.NoError(t, err)
		want := []buffsvc.InitiativeModifier{
			// Speed 5 lowered by the melee "weapon becomes +2" (its condition
			// is the sword's kind): 3.
			{Source: buffsvc.InitiativeWeapon, Value: 3, ItemID: "sword", Name: "Long sword"},
			{Source: buffsvc.InitiativeEffect, Value: -2, Name: "Haste", SpellID: "haste"},
			{Source: buffsvc.InitiativeEffect, Value: -1, ItemID: "boots", Name: "Boots of speed"},
		}
		assert.Equal(t, buffsvc.Initiative{D10: 6, Modifiers: want, LogEntryID: 2}, got)
		assert.Equal(t, 6, got.Total(), "6 + 3 - 2 - 1")
		f.dice.AssertNumberOfCalls(t, "RollDie", 1)
		assert.Equal(t, &got, f.initiatives[0], "stored with its modifiers")
		assert.Equal(t, map[string]any{
			"round": 6.0, "d10": 6.0, "total": 6.0,
			"modifiers": []any{
				map[string]any{"source": "weapon", "value": 3.0, "itemId": "sword", "name": "Long sword"},
				map[string]any{"source": "effect", "value": -2.0, "name": "Haste", "spellId": "haste"},
				map[string]any{"source": "effect", "value": -1.0, "itemId": "boots", "name": "Boots of speed"},
			},
		}, initiativePayload(t, f.entries[1]))
		assert.JSONEq(t, `{"d10":6,"modifiers":[
			{"source":"weapon","value":3,"itemId":"sword","name":"Long sword"},
			{"source":"effect","value":-2,"name":"Haste","spellId":"haste"},
			{"source":"effect","value":-1,"itemId":"boots","name":"Boots of speed"}]}`,
			snaptest.State(t, f.entries[1].Snapshots[0].After), "undo and redo restore the modifiers")
	})

	t.Run("no weapon in hand adds nothing", func(t *testing.T) {
		f := newFixture(t)
		advanceWith(f, nil, nil, nil, 4)

		_, _, _, got, err := f.svc.AdvanceRound(context.Background(), charID)
		require.NoError(t, err)
		assert.Nil(t, got.Modifiers)
		assert.Equal(t, 4, got.Total())
	})

	t.Run("a buff that expires with this advance does not count", func(t *testing.T) {
		f := newFixture(t)
		ending := buffsvc.BuffEffect{ID: "e", Name: "Ending", Duration: 1, Effects: buffsvc.BuffEffects{Initiative: intPtr(-2)}}
		advanceWith(f, []buffsvc.BuffEffect{ending}, nil, nil, 4)

		_, _, _, got, err := f.svc.AdvanceRound(context.Background(), charID)
		require.NoError(t, err)
		assert.Equal(t, 4, got.Total())
	})
}

// ADR 0035 item 6.4 (ADR 0013 step 4): a reroll rolls one new d10 and keeps
// the replaced initiative's modifiers.
func TestInitiative_ADR0035Item6_4_RerollKeepsTheModifiers(t *testing.T) {
	mods := []buffsvc.InitiativeModifier{{Source: buffsvc.InitiativeWeapon, Value: 4, ItemID: "sword", Name: "Long sword"}}
	current := &buffsvc.Initiative{D10: 4, Modifiers: mods, LogEntryID: 12}
	f := newFixture(t)
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(3, nil).Once()
	f.initiativeIs(current)
	f.log.EXPECT().TopID(mock.Anything, charID).Return(12, true, nil).Once()
	f.initiativeIs(current)
	f.dice.EXPECT().RollDie(10).Return(9).Once()
	f.records()
	f.savesInitiative()

	got, err := f.svc.RerollInitiative(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, buffsvc.Initiative{D10: 9, Modifiers: mods, LogEntryID: 1}, got)
	assert.Equal(t, 13, got.Total())
	f.dice.AssertNumberOfCalls(t, "RollDie", 1)
	assert.JSONEq(t, `{"d10":4,"entryId":12,"modifiers":[{"source":"weapon","value":4,"itemId":"sword","name":"Long sword"}]}`,
		snaptest.State(t, f.entries[0].Snapshots[0].Before), "undo gives the replaced roll back, modifiers and all")
}

// ADR 0035 item 6.2: a spell cast or an item used replaces the round's
// initiative by the same d10 plus the cause's term and the effects recorded
// at the roll; the weapon's speed (or an earlier cause) goes. No die is
// rolled. One initiative_correct entry stamped with the round, its snapshot
// keeping the replaced initiative, and the new one stored with its id.
func TestInitiative_ADR0035Item6_2_ACastOrAnItemUsedCorrectsTheRound(t *testing.T) {
	ctx := context.Background()
	haste := buffsvc.InitiativeModifier{Source: buffsvc.InitiativeEffect, Value: -2, Name: "Haste", SpellID: "haste"}
	rolled := &buffsvc.Initiative{D10: 6, LogEntryID: 40, Modifiers: []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeWeapon, Value: 5, ItemID: "sword", Name: "Long sword"}, haste,
	}}
	missile := buffsvc.SpellInitiative("magic_missile", "Magic Missile", spellstats.ParseCastingTime("1"), 5)

	t.Run("a spell: same d10, its casting time, the effects kept", func(t *testing.T) {
		f := newFixture(t) // the dice mock fails on any roll
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(2, nil).Once()
		f.initiativeIs(rolled)
		f.records()
		f.savesInitiative()

		got, err := f.svc.CorrectInitiative(ctx, charID, missile)
		require.NoError(t, err)
		want := buffsvc.Initiative{D10: 6, LogEntryID: 1, Modifiers: []buffsvc.InitiativeModifier{missile, haste}}
		assert.Equal(t, &want, got)
		assert.Equal(t, 5, got.Total(), "6 + 1 - 2")
		assert.Equal(t, &want, f.initiatives[0])
		assert.Equal(t, []string{"record", "save initiative"}, f.calls)
		e := f.entries[0]
		assert.Equal(t, actionlog.KindInitiativeCorrect, e.Kind)
		assert.Equal(t, 2, e.Round)
		assert.Empty(t, e.Summary)
		assert.Equal(t, map[string]any{
			"round": 2.0, "d10": 6.0, "before": 9.0, "after": 5.0,
			"modifiers": []any{
				map[string]any{"source": "spell", "value": 1.0, "spellId": "magic_missile", "name": "Magic Missile"},
				map[string]any{"source": "effect", "value": -2.0, "name": "Haste", "spellId": "haste"},
			},
			"cause": map[string]any{"source": "spell", "value": 1.0, "spellId": "magic_missile", "name": "Magic Missile"},
		}, payload(t, e))
		require.Equal(t, []string{buffsvc.StateKeyInitiative}, snapKeys(e))
		assert.JSONEq(t, `{"d10":6,"entryId":40,"modifiers":[
			{"source":"weapon","value":5,"itemId":"sword","name":"Long sword"},
			{"source":"effect","value":-2,"name":"Haste","spellId":"haste"}]}`,
			snaptest.State(t, e.Snapshots[0].Before), "undo gives the rolled initiative back")
		assert.JSONEq(t, `{"d10":6,"modifiers":[
			{"source":"spell","value":1,"spellId":"magic_missile","name":"Magic Missile"},
			{"source":"effect","value":-2,"name":"Haste","spellId":"haste"}]}`,
			snaptest.State(t, e.Snapshots[0].After), "redo names the redone entry (replay)")
	})

	t.Run("a second cause replaces the first", func(t *testing.T) {
		f := newFixture(t)
		corrected := &buffsvc.Initiative{D10: 6, LogEntryID: 41, Modifiers: []buffsvc.InitiativeModifier{missile, haste}}
		wand := buffsvc.ItemInitiative("wand", "Wand of frost", "wand", 3)
		f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(2, nil).Once()
		f.initiativeIs(corrected)
		f.records()
		f.savesInitiative()

		got, err := f.svc.CorrectInitiative(ctx, charID, wand)
		require.NoError(t, err)
		assert.Equal(t, []buffsvc.InitiativeModifier{wand, haste}, got.Modifiers)
		assert.Equal(t, 7, got.Total())
		assert.Equal(t, 5.0, payload(t, f.entries[0])["before"])
	})

	for _, tt := range []struct {
		name  string
		round int
		cur   *buffsvc.Initiative
	}{
		{"out of combat", 0, nil},
		{"round 0 with a stale initiative", 0, rolled},
		{"no initiative", 3, nil},
	} {
		t.Run(tt.name+": nothing", func(t *testing.T) {
			f := newFixture(t) // no record, no write expected
			f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(tt.round, nil).Once()
			f.initiativeIs(tt.cur)
			got, err := f.svc.CorrectInitiative(ctx, charID, missile)
			require.NoError(t, err)
			assert.Nil(t, got)
		})
	}

	t.Run("only a spell or an item corrects", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.svc.CorrectInitiative(ctx, charID, buffsvc.InitiativeModifier{Source: buffsvc.InitiativeWeapon, Value: 1})
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})
}

// Snapshots recorded before the modifiers restore a plain d10; new ones
// restore the modifiers too (ADR 0013 steps 2 and 4: undo and redo).
func TestInitiativeRestorer_ADR0035Item6_RestoresTheModifiers(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.dal.EXPECT().SaveInitiative(mock.Anything, charID, &buffsvc.Initiative{D10: 4, LogEntryID: 12}).Return(nil).Once()
	require.NoError(t, f.svc.Module().Restorer(buffsvc.StateKeyInitiative).Restore(ctx, charID, snaptest.Env(`{"d10":4,"entryId":12}`)))
	f.dal.EXPECT().SaveInitiative(mock.Anything, charID, &buffsvc.Initiative{D10: 4, LogEntryID: 12, Modifiers: []buffsvc.InitiativeModifier{
		{Source: buffsvc.InitiativeSpell, SpellID: "fly", Name: "Fly", Timing: buffsvc.TimingEndOfRound},
	}}).Return(nil).Once()
	require.NoError(t, f.svc.Module().Restorer(buffsvc.StateKeyInitiative).Restore(ctx, charID,
		snaptest.Env(`{"d10":4,"entryId":12,"modifiers":[{"source":"spell","value":0,"spellId":"fly","name":"Fly","timing":"end_of_round"}]}`)))
}
