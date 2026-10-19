package combatsvc_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/catalogueadpt"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

const charID = int64(1)

// bard is the campaign Bard: derived THAC0 13 (ADR 0030 step 5), sheet STR
// 12, DEX 17, and distinct derived save targets (step 6). Combat reads the
// effective STR from the buffs port, never from the sheet.
func bard() *charactersvc.Character {
	return &charactersvc.Character{
		ID: charID, Str: 12, Dex: 17, Class: "bard", Race: "half_elf",
		Derived: charactersvc.Derived{RealizedLevel: 15, THAC0: 13, Saves: charactersvc.Saves{
			ParaPoison: 11, RodStaffWand: 13, PetriPoly: 12, Breath: 15, Spell: 14,
		}},
	}
}

// str12 is the test bard's sheet Strength as Aggregate receives it.
var str12 = buffsvc.BaseStrength{StrScore: buffsvc.StrScore{Score: 12}}

// strMods is "no buff effect" at effective STR str.
func strMods(str int) buffsvc.BuffModifiers {
	return buffsvc.BuffModifiers{SpellsBuff: 1.0, Multipliers: 1.0, EffectiveStr: str}
}

// phb is the real PHB Table 1/2 lookup from the testdata catalogue fixture
// (format 3, ADR 0021 step 9.7), so the expected numbers below are checked
// against the printed tables.
func phb(t *testing.T) *ruletablesvc.Tables {
	t.Helper()
	artifact, err := catalogueadpt.LoadArtifact(filepath.Join("..", "..", "..", "..", "testdata", "catalogue.json"))
	require.NoError(t, err)
	tables, err := ruletablesadpt.Parse(artifact.Catalogue.Format, artifact.RuleTables, artifact.Equipment, artifact.Languages)
	require.NoError(t, err)
	return tables
}

type fixture struct {
	chars   *mocks.MockCharacters
	buffs   *mocks.MockBuffs
	dice    *mocks.MockDice
	weapons *mocks.MockWeapons
	items   *mocks.MockItems
	forms   *mocks.MockFormStore
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
		forms:   mocks.NewMockFormStore(t),
		crit:    mocks.NewMockCritStore(t),
		log:     mocks.NewMockLog(t),
	}
	f.crit.EXPECT().List(mock.Anything).RunAndReturn(
		func(context.Context) ([]combatsvc.CritRow, error) { return f.critRows, nil }).Maybe()
	svc, err := combatsvc.NewWithConfig(combatsvc.DefaultConfig(), combatsvc.Deps{
		Chars: f.chars, Buffs: f.buffs, Dice: f.dice, Weapons: f.weapons, Crit: f.crit,
		Items: f.items, Forms: f.forms, Rules: phb(t), Log: f.log, Tx: noTx{},
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
	}{
		{"para/poison pass", combatsvc.SaveParaPoison, 11, 0, 11, true, 0},
		{"para/poison fail", combatsvc.SaveParaPoison, 10, 0, 11, false, -1},
		{"rod/staff/wand", combatsvc.SaveRodStaffWand, 13, 0, 13, true, 0},
		{"petrify/poly", combatsvc.SavePetriPoly, 5, 0, 12, false, -7},
		{"breath", combatsvc.SaveBreath, 20, 0, 15, true, 5},
		{"spell", combatsvc.SaveSpell, 14, 0, 14, true, 0},
		{"buff bonus turns a fail into a pass", combatsvc.SaveSpell, 12, 2, 14, true, 0},
		{"negative buff bonus", combatsvc.SaveSpell, 14, -1, 14, false, -1},
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
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(mods, nil)
			f.records()

			res, err := f.svc.ResolveSavingThrow(context.Background(), charID, tt.category)
			require.NoError(t, err)
			assert.Equal(t, int64(42), res.LogEntryID)
			assert.Equal(t, string(tt.category), res.Category)
			assert.Zero(t, res.Adjustment)
			require.Len(t, f.entries, 1)
			assert.Equal(t, actionlog.KindSave, f.entries[0].Kind)
			assert.Empty(t, f.entries[0].Snapshots, "a save changes no state")
			assert.Equal(t, tt.d20, res.D20Roll)
			assert.Equal(t, tt.wantTarget, res.Target)
			assert.Equal(t, tt.wantTarget, res.AdjustedTarget)
			assert.Equal(t, tt.wantHit, res.Passed)
			assert.Equal(t, tt.wantMargin, res.Margin)
			// ADR 0006 step 2: d20 + modifiers, compared with the target.
			assert.Equal(t, tt.saveBonus, res.RollTotal-res.D20Roll)
			assert.Equal(t, tt.d20+tt.saveBonus, res.Total)
			if tt.saveBonus != 0 {
				assert.Equal(t, []combatsvc.BreakdownModifier{{Kind: string(combatsvc.ModBuff), Detail: "Prayer", Value: tt.saveBonus}}, res.Modifiers)
			} else {
				assert.Empty(t, res.Modifiers)
			}
		})
	}
}

// The d20 is plain arithmetic: a natural 1 passes when the bonuses carry it to
// the target and a natural 20 fails when they pull it below.
// ADR 0030 step 22.1 (amending ADR 0006 step 1): the race's save bonus is
// one named modifier on the roll, before the buffs' (kind race, the race code
// as detail): it moves the roll, never the target. A category the race gets
// no bonus in has no race modifier.
func TestResolveSavingThrow_Step22_1_RaceBonusIsANamedModifier(t *testing.T) {
	dwarf := func() *charactersvc.Character {
		c := bard()
		c.Race = "dwarf"
		c.Derived.RaceSaveBonus = charactersvc.Saves{RodStaffWand: 4, Spell: 5}
		return c
	}
	t.Run("rod/staff/wand", func(t *testing.T) {
		f := newFixture(t)
		f.withChar(dwarf())
		f.dice.EXPECT().RollD20().Return(9).Once()
		mods := strMods(12)
		mods.SaveBonus = 1
		mods.SaveBonusFrom = []buffsvc.Contribution{{Buff: "Prayer", Value: 1}}
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(mods, nil).Once()
		f.records()

		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveRodStaffWand)
		require.NoError(t, err)
		assert.Equal(t, []combatsvc.BreakdownModifier{
			{Kind: string(combatsvc.ModRace), Detail: "dwarf", Value: 4},
			{Kind: string(combatsvc.ModBuff), Detail: "Prayer", Value: 1},
		}, res.Modifiers)
		assert.Equal(t, 13, res.Target, "the target is the derived one, unmoved")
		assert.Equal(t, 14, res.RollTotal, "9 + 4 + 1")
		assert.True(t, res.Passed)
	})
	t.Run("spell: the category's own bonus", func(t *testing.T) {
		f := newFixture(t)
		f.withChar(dwarf())
		f.dice.EXPECT().RollD20().Return(9).Once()
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Once()
		f.records()
		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
		require.NoError(t, err)
		assert.Equal(t, []combatsvc.BreakdownModifier{{Kind: string(combatsvc.ModRace), Detail: "dwarf", Value: 5}}, res.Modifiers)
		assert.Equal(t, 14, res.RollTotal)
	})
	t.Run("para/poison: none", func(t *testing.T) {
		f := newFixture(t)
		f.withChar(dwarf())
		f.dice.EXPECT().RollD20().Return(9).Once()
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil).Once()
		f.records()
		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveParaPoison)
		require.NoError(t, err)
		assert.Empty(t, res.Modifiers)
		assert.Equal(t, 11, res.Target)
	})
}

func TestResolveSavingThrow_NaturalOneAndTwentyArePlain(t *testing.T) {
	tests := []struct {
		name      string
		d20       int
		saveBonus int
		wantHit   bool
		wantTotal int
	}{
		{"natural 1 passes with +13", 1, 13, true, 14},
		{"natural 1 fails with +12", 1, 12, false, 13},
		{"natural 20 fails with -7", 20, -7, false, 13},
		{"natural 20 passes with -6", 20, -6, true, 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			f.withChar(bard())
			f.dice.EXPECT().RollD20().Return(tt.d20).Once()
			mods := strMods(12)
			mods.SaveBonus = tt.saveBonus
			mods.SaveBonusFrom = []buffsvc.Contribution{{Buff: "Luck", Value: tt.saveBonus}}
			f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(mods, nil)
			f.records()

			res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
			require.NoError(t, err)
			assert.Equal(t, tt.wantHit, res.Passed)
			assert.Equal(t, tt.wantTotal, res.Total, "against target 14")
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
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(buffsvc.BuffModifiers{}, boom)

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
		f.buffs.EXPECT().Aggregate(mock.Anything, charID, str12, mock.Anything).Return(strMods(12), nil)
		f.log.EXPECT().Record(mock.Anything, mock.Anything).Return(0, boom)

		res, err := f.svc.ResolveSavingThrow(context.Background(), charID, combatsvc.SaveSpell)
		assert.Nil(t, res)
		assert.ErrorIs(t, err, boom)
	})
}

// noTx runs the unit of work directly.
type noTx struct{}

func (noTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
