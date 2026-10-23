package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	"github.com/ceffo/critical/api/internal/domain/slots"
)

// picked are a level 15 bard's pools with Intelligence 14, two non-weapon picks (Tumbling with
// an extra slot, target Dex 17 + 1, and an id the catalogue no longer holds) and two weapon
// picks, with their slot warnings.
func picked() *skillsvc.Proficiencies {
	seven, three := 7, 3
	return &skillsvc.Proficiencies{
		Slots: &skills.ProficiencySlots{
			Nonweapon: slots.NonweaponPool{Initial: 3, PerLevels: 4, LevelGained: 3, Intelligence: 4, Available: 10, Spent: 3, Left: 7},
			Weapon:    slots.WeaponPool{Initial: 2, PerLevels: 4, LevelGained: 3, Available: 5, Spent: 2, Left: 3},
			Picks: []skills.PickFigures{
				{ID: "tumbling", Slots: 2, Cost: 1, Extra: 1, Ability: "dex", Known: true, Target: 18, HasCheck: true},
				{ID: "old_skill", Slots: 1, Cost: 1},
			},
			WeaponPicks: []skills.WeaponPickFigures{{Type: "long_sword", Slots: 1, Allowed: true}, {Type: "dagger", Slots: 1, Allowed: true}},
			WeaponTypes: []skills.WeaponTypeFigure{{Type: "long_sword"}, {Type: "dagger", SpecializationSlots: 3}},
		},
		Warnings: []skills.Warning{
			{Code: "slots_to_spend", Field: "nonweapon", Pool: "nonweapon", Left: &seven},
			{Code: "slots_to_spend", Field: "weapon", Pool: "weapon", Left: &three},
			{Code: "proficiency_unknown", Field: "nonweapon.old_skill"},
		},
	}
}

const pickedBody = `{"nonweapon":[{"id":"tumbling","slots":2},{"id":"old_skill","slots":1}],
	"weapon":[{"type":"long_sword","slots":1},{"type":"dagger","slots":1}]}`

const pickedAnswer = `{
	"proficiencySlots":{
		"nonWeapon":{"initial":3,"perLevels":4,"levelGained":3,"intelligence":4,"available":10,"spent":3,"left":7,"languages":0},
		"weapon":{"initial":2,"perLevels":4,"levelGained":3,"available":5,"spent":2,"left":3},
		"weaponPicks":[{"type":"long_sword","slots":1,"allowed":true},{"type":"dagger","slots":1,"allowed":true}],
		"weaponTypes":[{"type":"long_sword","specializationSlots":0},{"type":"dagger","specializationSlots":3}],
		"picks":[
			{"id":"tumbling","slots":2,"cost":1,"extraSlots":1,"ability":"dex","modifier":0,"known":true,"target":18},
			{"id":"old_skill","slots":1,"cost":1,"extraSlots":0,"ability":"","modifier":0,"known":false,"target":null}
		]},
	"warnings":[
		{"code":"slots_to_spend","field":"nonweapon","pool":"nonweapon","left":7},
		{"code":"slots_to_spend","field":"weapon","pool":"weapon","left":3},
		{"code":"proficiency_unknown","field":"nonweapon.old_skill"}
	]}`

// ADR 0037 steps 7 and 12: PUT /api/proficiencies takes the whole lists {nonweapon: [{id,
// slots}], weapon: [{type, slots}]} and answers both pools with their terms, each non-weapon
// pick's cost, extra slots and check target, each weapon pick, and the slot warnings; POST
// /api/proficiencies/preview answers the same shape for the same body.
func TestProficiencies_Step12_WireShape(t *testing.T) {
	want := skillsvc.Edit{Picks: skills.Picks{
		Nonweapon: []skills.Pick{{ID: "tumbling", Slots: 2}, {ID: "old_skill", Slots: 1}},
		Weapon:    []skills.WeaponPick{{Type: "long_sword", Slots: 1}, {Type: "dagger", Slots: 1}},
	}}
	for _, tc := range []struct{ method, path string }{
		{"PUT", "/api/proficiencies"},
		{"POST", "/api/proficiencies/preview"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			h, m := newRouter(t)
			if tc.method == "PUT" {
				m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), want).Return(picked(), nil).Once()
			} else {
				m.skills.EXPECT().PreviewProficiencies(mock.Anything, int64(1), want).Return(picked(), nil).Once()
			}
			rec := do(h, tc.method, tc.path, pickedBody)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.JSONEq(t, pickedAnswer, rec.Body.String())
		})
	}
	t.Run("an empty list removes every pick", func(t *testing.T) {
		h, m := newRouter(t)
		none := skillsvc.Edit{Picks: skills.Picks{Nonweapon: []skills.Pick{}}}
		m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), none).Return(&skillsvc.Proficiencies{}, nil).Once()
		rec := do(h, "PUT", "/api/proficiencies", `{"nonweapon":[]}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, `{"proficiencySlots":null,"warnings":[]}`, rec.Body.String())
	})
	t.Run("an absent pool is left as stored (nil to the service)", func(t *testing.T) {
		h, m := newRouter(t)
		weaponOnly := skillsvc.Edit{Picks: skills.Picks{Weapon: []skills.WeaponPick{{Type: "sling", Slots: 1}}}}
		m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), weaponOnly).Return(&skillsvc.Proficiencies{}, nil).Once()
		rec := do(h, "PUT", "/api/proficiencies", `{"weapon":[{"type":"sling","slots":1}]}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		nonweaponOnly := skillsvc.Edit{Picks: skills.Picks{Nonweapon: []skills.Pick{}}}
		m.skills.EXPECT().PreviewProficiencies(mock.Anything, int64(1), nonweaponOnly).Return(&skillsvc.Proficiencies{}, nil).Once()
		rec = do(h, "POST", "/api/proficiencies/preview", `{"nonweapon":[]}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}

// ADR 0037 step 18: the body's `languages` (the codes; absent or null left as stored) reach the
// service with no pick pool, alone or with one; a languages refusal is the 409 over_budget on
// field languages, as the details' save was.
func TestProficiencies_Step18_TheLanguagesPool(t *testing.T) {
	h, m := newRouter(t)
	alone := skillsvc.Edit{Languages: []string{"elvish", "gnomish"}}
	m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), alone).Return(picked(), nil).Once()
	rec := do(h, "PUT", "/api/proficiencies", `{"languages":["elvish","gnomish"]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, pickedAnswer, rec.Body.String())

	both := skillsvc.Edit{Picks: skills.Picks{Nonweapon: []skills.Pick{{ID: "tumbling", Slots: 1}}}, Languages: []string{}}
	m.skills.EXPECT().PreviewProficiencies(mock.Anything, int64(1), both).Return(picked(), nil).Once()
	rec = do(h, "POST", "/api/proficiencies/preview", `{"nonweapon":[{"id":"tumbling","slots":1}],"languages":[]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.BudgetError{Field: "languages", Spent: 11, Limit: 10}).Once()
	rec = do(h, "PUT", "/api/proficiencies", `{"languages":["elvish","gnomish","orcish"]}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"languages over budget: 11 of 10","code":"over_budget","field":"languages","spent":11,"limit":10}`, rec.Body.String())
}

// The refusals: a body with no pool or with an unknown field is a 400; a refused pick a 400
// naming its field and code; a save that grows an overspend a 409 over_budget with the field,
// the slots spent and the slots available (ADR 0037 step 10).
func TestProficiencies_Step12_Refusals(t *testing.T) {
	for _, body := range []string{`{}`, `{"nonweapon":null,"weapon":null,"languages":null}`, `{"weapon":[],"skills":[]}`, `nope`} {
		h, _ := newRouter(t) // the service mock fails any call
		rec := do(h, "PUT", "/api/proficiencies", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
	h, m := newRouter(t)
	m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.FieldError{Field: "nonweapon.juggling", Message: "unknown"}).Once()
	rec := do(h, "PUT", "/api/proficiencies", `{"nonweapon":[{"id":"juggling","slots":1}]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"field":"nonweapon.juggling"`)

	m.skills.EXPECT().PreviewProficiencies(mock.Anything, int64(1), mock.Anything).
		Return(nil, fmt.Errorf("preview: %w", &domain.BudgetError{Field: "nonweapon", Spent: 11, Limit: 10})).Once()
	rec = do(h, "POST", "/api/proficiencies/preview", `{"nonweapon":[{"id":"tumbling","slots":11}]}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"preview: nonweapon over budget: 11 of 10","code":"over_budget","field":"nonweapon","spent":11,"limit":10}`, rec.Body.String())

	m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.FieldError{Field: "weapon.long_sword", Message: "weapon_not_allowed"}).Once()
	rec = do(h, "PUT", "/api/proficiencies", `{"weapon":[{"type":"long_sword","slots":1}]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"weapon_not_allowed","code":"invalid_input","field":"weapon.long_sword"}`, rec.Body.String())

	m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), mock.Anything).
		Return(nil, fmt.Errorf("save: %w", &domain.BudgetError{Field: "weapon", Spent: 6, Limit: 5})).Once()
	rec = do(h, "PUT", "/api/proficiencies", `{"weapon":[{"type":"dagger","slots":1}]}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.JSONEq(t, `{"error":"save: weapon over budget: 6 of 5","code":"over_budget","field":"weapon","spent":6,"limit":5}`, rec.Body.String())
}
