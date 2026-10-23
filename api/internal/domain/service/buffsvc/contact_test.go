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

// contactOn is a contact row on target kept for kept rounds, contact made
// at from (ADR 0044 step 10).
func contactOn(id, target string, kept, from int) buffsvc.BuffEffect {
	b := mindRow(id, target, 0, kept)
	b.ContactFrom = from
	return b
}

// usedOn is inflict pain's row (2 PSPs a round) kept up through the contact
// with target.
func usedOn(id, target string, kept int) buffsvc.BuffEffect {
	b := maintained(id, "inflict_pain", 2, 1, kept)
	b.Target = target
	return b
}

// ADR 0044 step 9: the contact power's success turns target's tangent row
// into a contact row (its id and rounds kept stay, contact made at its
// rounds kept) or puts a new one on; a mind already contacted is renewed,
// nothing written; another mind's tangent row stays.
func TestMakeContact_Step9_TurnsOrPutsTheContactRow(t *testing.T) {
	ctx := context.Background()

	t.Run("a new row", func(t *testing.T) {
		f := newFixture(t)
		goblin := mindRow("g", "Goblin", 1, 1)
		f.state([]buffsvc.BuffEffect{goblin}, 4)
		var saved []buffsvc.BuffEffect
		f.savesAny(&saved)
		ch, snap, err := f.svc.MakeContact(ctx, charID, " Orc ")
		require.NoError(t, err)
		require.Len(t, saved, 2)
		assert.Equal(t, goblin, saved[0], "another mind's tangents stay")
		row := saved[1]
		want := mindRow(row.ID, "Orc", 0, 0)
		want.StartRound = 4
		assert.Equal(t, want, row)
		assert.True(t, row.ContactRow())
		assert.Equal(t, buffsvc.TangentsChange{Target: "Orc", Before: 0, After: buffsvc.MaxTangents + 1, Contact: true, Row: row}, ch)
		assert.Equal(t, buffsvc.StateKeyBuffs, snap.Key)
	})

	t.Run("a tangent row turned", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{mindRow("o", "Orc", 2, 3)}, 6)
		turned := contactOn("o", "Orc", 3, 3)
		f.saves([]buffsvc.BuffEffect{turned})
		ch, snap, err := f.svc.MakeContact(ctx, charID, "orc")
		require.NoError(t, err)
		assert.Equal(t, buffsvc.TangentsChange{Target: "Orc", Before: 2, After: buffsvc.MaxTangents + 1, Contact: true, Row: turned}, ch,
			"the row's label, whatever the case typed")
		assert.NotEmpty(t, snap.Before)
	})

	t.Run("a mind already contacted: renewed, nothing written", func(t *testing.T) {
		f := newFixture(t)
		held := contactOn("o", "Orc", 5, 1)
		f.state([]buffsvc.BuffEffect{held}, 6)
		ch, snap, err := f.svc.MakeContact(ctx, charID, "Orc")
		require.NoError(t, err)
		assert.Equal(t, buffsvc.TangentsChange{Target: "Orc", Before: 3, After: 3, Row: held}, ch)
		assert.Empty(t, snap.Key)
	})

	t.Run("no label", func(t *testing.T) {
		f := newFixture(t)
		_, _, err := f.svc.MakeContact(ctx, charID, "  ")
		var fe *domain.FieldError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, "target", fe.Field)
	})
}

// ADR 0044 step 10: ContactBefore finds target's contact row (ignoring
// case) only when it was kept past the round contact was made.
func TestContactBefore_Step10_AnEarlierRoundsContact(t *testing.T) {
	orc := contactOn("o", "Orc", 3, 2)
	for _, c := range []struct {
		name   string
		rows   []buffsvc.BuffEffect
		target string
		ok     bool
	}{
		{"kept a round since", []buffsvc.BuffEffect{orc}, "orc", true},
		{"made this round", []buffsvc.BuffEffect{contactOn("o", "Orc", 2, 2)}, "Orc", false},
		{"a new row this round", []buffsvc.BuffEffect{contactOn("o", "Orc", 0, 0)}, "Orc", false},
		{"tangents only", []buffsvc.BuffEffect{mindRow("o", "Orc", 2, 4)}, "Orc", false},
		{"another mind", []buffsvc.BuffEffect{orc}, "Goblin", false},
		{"a power's row on the target", []buffsvc.BuffEffect{usedOn("p", "Orc", 4)}, "Orc", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			row, ok := buffsvc.ContactBefore(c.rows, c.target)
			assert.Equal(t, c.ok, ok)
			if c.ok {
				assert.Equal(t, orc, row)
			}
		})
	}
}

// ADR 0044 step 10 (Q12): the contact row is free while a power kept up
// through it is on: Next Round charges inflict pain's 2, not 2 + 1. Without
// such a power, or when the power ends unpaid, the contact pays its 1.
func TestContactPower_Step10_ContactFreeWhileAPowerUsesIt(t *testing.T) {
	t.Run("a power on the contacted mind", func(t *testing.T) {
		f := newFixture(t)
		c, p := contactOn("c", "Orc", 3, 2), usedOn("p", "Orc", 1)
		f.pspIs(20)
		active, expired, e := f.advances(t, []buffsvc.BuffEffect{c, p}, 5)
		assert.Equal(t, []buffsvc.BuffEffect{keptMore(c, 1), keptMore(p, 1)}, active)
		assert.Empty(t, expired)
		assert.Equal(t, -2, e.PSPDelta, "the power's 2 only")
	})

	t.Run("a power on another mind", func(t *testing.T) {
		f := newFixture(t)
		c, p := contactOn("c", "Orc", 3, 2), usedOn("p", "Goblin", 1)
		f.pspIs(20)
		_, _, e := f.advances(t, []buffsvc.BuffEffect{c, p}, 5)
		assert.Equal(t, -3, e.PSPDelta, "2 + 1")
	})

	t.Run("short of PSPs: the power pays first, the contact is free", func(t *testing.T) {
		f := newFixture(t)
		c, p := contactOn("c", "Orc", 3, 2), usedOn("p", "Orc", 1)
		f.pspIs(2)
		active, expired, e := f.advances(t, []buffsvc.BuffEffect{c, p}, 5)
		assert.Equal(t, []buffsvc.BuffEffect{keptMore(c, 1), keptMore(p, 1)}, active, "both kept")
		assert.Empty(t, expired)
		assert.Equal(t, -2, e.PSPDelta, "inflict pain paid 2, the contact free")
	})

	t.Run("the power ends unpaid: the contact pays", func(t *testing.T) {
		f := newFixture(t)
		c, p := contactOn("c", "Orc", 3, 2), usedOn("p", "Orc", 1)
		f.pspIs(1)
		active, expired, e := f.advances(t, []buffsvc.BuffEffect{p, c}, 5)
		assert.Equal(t, []buffsvc.BuffEffect{keptMore(c, 1)}, active)
		assert.Equal(t, []buffsvc.BuffEffect{p}, expired)
		assert.Equal(t, -1, e.PSPDelta)
	})
}

// ADR 0044 step 10: dismissing a contact row ends the powers kept up
// through it in the same entry, its payload listing them; dismissing such
// a power leaves the contact.
func TestContactPower_Step10_EndingContactEndsItsPowers(t *testing.T) {
	ctx := context.Background()
	c, p := contactOn("c", "Orc", 3, 2), usedOn("p", "Orc", 1)
	other := usedOn("q", "Goblin", 1)
	lev := maintained("l", "levitation", 3, 1, 1)

	t.Run("the contact", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{c, p, other, lev}, 5)
		f.records()
		f.saves([]buffsvc.BuffEffect{other, lev})
		got, _, err := f.svc.Dismiss(ctx, charID, "c", "")
		require.NoError(t, err)
		assert.Equal(t, []buffsvc.BuffEffect{other, lev}, got)
		require.Len(t, f.entries, 1, "one entry")
		e := f.entries[0]
		assert.Zero(t, e.PSPDelta)
		pl := payload(t, e)
		assert.Equal(t, []any{"inflict_pain"}, pl["contactPowersEnded"])
		assert.Equal(t, "Orc", pl["target"])
		require.Equal(t, []string{buffsvc.StateKeyBuffs}, snapKeys(e), "one undo brings both back")
		before := decodeSnap(t, e.Snapshots[0].Before)
		require.Len(t, before, 4)
		assert.Equal(t, "Orc", before[0]["target"])
		assert.InDelta(t, 2, before[0]["contact_from"], 0, "the contact row comes back with its ContactFrom")
		assert.Equal(t, "inflict_pain", before[1]["power_id"])
		assert.Equal(t, "Orc", before[1]["target"], "and its power, on its target")
	})

	t.Run("the power", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{c, p}, 5)
		f.records()
		f.saves([]buffsvc.BuffEffect{c})
		_, _, err := f.svc.Dismiss(ctx, charID, "p", "")
		require.NoError(t, err)
		_, listed := payload(t, f.entries[0])["contactPowersEnded"]
		assert.False(t, listed)
	})
}

// ADR 0044 step 10: each use of a power on a mind is maintained for that
// mind alone. Inflict pain used on Goblin leaves its row on Orc: two rows;
// only the same power on the same mind (ignoring case) renews.
func TestAdd_Step10_APowerOnTwoMindsIsTwoRows(t *testing.T) {
	ctx := context.Background()
	co, cg, onOrc := contactOn("co", "Orc", 3, 1), contactOn("cg", "Goblin", 3, 1), usedOn("p", "Orc", 2)
	pain := func(target string) buffsvc.BuffEffect {
		b := maintained("", "inflict_pain", 2, 1, 0)
		b.Target = target
		return b
	}

	t.Run("on another mind: a second row", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{co, cg, onOrc}, 5)
		var saved []buffsvc.BuffEffect
		f.savesAny(&saved)
		row, _, err := f.svc.Add(ctx, charID, pain("Goblin"))
		require.NoError(t, err)
		require.Len(t, saved, 4)
		assert.Equal(t, []buffsvc.BuffEffect{co, cg, onOrc}, saved[:3], "Orc's row stays")
		assert.Equal(t, "Goblin", saved[3].Target)
		assert.Equal(t, row, saved[3])
	})

	t.Run("on the same mind: renewed", func(t *testing.T) {
		f := newFixture(t)
		f.state([]buffsvc.BuffEffect{co, onOrc}, 5)
		var saved []buffsvc.BuffEffect
		f.savesAny(&saved)
		_, _, err := f.svc.Add(ctx, charID, pain("orc"))
		require.NoError(t, err)
		require.Len(t, saved, 2)
		assert.Equal(t, co, saved[0])
		assert.Zero(t, saved[1].RoundsKept, "the renewed row starts again")
	})
}

// ADR 0044 step 10: two minds each with a contact and inflict pain on it:
// Next Round charges 2 + 2 and both contacts are free.
func TestContactPower_Step10_TwoMindsPayTheirOwnMaintenance(t *testing.T) {
	f := newFixture(t)
	rows := []buffsvc.BuffEffect{
		contactOn("co", "Orc", 3, 1), contactOn("cg", "Goblin", 3, 1),
		usedOn("po", "Orc", 2), usedOn("pg", "Goblin", 1),
	}
	f.pspIs(20)
	active, expired, e := f.advances(t, rows, 5)
	assert.Len(t, active, 4)
	assert.Empty(t, expired)
	assert.Equal(t, -4, e.PSPDelta)
}

// ADR 0044 step 10: a contact made this round is not yet one from an
// earlier round; after a Next Round, or passing time out of combat, it is.
func TestContactBefore_Step10_AfterNextRoundOrPassingTime(t *testing.T) {
	made := contactOn("c", "Orc", 0, 0)
	_, ok := buffsvc.ContactBefore([]buffsvc.BuffEffect{made}, "Orc")
	require.False(t, ok, "this round")

	t.Run("Next Round", func(t *testing.T) {
		f := newFixture(t)
		f.pspIs(20)
		active, _, _ := f.advances(t, []buffsvc.BuffEffect{made}, 5)
		_, ok := buffsvc.ContactBefore(active, "Orc")
		assert.True(t, ok)
	})

	t.Run("passing time", func(t *testing.T) {
		f := newFixture(t)
		f.pspIs(20)
		active, _, _ := f.passes(t, []buffsvc.BuffEffect{made}, 0, 0.1)
		_, ok := buffsvc.ContactBefore(active, "Orc")
		assert.True(t, ok)
	})
}

// ADR 0043 step 7: a long rest ends the contact row and the power kept up
// through it, both coming back on undo.
func TestEndMaintained_Step10_TheContactAndItsPower(t *testing.T) {
	f := newFixture(t)
	c, p := contactOn("c", "Orc", 3, 1), usedOn("p", "Orc", 2)
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{c, p}, nil).Once()
	f.saves([]buffsvc.BuffEffect{})
	ended, snap, err := f.svc.EndMaintained(context.Background(), charID)
	require.NoError(t, err)
	assert.Equal(t, []buffsvc.BuffEffect{c, p}, ended)
	before := decodeSnap(t, snap.Before)
	require.Len(t, before, 2)
	assert.InDelta(t, 1, before[0]["contact_from"], 0)
}
