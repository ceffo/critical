package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maxDice rolls the maximum on every die.
type maxDice struct{}

func (maxDice) RollDie(sides int) int { return sides }
func (maxDice) RollD20() int          { return 20 }
func (maxDice) RollD100() int         { return 100 }

func bootMaxDice(t *testing.T) *composition.BackingServices {
	t.Helper()
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:      filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir: filepath.Join("..", "..", "..", "seeds"),
		Dice:     maxDice{},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	_, err = bs.DB.ExecContext(ctx, `UPDATE characters SET str = 12 WHERE id = ?`, bs.CharacterID)
	require.NoError(t, err)
	return bs
}

// Strength received from a party wizard (rogue 1d6 = 6) takes STR 12 to 18:
// the Power Gem's "weapon becomes +5" applies on the next attack. Voiding the
// receive takes it away again.
func TestReceivedStrengthActivatesThePowerGem(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{
		Name:    "Power Gem",
		Effects: buffsvc.BuffEffects{MinStr: intPtr(18), WeaponBuff: intPtr(5), Multiplier: floatPtr(2)},
	})
	require.NoError(t, err)
	attack := combatsvc.AttackRequest{WeaponID: "mace", TargetAC: 5}

	before, err := bs.Combat.ResolveAttack(ctx, id, attack)
	require.NoError(t, err)
	assert.Equal(t, 1, before.WeaponBuff, "the mace's own +1 below STR 18")

	res, err := bs.Cast.Receive(ctx, id, castsvc.ReceiveRequest{SpellID: "strength", CasterLevel: 9})
	require.NoError(t, err)
	require.NotNil(t, res.Buff)
	assert.Equal(t, 540, res.Buff.Buff.Duration)

	str, err := bs.Buffs.GetEffectiveStr(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, buffsvc.Strength{Base: 12, Effective: 18}, str)
	after, err := bs.Combat.ResolveAttack(ctx, id, attack)
	require.NoError(t, err)
	assert.Equal(t, 5, after.WeaponBuff, "Power Gem: the weapon becomes +5")
	assert.InDelta(t, 2.0, after.Multipliers, 0.001)

	// Void the two attacks, then the receive: Strength is gone.
	_, err = bs.Log.Void(ctx, id, after.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, id, res.EntryID)
	require.NoError(t, err)
	str, err = bs.Buffs.GetEffectiveStr(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 12, str.Effective)
}

// Casting Stoneskin spends the slot and adds the layers in one entry;
// voiding it gives the slot back and removes the buff.
func TestCastIsOneEntryAndVoidRestoresSlotAndBuff(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.Prep.Prepare(ctx, id, []string{"stoneskin", "fireball"})
	require.NoError(t, err)

	res, err := bs.Cast.Cast(ctx, id, 1, castsvc.CastRequest{})
	require.NoError(t, err)
	require.NotNil(t, res.Buff)
	assert.Equal(t, 4+7, res.Buff.Buff.Charges, "1d4 + 15/2 layers")
	assert.Equal(t, 1, usedSlots(t, bs))
	buffs, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"Stoneskin"}, names(buffs))

	fire, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	require.NotNil(t, fire.Damage)
	assert.Equal(t, 60, fire.Damage.Total, "10d6 at level 15")

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 3, "prepare + two casts")
	assert.Equal(t, actionlog.KindCast, entries[0].Kind)

	_, err = bs.Log.Void(ctx, id, fire.EntryID)
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, id, res.EntryID)
	require.NoError(t, err)
	assert.Equal(t, 0, usedSlots(t, bs))
	buffs, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, buffs)
}
