package composition_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The weapon editor on the real stack: each add / edit /
// remove is one log entry, only the newest entry can be popped, and the
// attack math reads the edited weapon on the next attack.

func newWeaponStack(t *testing.T) (*composition.BackingServices, *scriptedDice) {
	t.Helper()
	dice := newScriptedDice(t)
	ctx := context.Background()
	bs, err := composition.New(ctx, composition.Config{
		DSN:      filepath.Join(t.TempDir(), "critical.db"),
		SeedsDir: filepath.Join("..", "..", "testdata", "seeds"),
		Dice:     dice,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(ctx) })
	return bs, dice
}

func weaponList(t *testing.T, bs *composition.BackingServices) map[string]combatsvc.Weapon {
	t.Helper()
	ws, err := bs.Combat.ListWeapons(context.Background())
	require.NoError(t, err)
	out := map[string]combatsvc.Weapon{}
	for _, w := range ws {
		out[w.ID] = w
	}
	return out
}

func weaponIDs(t *testing.T, bs *composition.BackingServices) []string {
	t.Helper()
	ws, err := bs.Combat.ListWeapons(context.Background())
	require.NoError(t, err)
	var ids []string
	for _, w := range ws {
		ids = append(ids, w.ID)
	}
	return ids
}

// magic sums the weapon and ammo to-hit modifiers of an attack.
func toHitMagic(r *combatsvc.RollResult) int {
	n := 0
	for _, m := range r.Modifiers {
		if m.Kind == combatsvc.ModWeaponMagic || m.Kind == combatsvc.ModAmmoMagic {
			n += m.Value
		}
	}
	return n
}

// Acceptance 1: Bastard Sword +2 -> +3 to hit and +4 damage (distinct on
// purpose); the next attack uses exactly those, with the dice scripted
// 2d4 = 3 + 2; a pop restores the +2 / +2.
func TestWeapons_EditBastardSwordIsUsedByNextAttackAndPopRestores(t *testing.T) {
	bs, dice := newWeaponStack(t)
	ctx := context.Background()
	atk := combatsvc.AttackRequest{WeaponID: "bastard_sword", TargetAC: new(1000)}

	res, err := bs.Weapons.Update(ctx, bs.CharacterID, "bastard_sword", weaponsvc.WeaponPatch{MagicToHit: ptr(3), MagicDamage: ptr(4)})
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindWeaponEdit, newestEntry(t, bs).Kind)
	assert.Equal(t, 3, weaponList(t, bs)["bastard_sword"].MagicToHit)

	dice.expect(roll{"d20", 11}, roll{"d100", 37}, roll{"d100", 58}, roll{"d4", 3}, roll{"d4", 2})
	at, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, atk)
	require.NoError(t, err)
	assert.Equal(t, 3, toHitMagic(at), "edited to-hit magic")
	assert.Equal(t, 4, at.WeaponBuff, "edited damage magic, separate from to-hit")
	assert.Equal(t, 5, at.Rolled.Total, "2d4 = 3 + 2")
	assert.Equal(t, "Bastard Sword +2", at.WeaponName, "the name did not change")

	// The attack is newer: the edit cannot be popped before it.
	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
	_, err = bs.Log.Void(ctx, bs.CharacterID, at.LogEntryID)
	require.NoError(t, err)
	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)

	dice.expect(roll{"d20", 12}, roll{"d100", 41}, roll{"d100", 59}, roll{"d4", 1}, roll{"d4", 4})
	back, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, atk)
	require.NoError(t, err)
	assert.Equal(t, 2, toHitMagic(back), "popped: +2 to hit again")
	assert.Equal(t, 2, back.WeaponBuff, "popped: +2 damage again")
}

// Dice and size edits are read by the next attack too: the new large dice are
// rolled against a large target.
func TestWeapons_EditedDamageDiceAreRolled(t *testing.T) {
	bs, dice := newWeaponStack(t)
	ctx := context.Background()
	_, err := bs.Weapons.Update(ctx, bs.CharacterID, "mace", weaponsvc.WeaponPatch{DamageL: ptr("3d10")})
	require.NoError(t, err)
	dice.expect(roll{"d20", 11}, roll{"d100", 37}, roll{"d100", 58}, roll{"d10", 4}, roll{"d10", 6}, roll{"d10", 9})
	at, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "mace", TargetAC: new(1000), TargetSize: combatsvc.SizeLarge})
	require.NoError(t, err)
	assert.Equal(t, "3d10", at.Rolled.Dice)
	assert.Equal(t, 19, at.Rolled.Total)
}

// Acceptance 2: a Long Bow (missile, arrow) joins the list and is fed arrow
// items (exactly one spent), and the same bow switched to bolt refuses them.
func TestWeapons_AddLongBowFiresArrowItems(t *testing.T) {
	bs, dice := newWeaponStack(t)
	ctx := context.Background()
	add, err := bs.Weapons.Create(ctx, bs.CharacterID, weaponsvc.WeaponInput{
		Name: "Long Bow", DamageSM: "1d8", DamageL: "1d8", MagicToHit: 1, MagicDamage: 2, Kind: "missile", TwoHanded: true, AmmoKind: "arrow",
	})
	require.NoError(t, err)
	assert.Equal(t, "long_bow", add.Weapon.ID)
	assert.Equal(t, actionlog.KindWeaponAdd, newestEntry(t, bs).Kind)
	got := weaponList(t, bs)["long_bow"]
	assert.Equal(t, combatsvc.WeaponMissile, got.Kind)
	assert.Equal(t, "arrow", got.AmmoKind)
	assert.True(t, got.FiresAmmo())

	before := itemCount(t, bs, "arrows_plus1")
	dice.expect(roll{"d20", 11}, roll{"d100", 37}, roll{"d100", 58}, roll{"d8", 7})
	at, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "long_bow", AmmoItemID: "arrows_plus1", TargetAC: new(1000)})
	require.NoError(t, err)
	assert.Equal(t, before-1, itemCount(t, bs, "arrows_plus1"), "one arrow spent")
	assert.Equal(t, 2, toHitMagic(at), "bow +1 and Arrows +1 to hit")
	assert.Equal(t, 3, at.WeaponBuff, "bow +2 and Arrows +1 to damage")
}

func TestWeapons_AmmoKindEditedToBoltRefusesArrows(t *testing.T) {
	bs, _ := newWeaponStack(t) // no dice scripted: the attack is refused before any roll
	ctx := context.Background()
	_, err := bs.Weapons.Update(ctx, bs.CharacterID, "shortbow", weaponsvc.WeaponPatch{AmmoKind: ptr("bolt")})
	require.NoError(t, err)
	_, err = bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "shortbow", AmmoItemID: "arrows_plus1", TargetAC: new(10)})
	require.Error(t, err)
	assert.Equal(t, 18, itemCount(t, bs, "arrows_plus1"), "nothing spent")
}

func TestWeapons_AddThenPopDeletesIt(t *testing.T) {
	bs, _ := newWeaponStack(t)
	ctx := context.Background()
	orig := weaponIDs(t, bs)
	add, err := bs.Weapons.Create(ctx, bs.CharacterID, weaponsvc.WeaponInput{Name: "Spear", DamageSM: "1d6", DamageL: "1d8", Kind: "melee"})
	require.NoError(t, err)
	assert.Equal(t, append(append([]string{}, orig...), "spear"), weaponIDs(t, bs), "last in the list")
	_, err = bs.Log.Void(ctx, bs.CharacterID, add.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, orig, weaponIDs(t, bs))
}

// Acceptance 3: remove the darts. They leave the list (and cannot be used to
// attack), the earlier attack line still reads Darts, and a pop restores
// them with the same id, position and fields.
func TestWeapons_RemoveDartsKeepsOldLogLinesAndPopRestores(t *testing.T) {
	bs, dice := newWeaponStack(t)
	ctx := context.Background()
	order, orig := weaponIDs(t, bs), weaponList(t, bs)["darts"]

	dice.expect(roll{"d20", 11}, roll{"d100", 37}, roll{"d100", 58}, roll{"d3", 2})
	atk, err := bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "darts", TargetAC: new(1000)})
	require.NoError(t, err)

	rm, err := bs.Weapons.Delete(ctx, bs.CharacterID, "darts")
	require.NoError(t, err)
	assert.Equal(t, actionlog.KindWeaponRemove, newestEntry(t, bs).Kind)
	assert.NotContains(t, weaponIDs(t, bs), "darts")
	_, err = bs.Combat.ResolveAttack(ctx, bs.CharacterID, combatsvc.AttackRequest{WeaponID: "darts", TargetAC: new(10)})
	require.ErrorIs(t, err, combatsvc.ErrWeaponNotFound)

	entries, err := bs.Log.List(ctx, bs.CharacterID, 10)
	require.NoError(t, err)
	var payload string
	for _, e := range entries {
		if e.ID == atk.LogEntryID {
			payload = string(e.Payload)
		}
	}
	assert.Contains(t, payload, `"weaponName":"Darts"`, "the earlier attack still reads Darts")

	// The remove is the newest entry: the attack before it cannot be popped.
	_, err = bs.Log.Void(ctx, bs.CharacterID, atk.LogEntryID)
	require.ErrorIs(t, err, actionlogsvc.ErrNotLatest)
	_, err = bs.Log.Void(ctx, bs.CharacterID, rm.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, order, weaponIDs(t, bs), "back in its place")
	assert.Equal(t, orig, weaponList(t, bs)["darts"])
}

// Every field of an edit comes back on a pop, not only the bonuses.
func TestWeapons_EditAllFieldsAndPopRestoresThem(t *testing.T) {
	bs, _ := newWeaponStack(t)
	ctx := context.Background()
	orig := weaponList(t, bs)["longsword"]
	res, err := bs.Weapons.Update(ctx, bs.CharacterID, "longsword", weaponsvc.WeaponPatch{
		Name: ptr("Axe"), DamageSM: ptr("1d10"), DamageL: ptr("2d6"), MagicToHit: ptr(-1), MagicDamage: ptr(5),
		Kind: ptr("missile"), TwoHanded: ptr(true), AmmoKind: ptr("bolt"),
	})
	require.NoError(t, err)
	assert.Equal(t, "Axe", weaponList(t, bs)["longsword"].Name)
	_, err = bs.Log.Void(ctx, bs.CharacterID, res.LogEntryID)
	require.NoError(t, err)
	assert.Equal(t, orig, weaponList(t, bs)["longsword"])
}

func TestWeapons_InvalidEditsLogNothing(t *testing.T) {
	bs, _ := newWeaponStack(t)
	ctx := context.Background()
	before := logCount(t, bs)
	_, err := bs.Weapons.Create(ctx, bs.CharacterID, weaponsvc.WeaponInput{Name: "X", DamageSM: "2x6", DamageL: "1d6", Kind: "melee"})
	require.Error(t, err)
	_, err = bs.Weapons.Update(ctx, bs.CharacterID, "mace", weaponsvc.WeaponPatch{MagicToHit: ptr(50)})
	require.Error(t, err)
	_, err = bs.Weapons.Delete(ctx, bs.CharacterID, "nope")
	require.ErrorIs(t, err, weaponsvc.ErrWeaponNotFound)
	assert.Equal(t, before, logCount(t, bs))
	assert.Equal(t, 1, weaponList(t, bs)["mace"].MagicToHit)
}
