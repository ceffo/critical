package combatsvc_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

const charID = int64(1)

// bard is the campaign Bard: THAC0 13, sheet STR 12, DEX 17. Combat reads the
// effective STR from the buffs port, never from the sheet.
func bard() *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Str: 12, Dex: 17, Thac0Base: 13,
		SaveParaPoison: 11, SaveRodStaffWand: 13, SavePetriPoly: 12, SaveBreath: 15, SaveSpell: 14,
	}
}

// strMods is "no buff effect" at effective STR str.
func strMods(str int) buffsvc.BuffModifiers {
	return buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: str}
}

// phb is the real PHB Table 1/2 lookup from the testdata fixture, so the
// expected numbers below are checked against the printed tables.
func phb(t *testing.T) *ruletablesvc.Tables {
	t.Helper()
	tables, err := ruletablesadpt.New(filepath.Join("..", "..", "..", "..", "testdata", "seeds"))
	require.NoError(t, err)
	return tables
}

type fixture struct {
	chars   *mocks.MockCharacters
	buffs   *mocks.MockBuffs
	dice    *mocks.MockDice
	weapons *mocks.MockWeapons
	items   *mocks.MockItems
	crit    *mocks.MockCritStore
	// critRows is what the crit store returns (none: the placeholder default).
	critRows []combatsvc.CritRow
	log      *mocks.MockLog
	svc      combatsvc.SVC
	entries  []actionlog.NewEntry
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		chars:   mocks.NewMockCharacters(t),
		buffs:   mocks.NewMockBuffs(t),
		dice:    mocks.NewMockDice(t),
		weapons: mocks.NewMockWeapons(t),
		items:   mocks.NewMockItems(t),
		crit:    mocks.NewMockCritStore(t),
		log:     mocks.NewMockLog(t),
	}
	f.crit.EXPECT().List(mock.Anything).RunAndReturn(
		func(context.Context) ([]combatsvc.CritRow, error) { return f.critRows, nil }).Maybe()
	svc, err := combatsvc.NewWithConfig(combatsvc.DefaultConfig(), combatsvc.Deps{
		Chars: f.chars, Buffs: f.buffs, Dice: f.dice, Weapons: f.weapons, Crit: f.crit,
		Items: f.items, Rules: phb(t), Log: f.log, Tx: noTx{},
	})
	require.NoError(t, err)
	f.svc = svc
	return f
}

func (f *fixture) withChar(c *charactersvc.Character) {
	f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(c, nil)
}

// d100s expects exactly two d100 rolls per attack, in order: the crit check
// then the multiplier (ADR 0001 steps 1 and 3). The values must differ so a
// roll used twice, or for the wrong purpose, shows in the results.
func (f *fixture) d100s(t *testing.T, check, multiplier int) {
	t.Helper()
	require.NotEqual(t, check, multiplier, "use distinct d100 values")
	f.dice.EXPECT().RollD100().Return(check).Once()
	f.dice.EXPECT().RollD100().Return(multiplier).Once()
}

// noCritMult is the multiplier d100 of tests where it does not matter.
const noCritMult = 77

// records captures every logged entry and returns id 42.
func (f *fixture) records() {
	f.log.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, e actionlog.NewEntry) (int64, error) {
			f.entries = append(f.entries, e)
			return 42, nil
		})
}

func TestNew_NilDependency(t *testing.T) {
	_, err := combatsvc.New(combatsvc.Deps{Buffs: mocks.NewMockBuffs(t), Dice: mocks.NewMockDice(t)})
	require.Error(t, err)
	_, err = combatsvc.NewWithConfig(nil, combatsvc.Deps{})
	require.Error(t, err)
}

func TestResolveSavingThrow(t *testing.T) {
	tests := []struct {
		name       string
		category   combatsvc.SaveCategory
		d20        int
		saveBonus  int
		wantTarget int
		wantHit    bool
		wantMargin int
		wantText   string
	}{
		{"para/poison pass", combatsvc.SaveParaPoison, 11, 0, 11, true, 0, "Roll: 11 + Mod: 0 = 11 vs Target: 11 (Paralyze/Poison)"},
		{"para/poison fail", combatsvc.SaveParaPoison, 10, 0, 11, false, -1, "Roll: 10 + Mod: 0 = 10 vs Target: 11 (Paralyze/Poison)"},
		{"rod/staff/wand", combatsvc.SaveRodStaffWand, 13, 0, 13, true, 0, "(Rod/Staff/Wand)"},
		{"petrify/poly", combatsvc.SavePetriPoly, 5, 0, 12, false, -7, "(Petrify/Polymorph)"},
		{"breath", combatsvc.SaveBreath, 20, 0, 15, true, 5, "(Breath Weapon)"},
		{"spell", combatsvc.SaveSpell, 14, 0, 14, true, 0, "(Spells)"},
		{"buff bonus turns a fail into a pass", combatsvc.SaveSpell, 12, 2, 14, true, 0, "Roll: 12 + Mod: 2 = 14 vs Target: 14 (Spells)"},
		{"negative buff bonus", combatsvc.SaveSpell, 14, -1, 14, false, -1, "Roll: 14 + Mod: -1 = 13 vs Target: 14 (Spells)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.withChar(bard())
			f.dice.EXPECT().RollD20().Return(tt.d20)
			mods := strMods(12)
			if tt.saveBonus != 0 {
				mods.SaveBonus = tt.saveBonus
				mods.SaveBonusFrom = []buffsvc.Contribution{{Buff: "Prayer", Value: tt.saveBonus}}
			}
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(mods, nil)
			f.records()

			res, err := f.svc.ResolveSavingThrow(context.Background(), charID, tt.category)
			require.NoError(t, err)
			assert.Equal(t, int64(42), res.LogEntryID)
			assert.Equal(t, tt.category, res.SaveCategory)
			assert.Zero(t, res.Adjustment)
			require.Len(t, f.entries, 1)
			assert.Equal(t, actionlog.KindSave, f.entries[0].Kind)
			assert.Empty(t, f.entries[0].Snapshots, "a save changes no state")
			assert.Equal(t, tt.d20, res.D20Roll)
			assert.Equal(t, tt.wantTarget, res.BaseThac0)
			assert.Equal(t, tt.wantTarget, res.EffectiveThac0)
			assert.Equal(t, tt.wantHit, res.Hit)
			assert.Equal(t, tt.wantMargin, res.HitMargin)
			assert.Contains(t, res.DamageBreakdown, tt.wantText)
			if tt.saveBonus != 0 {
				assert.Equal(t, []combatsvc.Modifier{{Kind: combatsvc.ModBuff, Source: "Prayer", Detail: "Prayer", Value: tt.saveBonus}}, res.Modifiers)
			} else {
				assert.Empty(t, res.Modifiers)
			}
		})
	}
}

func TestResolveSavingThrow_Errors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("character error", func(t *testing.T) {
		f := newFixture(t)
		f.chars.EXPECT().GetCharacter(mock.Anything, charID).Return(nil, domain.ErrCharacterNotFound)

		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, domain.ErrCharacterNotFound))
	})

	t.Run("aggregate error", func(t *testing.T) {
		f := newFixture(t)
		f.withChar(bard())
		f.dice.EXPECT().RollD20().Return(10)
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(buffsvc.BuffModifiers{}, boom)

		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
		assert.Nil(t, res)
		assert.True(t, errors.Is(err, boom))
	})

	t.Run("unknown category", func(t *testing.T) {
		f := newFixture(t) // no call reaches a port
		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, "fire")
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("record error", func(t *testing.T) {
		f := newFixture(t)
		f.withChar(bard())
		f.dice.EXPECT().RollD20().Return(10)
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, 12).Return(strMods(12), nil)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(0, boom)

		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, boom)
	})
}

// noTx runs the unit of work directly.
type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
