package itemsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

var scroll = itemsvc.Item{
	ID: "scroll", Position: 2, Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicScroll,
	State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
}

// spellsKnowing is a spell catalogue that knows the spells of casting times
// (id -> PHB casting time) and no other.
func spellsKnowing(t *testing.T, castingTimes map[string]string) *mocks.MockSpells {
	t.Helper()
	m := mocks.NewMockSpells(t)
	m.EXPECT().Describe(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, id string) (spellsvc.Spell, bool, error) {
			raw, ok := castingTimes[id]
			if !ok {
				return spellsvc.Spell{}, false, nil
			}
			return spellsvc.Spell{ID: id, Stats: spellstats.Stats{CastingTime: spellstats.ParseCastingTime(raw)}}, true, nil
		}).Maybe()
	return m
}

// ADR 0033 step 2 (a scroll names its spell): the spell is set and cleared by
// an edit, logged as a spellId change, held by the snapshots so undo gives
// the previous spell back.
func TestUpdate_ADR0033Step2_AScrollNamesItsSpell(t *testing.T) {
	named := scroll
	named.SpellID = "magic_missile"
	tests := []struct {
		name          string
		old           itemsvc.Item
		set           string
		want          itemsvc.Item
		before, after string
	}{
		{"set", scroll, " Magic_Missile ", named, "", "magic_missile"},
		{"cleared", named, "", scroll, "magic_missile", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := deps(t)
			f.Spells = spellsKnowing(t, map[string]string{"magic_missile": "1"})
			svc, _ := itemsvc.New(dal, f.Deps)
			dal.EXPECT().Get(mock.Anything, int64(1), "scroll").Return(tt.old, nil)
			dal.EXPECT().Update(mock.Anything, int64(1), tt.want).Return(nil).Once()
			var got actionlog.NewEntry
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(9), nil)

			res, err := svc.Update(context.Background(), 1, "scroll", itemsvc.ItemPatch{SpellID: ptr(tt.set)})
			require.NoError(t, err)
			assert.Equal(t, tt.want.SpellID, res.Item.SpellID)
			assert.Contains(t, string(got.Payload), `"changes":[{"field":"spellId","before":"`+tt.before+`","after":"`+tt.after+`"}]`)

			// Undo writes the "before" back: the spell the scroll had.
			pop := mocks.NewMockDAL(t)
			popSvc, _ := itemsvc.New(pop, deps(t).Deps)
			pop.EXPECT().Update(mock.Anything, int64(1), tt.old).Return(nil).Once()
			require.NoError(t, popSvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].Before))
		})
	}
}

// ADR 0033 step 2: only a magic scroll holds a spell (spellId not_allowed),
// and it is a spell of the catalogue (spellId unknown); nothing is stored.
func TestItems_ADR0033Step2_OnlyAScrollNamesAKnownSpell(t *testing.T) {
	refused := func(t *testing.T, err error, code string) {
		t.Helper()
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "spellId", fe.Field)
		assert.Equal(t, code, fe.Message)
	}
	known := map[string]string{"magic_missile": "1"}
	t.Run("a wand names no spell", func(t *testing.T) {
		f := deps(t)
		f.Spells = spellsKnowing(t, known)
		svc, _ := itemsvc.New(mocks.NewMockDAL(t), f.Deps)
		_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{
			Name: localized.Of("Wand"), Category: itemsvc.CategoryMagic, MagicKind: "wand", SpellID: "magic_missile",
			Behaviour: itemsvc.BehaviourCharges, Count: 3,
		})
		refused(t, err, itemsvc.CodeNotAllowed)
	})
	t.Run("an unknown spell on a new scroll", func(t *testing.T) {
		f := deps(t)
		f.Spells = spellsKnowing(t, known)
		svc, _ := itemsvc.New(mocks.NewMockDAL(t), f.Deps)
		_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{
			Name: localized.Of("Scroll"), Category: itemsvc.CategoryMagic, MagicKind: "scroll", SpellID: "no_such_spell",
			Behaviour: itemsvc.BehaviourStock, Count: 1,
		})
		refused(t, err, itemsvc.CodeUnknown)
	})
	t.Run("an unknown spell set by an edit", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		f.Spells = spellsKnowing(t, known)
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "scroll").Return(scroll, nil)
		_, err := svc.Update(context.Background(), 1, "scroll", itemsvc.ItemPatch{SpellID: ptr("no_such_spell")})
		refused(t, err, itemsvc.CodeUnknown)
	})
	t.Run("a spell set on a wand by an edit", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		f.Spells = spellsKnowing(t, known)
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().Get(mock.Anything, int64(1), "wand_lightning").Return(wand4, nil)
		_, err := svc.Update(context.Background(), 1, "wand_lightning", itemsvc.ItemPatch{SpellID: ptr("magic_missile")})
		refused(t, err, itemsvc.CodeNotAllowed)
	})
	t.Run("a scroll that becomes a potion drops its spell", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		named := scroll
		named.SpellID = "magic_missile"
		dal.EXPECT().Get(mock.Anything, int64(1), "scroll").Return(named, nil)
		after := scroll
		after.MagicKind = "potion"
		dal.EXPECT().Update(mock.Anything, int64(1), after).Return(nil).Once()
		dal.EXPECT().DeleteReviews(mock.Anything, int64(1), "scroll").Return(nil).Once() // a new kind settles it
		var got actionlog.NewEntry
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(9), nil)
		_, err := svc.Update(context.Background(), 1, "scroll", itemsvc.ItemPatch{MagicKind: ptr("potion")})
		require.NoError(t, err)
		assert.Contains(t, string(got.Payload), `{"field":"spellId","before":"magic_missile","after":""}`)
	})
}

// ADR 0035 item 6.2 with ADR 0033 step 15.1 (a scroll as the spell it
// holds): using a scroll that names its spell corrects the round's
// initiative with that spell's casting time, by the cast's rules
// (buffsvc.ScrollInitiative); a spell the catalogue no longer knows reads as
// special; an unnamed scroll is still not counted.
func TestUseItem_ADR0035Item6_2_ANamedScrollAddsItsSpellsCastingTime(t *testing.T) {
	catalogue := map[string]string{"magic_missile": "1", "fireball": "3", "identify": "1 rd."}
	term := func(spell string, value int, timing buffsvc.CastTiming) buffsvc.InitiativeModifier {
		return buffsvc.InitiativeModifier{
			Source: buffsvc.InitiativeItem, Value: value, ItemID: "scroll", Name: "Scroll", MagicKind: "scroll",
			SpellID: spell, Timing: timing,
		}
	}
	tests := []struct {
		spell string
		want  buffsvc.InitiativeModifier
	}{
		{"magic_missile", term("magic_missile", 1, buffsvc.TimingSegments)},
		{"fireball", term("fireball", 3, buffsvc.TimingSegments)},
		{"identify", term("identify", 0, buffsvc.TimingEndOfRound)},
		{"gone_from_the_catalogue", term("gone_from_the_catalogue", 0, buffsvc.TimingSpecial)},
		{"", term("", 0, buffsvc.TimingSpellCastingTime)},
	}
	for _, tt := range tests {
		t.Run("spell "+tt.spell, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := deps(t)
			f.InitiativeItems = table56(t)
			f.Spells = spellsKnowing(t, catalogue)
			svc, err := itemsvc.New(dal, f.Deps)
			require.NoError(t, err)
			used := scroll
			used.SpellID = tt.spell
			used.Count = 0
			dal.EXPECT().Decrement(mock.Anything, int64(1), "scroll").Return(used, nil)
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil).Once()
			f.initiative.EXPECT().CorrectInitiative(mock.Anything, int64(1), tt.want).Return(nil, nil).Once()

			_, err = svc.UseItem(context.Background(), 1, "scroll")
			require.NoError(t, err)
		})
	}
}
