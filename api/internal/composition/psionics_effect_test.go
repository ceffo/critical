package composition_test

// ADR 0043 step 17 end to end on the fixture catalogue: a power's effect read
// from the artifact moves the derived numbers while its row lasts.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
)

var psychometabolist = []psionics.Pick{
	{Kind: psionics.PickDiscipline, ID: "psychometabolism"},
	{Kind: psionics.PickPower, ID: "displacement"},
	{Kind: psionics.PickPower, ID: "graft_weapon"},
}

// derivedAC is the character's derived AC against melee.
func derivedAC(t *testing.T, bs *composition.BackingServices, id int64) int {
	t.Helper()
	a, err := bs.AC.Get(context.Background(), id)
	require.NoError(t, err)
	return a.Value
}

// Displacement (Con -3, 6 PSPs, 3 a round; the fixture's effect ac -2) on
// the thief/psionicist (Con 16, 181 PSPs), every d20 a 5: the derived AC is
// 2 better from the check, through a paid Next Round, and back once the row
// goes (Stop, the check undone, an unpaid Next Round). Graft Weapon's +1 to
// hit and damage (melee only) shows on a melee attack's preview, not on a
// missile one's.
func TestMaintenance_Step17_TheEffectMovesTheDerivedAC(t *testing.T) {
	ctx := context.Background()
	use := func(t *testing.T, bs *composition.BackingServices, id int64, power string) {
		t.Helper()
		res, err := bs.Psionics.Use(ctx, id, power, psionicsvc.UseRequest{})
		require.NoError(t, err)
		require.True(t, res.Check.Success)
		require.NotEmpty(t, res.BuffID)
	}
	nextRound := func(t *testing.T, bs *composition.BackingServices, id int64) {
		t.Helper()
		_, _, _, _, _, err := bs.Buffs.AdvanceRound(ctx, id)
		require.NoError(t, err)
	}

	t.Run("kept after a paid Next Round, back after Stop", func(t *testing.T) {
		bs, id := thiefPsionicistRolling(t, psychometabolist, fives(t))
		base := derivedAC(t, bs, id)
		use(t, bs, id, "displacement")
		assert.Equal(t, base-2, derivedAC(t, bs, id), "after the check")
		nextRound(t, bs, id) // the activation round: free
		nextRound(t, bs, id)
		require.Equal(t, 181-6-3, currentPSP(t, bs, id), "the maintenance paid")
		assert.Equal(t, base-2, derivedAC(t, bs, id), "kept while maintained")

		_, err := bs.Psionics.Stop(ctx, id, "displacement")
		require.NoError(t, err)
		assert.Equal(t, base, derivedAC(t, bs, id), "back after Stop")
	})
	t.Run("back after the check is undone", func(t *testing.T) {
		bs, id := thiefPsionicistRolling(t, psychometabolist, fives(t))
		base := derivedAC(t, bs, id)
		use(t, bs, id, "displacement")
		require.Equal(t, base-2, derivedAC(t, bs, id))
		undoTop(t, bs, id)
		assert.Equal(t, base, derivedAC(t, bs, id))
		redoTop(t, bs, id)
		assert.Equal(t, base-2, derivedAC(t, bs, id), "the redo puts it back")
	})
	t.Run("back when unpaid maintenance ends the row", func(t *testing.T) {
		bs, id := thiefPsionicistRolling(t, psychometabolist, fives(t))
		base := derivedAC(t, bs, id)
		use(t, bs, id, "displacement")
		nextRound(t, bs, id) // the activation round: free
		spendPSP(t, bs, id, 173)
		require.Equal(t, 2, currentPSP(t, bs, id))
		nextRound(t, bs, id)
		assert.Empty(t, maintainedRows(t, bs.Buffs, id), "3 PSPs unpaid")
		assert.Equal(t, base, derivedAC(t, bs, id))
	})
	t.Run("used twice while maintained: one row, not a doubled bonus", func(t *testing.T) {
		bs, id := thiefPsionicistRolling(t, psychometabolist, fives(t))
		base := derivedAC(t, bs, id)
		use(t, bs, id, "displacement")
		use(t, bs, id, "displacement")
		assert.Len(t, maintainedRows(t, bs.Buffs, id), 1, "the second use renews the row")
		assert.Equal(t, base-2, derivedAC(t, bs, id), "AC -2, not -4")
	})
	t.Run("beside a spell's AC buff: two rows, the power's named by its id", func(t *testing.T) {
		bs, id := thiefPsionicistRolling(t, psychometabolist, fives(t))
		base := derivedAC(t, bs, id)
		_, _, err := bs.Buffs.Add(ctx, id, buffsvc.BuffEffect{Name: "Shield", SpellID: "shield", Effects: buffsvc.BuffEffects{AC: new(-1)}})
		require.NoError(t, err)
		use(t, bs, id, "displacement")
		a, err := bs.AC.Get(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, base-3, a.Value)
		assert.Contains(t, a.Rows, acsvc.Row{Term: acsvc.TermBuffs, Buff: "Shield", SpellID: "shield", Value: -1})
		assert.Contains(t, a.Rows, acsvc.Row{Term: acsvc.TermBuffs, Buff: "Displacement", PowerID: "displacement", Value: -2})
	})
	t.Run("a long rest ends the row and its bonus", func(t *testing.T) {
		bs, id := thiefPsionicistRolling(t, psychometabolist, fives(t))
		base := derivedAC(t, bs, id)
		use(t, bs, id, "displacement")
		_, err := bs.Rest.LongRest(ctx, id)
		require.NoError(t, err)
		assert.Empty(t, maintainedRows(t, bs.Buffs, id))
		assert.Equal(t, base, derivedAC(t, bs, id))
	})
	t.Run("graft weapon on a melee preview, not a missile one", func(t *testing.T) {
		bs, id := thiefPsionicistRolling(t, psychometabolist, fives(t))
		add := func(input itemsvc.ItemInput) string {
			t.Helper()
			res, err := bs.Items.Create(ctx, id, input)
			require.NoError(t, err)
			return res.Item.ID
		}
		// preview wields the item alone and reads its attack preview.
		preview := func(itemID string) combatsvc.AttackPreview {
			t.Helper()
			_, err := bs.Items.SetState(ctx, id, itemID, itemsvc.StateWielded, true)
			require.NoError(t, err)
			view, err := bs.Combat.UpdateAttackForm(ctx, id, combatsvc.AttackFormPatch{WeaponID: &itemID, TargetAC: new(5)})
			require.NoError(t, err)
			require.NotNil(t, view.Preview)
			return *view.Preview
		}
		darts := newWeapon("Darts", "1d3", "1d2")
		darts.WeaponKind = "missile"
		dagger, dart := add(newWeapon("Dagger", "1d4", "1d3")), add(darts)
		meleeBefore, missileBefore := preview(dagger), preview(dart)
		use(t, bs, id, "graft_weapon")

		melee := preview(dagger)
		assert.Equal(t, meleeBefore.ToHit+1, melee.ToHit, "+1 to hit")
		assert.Equal(t, meleeBefore.EffectiveThac0-1, melee.EffectiveThac0)
		assert.Equal(t, meleeBefore.DamageBonus+1, melee.DamageBonus, "+1 damage")
		assert.Contains(t, melee.Modifiers, combatsvc.Modifier{Kind: combatsvc.ModBuff, Detail: "Graft Weapon", Value: 1})

		missile := preview(dart)
		assert.Equal(t, missileBefore, missile, "the effect is melee only")
	})
}
