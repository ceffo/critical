package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
)

// mindRow is a row holding target's mind (ADR 0044 step 3): tangents 1 or
// 2, or 0 for full contact, kept for kept rounds already.
func mindRow(id, target string, tangents, kept int) buffsvc.BuffEffect {
	return buffsvc.BuffEffect{
		ID: id, Name: target, Target: target, Tangents: tangents,
		MaintenanceCost: buffsvc.LinkMaintenanceCost, MaintenancePeriod: 1, RoundsKept: kept,
	}
}

// savesAny accepts one SaveActiveBuffs and captures the rows written.
func (f *fixture) savesAny(out *[]buffsvc.BuffEffect) {
	f.dal.EXPECT().SaveActiveBuffs(mock.Anything, charID, mock.Anything).RunAndReturn(
		func(_ context.Context, _ int64, rows []buffsvc.BuffEffect) error {
			*out = rows
			return nil
		}).Once()
}

// ADR 0044 step 3: a won contest puts one tangent row on, maintained at 1
// PSP a round; the third tangent turns it into a contact row; contact rows
// are one per target, any number, and each row is charged 1 PSP from the
// Next Round after the round it was made.
func TestTangents_Step3_ARowPerTargetChargedOnePSP(t *testing.T) {
	ctx := context.Background()
	bless := buffsvc.BuffEffect{ID: "x", Name: "Bless"}

	t.Run("the first tangent puts the row on", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{bless}, 3)
		var saved []buffsvc.BuffEffect
		f.savesAny(&saved)
		ch, snap, err := f.svc.AddTangents(ctx, charID, "  Grey Ogre ", 1)
		require.NoError(t, err)
		require.Len(t, saved, 2)
		row := saved[1]
		assert.NotEmpty(t, row.ID)
		want := mindRow(row.ID, "Grey Ogre", 1, 0)
		want.StartRound = 3
		assert.Equal(t, want, row, "maintained at 1 PSP every round, from its round")
		assert.True(t, row.TangentRow())
		assert.True(t, row.Maintained())
		assert.Equal(t, buffsvc.TangentsChange{Target: "Grey Ogre", Before: 0, After: 1, Row: row}, ch)
		assert.NotEmpty(t, snap.Before, "the attack's entry records it")
	})

	t.Run("the third tangent is full contact, the same row", func(t *testing.T) {
		f := newFixture(t)
		held := mindRow("o", "Orc", 1, 2)
		f.state([]buffsvc.BuffEffect{held, bless}, 5)
		contact := mindRow("o", "Orc", 0, 2)
		f.saves([]buffsvc.BuffEffect{contact, bless})
		ch, _, err := f.svc.AddTangents(ctx, charID, "Orc", 2)
		require.NoError(t, err)
		assert.True(t, ch.Row.ContactRow())
		assert.Equal(t, buffsvc.TangentsChange{Target: "Orc", Before: 1, After: 3, Contact: true, Row: contact}, ch,
			"its id, start and rounds kept stay: it was paid for")
	})

	t.Run("a target typed in another case is the same row", func(t *testing.T) {
		f := newFixture(t)
		held := mindRow("o", "Orc", 1, 2)
		f.state([]buffsvc.BuffEffect{held}, 5)
		more := mindRow("o", "Orc", 2, 2)
		f.saves([]buffsvc.BuffEffect{more})
		ch, _, err := f.svc.AddTangents(ctx, charID, "orc", 1)
		require.NoError(t, err)
		assert.Nil(t, ch.Broken, "no other mind: nothing breaks")
		assert.Equal(t, buffsvc.TangentsChange{Target: "Orc", Before: 1, After: 2, Row: more}, ch)
		assert.Equal(t, 1, buffsvc.TangentsOn([]buffsvc.BuffEffect{held}, "ORC"), "TangentsOn ignores case")
	})

	t.Run("a second tangent past contact counts for nothing", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{mindRow("o", "Orc", 2, 1)}, 5)
		contact := mindRow("o", "Orc", 0, 1)
		contact.ContactFrom = 1 // contact made at its rounds kept (step 10)
		f.saves([]buffsvc.BuffEffect{contact})
		ch, _, err := f.svc.AddTangents(ctx, charID, "Orc", 2)
		require.NoError(t, err)
		assert.Equal(t, 3, ch.After)
		assert.True(t, ch.Contact)
	})

	t.Run("a contacted mind takes no tangent: renewed, never doubled", func(t *testing.T) {
		f := newFixture(t)
		ogre, troll := mindRow("g", "Ogre", 0, 4), mindRow("t", "Troll", 0, 1)
		f.state([]buffsvc.BuffEffect{ogre, troll}, 5)
		ch, snap, err := f.svc.AddTangents(ctx, charID, "Troll", 1)
		require.NoError(t, err)
		assert.Equal(t, buffsvc.TangentsChange{Target: "Troll", Before: 3, After: 3, Row: troll}, ch)
		assert.Empty(t, snap.Before, "nothing written: the contact rows of two minds stay")
	})

	t.Run("Next Round charges each row 1 PSP, not in the round it is made", func(t *testing.T) {
		f := newFixture(t)
		orc, ogre, troll := mindRow("o", "Orc", 2, 1), mindRow("g", "Ogre", 0, 3), mindRow("t", "Troll", 0, 0)
		f.pspIs(20)
		active, expired, e := f.advances(t, []buffsvc.BuffEffect{orc, ogre, troll}, 4)
		assert.Equal(t, []buffsvc.BuffEffect{keptMore(orc, 1), keptMore(ogre, 1), keptMore(troll, 1)}, active)
		assert.Empty(t, expired)
		assert.Equal(t, -2, e.PSPDelta, "Orc and Ogre; Troll's contact was made this round")
		assert.JSONEq(t, `{"action":"next_round","fromRound":4,"toRound":5,
			"maintenance":[{"powerId":"","target":"Orc","cost":1},{"powerId":"","target":"Ogre","cost":1}]}`, string(e.Payload))
	})

	t.Run("a label is required", func(t *testing.T) {
		f := newFixture(t)
		_, _, err := f.svc.AddTangents(ctx, charID, "  ", 1)
		require.ErrorIs(t, err, domain.ErrInvalidInput)
	})
}

// ADR 0044 step 3.3: an attack mode used on another mind ends the tangent
// row (one mind at a time), won or lost; contact rows stay.
func TestTangents_Step3_AnotherTargetBreaksThem(t *testing.T) {
	ctx := context.Background()
	orc, ogre := mindRow("o", "Orc", 2, 3), mindRow("g", "Ogre", 0, 3)

	t.Run("a lost attack on another mind", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{orc, ogre}, 6)
		f.saves([]buffsvc.BuffEffect{ogre})
		ch, snap, err := f.svc.AddTangents(ctx, charID, "Troll", 0)
		require.NoError(t, err)
		require.NotNil(t, ch.Broken)
		assert.Equal(t, orc, *ch.Broken)
		assert.Equal(t, 0, ch.After)
		assert.NotEmpty(t, snap.Before)
	})

	t.Run("a won attack on another mind", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{orc, ogre}, 6)
		var saved []buffsvc.BuffEffect
		f.savesAny(&saved)
		ch, _, err := f.svc.AddTangents(ctx, charID, "Troll", 1)
		require.NoError(t, err)
		require.Len(t, saved, 2)
		assert.Equal(t, ogre, saved[0])
		assert.Equal(t, "Troll", saved[1].Target)
		assert.Equal(t, 1, saved[1].Tangents)
		assert.Equal(t, orc, *ch.Broken)
	})

	t.Run("an attack on the contacted mind", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{orc, ogre}, 6)
		f.saves([]buffsvc.BuffEffect{ogre})
		ch, _, err := f.svc.AddTangents(ctx, charID, "Ogre", 1)
		require.NoError(t, err)
		assert.Equal(t, orc, *ch.Broken, "the contact row is another target too")
	})

	t.Run("the same mind keeps them", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{orc, ogre}, 6)
		ch, _, err := f.svc.AddTangents(ctx, charID, "Orc", 0)
		require.NoError(t, err)
		assert.Nil(t, ch.Broken)
		assert.Equal(t, 2, ch.After)
	})
}

// ADR 0044 step 3.3: a charge the PSPs left cannot pay ends the row, logged
// as an unpaid mind; a row after it may still pay.
func TestTangents_Step3_UnpaidBreaks(t *testing.T) {
	f := newFixture(t)
	levitation := maintained("l", "levitation", 3, 1, 2)
	orc, ogre := mindRow("o", "Orc", 1, 1), mindRow("g", "Ogre", 0, 2)
	f.pspIs(1)

	active, expired, e := f.advances(t, []buffsvc.BuffEffect{levitation, orc, ogre}, 7)

	assert.Equal(t, []buffsvc.BuffEffect{keptMore(orc, 1)}, active, "Orc pays the last PSP")
	assert.Equal(t, []buffsvc.BuffEffect{levitation, ogre}, expired)
	assert.Equal(t, -1, e.PSPDelta)
	assert.JSONEq(t, `{"action":"next_round","fromRound":7,"toRound":8,
		"maintenance":[{"powerId":"","target":"Orc","cost":1}],
		"maintenanceUnpaid":["levitation"],"unpaidTargets":["Ogre"],
		"powerNames":{"levitation":{"en":"En levitation","fr":"Fr levitation"}}}`, string(e.Payload))
}

// ADR 0044 step 3: a buff named like a mind's label neither renews nor is
// renewed by its row, and a buff entered by hand holds no mind.
func TestTangents_Step3_ABuffNamedLikeTheTargetLeavesTheRow(t *testing.T) {
	f := newFixture(t)
	orc := mindRow("o", "Orc", 1, 1)
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{orc}, nil).Once()
	f.dal.EXPECT().GetRoundNumber(mock.Anything, charID).Return(2, nil).Once()
	named := buffsvc.BuffEffect{ID: "n", Name: "orc"}
	want := named
	want.StartRound = 2
	f.saves([]buffsvc.BuffEffect{orc, want})
	_, _, err := f.svc.Add(context.Background(), charID, named)
	require.NoError(t, err)
}
