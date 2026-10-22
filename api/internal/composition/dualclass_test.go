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
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
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

// fixedDice answers each die in order from values, sides checked; any other
// roll fails the test.
type fixedDice struct {
	t      *testing.T
	values [][2]int // sides, value
}

func (d *fixedDice) next(sides int) int {
	d.t.Helper()
	require.NotEmpty(d.t, d.values, "an unexpected d%d", sides)
	v := d.values[0]
	d.values = d.values[1:]
	require.Equal(d.t, v[0], sides, "the die rolled")
	return v[1]
}

func (d *fixedDice) RollDie(sides int) int { return d.next(sides) }
func (d *fixedDice) RollD20() int          { return d.next(20) }
func (d *fixedDice) RollD100() int         { return d.next(100) }

// ADR 0042 step 11 over the elf's next levels: from mage 7 with the XP of
// mage 10, levels 8 and 9 (not above the fighter's 9) gain no hit points and
// roll no die; level 10 rolls the mage's d4 plus Constitution 15's +1.
func TestRoll_Step11_TheElfGainsHitPointsOnlyPastTheFighter(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	cfg := fixtureConfig()
	cfg.Dice = &fixedDice{t: t, values: [][2]int{{4, 3}}}
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id, err := m.AddCharacter(ctx, charactersvc.Character{
		Name: "Elf", Race: "elf", Class: "mage", RealizedLevel: 7, XPTotal: 250_000,
		Str: 17, Dex: 16, Con: 15, Int: 17, Wis: 10, Cha: 10, HPMax: 40,
		Classes: []classes.Entry{
			{Class: "fighter", RealizedLevel: 9, StartingXP: 250_000, State: classes.EntryFrozen, LeftAtLevel: 9},
			classes.SingleEntry("mage", 7, 250_000),
		},
	})
	require.NoError(t, err)

	for _, want := range []struct{ level, gain, maxHP int }{{8, 0, 40}, {9, 0, 40}, {10, 4, 44}} {
		got, err := bs.LevelUp.Roll(ctx, id, levelupsvc.RollInput{Class: "mage", Level: want.level})
		require.NoError(t, err, "level %d", want.level)
		assert.Equal(t, want.gain, got.Gain, "level %d", want.level)
		_, maxHP, _ := levelUpOf(t, bs, id)
		assert.Equal(t, want.maxHP, maxHP, "level %d", want.level)
	}
}

// ADR 0042 step 13 with step 14 end to end: a mage left at 9, now a cleric
// 5, learns, prepares and casts sleep from the mage's own slot, at the mage's
// 9; the cast answers warnings ["dual_class_old_ability"] and class "mage",
// and its journal entry carries both.
func TestCast_Step13_AnOldClassCastIsWarnedAndLogged(t *testing.T) {
	ctx := context.Background()
	m := memory.New()
	cfg := fixtureConfig()
	cfg.Dice = &fixedDice{t: t, values: [][2]int{{100, 1}}} // the chance to learn
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id, err := m.AddCharacter(ctx, charactersvc.Character{
		Name: "Dual", Race: "human", Class: "cleric", RealizedLevel: 5, XPTotal: 15_000,
		Str: 12, Dex: 12, Con: 12, Int: 17, Wis: 15, Cha: 10, HPMax: 40,
		Classes: []classes.Entry{
			{Class: "mage", RealizedLevel: 9, StartingXP: 135_000, State: classes.EntryFrozen, LeftAtLevel: 9},
			classes.SingleEntry("cleric", 5, 15_000),
		},
	})
	require.NoError(t, err)

	learned, err := bs.Learn.Learn(ctx, id, "sleep")
	require.NoError(t, err)
	require.True(t, learned.Learned)
	slots, err := bs.Prep.PrepareFor(ctx, id, "mage", []string{"sleep"})
	require.NoError(t, err)
	idx := -1
	for i, s := range slots {
		if s.SpellID == "sleep" {
			idx = i
		}
	}
	require.GreaterOrEqual(t, idx, 0, "sleep prepared in a slot: %+v", slots)

	res, err := bs.Cast.Cast(ctx, id, idx, castsvc.CastRequest{})
	require.NoError(t, err)
	assert.Equal(t, []string{classes.WarnOldAbility}, res.Warnings)
	assert.Equal(t, "mage", res.Class)
	assert.Equal(t, 9, res.CasterLevel, "the mage's left-at level")
	entries, err := bs.Log.List(ctx, id, 1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Contains(t, string(entries[0].Payload), `"warnings":["dual_class_old_ability"]`)
	assert.Contains(t, string(entries[0].Payload), `"class":"mage"`)
}
