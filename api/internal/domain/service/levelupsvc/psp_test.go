package levelupsvc_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt/cataloguetest"
	"github.com/ceffo/critical/api/internal/domain/actionlog/snaptest"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
)

// ADR 0043 step 4.2: a psionicist level adds Table 5's Wisdom modifier plus 10 to the stored PSP
// maximum, no die (Wis 17: +2 + 10 = 12), not divided by the classes (Q2). The party's thief
// 5/psionicist 4 (Wis 17) takes its psionicist's 5 with 181 PSPs: the maximum becomes 193 in
// the same transaction, the level_up entry carries the gain and its terms, and its snapshot
// carries the maximum before and after, which the undo writes back. The thief's level gains no
// PSPs and leaves the maximum alone.
func TestRoll_Step4_2_ALevelUpAddsWisdomModifierPlusTen(t *testing.T) {
	tables := cataloguetest.Tables(t)
	psionic := func(es ...owing) *charactersvc.Character {
		c := multi(14, 30, es...)
		c.Wis, c.Int, c.Con, c.PSPMax = 17, 15, 14, 181
		return c
	}
	t.Run("a psionicist level: +12, recorded and snapshotted", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).
			Return(psionic(owing{class: "thief", level: 5}, owing{class: "psionicist", level: 4, pending: []charactersvc.PendingLevel{d6(5)}}), nil).Once()
		f.dice.EXPECT().RollDie(6).Return(5).Once()
		f.rules.EXPECT().ConstitutionFor(14).Return(con(0, 0, 0), nil).Once()
		f.rules.EXPECT().PSPMaxFor(mock.Anything, mock.Anything, 2).RunAndReturn(tables.PSPMaxFor).Once()
		f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, "psionicist", 5, 32).Return(nil).Once()
		f.dal.EXPECT().SetPSPMax(mock.Anything, charID, 193).Return(nil).Once()
		got := f.recorded()

		res, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: "psionicist", Level: 5})
		require.NoError(t, err)
		assert.Equal(t, &levelupsvc.PSPGain{Ability: "wis", Score: 17, Modifier: 2, Plus: 10, Gain: 12, MaxBefore: 181, MaxAfter: 193}, res.PSP)
		require.Len(t, *got, 1)
		e := (*got)[0]
		assert.JSONEq(t, `{"ability":"wis","score":17,"modifier":2,"plus":10,"gain":12,"maxBefore":181,"maxAfter":193}`,
			jsonField(t, e.Payload, "psp"))
		assert.Equal(t, 0, e.PSPDelta, "the current PSPs rise with the maximum, no delta")
		assert.JSONEq(t, `{"class":"psionicist","realizedLevel":4,"hpMax":30,"pspMax":181}`, snaptest.State(t, e.Snapshots[0].Before))
		assert.JSONEq(t, `{"class":"psionicist","realizedLevel":5,"hpMax":32,"pspMax":193}`, snaptest.State(t, e.Snapshots[0].After))
	})
	t.Run("the thief's level: no PSPs", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).
			Return(psionic(owing{class: "thief", level: 5, pending: []charactersvc.PendingLevel{d6(6)}}, owing{class: "psionicist", level: 4}), nil).Once()
		f.dice.EXPECT().RollDie(6).Return(5).Once()
		f.rules.EXPECT().ConstitutionFor(14).Return(con(0, 0, 0), nil).Once()
		f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, "thief", 6, 32).Return(nil).Once()
		got := f.recorded()

		res, err := f.svc.Roll(context.Background(), charID, levelupsvc.RollInput{Class: "thief", Level: 6})
		require.NoError(t, err)
		assert.Nil(t, res.PSP)
		assert.Empty(t, jsonField(t, (*got)[0].Payload, "psp"))
		assert.JSONEq(t, `{"class":"thief","realizedLevel":5,"hpMax":30}`, snaptest.State(t, (*got)[0].Snapshots[0].Before))
	})
	t.Run("the undo writes the maximum back; a snapshot without one leaves it", func(t *testing.T) {
		f := newFixture(t)
		r := f.svc.Module().Restorer(levelupsvc.StateKey)
		f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, "psionicist", 4, 30).Return(nil).Once()
		f.dal.EXPECT().SetPSPMax(mock.Anything, charID, 181).Return(nil).Once()
		require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"class":"psionicist","realizedLevel":4,"hpMax":30,"pspMax":181}`)))
		f.dal.EXPECT().SetRealizedLevel(mock.Anything, charID, "thief", 5, 30).Return(nil).Once()
		require.NoError(t, r.Restore(context.Background(), charID, snaptest.Env(`{"class":"thief","realizedLevel":5,"hpMax":30}`)))
	})
}

// jsonField is the raw JSON of payload's field name, "" when absent.
func jsonField(t *testing.T, payload []byte, name string) string {
	t.Helper()
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(payload, &m))
	return string(m[name])
}
