package composition_test

// ADR 0042's dual-class steps end to end over the in-memory storage: the
// real services, the fixture's rule tables and the action log's undo.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// dualClassElf stores the party's elf with the fighter frozen at 9 and the
// mage realized at mageLevel with mageXP, max HP 40, and returns its id.
func dualClassElf(t *testing.T, mageLevel, mageXP int) (*composition.BackingServices, int64) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id, err := m.AddCharacter(ctx, charactersvc.Character{
		Name: "Elf", Race: "elf", Class: "mage", RealizedLevel: mageLevel, XPTotal: mageXP,
		Str: 17, Dex: 16, Con: 15, Int: 17, Wis: 10, Cha: 10, HPMax: 40,
		Classes: []classes.Entry{
			{Class: "fighter", RealizedLevel: 9, StartingXP: 250_000, State: classes.EntryFrozen, LeftAtLevel: 9},
			classes.SingleEntry("mage", mageLevel, mageXP),
		},
	})
	require.NoError(t, err)
	return bs, id
}

// ADR 0042 step 11: the zero level-up is one level_up entry like any other:
// the elf's mage 9 (not above the fighter's 9) is taken without hit points,
// its undo makes it pending again at the same max HP, its redo takes it again.
// The mage's level is taken with its other benefits.
func TestRoll_Step11_TheZeroLevelUpIsUndoable(t *testing.T) {
	ctx := context.Background()
	bs, id := dualClassElf(t, 8, 135_000)
	// Its other benefits stay (step 11): Table 34's non-weapon slot at 9
	// and the free 5th-level spell the level opens (ADR 0030 step 20.8).
	owed := []charactersvc.PendingLevel{{
		Class: "mage", Classes: 1, Level: 9, NoHitPoints: true,
		Grants: skills.LevelGrants{NonweaponSlots: 1}, FreeSpells: []spellbook.FreeSpell{{Level: 9, SpellLevel: 5}},
	}}
	lu, maxHP, _ := levelUpOf(t, bs, id)
	assert.Equal(t, owed, lu.Pending)
	assert.Equal(t, 40, maxHP)

	got, err := bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Class: "mage", Level: 9})
	require.NoError(t, err)
	assert.Equal(t, 0, got.Gain)
	assert.Equal(t, levelupsvc.CodeNoHitPoints, got.Reason)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, 9, c.Derived.CasterLevel(), "the level is taken")
	lu, maxHP, _ = levelUpOf(t, bs, id)
	assert.Empty(t, lu.Pending)
	assert.Equal(t, 40, maxHP)

	undoTop(t, bs, id)
	lu, maxHP, _ = levelUpOf(t, bs, id)
	assert.Equal(t, owed, lu.Pending, "pending again")
	assert.Equal(t, 40, maxHP)
	c, err = bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, classes.EntryFrozen, c.Classes[0].State, "the fighter stays frozen")
	assert.Equal(t, 9, c.Classes[0].LeftAtLevel)

	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	_, err = bs.Log.Redo(ctx, id, redo.ID)
	require.NoError(t, err)
	lu, _, _ = levelUpOf(t, bs, id)
	assert.Empty(t, lu.Pending)
}
