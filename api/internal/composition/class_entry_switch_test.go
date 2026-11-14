package composition_test

// ADR 0050 step 15 end to end over the in-memory storage: an elf mage 14 (the user's friend's
// character) becomes a transmuter as one undoable action.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// elfMage14 stores an elf mage of level 14 (Table 21's XP: 1,500,000, Int 18) whose book holds
// four spells, each for a reason the transmuter's school lists (PHB Table 22: alteration is his,
// abjuration and necromancy are opposed): Enlarge (alteration, his), Magic Missile (invocation,
// other), Alarm (abjuration and invocation: opposed) and Magic Jar (necromancy: opposed). All
// four are prepared, Enlarge's slot is used.
func elfMage14(t *testing.T) (*composition.BackingServices, *memory.Store, int64) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id, err := m.AddCharacter(ctx, charactersvc.Character{
		Name: "Ilsevel", Race: "elf", Class: "mage", RealizedLevel: 14, XPTotal: 1_500_000,
		Str: 9, Dex: 16, Con: 12, Int: 18, Wis: 12, Cha: 10, HPMax: 37, Details: charactersvc.Details{Alignment: "neutral_good"},
	})
	require.NoError(t, err)
	for _, spell := range []string{"enlarge", "magic_missile", "alarm", "magic_jar"} {
		_, err = bs.Learn.Add(ctx, id, spell)
		require.NoError(t, err, spell)
	}
	slots, err := bs.Prep.Prepare(ctx, id, []string{"enlarge", "magic_missile", "alarm", "magic_jar"})
	require.NoError(t, err)
	for i := range slots {
		slots[i].Used = slots[i].SpellID == "enlarge"
	}
	require.NoError(t, m.Spells().SaveSlots(ctx, id, slots))
	return bs, m, id
}

func slotsOf(t *testing.T, m *memory.Store, id int64) []spellsvc.SpellSlot {
	t.Helper()
	slots, err := m.Spells().GetSlots(context.Background(), id)
	require.NoError(t, err)
	return slots
}

// Step 15.2: the elf mage's entry becomes a transmuter and only its class changes: XP, level and
// HP are as they were, the entry is the same one, and the race warning is shown, not a refusal
// (an elf is no transmuter in Table 22). The switch is one journal entry (15.5).
func TestSwitch_Step15_AMageEntryBecomesATransmuter(t *testing.T) {
	ctx := context.Background()
	bs, _, id := elfMage14(t)
	before, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	logged, err := bs.Log.List(ctx, id, 50)
	require.NoError(t, err)

	got, err := bs.ClassEntry.Switch(ctx, id, 0, "transmuter")
	require.NoError(t, err)
	assert.Equal(t, "mage", got.From)
	assert.Equal(t, "transmuter", got.To)

	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	require.Len(t, c.Classes, 1, "the same one entry")
	assert.Equal(t, classes.Entry{
		Class: "transmuter", RealizedLevel: 14, StartingXP: before.Classes[0].StartingXP, State: classes.EntryActive,
	}, c.Classes[0], "only the class changed")
	assert.Equal(t, "transmuter", c.Class, "the mirror follows the entry")
	assert.Equal(t, 1_500_000, c.XPTotal)
	assert.Equal(t, 37, c.HPMax)
	e, ok := c.Derived.Entry("transmuter")
	require.True(t, ok)
	pre, _ := before.Derived.Entry("mage")
	assert.Equal(t, pre.XP, e.XP)
	assert.Equal(t, 14, e.RealizedLevel)
	assert.Equal(t, "race transmuter elf", warningCodeOf(c.Warnings, charactersvc.CodeSpecialistRace), "warned, not refused")

	entries, err := bs.Log.List(ctx, id, 50)
	require.NoError(t, err)
	require.Len(t, entries, len(logged)+1, "one journal entry")
	assert.Equal(t, actionlog.KindClassEntrySwitch, entries[0].Kind)
	assert.JSONEq(t, `{"entry":0,"from":"mage","to":"transmuter"}`, string(entries[0].Payload))
}

func warningCodeOf(ws []charactersvc.Warning, code string) string {
	for _, w := range ws {
		if w.Code == code {
			return "race " + w.Class + " " + w.Race
		}
	}
	return ""
}

// Steps 15.3 and 15.4: the book keeps every spell, the opposed ones served `opposed: true`; the
// slot list loses the prepared opposed spells (Alarm, Magic Jar) and keeps the others' slots
// with their used flags (Enlarge stays used); an opposed spell cannot be prepared again.
func TestSwitch_Step15_OpposedSpellsKeptAndUnprepared(t *testing.T) {
	ctx := context.Background()
	bs, m, id := elfMage14(t)
	_, err := bs.ClassEntry.Switch(ctx, id, 0, "transmuter")
	require.NoError(t, err)

	info, err := bs.Prep.Info(ctx, id)
	require.NoError(t, err)
	opposed := map[string]bool{}
	for _, e := range info.Spellbook {
		opposed[e.ID] = e.Opposed
	}
	assert.Equal(t, map[string]bool{"enlarge": false, "magic_missile": false, "alarm": true, "magic_jar": true}, opposed,
		"the book keeps all four; the opposed two are flagged")

	var kept []string
	for _, sl := range slotsOf(t, m, id) {
		kept = append(kept, sl.SpellID)
		assert.Equal(t, sl.SpellID == "enlarge", sl.Used, "%s keeps its used flag", sl.SpellID)
	}
	assert.Equal(t, []string{"enlarge", "magic_missile"}, kept, "Alarm and Magic Jar left the slots")

	_, err = bs.Prep.Prepare(ctx, id, []string{"enlarge", "alarm"})
	require.ErrorIs(t, err, spellbook.ErrSchoolOpposed, "an opposed spell is not prepared (step 11)")
}

// Step 15.5: one undo gives the class and the opposed spells' slots back exactly; redo does the
// switch again, the slots as the switch left them.
func TestSwitch_Step15_UndoRestoresClassAndSlots(t *testing.T) {
	ctx := context.Background()
	bs, m, id := elfMage14(t)
	entriesBefore, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	slotsBefore := slotsOf(t, m, id)

	_, err = bs.ClassEntry.Switch(ctx, id, 0, "transmuter")
	require.NoError(t, err)
	slotsAfter := slotsOf(t, m, id)
	require.Len(t, slotsAfter, len(slotsBefore)-2)

	undoTop(t, bs, id)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, entriesBefore.Entries(), c.Entries())
	assert.Equal(t, "mage", c.Class)
	assert.Equal(t, slotsBefore, slotsOf(t, m, id), "the opposed spells' slots are back, flags included")

	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	_, err = bs.Log.Redo(ctx, id, redo.ID)
	require.NoError(t, err)
	c, err = bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "transmuter", c.Entries()[0].Class)
	assert.Equal(t, slotsAfter, slotsOf(t, m, id))
}

// Step 15.5 with an extra slot a spell granted: undo then redo give back slots, used flags, the
// extra slot and the class entry exactly, each time (the snapshots hold the whole slot list).
func TestSwitch_Step15_UndoThenRedoIsExactWithAnExtraSlot(t *testing.T) {
	ctx := context.Background()
	bs, m, id := elfMage14(t)
	slots := append(slotsOf(t, m, id), spellsvc.SpellSlot{Level: 2, Extra: true, Used: true})
	require.NoError(t, m.Spells().SaveSlots(ctx, id, slots))
	charBefore, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	slotsBefore := slotsOf(t, m, id)

	_, err = bs.ClassEntry.Switch(ctx, id, 0, "transmuter")
	require.NoError(t, err)
	slotsAfter := slotsOf(t, m, id)
	charAfter, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	require.Len(t, slotsAfter, len(slotsBefore)-2)
	assert.Contains(t, slotsAfter, spellsvc.SpellSlot{Level: 2, Extra: true, Used: true}, "the extra slot is kept")

	for round := 0; round < 2; round++ {
		undoTop(t, bs, id)
		c, err := bs.Character.GetCharacter(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, charBefore.Entries(), c.Entries(), "round %d: the class entry is back", round)
		assert.Equal(t, slotsBefore, slotsOf(t, m, id), "round %d: slots, used flags and the extra slot are back", round)

		redo, err := bs.Log.RedoTarget(ctx, id)
		require.NoError(t, err)
		_, err = bs.Log.Redo(ctx, id, redo.ID)
		require.NoError(t, err)
		c, err = bs.Character.GetCharacter(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, charAfter.Entries(), c.Entries(), "round %d: the switch is done again", round)
		assert.Equal(t, slotsAfter, slotsOf(t, m, id), "round %d", round)
	}
}

// Step 15.5 when nothing is prepared that the target opposes: the slot list is untouched, and an
// undo gives the class back with the very same slots.
func TestSwitch_Step15_NothingOpposedPreparedUndoIsExact(t *testing.T) {
	ctx := context.Background()
	bs, m, id := elfMage14(t)
	_, err := bs.Prep.Prepare(ctx, id, []string{"enlarge", "magic_missile"})
	require.NoError(t, err)
	slotsBefore := slotsOf(t, m, id)
	charBefore, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)

	_, err = bs.ClassEntry.Switch(ctx, id, 0, "transmuter")
	require.NoError(t, err)
	assert.Equal(t, slotsBefore, slotsOf(t, m, id), "no slot left")

	undoTop(t, bs, id)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, charBefore.Entries(), c.Entries())
	assert.Equal(t, slotsBefore, slotsOf(t, m, id))
}

// switchFrom stores a human mage 14 whose book holds the four spells (learned as a mage: a
// specialist cannot learn an opposed one), nothing prepared.
func switchFrom(t *testing.T) (*composition.BackingServices, *memory.Store, int64) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id, err := m.AddCharacter(ctx, charactersvc.Character{
		Name: "Ilsevel", Race: "human", Class: "mage", RealizedLevel: 14, XPTotal: 1_500_000,
		Str: 9, Dex: 16, Con: 16, Int: 18, Wis: 16, Cha: 10, HPMax: 37, Details: charactersvc.Details{Alignment: "neutral_good"},
	})
	require.NoError(t, err)
	for _, spell := range []string{"enlarge", "magic_missile", "alarm", "magic_jar"} {
		_, err = bs.Learn.Add(ctx, id, spell)
		require.NoError(t, err, spell)
	}
	return bs, m, id
}

func slotSpells(t *testing.T, m *memory.Store, id int64) []string {
	t.Helper()
	var out []string
	for _, sl := range slotsOf(t, m, id) {
		out = append(out, sl.SpellID)
	}
	return out
}

// Step 15.4: the spells taken out are the target's opposed ones only. A transmuter (alteration;
// opposes abjuration and necromancy) holding Enlarge and Magic Missile goes to an abjurer, who
// opposes alteration: Enlarge leaves; back to a mage nothing is opposed and nothing leaves.
func TestSwitch_Step15_TheOpposedSetIsTheTargetsOnly(t *testing.T) {
	ctx := context.Background()
	bs, m, id := switchFrom(t)
	_, err := bs.ClassEntry.Switch(ctx, id, 0, "transmuter")
	require.NoError(t, err)
	_, err = bs.Prep.Prepare(ctx, id, []string{"enlarge", "magic_missile"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"enlarge", "magic_missile"}, slotSpells(t, m, id))

	_, err = bs.ClassEntry.Switch(ctx, id, 0, "abjurer")
	require.NoError(t, err)
	assert.Equal(t, []string{"magic_missile"}, slotSpells(t, m, id), "the specialist to another: the abjurer's opposed set, not the transmuter's")

	_, err = bs.ClassEntry.Switch(ctx, id, 0, "mage")
	require.NoError(t, err)
	assert.Equal(t, []string{"magic_missile"}, slotSpells(t, m, id), "a specialist to a mage: nothing is opposed")
	info, err := bs.Prep.Info(ctx, id)
	require.NoError(t, err)
	for _, e := range info.Spellbook {
		assert.False(t, e.Opposed, "%s: a mage opposes nothing", e.ID)
	}

	// The transmuter's own set (abjuration, necromancy) is no longer applied after the first switch.
	_, err = bs.ClassEntry.Switch(ctx, id, 0, "transmuter")
	require.NoError(t, err)
	assert.Equal(t, []string{"magic_missile"}, slotSpells(t, m, id))
}
