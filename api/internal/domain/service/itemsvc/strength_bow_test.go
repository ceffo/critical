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
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// A bow's Strength rating (ADR 0033 step 3, the strength bow of ADR 0002 step 4).

var longBow = itemsvc.Item{
	ID: "long_bow", Position: 1, Name: localized.Of("Long Bow"), Category: itemsvc.CategoryWeapon,
	State: itemsvc.StateWielded, Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1,
	WeaponType: itemsvc.TypeLongBow, WeaponKind: itemsvc.WeaponKindMissile, Fires: "arrow", Speed: 8, TwoHanded: true,
}

func rated(i itemsvc.Item, score, exceptional int) itemsvc.Item {
	i.StrengthRating = itemsvc.StrengthRating{Score: score, Exceptional: exceptional}
	return i
}

// ADR 0033 step 3: the rating is set and cleared by an edit, logged as a
// strengthRating change ("18/50", "" for none), held by the snapshots so undo
// gives the previous rating back.
func TestUpdate_ADR0033Step3_ABowsStrengthRatingIsSetAndCleared(t *testing.T) {
	tests := []struct {
		name          string
		old           itemsvc.Item
		set           itemsvc.StrengthRating
		want          itemsvc.Item
		before, after string
	}{
		{"set 18/50", longBow, itemsvc.StrengthRating{Score: 18, Exceptional: 50}, rated(longBow, 18, 50), "", "18/50"},
		{"set 17", longBow, itemsvc.StrengthRating{Score: 17}, rated(longBow, 17, 0), "", "17"},
		{"changed", rated(longBow, 17, 0), itemsvc.StrengthRating{Score: 18, Exceptional: 100}, rated(longBow, 18, 100), "17", "18/100"},
		{"cleared", rated(longBow, 18, 50), itemsvc.StrengthRating{}, longBow, "18/50", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dal := mocks.NewMockDAL(t)
			f := readsCharacter(t)
			svc, _ := itemsvc.New(dal, f.Deps)
			dal.EXPECT().Get(mock.Anything, int64(1), "long_bow").Return(tt.old, nil)
			dal.EXPECT().Update(mock.Anything, int64(1), tt.want).Return(nil).Once()
			var got actionlog.NewEntry
			f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(9), nil)

			set := tt.set
			res, err := svc.Update(context.Background(), 1, "long_bow", itemsvc.ItemPatch{StrengthRating: &set})
			require.NoError(t, err)
			assert.Equal(t, tt.want.StrengthRating, res.Item.StrengthRating)
			assert.Contains(t, string(got.Payload), `"changes":[{"field":"strengthRating","before":"`+tt.before+`","after":"`+tt.after+`"}]`)

			// Undo writes the "before" back: the rating the bow had.
			pop := mocks.NewMockDAL(t)
			popSvc, _ := itemsvc.New(pop, readsCharacter(t).Deps)
			pop.EXPECT().Update(mock.Anything, int64(1), tt.old).Return(nil).Once()
			require.NoError(t, popSvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].Before))
		})
	}
}

// ADR 0033 step 3: a bow that becomes another weapon drops its rating (logged
// with the edit); an edit that leaves the rating alone keeps it.
func TestUpdate_ADR0033Step3_ABowThatStopsBeingOneDropsItsRating(t *testing.T) {
	t.Run("retyped as a crossbow", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := readsCharacter(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		want := longBow
		want.WeaponType = itemsvc.TypeLightCrossbow
		dal.EXPECT().Get(mock.Anything, int64(1), "long_bow").Return(rated(longBow, 18, 0), nil)
		dal.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
		// The weapon type classifies the piece (ADR 0033 step 27): its review is settled.
		dal.EXPECT().DeleteReviews(mock.Anything, int64(1), "long_bow").Return(nil).Once()
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil)

		res, err := svc.Update(context.Background(), 1, "long_bow", itemsvc.ItemPatch{WeaponType: ptr(itemsvc.TypeLightCrossbow)})
		require.NoError(t, err)
		assert.False(t, res.Item.StrengthRating.Rated())
	})
	t.Run("another field edited", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := readsCharacter(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		want := rated(longBow, 18, 0)
		want.Notes = "yew"
		dal.EXPECT().Get(mock.Anything, int64(1), "long_bow").Return(rated(longBow, 18, 0), nil)
		dal.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil)

		_, err := svc.Update(context.Background(), 1, "long_bow", itemsvc.ItemPatch{Notes: ptr("yew")})
		require.NoError(t, err)
	})
}

// ADR 0033 step 3: only a bow carries a rating (strengthRating not_allowed on
// a crossbow, a sword or a piece that is no weapon), a Strength score of 1 to
// 25 with an exceptional part only at 18; nothing is stored.
func TestValidate_ADR0033Step3_OnlyABowHasAStrengthRating(t *testing.T) {
	crossbow := longBow
	crossbow.WeaponType, crossbow.Fires = itemsvc.TypeLightCrossbow, "quarrel"
	sword := itemsvc.Item{
		ID: "sword", Name: localized.Of("Sword"), Category: itemsvc.CategoryWeapon, State: itemsvc.StateCarried,
		Behaviour: itemsvc.BehaviourStock, Count: 1, WeaponType: itemsvc.TypeLongSword, WeaponKind: itemsvc.WeaponKindMelee,
		DamageSM: "1d8", DamageL: "1d12", DamageTypes: []string{"s"},
	}
	untypedBow := longBow
	untypedBow.WeaponType = ""
	gear := itemsvc.Item{ID: "rope", Name: localized.Of("Rope"), Category: itemsvc.CategoryGear, State: itemsvc.StateCarried, Behaviour: itemsvc.BehaviourStock, Count: 1}
	short := longBow
	short.WeaponType = itemsvc.TypeShortBow
	tests := []struct {
		name string
		item itemsvc.Item
		// msg is the field error's message on strengthRating, "" for none.
		msg string
	}{
		{"long bow rated 18/50", rated(longBow, 18, 50), ""},
		{"short bow rated 17", rated(short, 17, 0), ""},
		{"long bow rated 18/00", rated(longBow, 18, 100), ""},
		{"long bow rated 25", rated(longBow, 25, 0), ""},
		{"unrated long bow", longBow, ""},
		{"crossbow", rated(crossbow, 17, 0), itemsvc.CodeNotAllowed},
		{"long sword", rated(sword, 17, 0), itemsvc.CodeNotAllowed},
		{"bow with no weapon type", rated(untypedBow, 17, 0), itemsvc.CodeNotAllowed},
		{"gear", rated(gear, 17, 0), itemsvc.CodeNotAllowed},
		{"score 0 with an exceptional part", rated(longBow, 0, 50), "Strength rating must be between 1 and 25"},
		{"score 26", rated(longBow, 26, 0), "Strength rating must be between 1 and 25"},
		{"17/50", rated(longBow, 17, 50), "Only a Strength of 18 has an exceptional part"},
		{"18/101", rated(longBow, 18, 101), "Exceptional Strength must be between 1 and 100"},
		{"18/-1", rated(longBow, 18, -1), "Exceptional Strength must be between 1 and 100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.Validate()
			if tt.msg == "" {
				require.NoError(t, err)
				return
			}
			var fe *domain.FieldError
			require.ErrorAs(t, err, &fe)
			assert.Equal(t, "strengthRating", fe.Field)
			assert.Equal(t, tt.msg, fe.Message)
		})
	}
}

// ADR 0002 step 4.1 (proposed): a wielded bow rated for exceptional Strength
// (18/01 or more) above the character's Strength warns bow_too_strong (PHB
// p.73: a bend bars/lift gates roll to use it, which the app does not make);
// a rating of 18 or less never warns, and neither does a carried bow.
func TestBowWarnings_Step4_1_ABowBuiltForMoreThanTheArchersExceptionalStrength(t *testing.T) {
	carried := rated(longBow, 18, 90)
	carried.State = itemsvc.StateCarried
	tests := []struct {
		name     string
		str, exc int
		bow      itemsvc.Item
		warn     bool
	}{
		{"17 with a bow rated 18/50", 17, 0, rated(longBow, 18, 50), true},
		{"18/50 with a bow rated 18/90", 18, 50, rated(longBow, 18, 90), true},
		{"18/90 with a bow rated 18/90", 18, 90, rated(longBow, 18, 90), false},
		{"18/00 with a bow rated 18/90", 18, 100, rated(longBow, 18, 90), false},
		{"19 with a bow rated 18/00", 19, 0, rated(longBow, 18, 100), false},
		{"18 with a bow rated 19", 18, 100, rated(longBow, 19, 0), true},
		{"12 with a bow rated 18", 12, 0, rated(longBow, 18, 0), false},
		{"12 with a bow rated 17", 12, 0, rated(longBow, 17, 0), false},
		{"12 with an unrated bow", 12, 0, longBow, false},
		{"12 with a carried bow rated 18/90", 12, 0, carried, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := itemsvc.BowWarnings(tt.str, tt.exc, []itemsvc.Item{tt.bow})
			if tt.warn {
				assert.Equal(t, []itemsvc.Warning{{ItemID: tt.bow.ID, Code: itemsvc.WarnBowTooStrong}}, got)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}
