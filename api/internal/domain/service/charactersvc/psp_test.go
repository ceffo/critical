package charactersvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// ADR 0043 steps 4 and 5 on a character, on the fixture catalogue's real
// tables (its psionicist and CPsiH Table 5). The party's human
// thief/psionicist has the book's example scores: Wisdom 17, Intelligence 15,
// Constitution 16 (25 at level 1, 12 a level from 2).

// pspLedger is an HP ledger whose PSP totals are psp, counting the reads.
type pspLedger struct {
	hpLedger
	psp   map[int64]int
	reads int
}

func (p *pspLedger) PSPTotal(_ context.Context, id int64) (int, error) {
	p.reads++
	return p.psp[id], nil
}

// thiefPsionicist is the party's human thief 15/psionicist 14 with psp max
// stored.
func thiefPsionicist(pspMax int) *charactersvc.Character {
	c := multi("human", entry("thief", 15, 0), entry("psionicist", 14, 0))
	c.Wis, c.Int, c.Con, c.PSPMax = 17, 15, 16, pspMax
	return c
}

// readOn reads c through a service on the fixture's tables and the ledger.
func readOn(t *testing.T, c *charactersvc.Character, ledger *pspLedger) *charactersvc.Character {
	t.Helper()
	e := newEnv(t)
	svc, err := charactersvc.New(charactersvc.Deps{
		Reader: e.reader, XP: e.xp, Rules: cataloguetest.Tables(t), Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
		Armour: e.armour, HP: ledger, Spells: e.spells, Book: e.book, FreeSpells: e.free,
	})
	require.NoError(t, err)
	e.reader.EXPECT().GetCharacter(mock.Anything, c.ID).Return(c, nil).Once()
	e.xp.EXPECT().Awards(mock.Anything, c.ID).Return(nil, nil).Once()
	got, err := svc.GetCharacter(context.Background(), c.ID)
	require.NoError(t, err)
	return got
}

// Step 5: the derived character serves psp {current, max} for a character
// with the psionics feature: the stored maximum, and the maximum plus the
// ledger's total, shown within 0..max. A character without it has none and
// pays no ledger read.
func TestGetCharacter_Step5_ThePSPPool(t *testing.T) {
	for _, tc := range []struct {
		name         string
		total, want  int
		pspMax, wMax int
	}{
		{"the maximum plus the total", -40, 141, 181, 181},
		{"a full pool", 0, 181, 181, 181},
		{"never shown under 0", -200, 0, 181, 181},
		{"never over a maximum entered lower", 0, 90, 90, 90},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := &pspLedger{psp: map[int64]int{7: tc.total}}
			got := readOn(t, thiefPsionicist(tc.pspMax), ledger)
			require.NotNil(t, got.Derived.PSP)
			assert.Equal(t, charactersvc.PSPPool{Current: tc.want, Max: tc.wMax}, *got.Derived.PSP)
			assert.Equal(t, 1, ledger.reads)
		})
	}
	t.Run("no psionics, no pool", func(t *testing.T) {
		ledger := &pspLedger{psp: map[int64]int{7: -5}}
		got := readOn(t, multi("human", entry("fighter", 5, 0), entry("thief", 5, 0)), ledger)
		assert.Nil(t, got.Derived.PSP)
		assert.Zero(t, ledger.reads)
	})
}

// Step 4 and the multi-class criterion: the maximum is the psionicist entry's
// alone, at its own level (a thief 15/psionicist 14 reads level 14: 25 + 13 x
// 12 = 181, not 193), not divided by the classes (Q2).
func TestPSPMaxOf_Step4_MultiClassReadsThePsionicistEntry(t *testing.T) {
	tables := cataloguetest.Tables(t)
	got, ok, err := charactersvc.PSPMaxOf(thiefPsionicist(0), tables)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 181, got.Max)
	assert.Equal(t, 25, got.InherentPotential)
	assert.Equal(t, 12, got.PerLevel)

	single := multi("human", entry("psionicist", 15, 0))
	single.Wis, single.Int, single.Con = 17, 15, 16
	got, ok, err = charactersvc.PSPMaxOf(single, tables)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 193, got.Max, "a psionicist 15 alone")

	_, ok, err = charactersvc.PSPMaxOf(multi("human", entry("fighter", 5, 0), entry("thief", 5, 0)), tables)
	require.NoError(t, err)
	assert.False(t, ok, "no psionics entry, no maximum")
}

// Step 7: a night's sleep is 8 hours at Table 6's sleeping rate (12 an hour:
// 96), whatever the level or the maximum, for a character with a psionics
// entry; a character with none recovers nothing. The cap at the PSPs spent is
// the recording path's (step 5).
func TestNightsPSPRecovery_Step7_EightHoursAtTheSleepingRate(t *testing.T) {
	recovery := func(c *charactersvc.Character) int {
		t.Helper()
		e := newEnv(t)
		svc, err := charactersvc.New(charactersvc.Deps{
			Reader: e.reader, XP: e.xp, Rules: cataloguetest.Tables(t), Owners: e.owners, Tx: e.tx, Sessions: e.sessions,
			Armour: e.armour, HP: &pspLedger{}, Spells: e.spells, Book: e.book, FreeSpells: e.free,
		})
		require.NoError(t, err)
		e.reader.EXPECT().GetCharacter(mock.Anything, c.ID).Return(c, nil).Once()
		got, err := svc.NightsPSPRecovery(context.Background(), c.ID)
		require.NoError(t, err)
		return got
	}
	assert.Equal(t, 96, recovery(thiefPsionicist(181)))
	assert.Zero(t, recovery(multi("human", entry("fighter", 5, 0), entry("thief", 5, 0))))
}
