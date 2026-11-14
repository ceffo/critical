package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// pendingConversion is the campaign's seeded database as the previous releases left it before the
// conversion: every item's abilities NULL, the Power Gem used today (count 0) and its hammer on,
// the Wand of lightning's four charges as its count (the seed now stores a quantity of 1).
func pendingConversion(t *testing.T) (*fakeStore, int64) {
	t.Helper()
	f := newFakeStore()
	require.NoError(t, seedOn(f, campaignSeeds()))
	owner := f.t.Characters[0]
	clear(f.t.Abilities)
	gem := f.t.Items[rowKey(owner, "power_gem_dispel")]
	gem.Count = 0
	f.t.Items[rowKey(owner, "power_gem_dispel")] = gem
	wand := f.t.Items[rowKey(owner, "wand_lightning")]
	wand.Count = 4
	f.t.Items[rowKey(owner, "wand_lightning")] = wand
	return f, owner
}

func abilitiesOf(t *testing.T, f *fakeStore, owner int64, id string) []itemsvc.Ability {
	t.Helper()
	raw, ok := f.t.Abilities[rowKey(owner, id)]
	require.True(t, ok, "%s converted", id)
	var js []json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &js))
	got, err := itemsvc.DecodeAbilities(raw, true)
	require.NoError(t, err)
	return got
}

// ADR 0049 steps 2 and 18: the boot step converts every row with NULL abilities in one transaction,
// sets the count of the items whose counter is an ability's to their quantity 1 (the ability's left
// keeps what the count was), keeps every other count and the old columns as stored, names the Power
// Gem's ability from the seed row's abilities, puts the review reasons on the list, and a second
// run converts nothing.
func TestConvertItems_ADR0049Step18_ConvertsOnceAndKeepsTheOldColumns(t *testing.T) {
	f, owner := pendingConversion(t)
	before := map[string]int64{}
	for k, it := range f.t.Items {
		before[k] = it.Count
	}
	n := len(f.t.Items)

	got, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	assert.Equal(t, n, got.Converted)
	assert.Positive(t, got.Review, "the wand's charges total is on the review list")

	assert.Equal(t, 2, got.Quantities, "the gem and the wand are one object each")
	for k, it := range f.t.Items {
		switch k {
		case rowKey(owner, "power_gem_dispel"), rowKey(owner, "wand_lightning"):
			assert.Equal(t, int64(1), it.Count, "%s: the quantity of one object", k)
		default:
			assert.Equal(t, before[k], it.Count, "%s keeps its count", k)
		}
	}
	gem := abilitiesOf(t, f, owner, "power_gem_dispel")
	require.Len(t, gem, 1)
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 1, Left: 0}, gem[0].Limit, "used today stays used")
	assert.Equal(t, "Dispel Magic", gem[0].Name["en"])
	assert.Equal(t, "Dissipation de la magie", gem[0].Name["fr"])
	wand := abilitiesOf(t, f, owner, "wand_lightning")
	require.Len(t, wand, 1)
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 4, Left: 4, Cost: 1}, wand[0].Limit)
	assert.True(t, f.t.Reviews[rowKey(owner, "wand_lightning")+"|"+itemsvc.ReasonChargesTotal])
	flask := abilitiesOf(t, f, owner, "fire_flask")
	require.Len(t, flask, 1)
	assert.Equal(t, itemsvc.LimitWithItem, flask[0].Limit.Kind)
	assert.Empty(t, abilitiesOf(t, f, owner, "arrows_plus1"))

	again, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{}, again, "a second boot converts nothing")
}

// ADR 0049 step 18: a row an older release wrote after a rollback (NULL again) is converted by the
// next start, and the rows already converted are not touched.
func TestConvertItems_ADR0049Step18_ConvertsOnlyTheRowsStillNull(t *testing.T) {
	f, owner := pendingConversion(t)
	_, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	delete(f.t.Abilities, rowKey(owner, "fire_flask"))
	f.t.Abilities[rowKey(owner, "arrows_plus1")] = `[{"id":"edited","limit":{"kind":"unlimited"},"effects":[]}]`

	got, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	assert.Equal(t, 1, got.Converted)
	assert.Equal(t, `[{"id":"edited","limit":{"kind":"unlimited"},"effects":[]}]`, f.t.Abilities[rowKey(owner, "arrows_plus1")])
}

// ADR 0049 step 2, slice 3's cut-over: a database the previous slice converted (abilities written,
// counts still the old counters) gets the quantity too at the next start: a count of 1 for every
// item holding a per_day or charges ability, the ability's left untouched (what a use between the
// two releases did to the old count is not read back), nothing for stock and unlimited abilities,
// and a second start rewrites nothing.
func TestConvertItems_ADR0049Step2_AlreadyConvertedRowsGetTheirQuantity(t *testing.T) {
	f, owner := pendingConversion(t)
	// What slice 2 left: abilities written, the counts as the old release moved them.
	_, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	for _, id := range []string{"power_gem_dispel", "wand_lightning"} {
		it := f.t.Items[rowKey(owner, id)]
		it.Count = 3
		f.t.Items[rowKey(owner, id)] = it
	}
	// The gem used between the two releases: its old count is 0 (spent), its left is the ability's.
	gem := f.t.Items[rowKey(owner, "power_gem_dispel")]
	gem.Count = 0
	f.t.Items[rowKey(owner, "power_gem_dispel")] = gem
	gemLeft := abilitiesOf(t, f, owner, "power_gem_dispel")[0].Limit.Left
	flask := f.t.Items[rowKey(owner, "fire_flask")]
	flask.Count = 3
	f.t.Items[rowKey(owner, "fire_flask")] = flask

	got, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{Quantities: 2}, got)
	assert.Equal(t, int64(1), f.t.Items[rowKey(owner, "power_gem_dispel")].Count)
	assert.Equal(t, int64(1), f.t.Items[rowKey(owner, "wand_lightning")].Count)
	assert.Equal(t, int64(3), f.t.Items[rowKey(owner, "fire_flask")].Count, "a stock keeps its quantity")
	assert.Equal(t, 4, abilitiesOf(t, f, owner, "wand_lightning")[0].Limit.Left, "left is the ability's, untouched")
	assert.Equal(t, gemLeft, abilitiesOf(t, f, owner, "power_gem_dispel")[0].Limit.Left, "a spent gem's left is untouched too")
	again, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{}, again)
}

// ADR 0049 step 23: a fresh database is seeded in the new shape, abilities written, nothing left
// for the boot step; the old columns mirror the abilities for the previous release.
func TestSeed_ADR0049Step23_FreshDatabaseIsSeededConverted(t *testing.T) {
	f := newFakeStore()
	require.NoError(t, seedOn(f, campaignSeeds()))
	owner := f.t.Characters[0]
	// Only the worn armour the skill armour carry-over adds (written by the migration-time insert,
	// which predates the column) is left to the boot step; every seeded item is converted already.
	got, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{Converted: 1}, got)
	again, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{}, again)
	gem := abilitiesOf(t, f, owner, "power_gem_dispel")
	require.Len(t, gem, 1)
	assert.Equal(t, "Dispel Magic", gem[0].Name["en"])
	row := f.t.Items[rowKey(owner, "power_gem_dispel")]
	assert.Equal(t, "daily", row.Behaviour)
	assert.Equal(t, int64(1), row.Count)
	assert.Equal(t, int64(1), row.MaxCount)
	wand := f.t.Items[rowKey(owner, "wand_lightning")]
	assert.Equal(t, "charges", wand.Behaviour)
	assert.Equal(t, int64(1), wand.Count, "the quantity of one object; the four charges are the ability's left")
	assert.Equal(t, "6d6", wand.EffectDice)
}

// ADR 0049 step 19: rows with values the abilities cannot hold (charges above 100, a daily number
// above 99, a daily count above its max, a potion with charges) are converted to lists the editor
// accepts, with the clamped ones on the review list; none is stored invalid.
func TestConvertItems_ADR0049Step19_OutOfRangeRowsConvertToValidLists(t *testing.T) {
	f, owner := pendingConversion(t)
	add := func(id, magicKind, behaviour string, count, maxCount int64) {
		f.t.Items[rowKey(owner, id)] = storeq.InsertItemParams{
			CharacterID: owner, ID: id, Category: string(itemsvc.CategoryMagic), MagicKind: magicKind,
			Behaviour: behaviour, Count: count, MaxCount: maxCount,
		}
	}
	add("wand_150", itemsvc.MagicWand, "charges", 150, 150)
	add("wand_0", itemsvc.MagicWand, "charges", 0, 0)
	add("rod_daily_150", itemsvc.MagicRod, "daily", 120, 150)
	add("rod_over_max", itemsvc.MagicRod, "daily", 5, 2)
	add("potion_charges", itemsvc.MagicPotion, "charges", 150, 150)

	_, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	for id, kind := range map[string]string{
		"wand_150": itemsvc.MagicWand, "wand_0": itemsvc.MagicWand, "rod_daily_150": itemsvc.MagicRod,
		"rod_over_max": itemsvc.MagicRod, "potion_charges": itemsvc.MagicPotion,
	} {
		got := abilitiesOf(t, f, owner, id)
		it := itemsvc.Item{
			ID: id, Name: localized.Of("Renamed"), Category: itemsvc.CategoryMagic, MagicKind: kind,
			Behaviour: itemsvc.BehaviourStock, State: itemsvc.StateCarried, Count: 1, Abilities: got,
		}
		if kind == itemsvc.MagicPotion {
			it.Count = 3
		}
		assert.NoError(t, it.Validate(), "%s: the converted list passes Item.Validate", id)
	}
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 100, Left: 100, Cost: 1}, abilitiesOf(t, f, owner, "wand_150")[0].Limit)
	assert.True(t, f.t.Reviews[rowKey(owner, "wand_150")+"|"+itemsvc.ReasonLimitClamped])
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitCharges, Max: 1, Left: 0, Cost: 1}, abilitiesOf(t, f, owner, "wand_0")[0].Limit)
	assert.False(t, f.t.Reviews[rowKey(owner, "wand_0")+"|"+itemsvc.ReasonLimitClamped])
	assert.Equal(t, itemsvc.Limit{Kind: itemsvc.LimitPerDay, Max: 99, Left: 99}, abilitiesOf(t, f, owner, "rod_daily_150")[0].Limit)
	assert.True(t, f.t.Reviews[rowKey(owner, "rod_daily_150")+"|"+itemsvc.ReasonLimitClamped])
	assert.True(t, f.t.Reviews[rowKey(owner, "rod_over_max")+"|"+itemsvc.ReasonLimitClamped])
	assert.Equal(t, itemsvc.LimitWithItem, abilitiesOf(t, f, owner, "potion_charges")[0].Limit.Kind)
}

// ADR 0049 step 18: a row the conversion cannot express (effect dice that do not parse) stops the
// boot with the row named, and nothing is stored invalid.
func TestConvertItems_ADR0049Step18_UnexpressibleRowFailsLoudly(t *testing.T) {
	f, owner := pendingConversion(t)
	f.t.Items[rowKey(owner, "bad_dice")] = storeq.InsertItemParams{
		CharacterID: owner, ID: "bad_dice", Category: string(itemsvc.CategoryMagic), MagicKind: itemsvc.MagicWand,
		Behaviour: "charges", Count: 3, MaxCount: 3, EffectDice: "lots",
	}
	_, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad_dice")
}

// scrollLevelsOf is a table of caster levels by spell, the values DMG p.145 gives (a 6th-level wizard
// spell at 13th level): an explicit table, so the conversion is not checked against its own formula.
func scrollLevelsOf(table map[string]int) itemsvc.ScrollLevels {
	return func(id string) (int, bool) {
		level, ok := table[id]
		return level, ok
	}
}

// addScroll puts a stored scroll naming spell on the campaign's database, its abilities as given
// (none: NULL, not converted yet).
func addScroll(f *fakeStore, owner int64, id, spell, abilities string) {
	f.t.Items[rowKey(owner, id)] = storeq.InsertItemParams{
		CharacterID: owner, ID: id, Category: string(itemsvc.CategoryMagic), MagicKind: itemsvc.MagicScroll,
		Behaviour: "stock", Count: 2, MaxCount: 2,
	}
	f.t.ScrollSpells[rowKey(owner, id)] = spell
	if abilities != "" {
		f.t.Abilities[rowKey(owner, id)] = abilities
	}
}

// ADR 0049 step 9: the boot step folds a scroll's spell into a spell effect on its with_item ability,
// at the caster level the levels give (DMG p.145), keeps spell_id as stored for the previous release,
// converts a scroll still NULL and folds it in the same run, and a second start changes nothing.
func TestConvertItems_ADR0049Step9_FoldsAScrollsSpellIntoItsAbility(t *testing.T) {
	f, owner := pendingConversion(t)
	addScroll(f, owner, "scroll_old", "wall_of_force", `[{"id":"use","limit":{"kind":"with_item"},"effects":[]}]`)
	addScroll(f, owner, "scroll_null", "teleport", "")
	levels := scrollLevelsOf(map[string]int{"wall_of_force": 13, "teleport": 15})

	got, err := convertItems(context.Background(), f, f, campaignSeeds(), levels)
	require.NoError(t, err)
	assert.Equal(t, 2, got.Scrolls)
	assert.Zero(t, got.ScrollsSkipped)

	for id, want := range map[string]itemsvc.Effect{
		"scroll_old":  {Kind: itemsvc.EffectSpell, SpellID: "wall_of_force", Level: 13},
		"scroll_null": {Kind: itemsvc.EffectSpell, SpellID: "teleport", Level: 15},
	} {
		got := abilitiesOf(t, f, owner, id)
		require.Len(t, got, 1, id)
		assert.Equal(t, itemsvc.LimitWithItem, got[0].Limit.Kind)
		assert.Equal(t, []itemsvc.Effect{want}, got[0].Effects, id)
	}
	assert.Equal(t, "wall_of_force", f.t.ScrollSpells[rowKey(owner, "scroll_old")], "spell_id stays, mirrored")
	assert.Equal(t, int64(2), f.t.Items[rowKey(owner, "scroll_old")].Count, "the quantity is untouched")

	stored := f.t.Abilities[rowKey(owner, "scroll_old")]
	again, err := convertItems(context.Background(), f, f, campaignSeeds(), levels)
	require.NoError(t, err)
	assert.Equal(t, itemsvc.BootConversion{}, again, "a second start folds nothing")
	assert.Equal(t, stored, f.t.Abilities[rowKey(owner, "scroll_old")])
}

// ADR 0049 step 9: a scroll already holding a spell effect (the player's edit, a level he set) is
// never rewritten; one whose spell the levels do not know, or that has lost its with_item ability, is
// left as stored and counted, so the boot never stores a level it could not give.
func TestConvertItems_ADR0049Step9_LeavesWhatItCannotOrNeedNotFold(t *testing.T) {
	f, owner := pendingConversion(t)
	edited := `[{"id":"use","limit":{"kind":"with_item"},"effects":[{"kind":"spell","spellId":"wall_of_force","level":20}]}]`
	noUse := `[{"id":"use","limit":{"kind":"unlimited"},"effects":[]}]`
	addScroll(f, owner, "scroll_edited", "wall_of_force", edited)
	addScroll(f, owner, "scroll_unknown", "gone_from_the_catalogue", `[{"id":"use","limit":{"kind":"with_item"},"effects":[]}]`)
	addScroll(f, owner, "scroll_no_use", "wall_of_force", noUse)
	addScroll(f, owner, "scroll_removed", "wall_of_force", `[]`)
	levels := scrollLevelsOf(map[string]int{"wall_of_force": 13})

	got, err := convertItems(context.Background(), f, f, campaignSeeds(), levels)
	require.NoError(t, err)
	assert.Zero(t, got.Scrolls)
	assert.Equal(t, 3, got.ScrollsSkipped)
	assert.Equal(t, edited, f.t.Abilities[rowKey(owner, "scroll_edited")])
	assert.Equal(t, noUse, f.t.Abilities[rowKey(owner, "scroll_no_use")])
	assert.Equal(t, `[]`, f.t.Abilities[rowKey(owner, "scroll_removed")])
	assert.Empty(t, itemsvc.FirstSpell(abilitiesOf(t, f, owner, "scroll_unknown")))

	// With no levels at all (a catalogue-less start) nothing is folded and the boot goes on.
	none, err := convertItems(context.Background(), f, f, campaignSeeds(), nil)
	require.NoError(t, err)
	assert.Zero(t, none.Scrolls)
}
