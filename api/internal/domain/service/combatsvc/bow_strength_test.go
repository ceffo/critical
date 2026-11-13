package combatsvc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// Strength with a bow (ADR 0002 step 4 and ADR 0003 step 7, PHB p.90-91 and p.99): a penalty
// always, a bonus only with a strength bow, up to its rating; a crossbow never. The Table 1
// numbers are the fixture catalogue's (the printed table).

// ratedBow is a short bow built for the Strength score/exceptional (ADR 0033 step 3).
func ratedBow(score, exceptional int) combatsvc.Weapon {
	w := plainShortBow
	w.StrengthRating = itemsvc.StrengthRating{Score: score, Exceptional: exceptional}
	return w
}

var bowLightCrossbow = combatsvc.Weapon{
	Wielded: true, ID: "light_crossbow", Name: localized.Of("Light Crossbow"), BaseDamage: "1d4", DamageLarge: "1d4",
	Kind: combatsvc.WeaponMissile, AmmoKind: "quarrel", TwoHanded: true, Type: itemsvc.TypeLightCrossbow,
}

type bowStrengthCase struct {
	name     string
	str, exc int // the effective Strength
	weapon   combatsvc.Weapon
	// want is the strength modifier and damage term expected; nil for none at all.
	want *bowStrengthWant
}

type bowStrengthWant struct {
	detail   string // the score whose Table 1 row applies
	exc      int
	hit, dmg int
}

var bowStrengthCases = []bowStrengthCase{
	// A strength bow: the lower of the Strength and the rating.
	{"18/50 with a bow rated 18/50: Table 1's +1 hit, +3 damage", 18, 50, ratedBow(18, 50), &bowStrengthWant{"18", 50, 1, 3}},
	{"18/51 with a bow rated 18/51: +2 hit, +3 damage", 18, 51, ratedBow(18, 51), &bowStrengthWant{"18", 51, 2, 3}},
	{"18/50 with a bow rated 17: the 17 adjustments", 18, 50, ratedBow(17, 0), &bowStrengthWant{"17", 0, 1, 1}},
	{"18/50 with a bow rated 18/00: the archer's 18/50", 18, 50, ratedBow(18, 100), &bowStrengthWant{"18", 50, 1, 3}},
	{"17 with a bow rated 18/50: the archer's 17", 17, 0, ratedBow(18, 50), &bowStrengthWant{"17", 0, 1, 1}},
	{"18/00 with a bow rated 18: plain 18", 18, 100, ratedBow(18, 0), &bowStrengthWant{"18", 0, 1, 2}},
	{"12 with a bow rated 18: no adjustment at 12", 12, 0, ratedBow(18, 0), &bowStrengthWant{"12", 0, 0, 0}},
	// An ordinary bow: no bonus, the penalty always.
	{"18/50 with an unrated bow: no Strength", 18, 50, plainShortBow, nil},
	{"5 with an unrated bow: the penalty", 5, 0, plainShortBow, &bowStrengthWant{"5", 0, -2, -1}},
	{"7 with an unrated bow: the to-hit penalty, no damage one", 7, 0, plainShortBow, &bowStrengthWant{"7", 0, -1, 0}},
	{"5 with a bow rated 18: the penalty", 5, 0, ratedBow(18, 0), &bowStrengthWant{"5", 0, -2, -1}},
	{"7 with a bow rated 3: the archer's penalty, no damage one", 7, 0, ratedBow(3, 0), &bowStrengthWant{"7", 0, -1, 0}},
	// A crossbow: never, bonus or penalty.
	{"18/50 with a light crossbow: none", 18, 50, bowLightCrossbow, nil},
	{"5 with a light crossbow: none", 5, 0, bowLightCrossbow, nil},
	// Unchanged: a melee weapon takes both.
	{"5 with a long sword: the penalty", 5, 0, raceLongSword, &bowStrengthWant{"5", 0, -2, -1}},
	{"18/50 with a long sword: the bonus", 18, 50, raceLongSword, &bowStrengthWant{"18", 50, 1, 3}},
}

// strengthChar is a level 5 fighter proficient with the weapon's type (STR as the case's, DEX 17).
func strengthChar(t *testing.T, w combatsvc.Weapon) charactersvc.View {
	t.Helper()
	return profChar(t, "fighter", w.Type)
}

// rollAtStr resolves one attack with w at the effective Strength str/exc against AC 5: d20 19 (every case hits),
// no crit, the damage die 3.
func rollAtStr(t *testing.T, str, exc int, w combatsvc.Weapon) *combatsvc.RollResult {
	t.Helper()
	f := newFixture(t)
	f.weapons.EXPECT().Get(mock.Anything, charID, w.ID).Return(w, nil)
	f.withChar(strengthChar(t, w))
	mods := strMods(str)
	mods.EffectiveStrExceptional = exc
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(mods, nil)
	req := combatsvc.AttackRequest{WeaponID: w.ID, TargetAC: intPtr(5)}
	if a := ammoFor(w); a != nil {
		f.items.EXPECT().Use(mock.Anything, charID, a.ID).Return(*a, nil).Once()
		req.AmmoItemID = a.ID
	}
	f.dice.EXPECT().RollD20().Return(19).Once()
	f.d100s(t, 90, noCritMult)
	f.dice.EXPECT().RollDie(mock.Anything).Return(3).Once()
	f.records()
	res, err := f.svc.ResolveAttack(t.Context(), charID, req)
	require.NoError(t, err)
	return res
}

// previewAtStr is the attack form's preview of an attack with w at the effective Strength str/exc.
func previewAtStr(t *testing.T, str, exc int, w combatsvc.Weapon) *combatsvc.AttackPreview {
	t.Helper()
	f := newFixture(t)
	form := combatsvc.AttackForm{WeaponID: w.ID, TargetAC: intPtr(5), TargetSize: combatsvc.SizeSmallMedium}
	items := []itemsvc.Item{}
	if a := ammoFor(w); a != nil {
		form.AmmoItemID = a.ID
		items = append(items, *a)
	}
	f.forms.EXPECT().Get(mock.Anything, charID).Return(form, true, nil).Once()
	f.weapons.EXPECT().List(mock.Anything, charID).Return([]combatsvc.Weapon{w}, nil)
	f.items.EXPECT().List(mock.Anything, charID).Return(items, nil).Maybe()
	f.withChar(strengthChar(t, w))
	mods := strMods(str)
	mods.EffectiveStrExceptional = exc
	f.buffs.EXPECT().Aggregate(mock.Anything, charID, mock.Anything, mock.Anything).Return(mods, nil)
	f.buffs.EXPECT().GetRound(mock.Anything, charID).Return(1, nil).Maybe()
	v, err := f.svc.GetAttackForm(t.Context(), charID)
	require.NoError(t, err)
	require.NotNil(t, v.Preview)
	return v.Preview
}

// strengthOfResult is the strength modifiers and Strength damage terms of an attack.
func strengthOfResult(res *combatsvc.RollResult) ([]combatsvc.Modifier, []combatsvc.DamageTerm) {
	var mods []combatsvc.Modifier
	for _, m := range res.Modifiers {
		if m.Kind == combatsvc.ModStrength {
			mods = append(mods, m)
		}
	}
	var terms []combatsvc.DamageTerm
	for _, d := range res.DamageTerms {
		if d.Kind == combatsvc.ModStrength {
			terms = append(terms, d)
		}
	}
	return mods, terms
}

// ADR 0002 step 4 and ADR 0003 step 7 as amended for the strength bow: one strength to-hit
// modifier and one strength damage term (both or neither), the Table 1 row of the score that
// applies, through the same path as every other weapon's Strength.
func TestResolveAttack_Step4_StrengthWithBowsAndCrossbows(t *testing.T) {
	for _, tc := range bowStrengthCases {
		t.Run(tc.name, func(t *testing.T) {
			res := rollAtStr(t, tc.str, tc.exc, tc.weapon)
			mods, terms := strengthOfResult(res)
			if tc.want == nil {
				assert.Empty(t, mods, "no strength modifier")
				assert.Empty(t, terms, "no strength damage term")
				return
			}
			w := tc.want
			assert.Equal(t, []combatsvc.Modifier{{Kind: combatsvc.ModStrength, Detail: w.detail, StrExceptional: w.exc, Value: w.hit}}, mods)
			assert.Equal(t, []combatsvc.DamageTerm{{Term: combatsvc.TermStrength, Kind: combatsvc.ModStrength, Detail: w.detail, StrExceptional: w.exc, Value: float64(w.dmg)}}, terms)
		})
	}
}

// ADR 0002 step 12: the attack form's preview takes the same Strength terms as the attack.
func TestPreviewAttack_Step4_StrengthWithBowsMatchesTheAttack(t *testing.T) {
	for _, tc := range bowStrengthCases {
		t.Run(tc.name, func(t *testing.T) {
			res := rollAtStr(t, tc.str, tc.exc, tc.weapon)
			p := previewAtStr(t, tc.str, tc.exc, tc.weapon)
			assert.Equal(t, res.Modifiers, p.Modifiers)
			assert.Equal(t, res.EffectiveThac0, p.EffectiveThac0)
		})
	}
}
