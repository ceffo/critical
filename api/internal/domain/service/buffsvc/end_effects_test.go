package buffsvc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/pkg/formula"
	"github.com/ceffo/critical/api/pkg/must"
)

// endSpells is a catalogue with a spell that heals 1d12+1 when ended by the
// player or when it runs out (the shape of Polymorph Self, with a constant
// so the payload shows it), and one without end effects.
func endSpells(f *fixture) {
	f.spells["shape"] = spelleffectsvc.Entry{ID: "shape", EndEffects: []spelleffectsvc.EndEffect{{
		Kind: spelleffectsvc.EndHeal, Dice: must.Do(formula.Parse("1d12+1")),
		On: []spelleffectsvc.EndCause{spelleffectsvc.EndVoluntary, spelleffectsvc.EndExpired},
	}}}
	f.spells["plain"] = spelleffectsvc.Entry{ID: "plain"}
}

// hpIs answers the reads of the HP around a heal: the ledger total and max HP.
func (f *fixture) hpIs(total, maxHP int) {
	f.log.EXPECT().HPTotal(mock.Anything, charID).Return(total, nil).Once()
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(&charactersvc.Character{ID: charID, Str: 10, HPMax: maxHP}, nil).Once()
}

// ADR 0004 step 21, the dismiss: a row whose spell has end effects needs a
// cause; the cause's effects are rolled through the dice port (exactly one
// d12) and their heal is the dismiss entry's HP delta, with the dice, the
// total and the HP before and after in its payload. Other rows ignore the
// cause and stop as before.
func TestBuffs_Step21_Dismiss(t *testing.T) {
	shape := buffsvc.BuffEffect{ID: "s", Name: "Shape", Duration: 30, SpellID: "shape"}
	plain := buffsvc.BuffEffect{ID: "p", Name: "Plain", Duration: 30, SpellID: "plain"}
	onOther := buffsvc.BuffEffect{ID: "o", Name: "Shape", Duration: 30, SpellID: "shape", OnOther: true}
	tests := []struct {
		name        string
		row         buffsvc.BuffEffect
		cause       spelleffectsvc.EndCause
		d12         int // 0: no die
		total, max  int
		wantErr     error
		wantDelta   int
		wantPayload string
		wantEnd     buffsvc.EndResult
	}{
		{
			name: "ended by me heals the dice", row: shape, cause: spelleffectsvc.EndVoluntary, d12: 7, total: -20, max: 40,
			wantDelta: 8,
			wantPayload: `{"action":"dismiss","buff":"Shape","spellId":"shape","cause":"voluntary","hpBefore":20,"hpAfter":28,
				"endEffects":[{"spellId":"shape","buff":"Shape","cause":"voluntary","kind":"heal","dice":"1d12+1","rolls":[{"sides":12,"value":7}],"constant":1,"total":8}]}`,
			wantEnd: buffsvc.EndResult{HPBefore: 20, HPAfter: 28, Effects: []buffsvc.EndEffectRoll{{
				SpellID: "shape", Buff: "Shape", Cause: spelleffectsvc.EndVoluntary, Kind: spelleffectsvc.EndHeal,
				Dice: "1d12+1", Rolls: []buffsvc.DieRoll{{Sides: 12, Value: 7}}, Constant: 1, Total: 8,
			}}},
		},
		{
			name: "the HP after is capped at max HP", row: shape, cause: spelleffectsvc.EndVoluntary, d12: 12, total: -3, max: 40,
			wantDelta: 13, // the ledger caps it to 3 when it records
			wantPayload: `{"action":"dismiss","buff":"Shape","spellId":"shape","cause":"voluntary","hpBefore":37,"hpAfter":40,
				"endEffects":[{"spellId":"shape","buff":"Shape","cause":"voluntary","kind":"heal","dice":"1d12+1","rolls":[{"sides":12,"value":12}],"constant":1,"total":13}]}`,
			wantEnd: buffsvc.EndResult{HPBefore: 37, HPAfter: 40, Effects: []buffsvc.EndEffectRoll{{
				SpellID: "shape", Buff: "Shape", Cause: spelleffectsvc.EndVoluntary, Kind: spelleffectsvc.EndHeal,
				Dice: "1d12+1", Rolls: []buffsvc.DieRoll{{Sides: 12, Value: 12}}, Constant: 1, Total: 13,
			}}},
		},
		{
			name: "at full HP the entry is still written with the roll and no HP gained", row: shape, cause: spelleffectsvc.EndVoluntary, d12: 5, total: 0, max: 40,
			wantDelta: 6, // the ledger caps it to 0 when it records
			wantPayload: `{"action":"dismiss","buff":"Shape","spellId":"shape","cause":"voluntary","hpBefore":40,"hpAfter":40,
				"endEffects":[{"spellId":"shape","buff":"Shape","cause":"voluntary","kind":"heal","dice":"1d12+1","rolls":[{"sides":12,"value":5}],"constant":1,"total":6}]}`,
			wantEnd: buffsvc.EndResult{HPBefore: 40, HPAfter: 40, Effects: []buffsvc.EndEffectRoll{{
				SpellID: "shape", Buff: "Shape", Cause: spelleffectsvc.EndVoluntary, Kind: spelleffectsvc.EndHeal,
				Dice: "1d12+1", Rolls: []buffsvc.DieRoll{{Sides: 12, Value: 5}}, Constant: 1, Total: 6,
			}}},
		},
		{
			name: "dispelled or slain heals nothing", row: shape, cause: spelleffectsvc.EndDispelled,
			wantPayload: `{"action":"dismiss","buff":"Shape","spellId":"shape","cause":"dispelled"}`,
		},
		{
			name: "no cause is refused", row: shape, wantErr: spelleffectsvc.ErrEndCauseRequired,
		},
		{
			name: "a spell without end effects ignores the cause", row: plain, cause: spelleffectsvc.EndVoluntary,
			wantPayload: `{"action":"dismiss","buff":"Plain","spellId":"plain"}`,
		},
		{
			name: "a spell without end effects needs no cause", row: plain,
			wantPayload: `{"action":"dismiss","buff":"Plain","spellId":"plain"}`,
		},
		{
			name: "a row on someone else asks nothing and heals nothing", row: onOther, cause: spelleffectsvc.EndVoluntary,
			wantPayload: `{"action":"dismiss","buff":"Shape","spellId":"shape"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			endSpells(f)
			f.state([]buffsvc.BuffEffect{tt.row}, 2)
			if tt.wantErr == nil {
				f.records()
				f.saves([]buffsvc.BuffEffect{})
			}
			if tt.d12 > 0 {
				f.dice.EXPECT().RollDie(12).Return(tt.d12).Once()
				f.hpIs(tt.total, tt.max)
			}

			got, end, err := f.svc.Dismiss(context.Background(), charID, tt.row.ID, tt.cause)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, f.entries, "nothing written")
				return
			}
			require.NoError(t, err)
			assert.Empty(t, got)
			assert.Equal(t, tt.wantEnd, end)
			require.Len(t, f.entries, 1, "one entry: the end and its heal")
			assert.Equal(t, actionlog.KindBuffChange, f.entries[0].Kind)
			assert.Equal(t, tt.wantDelta, f.entries[0].HPDelta)
			assert.JSONEq(t, tt.wantPayload, string(f.entries[0].Payload))
			assert.Equal(t, []string{buffsvc.StateKeyBuffs}, snapKeys(f.entries[0]), "the row comes back on undo; the HP with the entry's delta")
		})
	}

	t.Run("an unknown row is not found before any cause is asked", func(t *testing.T) {
		f := newFixture(t)
		endSpells(f)
		f.state([]buffsvc.BuffEffect{shape}, 2)
		_, _, err := f.svc.Dismiss(context.Background(), charID, "zzz", "")
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}

// ADR 0004 step 21, running out: Next Round and passing time apply the
// "expired" end effects of every row they expire, in row order, in their
// own entry (its HP delta, the rolls in its payload). Rows still running,
// rows whose spell has no end effect and rows on someone else roll nothing.
// On Next Round the end effects' dice come before the initiative's d10.
func TestBuffs_Step21_RunsOut(t *testing.T) {
	ending := []buffsvc.BuffEffect{
		{ID: "a", Name: "Shape", Duration: 1, SpellID: "shape"},
		{ID: "b", Name: "Plain", Duration: 1, SpellID: "plain"},
		{ID: "c", Name: "Shape", Duration: 1, SpellID: "shape", OnOther: true},
		{ID: "d", Name: "Shape", Duration: 1, SpellID: "shape"},
		{ID: "e", Name: "Shape", Duration: 500, SpellID: "shape"},
	}
	still := []buffsvc.BuffEffect{{ID: "e", Name: "Shape", Duration: 499, SpellID: "shape"}}
	stillAfterHour := []buffsvc.BuffEffect{{ID: "e", Name: "Shape", Duration: 440, SpellID: "shape"}}
	endEffects := `[
		{"spellId":"shape","buff":"Shape","cause":"expired","kind":"heal","dice":"1d12+1","rolls":[{"sides":12,"value":3}],"constant":1,"total":4},
		{"spellId":"shape","buff":"Shape","cause":"expired","kind":"heal","dice":"1d12+1","rolls":[{"sides":12,"value":9}],"constant":1,"total":10}]`
	tests := []struct {
		name string
		run  func(f *fixture) (buffsvc.EndResult, error)
		kind actionlog.Kind
		want string
	}{
		{
			name: "Next Round",
			run: func(f *fixture) (buffsvc.EndResult, error) {
				f.saves(still)
				f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 5).Return(nil).Once()
				f.rollsInitiative(6)
				_, _, _, ini, end, err := f.svc.AdvanceRound(context.Background(), charID)
				assert.Equal(t, 6, ini.D10)
				return end, err
			},
			kind: actionlog.KindRoundAdvance,
			want: `{"action":"next_round","fromRound":4,"toRound":5,"expired":["Shape","Plain","Shape","Shape"],
				"expiredSpellIds":["shape","plain","shape","shape"],"hpBefore":10,"hpAfter":24,"endEffects":` + endEffects + `}`,
		},
		{
			name: "passing time",
			run: func(f *fixture) (buffsvc.EndResult, error) {
				f.saves(stillAfterHour)
				f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
				_, _, _, _, end, err := f.svc.PassTime(context.Background(), charID, 1)
				return end, err
			},
			kind: actionlog.KindTimePassed,
			want: `{"action":"pass_time","hours":1,"rounds":60,"fromRound":4,"toRound":0,"expired":["Shape","Plain","Shape","Shape"],
				"expiredSpellIds":["shape","plain","shape","shape"],"hpBefore":10,"hpAfter":24,"endEffects":` + endEffects + `}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			endSpells(f)
			f.state(ending, 4)
			f.records()
			f.initiativeIs(nil)
			var order []int
			f.dice.EXPECT().RollDie(12).RunAndReturn(func(int) int {
				order = append(order, 12)
				return []int{3, 9}[len(order)-1]
			}).Times(2)
			f.hpIs(-30, 40)

			end, err := tt.run(f)
			require.NoError(t, err)
			assert.Equal(t, []int{12, 12}, order)
			require.Len(t, end.Effects, 2)
			assert.Equal(t, 10, end.HPBefore)
			assert.Equal(t, 24, end.HPAfter)
			e := f.entries[0]
			assert.Equal(t, tt.kind, e.Kind)
			assert.Equal(t, 14, e.HPDelta, "both heals in the action's own entry")
			assert.JSONEq(t, tt.want, string(e.Payload))
		})
	}

	t.Run("nothing runs out: no die, no HP read", func(t *testing.T) {
		f := newFixture(t)
		endSpells(f)
		f.state([]buffsvc.BuffEffect{{ID: "e", Name: "Shape", Duration: 500, SpellID: "shape"}}, 0)
		f.records()
		f.saves(stillAfterHour)
		f.dal.EXPECT().SaveRoundNumber(mock.Anything, charID, 0).Return(nil).Once()
		f.initiativeIs(nil)
		_, _, _, _, end, err := f.svc.PassTime(context.Background(), charID, 1)
		require.NoError(t, err)
		assert.Empty(t, end.Effects)
		assert.Zero(t, f.entries[0].HPDelta)
	})
}

// ADR 0004 step 21: the list says which rows ask how they ended: a row on
// the bearer whose spell has end effects; not a row on someone else, a row
// whose spell has none, nor one not from a spell.
func TestBuffs_Step21_ListAsksEndCause(t *testing.T) {
	f := newFixture(t)
	endSpells(f)
	f.base(10)
	f.dal.EXPECT().GetActiveBuffs(mock.Anything, charID).Return([]buffsvc.BuffEffect{
		{ID: "a", Name: "Shape", SpellID: "shape"},
		{ID: "b", Name: "Shape", SpellID: "shape", OnOther: true},
		{ID: "c", Name: "Plain", SpellID: "plain"},
		{ID: "d", Name: "Hand-made"},
		{ID: "e", Name: "Unknown", SpellID: "gone"},
	}, nil)
	got, err := f.svc.ListStatus(context.Background(), charID)
	require.NoError(t, err)
	asks := make([]bool, len(got))
	for i, b := range got {
		asks[i] = b.AsksEndCause
	}
	assert.Equal(t, []bool{true, false, false, false, false}, asks)
}
