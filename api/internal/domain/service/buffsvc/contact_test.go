package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.Len(t, decodeSnap(t, e.Snapshots[0].Before), 4)
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
