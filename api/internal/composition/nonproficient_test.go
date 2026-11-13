package composition_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type modView struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
	Value  int    `json:"value"`
}

func hasNonproficient(mods []modView) bool {
	for _, m := range mods {
		if m.Kind == "nonproficient" {
			return true
		}
	}
	return false
}

// ADR 0037 step 8 through the API on the fixture's seed character, narrowed by step 21.9: Kelsen's
// weapon picks are his weapons' codes (step 21.8.7, the seed's weapon_proficiencies), so none of
// his coded weapons takes the non-proficiency penalty, in the preview or in the roll; his Mace +1
// matches no catalogue row, so it has no code and takes the rogue's -3 (step 21.4, its detail
// empty), and so does his bastard sword on the fixture catalogue, whose placeholder names match no
// bastard sword row (the real artifact's do: seedcheck's TestKelsen_Step21_8_WeaponPicksAreHisWeapons).
// Once the long sword's pick is swapped for a sling's, the long sword's preview and roll both carry
// the -3 as the named modifier nonproficient, with the weapon's code as its detail, and the same
// effective THAC0.
func TestKelsen_Step8_OwnWeaponsTakeNoPenalty(t *testing.T) {
	tb, _ := newTable(t)
	root, _ := tb.login("")
	var me struct {
		CurrentID *int64 `json:"currentId"`
	}
	require.Equal(t, http.StatusOK, tb.call(root, "GET", "/api/characters", "", &me))
	kelsen := *me.CurrentID

	type formView struct {
		Preview *struct {
			Modifiers      []modView `json:"modifiers"`
			EffectiveThac0 int       `json:"effectiveThac0"`
		} `json:"preview"`
	}
	type rollView struct {
		Modifiers      []modView `json:"modifiers"`
		EffectiveThac0 int       `json:"effectiveThac0"`
	}
	hold := func(weapon string) formView {
		var v formView
		require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "PATCH", "/api/attack/form", `{"weaponId":"`+weapon+`","targetAc":5}`, &v))
		require.NotNil(t, v.Preview, weapon)
		return v
	}
	roll := func(body string) rollView {
		var r rollView
		require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "POST", "/api/roll/attack", body, &r), body)
		return r
	}

	for _, w := range []string{"longsword", "shortbow", "dagger", "darts"} {
		v := hold(w)
		assert.False(t, hasNonproficient(v.Preview.Modifiers), "%s: proficient through his picks", w)
	}
	for _, w := range []string{"mace", "bastard_sword"} {
		v := hold(w)
		assert.Contains(t, v.Preview.Modifiers, modView{Kind: "nonproficient", Value: -3}, "%s has no catalogue code", w)
	}
	for _, body := range []string{
		`{"weapon_id":"longsword","target_ac":5,"target_size":"sm"}`,
		`{"weapon_id":"shortbow","ammo_item_id":"arrows_plus1","target_ac":5,"target_size":"sm"}`,
	} {
		assert.False(t, hasNonproficient(roll(body).Modifiers), body)
	}

	swap := `{"weapon":[{"code":"short_bow","slots":1},{"code":"dagger","slots":1},{"code":"dart","slots":1},` +
		`{"code":"sling","slots":1},{"code":"bastard_sword_two_handed","slots":1}]}`
	require.Equal(t, http.StatusOK, tb.showing(root, kelsen, "PUT", "/api/proficiencies", swap, nil))

	v := hold("longsword")
	assert.Contains(t, v.Preview.Modifiers, modView{Kind: "nonproficient", Detail: "long_sword", Value: -3})
	r := roll(`{"weapon_id":"longsword","target_ac":5,"target_size":"sm"}`)
	assert.Equal(t, v.Preview.Modifiers, r.Modifiers, "the preview is the roll's breakdown")
	assert.Equal(t, v.Preview.EffectiveThac0, r.EffectiveThac0)
	assert.False(t, hasNonproficient(hold("dagger").Preview.Modifiers), "the dagger keeps its pick")
}
