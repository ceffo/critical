package composition_test

// ADR 0042 step 3 end to end over the in-memory storage: a character in play
// takes a new class as one undoable action.

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/dualclasssvc"
	"github.com/ceffo/critical/api/internal/domain/service/xpsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// fighterInPlay stores a fighter of the given race and scores at level 6 with
// 20,000 XP, max HP 60, and returns its id.
func fighterInPlay(t *testing.T, race string, str, intelligence, level int, entries []classes.Entry) (*composition.BackingServices, int64) {
	t.Helper()
	ctx := context.Background()
	m := memory.New()
	bs, err := composition.Build(ctx, composition.MemoryStorage(m), fixtureConfig())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	c := charactersvc.Character{
		Name: "Brenn", Race: race, Class: "fighter", RealizedLevel: level, XPTotal: 20_000,
		Str: str, Dex: 12, Con: 14, Int: intelligence, Wis: 17, Cha: 10, HPMax: 60,
		Details: charactersvc.Details{Alignment: "true_neutral"},
	}
	if entries != nil {
		c.Classes = entries
	}
	id, err := m.AddCharacter(ctx, c)
	require.NoError(t, err)
	return bs, id
}

func entryOf(t *testing.T, c *charactersvc.Character, class string) classes.Entry {
	t.Helper()
	for _, e := range c.Classes {
		if e.Class == class {
			return e
		}
	}
	require.Failf(t, "no such entry", "class %s in %v", class, c.Classes)
	return classes.Entry{}
}

// Step 3: the fighter (level 6, 20,000 XP plus a 1,000 award) is frozen at 6
// with 21,000 XP; the mage starts at level 1 with 0 XP; the hit points are
// kept; one class_switch entry holds it.
func TestTakeClass_Step3_FreezesTheOldAndStartsTheNewAtOne(t *testing.T) {
	ctx := context.Background()
	bs, id := fighterInPlay(t, "human", 17, 17, 6, nil)
	_, err := bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 1_000, Reason: "the ogres"})
	require.NoError(t, err)

	got, err := bs.DualClass.Switch(ctx, id, "mage")
	require.NoError(t, err)
	assert.Equal(t, &dualclasssvc.Result{From: "fighter", To: "mage", LeftAtLevel: 6, XP: 21_000}, got)

	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	require.Len(t, c.Classes, 2)
	fighter, mage := entryOf(t, c, "fighter"), entryOf(t, c, "mage")
	assert.Equal(t, classes.EntryFrozen, fighter.State)
	assert.Equal(t, 6, fighter.LeftAtLevel)
	assert.Equal(t, 21_000, fighter.StartingXP)
	assert.Equal(t, classes.EntryActive, mage.State)
	assert.Equal(t, 1, mage.RealizedLevel)
	assert.Equal(t, "mage", c.Class, "the mirror follows the first active entry")
	assert.Equal(t, 60, c.HPMax, "the hit points are kept")
	frozen, _ := c.Derived.Entry("fighter")
	taken, _ := c.Derived.Entry("mage")
	assert.Equal(t, 21_000, frozen.XP)
	assert.Equal(t, 0, taken.XP)
	assert.Equal(t, 1, taken.RealizedLevel)

	entries, err := bs.Log.List(ctx, id, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2, "the award and the switch")
	assert.Equal(t, actionlog.KindClassSwitch, entries[0].Kind)
	assert.JSONEq(t, `{"from":"fighter","to":"mage","leftAtLevel":6,"xp":21000}`, string(entries[0].Payload))
}

// Step 3: undo gives the single entry back exactly, redo takes the class again.
func TestTakeClass_Step3_UndoRestoresTheSingleEntry(t *testing.T) {
	ctx := context.Background()
	bs, id := fighterInPlay(t, "human", 17, 17, 6, nil)
	before, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)

	_, err = bs.DualClass.Switch(ctx, id, "mage")
	require.NoError(t, err)
	undoTop(t, bs, id)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before.Entries(), c.Entries())
	assert.Equal(t, "fighter", c.Class)
	assert.Equal(t, 60, c.HPMax)

	redo, err := bs.Log.RedoTarget(ctx, id)
	require.NoError(t, err)
	_, err = bs.Log.Redo(ctx, id, redo.ID)
	require.NoError(t, err)
	c, err = bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	require.Len(t, c.Classes, 2)
	assert.Equal(t, classes.EntryFrozen, entryOf(t, c, "fighter").State)
	assert.Equal(t, 1, entryOf(t, c, "mage").RealizedLevel)
	assert.Equal(t, "mage", c.Class)
}

// Step 1 (the cut-off): an award recorded before the switch, edited or removed
// afterwards, leaves the new class at 0 XP; an award after it counts.
func TestTakeClass_Step1_AnAwardBeforeTheSwitchLeavesTheNewClassAlone(t *testing.T) {
	ctx := context.Background()
	bs, id := fighterInPlay(t, "human", 17, 17, 6, nil)
	old, err := bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 1_000, Reason: "before"})
	require.NoError(t, err)
	oldID := old.Entries[0].ID
	_, err = bs.DualClass.Switch(ctx, id, "mage")
	require.NoError(t, err)
	_, err = bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 500, Reason: "after"})
	require.NoError(t, err)

	xpOf := func(class string) int {
		c, err := bs.Character.GetCharacter(ctx, id)
		require.NoError(t, err)
		e, ok := c.Derived.Entry(class)
		require.True(t, ok)
		return e.XP
	}
	assert.Equal(t, 500, xpOf("mage"))

	base := 9_000
	_, err = bs.XP.Update(ctx, id, oldID, xpsvc.EntryPatch{Base: &base})
	require.NoError(t, err)
	assert.Equal(t, 500, xpOf("mage"), "editing the older award leaves the mage alone")
	assert.Equal(t, 21_000, xpOf("fighter"), "the frozen XP never changes")

	_, err = bs.XP.Delete(ctx, id, oldID)
	require.NoError(t, err)
	assert.Equal(t, 500, xpOf("mage"), "removing it too")

	undoTop(t, bs, id)
	assert.Equal(t, 500, xpOf("mage"), "undoing that removal brings the old award back, still not the mage's")
	assert.Equal(t, 21_000, xpOf("fighter"))
}

// Step 2.2 and the new class's own conditions: each refusal is its sentinel
// and writes nothing (no entry changed, nothing in the log).
func TestTakeClass_Step2_2_EachRefusalWritesNothing(t *testing.T) {
	frozenFighter := func() []classes.Entry {
		return []classes.Entry{
			{Class: "fighter", RealizedLevel: 6, StartingXP: 20_000, State: classes.EntryFrozen, LeftAtLevel: 6},
			classes.SingleEntry("mage", 3, 2_500),
		}
	}
	multi := []classes.Entry{classes.SingleEntry("fighter", 6, 20_000), classes.SingleEntry("thief", 6, 20_000)}
	for _, tc := range []struct {
		name         string
		race         string
		str, intel   int
		level        int
		entries      []classes.Entry
		class        string
		wantErr      error
		withoutLift  bool
		wantFieldErr bool
	}{
		{name: "multi-class", race: "human", str: 17, intel: 17, entries: multi, class: "mage", wantErr: dualclasssvc.ErrNotAllowed},
		{name: "restricted race without the lift", race: "elf", str: 17, intel: 17, class: "mage", withoutLift: true, wantErr: dualclasssvc.ErrNotAllowed},
		{name: "old prime requisite under 15", race: "human", str: 14, intel: 17, class: "mage", wantErr: dualclasssvc.ErrPrimeRequisiteTooLow},
		{name: "new prime requisite under 17", race: "human", str: 17, intel: 16, class: "mage", wantErr: dualclasssvc.ErrPrimeRequisiteTooLow},
		{name: "level under 2", race: "human", str: 17, intel: 17, level: 1, class: "mage", wantErr: dualclasssvc.ErrLevelTooLow},
		{name: "back to a class already left", race: "human", str: 17, intel: 17, entries: frozenFighter(), class: "fighter", wantErr: dualclasssvc.ErrClassAlreadyLeft},
		{name: "the current class", race: "human", str: 17, intel: 17, class: "fighter", wantErr: dualclasssvc.ErrSameClass},
		{name: "unknown class", race: "human", str: 17, intel: 17, class: "no_such_class", wantFieldErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			level := tc.level
			if level == 0 {
				level = 6
			}
			bs, id := fighterInPlay(t, tc.race, tc.str, tc.intel, level, tc.entries)
			if tc.withoutLift {
				_, err := bs.Rulesets.SetRacialLift(ctx, uuid.New(), ruleset.RacialLift{MulticlassAnyRace: true})
				require.NoError(t, err)
			}
			before, err := bs.Character.GetCharacter(ctx, id)
			require.NoError(t, err)
			_, err = bs.DualClass.Switch(ctx, id, tc.class)
			if tc.wantFieldErr {
				var fe *domain.FieldError
				require.ErrorAs(t, err, &fe)
				assert.Equal(t, "class", fe.Field)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}
			after, err := bs.Character.GetCharacter(ctx, id)
			require.NoError(t, err)
			assert.Equal(t, before.Entries(), after.Entries())
			entries, err := bs.Log.List(ctx, id, 10)
			require.NoError(t, err)
			assert.Empty(t, entries)
		})
	}
}

// Step 2.2: a race the books restrict is accepted while the table's racial lift is on
// (it is by default), so an elf fighter takes the mage class.
func TestTakeClass_Step2_2_ARestrictedRaceIsAcceptedWithTheLift(t *testing.T) {
	ctx := context.Background()
	bs, id := fighterInPlay(t, "elf", 17, 17, 6, nil)
	_, err := bs.DualClass.Switch(ctx, id, "mage")
	require.NoError(t, err)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "mage", c.Class)
	assert.Equal(t, classes.EntryFrozen, entryOf(t, c, "fighter").State)
}

// Step 3: a second switch (mage to cleric) on a dual-classed character freezes the mage too, keeps
// the fighter frozen, cuts the cleric off at the latest award, and one Undo gives the previous two
// entries back.
func TestTakeClass_Step3_ASecondSwitchFreezesBothAndUndoRestoresThem(t *testing.T) {
	ctx := context.Background()
	entries := []classes.Entry{
		{Class: "fighter", RealizedLevel: 6, StartingXP: 20_000, State: classes.EntryFrozen, LeftAtLevel: 6},
		classes.SingleEntry("mage", 3, 2_500),
	}
	bs, id := fighterInPlay(t, "human", 17, 17, 6, entries)
	award, err := bs.XP.Add(ctx, id, xpsvc.EntryInput{Base: 700, Reason: "the lich"})
	require.NoError(t, err)
	latest := award.Entries[0].ID
	before, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)

	_, err = bs.DualClass.Switch(ctx, id, "cleric")
	require.NoError(t, err)
	c, err := bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	require.Len(t, c.Classes, 3)
	assert.Equal(t, classes.EntryFrozen, entryOf(t, c, "fighter").State)
	assert.Equal(t, entries[0], entryOf(t, c, "fighter"), "the fighter is untouched")
	mage := entryOf(t, c, "mage")
	assert.Equal(t, classes.EntryFrozen, mage.State)
	assert.Equal(t, 3, mage.LeftAtLevel)
	cleric := entryOf(t, c, "cleric")
	assert.Equal(t, classes.EntryActive, cleric.State)
	assert.Equal(t, 1, cleric.RealizedLevel)
	assert.Equal(t, latest, cleric.IgnoredAwardsThrough)
	assert.Equal(t, "cleric", c.Class)

	undoTop(t, bs, id)
	c, err = bs.Character.GetCharacter(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, before.Entries(), c.Entries())
}
