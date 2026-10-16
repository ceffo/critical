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
)

func TestSkills_ListWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.skills.EXPECT().List(mock.Anything, int64(1)).Return(&skillsvc.Skills{
		Proficiencies: []skillsvc.Proficiency{
			{
				ID: "musical_instrument", Ability: skillsvc.AbilityDex, AbilityScore: 17, Modifier: -1, Target: 16,
				Last: &skillsvc.LastCheck{EntryID: 7, Roll: 18, Target: 16, Adjustment: 3, AdjustmentReason: "rope", AdjustedTarget: 19, Success: true},
			},
		},
		BardSkills: []skillsvc.BardSkill{{ID: "climb_walls", Percent: 85}},
	}, nil)
	rec := do(h, "GET", "/api/skills", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"proficiencies":[{"id":"musical_instrument","ability":"dex","abilityScore":17,"modifier":-1,"target":16,
			"last":{"entryId":7,"roll":18,"target":16,"adjustment":3,"adjustmentReason":"rope","adjustedTarget":19,"success":true}}],
		"bardSkills":[{"id":"climb_walls","percent":85,"last":null}]}`, rec.Body.String())
}

func TestSkills_RollWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.skills.EXPECT().Roll(mock.Anything, int64(1), "tumbling", skillsvc.Adjustment{Value: -2, Reason: "rope"}).Return(&skillsvc.CheckResult{
		ID: "tumbling", Kind: skillsvc.KindProficiency, Die: 20, Roll: 20, Target: 17,
		Adjustment: -2, AdjustmentReason: "rope", AdjustedTarget: 15, Success: false, RolledSuccess: false,
		Ability: skillsvc.AbilityDex, AbilityScore: 17, Modifier: 0, LogEntryID: 12,
	}, nil)
	m.skills.EXPECT().Roll(mock.Anything, int64(1), "climb_walls", skillsvc.Adjustment{}).Return(&skillsvc.CheckResult{
		ID: "climb_walls", Kind: skillsvc.KindBardSkill, Die: 100, Roll: 40, Target: 85, AdjustedTarget: 85,
		Success: true, RolledSuccess: true, LogEntryID: 13,
	}, nil).Twice()
	m.skills.EXPECT().Roll(mock.Anything, int64(1), "juggling", skillsvc.Adjustment{}).Return(nil, fmt.Errorf("%w: unknown skill", domain.ErrNotFound))

	rec := do(h, "POST", "/api/roll/skill/tumbling", `{"adjustment":-2,"reason":"rope"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"tumbling","kind":"proficiency","die":20,"roll":20,"target":17,
		"adjustment":-2,"adjustmentReason":"rope","adjustedTarget":15,"success":false,"rolledSuccess":false,
		"ability":"dex","abilityScore":17,"modifier":0,"logEntryId":12}`, rec.Body.String())

	// No body is no adjustment, as is an empty object.
	for _, body := range []string{"", "{}"} {
		rec = do(h, "POST", "/api/roll/skill/climb_walls", body)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"id":"climb_walls","kind":"bardSkill","die":100,"roll":40,"target":85,
			"adjustment":0,"adjustedTarget":85,"success":true,"rolledSuccess":true,"logEntryId":13}`, rec.Body.String())
	}

	rec = do(h, "POST", "/api/roll/skill/juggling", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = do(h, "POST", "/api/roll/skill/tumbling", `{"adjustment":"two"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// POST /api/roll/skill/{id}/adjust passes the entry id, adjustment and reason
// to the service and returns the recomputed check.
func TestSkills_AdjustWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.skills.EXPECT().Adjust(mock.Anything, int64(1), int64(12), skillsvc.Adjustment{Value: 3, Reason: "help"}).Return(&skillsvc.CheckResult{
		ID: "tumbling", Kind: skillsvc.KindProficiency, Die: 20, Roll: 18, Target: 17,
		Adjustment: 3, AdjustmentReason: "help", AdjustedTarget: 20, Success: true, RolledSuccess: false,
		Ability: skillsvc.AbilityDex, AbilityScore: 17, Modifier: 0, LogEntryID: 12,
	}, nil)
	m.skills.EXPECT().Adjust(mock.Anything, int64(1), int64(12), skillsvc.Adjustment{Value: 0}).Return(&skillsvc.CheckResult{
		ID: "climb_walls", Kind: skillsvc.KindBardSkill, Die: 100, Roll: 90, Target: 85, AdjustedTarget: 85,
		Success: false, RolledSuccess: true, LogEntryID: 12,
	}, nil)

	rec := do(h, "POST", "/api/roll/skill/12/adjust", `{"adjustment":3,"reason":"help"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"tumbling","kind":"proficiency","die":20,"roll":18,"target":17,
		"adjustment":3,"adjustmentReason":"help","adjustedTarget":20,"success":true,"rolledSuccess":false,
		"ability":"dex","abilityScore":17,"modifier":0,"logEntryId":12}`, rec.Body.String())

	rec = do(h, "POST", "/api/roll/skill/12/adjust", `{"adjustment":0}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"climb_walls","kind":"bardSkill","die":100,"roll":90,"target":85,
		"adjustment":0,"adjustedTarget":85,"success":false,"rolledSuccess":true,"logEntryId":12}`, rec.Body.String())

	for _, c := range []struct{ name, path, body string }{
		{"adjustment required", "/api/roll/skill/12/adjust", `{"reason":"help"}`},
		{"no body", "/api/roll/skill/12/adjust", ""},
		{"bad entry id", "/api/roll/skill/tumbling/adjust", `{"adjustment":1}`},
	} {
		rec = do(h, "POST", c.path, c.body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, c.name)
	}
}
