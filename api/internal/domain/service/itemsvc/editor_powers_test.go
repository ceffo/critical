package itemsvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc/mocks"
)

// chainMail is a worn body armour without magic.
var chainMail = itemsvc.Item{
	ID: "chain_mail", Position: 2, Name: localized.Of("Chain mail"), Category: itemsvc.CategoryArmour, State: itemsvc.StateWorn,
	Behaviour: itemsvc.BehaviourStock, Count: 1, MaxCount: 1, BaseAC: 5, SkillColumn: "metal", ArmorRow: "chain",
}

// ADR 0033 step 17: the editor writes powers. An edit's powers replace the
// stored list; the change is the piece's one item_edit entry, with a powers
// change (the stored encodings), and its snapshot carries the powers before
// and after, so popping it restores them (ADR 0016 step 15's rule unchanged).
func TestEditor_ADR0033Step17_AnEditSetsThePowersAndItsSnapshotRestoresThem(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "chain_mail").Return(chainMail, nil).Once()
	want := chainMail
	want.Powers = []itemsvc.Power{itemsvc.MagicPower(2)}
	dal.EXPECT().Update(mock.Anything, int64(1), want).Return(nil).Once()
	var got actionlog.NewEntry
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Run(func(_ context.Context, e actionlog.NewEntry) { got = e }).Return(int64(7), nil).Once()

	powers := []itemsvc.Power{itemsvc.MagicPower(2)}
	res, err := svc.Update(context.Background(), 1, "chain_mail", itemsvc.ItemPatch{Powers: &powers})
	require.NoError(t, err)
	assert.Equal(t, want.Powers, res.Item.Powers)
	assert.Equal(t, actionlog.KindItemEdit, got.Kind)
	var rec struct {
		Changes []itemsvc.FieldChange `json:"changes"`
	}
	require.NoError(t, json.Unmarshal(got.Payload, &rec))
	assert.Equal(t, []itemsvc.FieldChange{{Field: "powers", Before: "[]", After: `[{"id":"magic","effects":{"ac":-2}}]`}}, rec.Changes)

	require.Len(t, got.Snapshots, 1)
	var after map[string]any
	require.NoError(t, json.Unmarshal([]byte(snaptest.State(t, got.Snapshots[0].After)), &after))
	assert.JSONEq(t, `[{"id":"magic","effects":{"ac":-2}}]`, mustJSON(t, after["item"].(map[string]any)["powers"]))

	// Popping the edit writes the armour back without its +N.
	restore := mocks.NewMockDAL(t)
	rsvc, _ := itemsvc.New(restore, deps(t).Deps)
	restore.EXPECT().Get(mock.Anything, int64(1), "chain_mail").Return(want, nil).Maybe()
	restore.EXPECT().Update(mock.Anything, int64(1), chainMail).Return(nil).Once()
	require.NoError(t, rsvc.Module().Restorer(itemsvc.StateKeyItems).Restore(context.Background(), 1, got.Snapshots[0].Before))
}

// ADR 0033 step 17: an edit without powers keeps them (ADR 0016 step 15),
// the same powers again is no change, and an empty list removes them.
func TestEditor_ADR0033Step17_AbsentKeepsSameIsNoChangeEmptyRemoves(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	dal.EXPECT().Get(mock.Anything, int64(1), "power_gem_dispel").Return(gem, nil)

	same := []itemsvc.Power{hammer}
	_, err := svc.Update(context.Background(), 1, "power_gem_dispel", itemsvc.ItemPatch{Powers: &same})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "nothing changed")

	cleared := gem
	cleared.Powers = nil
	dal.EXPECT().Update(mock.Anything, int64(1), cleared).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(8), nil).Once()
	none := []itemsvc.Power{}
	res, err := svc.Update(context.Background(), 1, "power_gem_dispel", itemsvc.ItemPatch{Powers: &none})
	require.NoError(t, err)
	assert.Nil(t, res.Item.Powers)
}

// ADR 0033 step 17: validation is the seed's (ADR 0016 step 9) and the
// reserved magic power is an armour's or a shield's "+N" only, one ac effect
// of -1 to -10, no trigger, no condition. Each refusal names the field
// powers, before anything is stored or logged; a power sent without an id
// gets the first free power_<n>.
func TestEditor_ADR0033Step17_PowersValidatedAsTheSeeds(t *testing.T) {
	ring := itemsvc.ItemInput{
		Name: localized.Of("Ring"), Category: itemsvc.CategoryMagic, MagicKind: itemsvc.MagicRing,
		Behaviour: itemsvc.BehaviourStock, Count: 1,
	}
	armour := itemsvc.ItemInput{
		Name: localized.Of("Plate"), Category: itemsvc.CategoryArmour, Behaviour: itemsvc.BehaviourStock, Count: 1,
		BaseAC: 3, SkillColumn: "metal", ArmorRow: "plate",
	}
	with := func(in itemsvc.ItemInput, ps ...itemsvc.Power) itemsvc.ItemInput {
		in.Powers = ps
		return in
	}
	magic := func(mut func(*itemsvc.Power)) itemsvc.Power {
		p := itemsvc.MagicPower(1)
		mut(&p)
		return p
	}
	refused := map[string]itemsvc.ItemInput{
		"a Strength effect":         with(ring, itemsvc.Power{ID: "x", Effects: buffsvc.BuffEffects{StrBonus: new(2)}}),
		"min_str as an effect":      with(ring, itemsvc.Power{ID: "x", Effects: buffsvc.BuffEffects{MinStr: new(18)}}),
		"a trigger out of range":    with(ring, itemsvc.Power{ID: "x", MinStr: new(26), Effects: buffsvc.BuffEffects{ToHit: new(1)}}),
		"a duplicate id":            with(ring, itemsvc.Power{ID: "x", Effects: buffsvc.BuffEffects{ToHit: new(1)}}, itemsvc.Power{ID: "x", Effects: buffsvc.BuffEffects{Damage: new(1)}}),
		"an id that is no slug":     with(ring, itemsvc.Power{ID: "Big Hit", Effects: buffsvc.BuffEffects{ToHit: new(1)}}),
		"a bad weapon kind":         with(ring, itemsvc.Power{ID: "x", Effects: buffsvc.BuffEffects{ToHit: new(1), WeaponKind: "thrown"}}),
		"a base_ac out of range":    with(ring, itemsvc.Power{ID: "x", Effects: buffsvc.BuffEffects{BaseAC: new(11)}}),
		"magic on a ring":           with(ring, itemsvc.MagicPower(1)),
		"magic with a trigger":      with(armour, magic(func(p *itemsvc.Power) { p.MinStr = new(18) })),
		"magic with another effect": with(armour, magic(func(p *itemsvc.Power) { p.Effects.SaveBonus = new(1) })),
		"magic with a condition":    with(armour, magic(func(p *itemsvc.Power) { p.Effects.WithoutMagicArmour = true })),
		"magic that worsens the AC": with(armour, magic(func(p *itemsvc.Power) { p.Effects.AC = new(1) })),
		"magic of +11":              with(armour, itemsvc.MagicPower(11)),
		"magic without an ac":       with(armour, magic(func(p *itemsvc.Power) { p.Effects.AC = nil })),
	}
	for name, in := range refused {
		t.Run(name, func(t *testing.T) {
			svc, _ := itemsvc.New(mocks.NewMockDAL(t), deps(t).Deps)
			_, err := svc.Create(context.Background(), 1, in)
			var fe *domain.FieldError
			require.True(t, errors.As(err, &fe), "%v", err)
			assert.Equal(t, "powers", fe.Field)
		})
	}

	t.Run("a power without an id gets one; an armour's +10 is fine", func(t *testing.T) {
		dal := mocks.NewMockDAL(t)
		f := deps(t)
		svc, _ := itemsvc.New(dal, f.Deps)
		dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
		dal.EXPECT().Insert(mock.Anything, int64(1), mock.Anything).Return(nil)
		dal.EXPECT().DeleteReviews(mock.Anything, int64(1), mock.Anything).Return(nil)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
		res, err := svc.Create(context.Background(), 1, with(ring,
			itemsvc.Power{Effects: buffsvc.BuffEffects{SaveBonus: new(1)}},
			itemsvc.Power{ID: "power_1", Effects: buffsvc.BuffEffects{ToHit: new(1)}},
			itemsvc.Power{Effects: buffsvc.BuffEffects{BaseAC: new(4)}},
		))
		require.NoError(t, err)
		ids := []string{}
		for _, p := range res.Item.Powers {
			ids = append(ids, p.ID)
		}
		assert.Equal(t, []string{"power_2", "power_1", "power_3"}, ids)
		_, err = svc.Create(context.Background(), 1, with(armour, itemsvc.MagicPower(10)))
		require.NoError(t, err)
	})
}

// ADR 0033 step 17: the "+N" is an armour's or a shield's. An edit that
// makes the piece something else drops its magic power (logged as the
// edit's powers change); its other powers stay.
func TestEditor_ADR0033Step17_ACategoryChangeDropsThePlusN(t *testing.T) {
	dal := mocks.NewMockDAL(t)
	f := deps(t)
	svc, _ := itemsvc.New(dal, f.Deps)
	other := itemsvc.Power{ID: "warmth", Effects: buffsvc.BuffEffects{SaveBonus: new(1)}}
	old := chainMail
	old.State = itemsvc.StateCarried
	old.Powers = []itemsvc.Power{itemsvc.MagicPower(1), other}
	dal.EXPECT().Get(mock.Anything, int64(1), "chain_mail").Return(old, nil).Once()
	dal.EXPECT().DeleteReviews(mock.Anything, int64(1), "chain_mail").Return(nil).Once()
	dal.EXPECT().Update(mock.Anything, int64(1), mock.MatchedBy(func(it itemsvc.Item) bool {
		return assert.Equal(t, []itemsvc.Power{other}, it.Powers)
	})).Return(nil).Once()
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(9), nil).Once()
	gear := itemsvc.CategoryGear
	_, err := svc.Update(context.Background(), 1, "chain_mail", itemsvc.ItemPatch{Category: &gear})
	require.NoError(t, err)
}

// ADR 0033 steps 17 and 24: a pick reads the powers too, so an armour or a
// shield picked from the catalogue keeps the "+N" given with it.
func TestCreate_ADR0033Step17_PickedArmourKeepsItsPlusN(t *testing.T) {
	svc, dal, f := pickSvc(t)
	dal.EXPECT().List(mock.Anything, int64(1)).Return(nil, nil)
	dal.EXPECT().Insert(mock.Anything, int64(1), mock.MatchedBy(func(it itemsvc.Item) bool {
		return assert.Equal(t, []itemsvc.Power{itemsvc.MagicPower(3)}, it.Powers)
	})).Return(nil).Once()
	dal.EXPECT().DeleteReviews(mock.Anything, int64(1), mock.Anything).Return(nil)
	f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(int64(1), nil)
	_, err := svc.Create(context.Background(), 1, itemsvc.ItemInput{CatalogueCode: "body_shield", Powers: []itemsvc.Power{itemsvc.MagicPower(3)}})
	require.NoError(t, err)
}
