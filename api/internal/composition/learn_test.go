package composition_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/learnsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/storage/memory"
)

// d100s answers the d100s in order and refuses every other die.
type d100s struct {
	t      *testing.T
	values []int
	calls  int
}

func (d *d100s) RollD100() int {
	d.t.Helper()
	require.NotEmpty(d.t, d.values, "an unexpected d100")
	d.calls++
	v := d.values[0]
	d.values = d.values[1:]
	return v
}

func (d *d100s) RollDie(sides int) int { d.t.Fatalf("unexpected d%d", sides); return 0 }
func (d *d100s) RollD20() int          { d.t.Fatal("unexpected d20"); return 0 }

func spellbookOf(t *testing.T, bs *composition.BackingServices, id int64) []string {
	t.Helper()
	info, err := bs.Prep.Info(context.Background(), id)
	require.NoError(t, err)
	out := []string{}
	for _, e := range info.Spellbook {
		out = append(out, e.ID)
	}
	return out
}

// ADR 0030 step 20 on a mage created in the app (Intelligence 15, PHB Table
// 4: 7th level, 65%, 11 per level), through the wiring and the in-memory
// store: a roll of 65 learns Sleep; 66 fails Light, which is then refused
// with no die until the undo lifts the block, after which a roll of 3 learns
// it. Three d100s, exactly. The mage prepares and casts from his spellbook
// (ADR 0030 step 8: Table 21 at level 1, one 1st-level spell).
func TestLearn_Step20_LearnFailUndoAndPrepareAMage(t *testing.T) {
	ctx := context.Background()
	dice := &d100s{t: t, values: []int{65, 66, 3}}
	cfg := fixtureConfig()
	cfg.Dice = dice
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	s, err := bs.Auth.DevLogin(ctx, "")
	require.NoError(t, err)
	id, err := bs.Character.Create(ctx, s.User.ID, charactersvc.NewCharacter{
		Name: "Elminster", Class: "mage", Race: "human", Str: 9, Dex: 12, Con: 11, Int: 15, Wis: 12, Cha: 10, HPMax: 3,
	})
	require.NoError(t, err)
	assert.Empty(t, spellbookOf(t, bs, id), "a created mage's spellbook starts empty")

	got, err := bs.Learn.Learn(ctx, id, "sleep")
	require.NoError(t, err)
	assert.Equal(t, learnsvc.Result{SpellID: "sleep", SpellLevel: 1, Roll: 65, Chance: 65, Learned: true, CharacterLevel: 1}, *got)
	got, err = bs.Learn.Learn(ctx, id, "light")
	require.NoError(t, err)
	assert.False(t, got.Learned, "66 is above 65")
	assert.Equal(t, 2, got.RetryLevel, "tried again from the next level")
	assert.Equal(t, []string{"sleep"}, spellbookOf(t, bs, id))
	entries, err := bs.Log.List(ctx, id, 2)
	require.NoError(t, err)
	assert.Equal(t, []actionlog.Kind{actionlog.KindSpellLearn, actionlog.KindSpellLearn}, []actionlog.Kind{entries[0].Kind, entries[1].Kind})

	_, err = bs.Learn.Learn(ctx, id, "light")
	require.ErrorIs(t, err, learnsvc.ErrFailedAtLevel, "refused until a higher level")
	_, err = bs.Learn.Learn(ctx, id, "incendiary_cloud")
	require.ErrorIs(t, err, learnsvc.ErrAboveIntelligence, "8th level above Intelligence 15's 7th")
	_, err = bs.Learn.Learn(ctx, id, "sleep")
	require.ErrorIs(t, err, learnsvc.ErrAlreadyKnown)

	undoTop(t, bs, id)
	got, err = bs.Learn.Learn(ctx, id, "light")
	require.NoError(t, err)
	assert.True(t, got.Learned, "the undo lifted the block")
	assert.Equal(t, []string{"sleep", "light"}, spellbookOf(t, bs, id))
	assert.Equal(t, 3, dice.calls, "one d100 per roll, none for a refusal")

	info, err := bs.Prep.Info(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []prepsvc.LevelLimit{{Level: 1, Max: 1}}, info.Limits, "PHB Table 21 at level 1")
	slots, err := bs.Prep.Prepare(ctx, id, []string{"sleep"})
	require.NoError(t, err)
	require.Len(t, slots, 1)
	assert.Equal(t, "sleep", slots[0].SpellID)

	undoTop(t, bs, id) // the preparation
	undoTop(t, bs, id) // learning Light
	assert.Equal(t, []string{"sleep"}, spellbookOf(t, bs, id), "undoing a success takes the spell out")
}

// Step 20.4: the seed character's spellbook (Kelsen's, imported) comes in as
// the seed file has it, with no learn roll and no journal entry.
func TestLearn_Step20_4_TheSeedSpellbookComesInWithoutRolls(t *testing.T) {
	ctx := context.Background()
	cfg := fixtureConfig()
	cfg.Dice = &d100s{t: t}
	bs, err := composition.Build(ctx, composition.MemoryStorage(memory.New()), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bs.Close(ctx)) })
	id := rootCharacter(t, bs)

	raw, err := os.ReadFile(filepath.Join(cfg.SeedsDir, "spellbook.json"))
	require.NoError(t, err)
	var seed []struct{ ID string }
	require.NoError(t, json.Unmarshal(raw, &seed))
	want := []string{}
	for _, e := range seed {
		want = append(want, e.ID)
	}
	assert.Equal(t, want, spellbookOf(t, bs, id))
	entries, err := bs.Log.List(ctx, id, 50)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotEqual(t, actionlog.KindSpellLearn, e.Kind)
	}
}
