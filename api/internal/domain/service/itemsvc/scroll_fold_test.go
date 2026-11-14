package itemsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// ADR 0049 step 9: the conversion's rule for a scroll that names a spell is one pure function: the
// spell effect goes on the first with_item ability as its last effect; nothing changes when an ability
// already casts a spell (a second run) or when there is no with_item ability to hold it.
func TestWithScrollSpell_ADR0049Step9_Table(t *testing.T) {
	spell := itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "wall_of_force", Level: 13}
	other := itemsvc.Effect{Kind: itemsvc.EffectSpell, SpellID: "light", Level: 6}
	dice := itemsvc.Effect{Kind: itemsvc.EffectDice, Dice: "1d6"}
	withItem := itemsvc.Limit{Kind: itemsvc.LimitWithItem}
	tests := []struct {
		name string
		in   []itemsvc.Ability
		want []itemsvc.Ability
	}{
		{"a with_item ability with no effect", []itemsvc.Ability{{ID: "use", Limit: withItem}}, []itemsvc.Ability{{ID: "use", Limit: withItem, Effects: []itemsvc.Effect{spell}}}},
		{"after the effects it has", []itemsvc.Ability{{ID: "use", Limit: withItem, Effects: []itemsvc.Effect{dice}}}, []itemsvc.Ability{{ID: "use", Limit: withItem, Effects: []itemsvc.Effect{dice, spell}}}},
		{
			"the first with_item one",
			[]itemsvc.Ability{{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}, {ID: "b", Limit: withItem}},
			[]itemsvc.Ability{{ID: "a", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}, {ID: "b", Limit: withItem, Effects: []itemsvc.Effect{spell}}},
		},
		{"already casting a spell: untouched", []itemsvc.Ability{{ID: "use", Limit: withItem, Effects: []itemsvc.Effect{other}}}, []itemsvc.Ability{{ID: "use", Limit: withItem, Effects: []itemsvc.Effect{other}}}},
		{"no with_item ability: untouched", []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}}, []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}}}},
		{"no ability: none given back", []itemsvc.Ability{}, []itemsvc.Ability{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemsvc.WithScrollSpell(tt.in, "wall_of_force", 13)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.want, itemsvc.WithScrollSpell(got, "wall_of_force", 13), "idempotent")
		})
	}
	assert.Equal(t, []itemsvc.Ability{{ID: "use", Limit: withItem}}, itemsvc.WithScrollSpell([]itemsvc.Ability{{ID: "use", Limit: withItem}}, "", 13), "no spell named")
}

// scrollLevelTable is an explicit table of caster levels (DMG p.145: a 6th-level wizard spell at 13th).
func scrollLevelTable(id string) (int, bool) {
	level, ok := map[string]int{"wall_of_force": 13, "magic_missile": 6}[id]
	return level, ok
}

// ADR 0049 step 9: the stored spell_id of a scroll mirrors the first spell effect of its abilities (the
// previous release still reads it): set by an edit of the abilities, cleared with the effect, and left
// alone by a piece that is not a scroll.
func TestUpdate_ADR0049Step9_AScrollsStoredSpellMirrorsItsSpellEffect(t *testing.T) {
	castsWall := []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: "wall_of_force", Level: 13}}}}
	plain := []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}}}
	for _, tt := range []struct {
		name      string
		stored    string
		abilities []itemsvc.Ability
		want      string
	}{
		{"set from the effect", "", castsWall, "wall_of_force"},
		{"cleared with the effect", "wall_of_force", plain, ""},
		{"replaced by another effect", "magic_missile", castsWall, "wall_of_force"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := deps(t)
			f.Spells = spellsKnowing(t, map[string]string{"wall_of_force": "1", "magic_missile": "1"})
			svc, _ := itemsvc.New(dal, f.Deps)
			old := withUse(scroll, itemsvc.Limit{Kind: itemsvc.LimitWithItem})
			old.SpellID = tt.stored
			dal.EXPECT().Get(mock.Anything, int64(1), "scroll").Return(old, nil)
			var got itemsvc.Item
			dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil).Once()
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil)

			_, err := svc.Update(context.Background(), 1, "scroll", itemsvc.ItemPatch{Abilities: &tt.abilities})
			require.NoError(t, err)
			assert.Equal(t, tt.want, got.SpellID)
		})
	}
	t.Run("a wand holds no spell column", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		f.Spells = spellsKnowing(t, map[string]string{"wall_of_force": "1"})
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "wand_lightning").Return(wand4, nil)
		var got itemsvc.Item
		dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil).Once()
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil)
		wandAbilities := []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}, Effects: castsWall[0].Effects}}
		_, err := svc.Update(context.Background(), 1, "wand_lightning", itemsvc.ItemPatch{Abilities: &wandAbilities})
		require.NoError(t, err)
		assert.Empty(t, got.SpellID)
	})
}

// scrollBefore is the "before" of an entry recorded when a scroll named its spell but before the spell
// effect existed: abilities (as slice 6 wrote them) with no effect, and the spell in the piece.
const scrollBefore = `{"count":1,"replace":true,"item":{"position":2,"name":{"en":"Scroll"},"type":"","behaviour":"stock","magicalBonus":0,"ammoKind":"","maxCount":1,"effectDice":"","effectSave":"","notes":"","powers":[],"abilities":[{"id":"use","limit":{"kind":"with_item"},"effects":[]}],"piece":{"category":"magic","magicKind":"scroll","spellId":"wall_of_force","state":"carried"}}}`

// scrollBeforeAbilities is the same scroll recorded by a release before abilities: no abilities key.
const scrollBeforeAbilities = `{"count":1,"replace":true,"item":{"position":2,"name":{"en":"Scroll"},"type":"","behaviour":"stock","magicalBonus":0,"ammoKind":"","maxCount":1,"effectDice":"","effectSave":"","notes":"","powers":[],"piece":{"category":"magic","magicKind":"scroll","spellId":"wall_of_force","state":"carried"}}}`

// ADR 0049 steps 9 and 20: popping an entry that holds a scroll naming its spell, whether recorded
// before abilities or before the spell effect, gives back the scroll with the spell folded into its
// with_item ability at the DMG p.145 level, the same result the boot conversion gives; a snapshot
// whose scroll casts a spell already, or a spell the levels do not know, is written as it was.
func TestRestore_ADR0049Step9_OldScrollSnapshotsGetTheirSpellEffect(t *testing.T) {
	want := []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: "wall_of_force", Level: 13}}}}
	restore := func(t *testing.T, levels itemsvc.ScrollLevels, state string) itemsvc.Item {
		t.Helper()
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		f.ScrollLevels = levels
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "scroll").Return(itemsvc.Item{ID: "scroll", Name: localized.Of("Scroll")}, nil).Maybe()
		var got itemsvc.Item
		dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil).Once()
		require.NoError(t, svc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, snaptest.EnvID("scroll", state)))
		return got
	}
	t.Run("recorded before the spell effect", func(t *testing.T) {
		got := restore(t, scrollLevelTable, scrollBefore)
		assert.Equal(t, want, got.Abilities)
		assert.Equal(t, "wall_of_force", got.SpellID, "the column stays")
	})
	t.Run("recorded before abilities", func(t *testing.T) {
		got := restore(t, scrollLevelTable, scrollBeforeAbilities)
		assert.Equal(t, want, got.Abilities)
	})
	t.Run("a spell the levels do not know", func(t *testing.T) {
		got := restore(t, func(string) (int, bool) { return 0, false }, scrollBefore)
		assert.Empty(t, itemsvc.FirstSpell(got.Abilities))
	})
	t.Run("no levels", func(t *testing.T) {
		got := restore(t, nil, scrollBefore)
		assert.Empty(t, itemsvc.FirstSpell(got.Abilities))
	})
	t.Run("a scroll casting a spell already is as it was", func(t *testing.T) {
		state := `{"count":1,"replace":true,"item":{"position":2,"name":{"en":"Scroll"},"type":"","behaviour":"stock","magicalBonus":0,"ammoKind":"","maxCount":1,"effectDice":"","effectSave":"","notes":"","powers":[],"abilities":[{"id":"use","limit":{"kind":"with_item"},"effects":[{"kind":"spell","spellId":"magic_missile","level":20}]}],"piece":{"category":"magic","magicKind":"scroll","spellId":"magic_missile","state":"carried"}}}`
		got := restore(t, scrollLevelTable, state)
		assert.Equal(t, []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: "magic_missile", Level: 20}}, got.Abilities[0].Effects, "the level the player set stays")
	})
}

// ADR 0049 step 9: a spell effect sent on a scroll with no level is written at the level DMG p.145 gives
// that spell (the editor leaves it to the server, which holds the class data); a level sent is kept; the
// same effect on a wand, which states its own level, is refused.
func TestCreate_ADR0049Step9_AScrollsSpellEffectWithNoLevelTakesTheBooksLevel(t *testing.T) {
	cast := func(level int) []itemsvc.Ability {
		return []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: "wall_of_force", Level: level}}}}
	}
	create := func(t *testing.T, magicKind string, abilities []itemsvc.Ability) (itemsvc.Item, error) {
		t.Helper()
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		f.Spells = spellsKnowing(t, map[string]string{"wall_of_force": "1"})
		f.ScrollLevels = scrollLevelTable
		svc, _ := itemsvc.New(dal, f.Deps)
		var got itemsvc.Item
		dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil).Maybe()
		dal.EXPECT().DeleteReviews(mock.Anything, int64(1), mock.Anything).Return(nil).Maybe()
		dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil).Maybe()
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil).Maybe()
		_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{
			Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: magicKind, Behaviour: itemsvc.BehaviourStock, Count: 1, Abilities: abilities,
		})
		return got, err
	}
	got, err := create(t, itemsvc.MagicScroll, cast(0))
	require.NoError(t, err)
	assert.Equal(t, 13, got.Abilities[0].Effects[0].Level)
	assert.Equal(t, "wall_of_force", got.SpellID, "mirrored")
	got, err = create(t, itemsvc.MagicScroll, cast(20))
	require.NoError(t, err)
	assert.Equal(t, 20, got.Abilities[0].Effects[0].Level, "a level the player sets is kept")
	_, err = create(t, itemsvc.MagicWand, []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitUnlimited}, Effects: cast(0)[0].Effects}})
	require.Error(t, err, "a wand states its own level")
}

// scrollWith is a stored scroll whose ability casts spell at level (none: no spell effect), the column
// as given.
func scrollWith(column, spell string, level int) itemsvc.Item {
	it := withUse(scroll, itemsvc.Limit{Kind: itemsvc.LimitWithItem})
	it.SpellID = column
	if spell != "" {
		it.Abilities[0].Effects = []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: spell, Level: level}}
	}
	return it
}

// ADR 0049 step 9: the old spellId field still works for an older client and carries into the spell
// effect, one mirror path both ways: another spell replaces the effect's spell at the book's level, the
// same spell changes nothing, "" removes the effect; the snapshot holds both, so undo gives back the
// column and the effect.
func TestUpdate_ADR0049Step9_SpellIdAloneCarriesIntoTheEffect(t *testing.T) {
	known := map[string]string{"wall_of_force": "1", "magic_missile": "1"}
	for _, tt := range []struct {
		name    string
		set     string
		want    string // the spell the effect casts after
		level   int
		changes bool
	}{
		{"another spell", "wall_of_force", "wall_of_force", 13, true},
		{"the same spell", "magic_missile", "magic_missile", 9, false},
		{"cleared", "", "", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := deps(t)
			f.Spells = spellsKnowing(t, known)
			f.ScrollLevels = scrollLevelTable
			svc, _ := itemsvc.New(dal, f.Deps)
			old := scrollWith("magic_missile", "magic_missile", 9)
			dal.EXPECT().Get(mock.Anything, int64(1), "scroll").Return(old, nil)
			var got itemsvc.Item
			dal.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil).Maybe()
			var entry actionlog.NewEntry
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { entry = e }).Return(int64(9), nil).Maybe()

			_, err := svc.Update(context.Background(), 1, "scroll", itemsvc.ItemPatch{SpellID: ptr(tt.set)})
			if !tt.changes {
				require.Error(t, err, "nothing changed")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.set, got.SpellID)
			assert.Equal(t, tt.want, itemsvc.FirstSpell(got.Abilities))
			if tt.want != "" {
				assert.Equal(t, tt.level, got.Abilities[0].Effects[0].Level)
			} else {
				assert.Empty(t, got.Abilities[0].Effects)
			}
			// Undo: the snapshot's before writes the old column and the old effect back.
			pop := mocks.NewMockDAL(t)
			popSvc, _ := itemsvc.New(pop, deps(t).Deps)
			var back itemsvc.Item
			pop.EXPECT().Get(mock.Anything, int64(1), "scroll").Return(got, nil).Maybe()
			pop.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { back = it }).Return(nil).Once()
			require.NoError(t, popSvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, entry.Snapshots[0].Before))
			assert.Equal(t, "magic_missile", back.SpellID)
			assert.Equal(t, old.Abilities[0].Effects, back.Abilities[0].Effects)
		})
	}
}

// ADR 0049 step 9: a scroll created with the old spellId field and no abilities gets its with_item
// ability with the spell effect at the book's level.
func TestCreate_ADR0049Step9_SpellIdAndNoAbilitiesGetsItsEffect(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	f.Spells = spellsKnowing(t, map[string]string{"wall_of_force": "1"})
	f.ScrollLevels = scrollLevelTable
	svc, _ := itemsvc.New(dal, f.Deps)
	var got itemsvc.Item
	dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	dal.EXPECT().DeleteReviews(mock.Anything, int64(1), mock.Anything).Return(nil).Maybe()
	dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Run(func(_ context.Context, _ int64, it itemsvc.Item) { got = it }).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil)
	_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{
		Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicScroll,
		Behaviour: itemsvc.BehaviourStock, Count: 1, SpellID: "wall_of_force",
	})
	require.NoError(t, err)
	require.Len(t, got.Abilities, 1)
	assert.Equal(t, itemsvc.LimitWithItem, got.Abilities[0].Limit.Kind)
	assert.Equal(t, []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: "wall_of_force", Level: 13}}, got.Abilities[0].Effects)
}

// ADR 0049 step 10: a spell effect whose spell data cannot be read at its level is refused when saved
// (field abilities), so the item list never has to leave its asks out.
func TestCreate_ADR0049Step10_ALevelTheSpellDataCannotBeReadAtIsRefused(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	f.Spells = spellsKnowing(t, map[string]string{"wall_of_force": "1"})
	f.LevelCheck = func(string, int) error { return assert.AnError }
	svc, _ := itemsvc.New(dal, f.Deps)
	_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{
		Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicScroll,
		Behaviour: itemsvc.BehaviourStock, Count: 1,
		Abilities: []itemsvc.Ability{{ID: "use", Limit: itemsvc.Limit{Kind: itemsvc.LimitWithItem}, Effects: []itemsvc.Effect{{Kind: itemsvc.EffectSpell, SpellID: "wall_of_force", Level: 29}}}},
	})
	var fe *domain.FieldError
	require.ErrorAs(t, err, &fe)
	assert.Equal(t, "abilities", fe.Field)
}
