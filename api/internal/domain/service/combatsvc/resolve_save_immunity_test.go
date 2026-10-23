package combatsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
)

// withImmunities is the test bard with Wisdom wis and the spell immunities
// Table 5 gives it (derived by charactersvc, ADR 0006 step 10.1).
func withImmunities(wis int, ids ...string) charactersvc.View {
	c := withWisdom("fighter", wis, 4)
	c.SpellImmunities = ids
	return c
}

// ADR 0006 step 10.2: a save against a spell the character is immune to rolls
// no d20 (the dice mock has no expectation: a roll fails the test), reads no
// buff, passes and names the immunity (source wisdom, the score); it is logged
// as a save with the spell. Wisdom 18 (no immunity) and Wisdom 19 against hold
// person roll as usual; Wisdom 20 is immune to hold person.
func TestResolveSavingThrow_Step10_WisdomSpellImmunity(t *testing.T) {
	at19 := []string{"charm_person", "command", "friends", "hypnotism"}
	at20 := append(append([]string{}, at19...), "forget", "hold_person", "hold_person_priest", "ray_of_enfeeblement", "scare")
	for _, c := range []struct {
		name   string
		char   charactersvc.View
		spell  string
		immune bool
	}{
		{"Wisdom 18 against charm person rolls", withImmunities(18), "charm_person", false},
		{"Wisdom 19 is immune to charm person", withImmunities(19, at19...), "charm_person", true},
		{"Wisdom 19 against hold person rolls", withImmunities(19, at19...), "hold_person", false},
		{"Wisdom 20 is immune to hold person", withImmunities(20, at20...), "hold_person", true},
		{"Wisdom 20 with no spell named rolls", withImmunities(20, at20...), "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFixture(t)
			f.withChar(c.char)
			if !c.immune {
				f.dice.EXPECT().RollD20().Return(14).Once()
				f.buffs.EXPECT().Aggregate(mock.Anything, charID, buffsvc.BaseStrengthOf(c.char), buffsvc.WeaponNone).Return(strMods(12), nil).Once()
			}
			f.records()

			res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell, combatsvc.SaveOptions{SpellID: c.spell})
			require.NoError(t, err)
			require.Len(t, f.entries, 1)
			assert.Equal(t, actionlog.KindSave, f.entries[0].Kind)
			p := payloadOf(t, f.entries[0].Payload)
			if !c.immune {
				assert.Nil(t, res.Immunity)
				assert.Equal(t, 14, res.D20Roll)
				assert.Nil(t, p["immunity"])
				return
			}
			assert.Equal(t, &combatsvc.Immunity{Source: "wisdom", Score: c.char.Wis}, res.Immunity)
			assert.Equal(t, 0, res.D20Roll, "no roll")
			assert.Empty(t, res.Modifiers)
			assert.True(t, res.Passed)
			assert.True(t, res.RolledPassed)
			assert.Equal(t, c.spell, res.SpellID)
			assert.Equal(t, map[string]any{"source": "wisdom", "score": float64(c.char.Wis)}, p["immunity"])
			assert.Equal(t, c.spell, p["spellId"])
			// No roll, so no roll side; the adjusted target is the target.
			require.NotZero(t, res.Target, "the fixture has a target")
			assert.Equal(t, res.Target, res.AdjustedTarget)
			assert.Equal(t, float64(res.Target), p["adjustedTarget"])
			assert.Zero(t, res.RollTotal)
			assert.Zero(t, res.Total)
			assert.Zero(t, res.Margin)
		})
	}
}

// ADR 0006 step 10.2: an immune save has no roll for the DM's adjustment to
// correct: adjusting it is refused (ErrSaveImmune, 409 save_immune) and nothing
// is amended.
func TestAdjustSave_Step10_AnImmuneSaveIsRefused(t *testing.T) {
	f := newFixture(t)
	f.withChar(withImmunities(19, "charm_person"))
	f.records()
	_, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell, combatsvc.SaveOptions{SpellID: "charm_person"})
	require.NoError(t, err)
	e := &actionlog.Entry{ID: 42, CharacterID: charID, Kind: f.entries[0].Kind, Payload: f.entries[0].Payload}
	amends(f, e)

	_, err = f.svc.AdjustSave(context.Background(), charID, 42, combatsvc.SaveAdjustment{Value: 2})
	require.Error(t, err)
	assert.True(t, errors.Is(err, combatsvc.ErrSaveImmune))
	assert.JSONEq(t, string(f.entries[0].Payload), string(e.EffectivePayload()), "nothing amended")
}
