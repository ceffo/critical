package composition_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
)

func slotIDs(t *testing.T, bs *composition.BackingServices) []string {
	t.Helper()
	slots, err := bs.Spells.GetSlots(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	ids := []string{}
	for _, s := range slots {
		ids = append(ids, s.SpellID)
	}
	return ids
}

func TestPrepare_RealStack_LimitsAreEnforcedAtLevel15(t *testing.T) {
	bs := boot(t)
	ctx := context.Background()

	_, err := bs.Prep.Prepare(ctx, bs.CharacterID, []string{"alarm", "burning_hands", "enlarge", "identify"})
	require.ErrorIs(t, err, prepsvc.ErrOverLimit, "4 level-1 spells at level 15")

	_, err = bs.Prep.Prepare(ctx, bs.CharacterID, []string{"alarm", "burning_hands", "enlarge"})
	require.NoError(t, err, "3 level-1 spells")

	_, err = bs.Prep.Prepare(ctx, bs.CharacterID, []string{"magic_jar"})
	require.ErrorIs(t, err, prepsvc.ErrNotInSpellbook)
	assert.Equal(t, []string{"alarm", "burning_hands", "enlarge"}, slotIDs(t, bs), "a refused call changes nothing")
}

func TestPrepare_RealStack_LongRestKeepsPreparationAndResetsUsed(t *testing.T) {
	bs := boot(t)
	ctx := context.Background()
	_, err := bs.Prep.Prepare(ctx, bs.CharacterID, []string{"mirror_image", "mirror_image", "fireball"})
	require.NoError(t, err)
	_, err = bs.Cast.Cast(ctx, bs.CharacterID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	require.Equal(t, 1, usedSlots(t, bs))

	res, err := bs.Rest.LongRest(ctx, bs.CharacterID)
	require.NoError(t, err)

	assert.Equal(t, 0, usedSlots(t, bs))
	assert.Equal(t, []string{"mirror_image", "mirror_image", "fireball"}, slotIDs(t, bs))
	assert.Len(t, res.Slots, 3)
}

// The snapshot key must have a Restorer registered in the composition root,
// otherwise voiding a preparation fails with "no restorer registered".
func TestPrepare_RealStack_VoidRestoresThePreviousPreparation(t *testing.T) {
	bs := boot(t)
	ctx := context.Background()
	_, err := bs.Prep.Prepare(ctx, bs.CharacterID, []string{"alarm"})
	require.NoError(t, err)
	_, err = bs.Prep.Prepare(ctx, bs.CharacterID, []string{"fireball", "haste"})
	require.NoError(t, err)

	entries, err := bs.Log.ActiveEntries(ctx, bs.CharacterID, actionlog.KindSpellPrep, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.JSONEq(t, `{"spells":[{"id":"fireball","name":"Fireball","count":1},{"id":"haste","name":"Haste","count":1}]}`,
		string(entries[0].Payload)) // newest first

	_, err = bs.Log.Undo(ctx, bs.CharacterID, entries[0].ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"alarm"}, slotIDs(t, bs))

	// The older entry changed the same state before: now it can be voided too.
	_, err = bs.Log.Undo(ctx, bs.CharacterID, entries[1].ID)
	require.NoError(t, err)
	assert.Len(t, slotIDs(t, bs), 2, "back to the two slots the test booted with")
}

// Every spellbook damage spell carries its dice at the Bard's level (15) and
// the others none; the lookup serves the same values.
func TestPrepare_RealStack_SpellbookDamageAtTheBardLevel(t *testing.T) {
	bs := boot(t)
	ctx := context.Background()
	info, err := bs.Prep.Info(ctx, bs.CharacterID)
	require.NoError(t, err)
	got := map[string]string{}
	for _, e := range info.Spellbook {
		if d := e.Damage; d != nil {
			assert.Equal(t, 15, d.CasterLevel, e.ID)
			got[e.ID] = fmt.Sprintf("%v %v x%v r%v %s", d.Dice.Dice, d.Dice.Constant, d.Instances.Constant, d.Rounds.Constant, d.Type)
			lookup, err := bs.Cast.DamageAt(ctx, e.ID, 15)
			require.NoError(t, err)
			assert.Equal(t, d, lookup, "lookup and spellbook serve the same damage for %s", e.ID)
		}
	}
	assert.Equal(t, map[string]string{
		"burning_hands":    "[{1 3}] 20 x1 r1 fire",
		"magic_missile":    "[{1 4}] 1 x5 r1 force",
		"melfs_acid_arrow": "[{2 4}] 0 x1 r6 acid",
		"fireball":         "[{10 6}] 0 x1 r1 fire",
	}, got)
}
