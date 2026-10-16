package composition_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
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
		DSN:       filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir:  filepath.Join("..", "..", "testdata", "seeds"),
		Catalogue: filepath.Join("..", "..", "testdata", "catalogue.json"),
		Dice:      maxDice{},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	_, err = bs.DB.ExecContext(ctx, `UPDATE characters SET str = 12 WHERE id = ?`, bs.CharacterID)
	require.NoError(t, err)
	return bs
}

// Strength received from a party wizard (rogue 1d6 = 6) takes STR 12 to 18:
// the seeded Power Gem's thunder hammer ("weapon becomes +5", an item power)
// applies on the next melee attack. Voiding the receive takes it away again.
func TestReceivedStrengthActivatesThePowerGem(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	attack := combatsvc.AttackRequest{WeaponID: "mace", TargetAC: new(5)}

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
	assert.InDelta(t, 2.0, after.DiceMultiplier, 0.001)

	// Void the two attacks, then the receive: Strength is gone.
	_, err = bs.Log.Undo(ctx, id, after.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Log.Undo(ctx, id, res.EntryID)
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

	_, err = bs.Log.Undo(ctx, id, fire.EntryID)
	require.NoError(t, err)
	_, err = bs.Log.Undo(ctx, id, res.EntryID)
	require.NoError(t, err)
	assert.Equal(t, 0, usedSlots(t, bs))
	buffs, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, buffs)
}

// Checklist 1: a long rest is logged with the slot list, so voiding a cast
// from before the rest is refused instead of re-marking slots used.
func TestLongRestBlocksVoidingAnEarlierCast(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.Prep.Prepare(ctx, id, []string{"fireball", "haste"})
	require.NoError(t, err)
	_, err = bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{}) // Fireball, used before
	require.NoError(t, err)
	haste, err := bs.Cast.Cast(ctx, id, 1, castsvc.CastRequest{})
	require.NoError(t, err)

	rest, err := bs.Rest.LongRest(ctx, id)
	require.NoError(t, err)
	require.NotZero(t, rest.EntryID)
	require.Equal(t, 0, usedSlots(t, bs))

	_, err = bs.Log.Undo(ctx, id, haste.EntryID)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
	assert.Contains(t, err.Error(), fmt.Sprintf("the newest entry is #%d (long_rest", rest.EntryID))
	assert.Equal(t, 0, usedSlots(t, bs), "the slots stay as the rest left them")

	// Voiding the rest itself puts the slots back as they were before it.
	_, err = bs.Log.Undo(ctx, id, rest.EntryID)
	require.NoError(t, err)
	assert.Equal(t, 2, usedSlots(t, bs))
}

// Checklist 3: casts touch the same slot list, so voids go newest first.
func TestCastVoidsGoNewestFirst(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.Prep.Prepare(ctx, id, []string{"magic_missile", "fireball"})
	require.NoError(t, err)
	fire, err := bs.Cast.Cast(ctx, id, 1, castsvc.CastRequest{})
	require.NoError(t, err)
	mm, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Equal(t, 25, mm.Damage.Total, "5 missiles of 1d4+1 at max")

	_, err = bs.Log.Undo(ctx, id, fire.EntryID)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged)
	assert.Contains(t, err.Error(), fmt.Sprintf("the newest entry is #%d (cast", mm.EntryID))

	_, err = bs.Log.Undo(ctx, id, mm.EntryID)
	require.NoError(t, err)
	_, err = bs.Log.Undo(ctx, id, fire.EntryID)
	require.NoError(t, err)
	assert.Equal(t, 0, usedSlots(t, bs))
}

// Checklist 5: Stoneskin on an ally rolls the layers, spends the slot, puts
// no buff on the Bard; voiding gives the slot back.
func TestStoneskinOnAnAlly(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.Prep.Prepare(ctx, id, []string{"stoneskin"})
	require.NoError(t, err)
	onAlly := false

	res, err := bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{OnSelf: &onAlly})
	require.NoError(t, err)
	assert.Equal(t, castsvc.TextOnlyNotOnSelf, res.TextOnly)
	require.NotNil(t, res.Buff)
	assert.False(t, res.Buff.Applied)
	assert.Equal(t, 11, res.Buff.Buff.Charges)
	assert.Equal(t, 1, usedSlots(t, bs))
	buffs, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, buffs)

	_, err = bs.Log.Undo(ctx, id, res.EntryID)
	require.NoError(t, err)
	assert.Equal(t, 0, usedSlots(t, bs))
}

// ADR 0004 step 15 on the real stack: a spell received mid-combat starts in
// the round it was received; voiding it leaves the list as it was, the older
// buff's start round included.
func TestReceivedBuffStartsInTheCurrentRoundAndVoidRestoresTheList(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.Buffs.Activate(ctx, id, buffsvc.BuffEffect{Name: "Haste", Duration: 20})
	require.NoError(t, err)
	for range 3 {
		_, _, _, _, err = bs.Buffs.AdvanceRound(ctx, id)
		require.NoError(t, err)
	}
	before, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	require.Len(t, before, 1)
	assert.Equal(t, 0, before[0].StartRound)

	res, err := bs.Cast.Receive(ctx, id, castsvc.ReceiveRequest{SpellID: "strength", CasterLevel: 9})
	require.NoError(t, err)
	after, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	require.Len(t, after, 2)
	assert.Equal(t, "Strength", after[1].Name)
	assert.Equal(t, 3, after[1].StartRound, "starts in the round it was received")

	_, err = bs.Log.Undo(ctx, id, res.EntryID)
	require.NoError(t, err)
	restored, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before, restored)
}

// ADR 0004 step 16: a cast charge buff starts with its total, spending a charge
// keeps it, and voiding the spend brings back both the charges and the total.
func TestCastChargeBuff_SpendThenVoidRestoresChargesAndTotal(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	_, err := bs.Prep.Prepare(ctx, id, []string{"stoneskin"})
	require.NoError(t, err)
	_, err = bs.Cast.Cast(ctx, id, 0, castsvc.CastRequest{})
	require.NoError(t, err)

	buffs, err := bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	require.Len(t, buffs, 1)
	assert.Equal(t, 11, buffs[0].Charges)
	assert.Equal(t, 11, buffs[0].ChargesTotal)

	_, err = bs.Buffs.Trigger(ctx, id, buffs[0].ID)
	require.NoError(t, err)
	buffs, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 10, buffs[0].Charges)
	assert.Equal(t, 11, buffs[0].ChargesTotal, "spending keeps the total")

	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	_, err = bs.Log.Undo(ctx, id, entries[0].ID)
	require.NoError(t, err)
	buffs, err = bs.Buffs.ListActive(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 11, buffs[0].Charges)
	assert.Equal(t, 11, buffs[0].ChargesTotal)
}

// ADR 0008 step 16 on the real stack: an exposure the Bard takes (his own
// Cloudkill, 1d10 at max = 10) is one entry that lowers his HP through the
// ledger; popping it gives the HP back. An exposure for someone else changes
// no HP.
func TestExposureOnTheBard_VoidGivesTheHPBack(t *testing.T) {
	bs := bootMaxDice(t)
	ctx := context.Background()
	id := bs.CharacterID
	before, err := bs.HP.GetState(ctx, id)
	require.NoError(t, err)

	other, err := bs.Cast.Expose(ctx, id, castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 15})
	require.NoError(t, err)
	assert.Equal(t, 10, other.Damage.Total)
	st, err := bs.HP.GetState(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before.CurrentHp, st.CurrentHp, "the DM's roll: no HP change")

	self, err := bs.Cast.Expose(ctx, id, castsvc.ExposureRequest{SpellID: "cloudkill", CasterLevel: 15, OnSelf: true})
	require.NoError(t, err)
	st, err = bs.HP.GetState(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before.CurrentHp-10, st.CurrentHp)

	_, err = bs.Log.Undo(ctx, id, other.EntryID)
	require.ErrorIs(t, err, actionlogsvc.ErrTopChanged, "newest first")
	_, err = bs.Log.Undo(ctx, id, self.EntryID)
	require.NoError(t, err)
	st, err = bs.HP.GetState(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before.CurrentHp, st.CurrentHp, "popping it gives the HP back")
}
