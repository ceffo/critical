package router_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// picked is a level 15 bard with Intelligence 14 and two picks: Tumbling with an extra slot
// (target Dex 17 + 1) and an id the catalogue no longer holds.
func picked() *charactersvc.Character {
	return &charactersvc.Character{ID: 1, Class: "bard", Derived: charactersvc.Derived{
		Proficiencies: &charactersvc.ProficiencySlots{
			Nonweapon: ruletablesvc.NonweaponPool{Initial: 3, PerLevels: 4, LevelGained: 3, Intelligence: 4, Available: 10, Spent: 3, Left: 7},
			Picks: []charactersvc.PickFigures{
				{ID: "tumbling", Slots: 2, Cost: 1, Extra: 1, Ability: "dex", Known: true, Target: 18, HasCheck: true},
				{ID: "old_skill", Slots: 1, Cost: 1},
			},
		},
	}}
}

const pickedBody = `{"nonweapon":[{"id":"tumbling","slots":2},{"id":"old_skill","slots":1}]}`

const pickedAnswer = `{
	"proficiencySlots":{
		"nonWeapon":{"initial":3,"perLevels":4,"levelGained":3,"intelligence":4,"available":10,"spent":3,"left":7},
		"picks":[
			{"id":"tumbling","slots":2,"cost":1,"extraSlots":1,"ability":"dex","modifier":0,"known":true,"target":18},
			{"id":"old_skill","slots":1,"cost":1,"extraSlots":0,"ability":"","modifier":0,"known":false,"target":null}
		]},
	"warnings":[
		{"code":"slots_to_spend","field":"nonweapon","pool":"nonweapon","left":7},
		{"code":"proficiency_unknown","field":"nonweapon.old_skill"}
	]}`

// ADR 0037 step 12: PUT /api/proficiencies takes the whole list {nonweapon: [{id, slots}]} and
// answers the pool with its terms, each pick's cost, extra slots and check target, and the slot
// warnings; POST /api/proficiencies/preview answers the same shape for the same body.
func TestProficiencies_Step12_WireShape(t *testing.T) {
	want := charactersvc.Picks{Nonweapon: []charactersvc.Pick{{ID: "tumbling", Slots: 2}, {ID: "old_skill", Slots: 1}}}
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
		none := charactersvc.Picks{Nonweapon: []charactersvc.Pick{}}
		m.skills.EXPECT().SetProficiencies(mock.Anything, int64(1), none).Return(&charactersvc.Character{ID: 1}, nil).Once()
		rec := do(h, "PUT", "/api/proficiencies", `{"nonweapon":[]}`)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.JSONEq(t, `{"proficiencySlots":null,"warnings":[]}`, rec.Body.String())
	})
}

// The refusals: a body without the list or with an unknown field is a 400; a refused pick a 400
// naming its field and code; a save that grows an overspend a 409 over_budget with the field,
// the slots spent and the slots available (ADR 0037 step 10).
func TestProficiencies_Step12_Refusals(t *testing.T) {
	for _, body := range []string{`{}`, `{"nonweapon":[],"weapon":[]}`, `nope`} {
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
}
