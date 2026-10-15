package composition_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain/service/castsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/restsvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func boot(t *testing.T) *composition.BackingServices {
	t.Helper()
	bs, err := composition.New(context.Background(), composition.Config{
		DSN:      filepath.Join(t.TempDir(), "t.db"),
		SeedsDir: filepath.Join("..", "..", "testdata", "seeds"),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bs.Close(context.Background()) })
	// The seeded Bard prepares no spells yet, so give the test two slots.
	_, err = bs.DB.Exec(`UPDATE spell_slots SET slots = ? WHERE character_id = ?`,
		`[{"level":1,"prepared_spell_name":"Armor","used":false},{"level":2,"prepared_spell_name":"Web","used":false}]`, bs.CharacterID)
	require.NoError(t, err)
	return bs
}

func usedSlots(t *testing.T, bs *composition.BackingServices) int {
	t.Helper()
	slots, err := bs.Spells.GetSlots(context.Background(), bs.CharacterID)
	require.NoError(t, err)
	n := 0
	for _, s := range slots {
		if s.Used {
			n++
		}
	}
	return n
}

func gemCount(t *testing.T, bs *composition.BackingServices) int {
	t.Helper()
	var n int
	require.NoError(t, bs.DB.QueryRow(`SELECT count FROM items WHERE id = 'power_gem_dispel'`).Scan(&n))
	return n
}

func spend(t *testing.T, bs *composition.BackingServices) {
	t.Helper()
	ctx := context.Background()
	_, err := bs.Cast.Cast(ctx, bs.CharacterID, 0, castsvc.CastRequest{})
	require.NoError(t, err)
	_, err = bs.Items.Use(ctx, "power_gem_dispel")
	require.NoError(t, err)
	require.Equal(t, 1, usedSlots(t, bs))
	require.Zero(t, gemCount(t, bs))
}

// A long rest refills the daily gem and nothing else: wand charges, arrows
// and flasks (stock) stay spent.
func TestLongRest_RefillsSlotsAndDailyItemsOnly(t *testing.T) {
	bs := boot(t)
	spend(t, bs)
	for _, id := range []string{"wand_lightning", "arrows_plus1", "fire_flask"} {
		_, err := bs.Items.Use(context.Background(), id)
		require.NoError(t, err)
	}

	res, err := bs.Rest.LongRest(context.Background(), bs.CharacterID)
	require.NoError(t, err)

	assert.Zero(t, usedSlots(t, bs))
	assert.Equal(t, 1, gemCount(t, bs))
	assert.Len(t, res.Items, 6)
	var wand int
	require.NoError(t, bs.DB.QueryRow(`SELECT count FROM items WHERE id = 'wand_lightning'`).Scan(&wand))
	assert.Equal(t, 3, wand, "charges stay spent")
	assert.Equal(t, 17, itemCount(t, bs, "arrows_plus1"), "stock is not refilled")
	assert.Equal(t, 4, itemCount(t, bs, "fire_flask"), "stock is not refilled")
	assert.Equal(t, []string{"Power Gem: Dispel Magic"}, refilled(t, bs, res.EntryID))

	// The rest is logged and voidable: the gem is spent again, the slot used.
	_, err = bs.Log.Void(context.Background(), bs.CharacterID, res.EntryID)
	require.NoError(t, err)
	assert.Zero(t, gemCount(t, bs))
	assert.Equal(t, 1, usedSlots(t, bs))
}

type failingItems struct{ itemsvc.SVC }

func (failingItems) ResetDaily(context.Context) ([]itemsvc.Item, error) {
	return nil, errors.New("items down")
}

func TestLongRest_FailureInItemsRollsBackSlots(t *testing.T) {
	bs := boot(t)
	spend(t, bs)
	svc, err := restsvc.New(bs.Spells, failingItems{bs.Items}, bs.Log, bs.DB)
	require.NoError(t, err)

	_, err = svc.LongRest(context.Background(), bs.CharacterID)
	require.ErrorContains(t, err, "items down")

	assert.Equal(t, 1, usedSlots(t, bs), "slot reset rolled back")
	assert.Zero(t, gemCount(t, bs))
}

func TestLongRest_FailureInSlotsLeavesGemSpent(t *testing.T) {
	bs := boot(t)
	spend(t, bs)
	// Break the slots table so ResetSlots fails inside the transaction.
	_, err := bs.DB.Exec(`ALTER TABLE spell_slots RENAME TO spell_slots_gone`)
	require.NoError(t, err)

	_, err = bs.Rest.LongRest(context.Background(), bs.CharacterID)
	require.Error(t, err)
	assert.Zero(t, gemCount(t, bs))
}

// refilled is the long rest entry's itemsRefilled list.
func refilled(t *testing.T, bs *composition.BackingServices, entryID int64) []string {
	t.Helper()
	var payload string
	require.NoError(t, bs.DB.QueryRow(`SELECT payload FROM action_log WHERE id = ?`, entryID).Scan(&payload))
	var rec struct {
		ItemsRefilled []string `json:"itemsRefilled"`
	}
	require.NoError(t, json.Unmarshal([]byte(payload), &rec))
	return rec.ItemsRefilled
}
