package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The attack form on the real stack: the choices survive a restart (they
// are in the database, not the client) and the preview's effective THAC0 is
// the one the attack with the same choices uses.
func TestAttackForm_PersistsAcrossRestartAndMatchesTheAttack(t *testing.T) {
	ctx := context.Background()
	dsn := filepath.Join(t.TempDir(), "critical.db")
	seeds := filepath.Join("..", "..", "testdata", "seeds")
	open := func(dice composition.Dice) *composition.BackingServices {
		bs, err := composition.New(ctx, composition.Config{DSN: dsn, SeedsDir: seeds, Dice: dice})
		require.NoError(t, err)
		return bs
	}

	bs := open(newScriptedDice(t))
	v, err := bs.Combat.GetAttackForm(ctx, bs.CharacterID)
	require.NoError(t, err)
	held := v.Form.WeaponID
	require.NotEmpty(t, held)

	bow, ammo, ac, size := "shortbow", "arrows_plus3", -1, combatsvc.SizeLarge
	_, err = bs.Combat.UpdateAttackForm(ctx, bs.CharacterID, combatsvc.AttackFormPatch{WeaponID: &bow})
	require.NoError(t, err)
	_, err = bs.Combat.UpdateAttackForm(ctx, bs.CharacterID, combatsvc.AttackFormPatch{
		AmmoItemID: &ammo, TargetAC: &ac, TargetSize: &size, Situational: &combatsvc.Situational{Value: -2, Label: "cover"},
	})
	require.NoError(t, err)
	require.NoError(t, bs.Close(ctx))

	dice := newScriptedDice(t)
	bs = open(dice)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	v, err = bs.Combat.GetAttackForm(ctx, bs.CharacterID)
	require.NoError(t, err)
	assert.Equal(t, combatsvc.AttackForm{
		WeaponID: "shortbow", PreviousWeaponID: held, AmmoItemID: "arrows_plus3", TargetAC: -1,
		TargetSize: combatsvc.SizeLarge, Situational: combatsvc.Situational{Value: -2, Label: "cover"},
	}, v.Form)
	assert.False(t, v.AmmoFromDefault)

	dice.expect(roll{"d20", 12}, roll{"d100", 40}, roll{"d100", 70}, roll{"d6", 3})
	res, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{
		WeaponID: v.Form.WeaponID, AmmoItemID: v.Form.AmmoItemID, TargetAC: v.Form.TargetAC, TargetSize: v.Form.TargetSize,
		Situational: &v.Form.Situational,
	})
	require.NoError(t, err)
	assert.Equal(t, res.Modifiers, v.Preview.Modifiers)
	assert.Equal(t, res.EffectiveThac0, v.Preview.EffectiveThac0)

	// The spent arrow shows in the next view's ammunition counts.
	after, err := bs.Combat.GetAttackForm(ctx, bs.CharacterID)
	require.NoError(t, err)
	for _, c := range after.AmmoChoices {
		if c.ItemID == "arrows_plus3" {
			assert.Equal(t, res.Ammo.Remaining, c.Count)
		}
	}
}
