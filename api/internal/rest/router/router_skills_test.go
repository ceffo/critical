package router_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
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
		BardSkills: []skillsvc.BardSkill{{SkillPercent: ruletablesvc.SkillPercent{
			Skill: "climb_walls", Base: 50, Points: 35, Race: 0, Dexterity: 0, Armor: -5, Percent: 80,
		}}},
	}, nil)
	rec := do(h, "GET", "/api/skills", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"proficiencies":[{"id":"musical_instrument","ability":"dex","abilityScore":17,"modifier":-1,"target":16,
			"last":{"entryId":7,"roll":18,"target":16,"adjustment":3,"adjustmentReason":"rope","adjustedTarget":19,"success":true}}],
		"bardSkills":[{"id":"climb_walls","base":50,"points":35,"race":0,"dexterity":0,"armor":-5,"percent":80,
			"maxPercent":95,"maxPoints":95,"usable":true,"last":null}],
		"thiefSkills":[]}`, rec.Body.String())
}

func TestSkills_ListUsableIsTheServersCall(t *testing.T) {
	h, m := newRouter(t)
	m.skills.EXPECT().List(mock.Anything, int64(1)).Return(&skillsvc.Skills{
		ThiefSkills: []skillsvc.ThiefSkill{{SkillPercent: ruletablesvc.SkillPercent{Skill: "climb_walls", Percent: 0}}},
		BardSkills:  []skillsvc.BardSkill{{SkillPercent: ruletablesvc.SkillPercent{Skill: "climb_walls", Percent: 0}}},
	}, nil)
	rec := do(h, "GET", "/api/skills", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		ThiefSkills []struct{ Usable bool } `json:"thiefSkills"`
		BardSkills  []struct{ Usable bool } `json:"bardSkills"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.False(t, got.ThiefSkills[0].Usable, "a thieving skill under 1% waits for points")
	assert.True(t, got.BardSkills[0].Usable, "a bard skill has no minimum")
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

// ADR 0028 steps 1 and 5: POST /api/roll/ability/{ability} takes no body,
// passes the key to the service and returns the full breakdown with the
// entry id; an unknown key is the service's 400.
func TestAbilityCheck_RollWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.skills.EXPECT().RollAbility(mock.Anything, int64(1), "str").Return(&skillsvc.AbilityCheckResult{
		Ability: skillsvc.AbilityStr, Die: 20, Roll: 14, AbilityScore: 12, Target: 16,
		AdjustedTarget: 16, Success: true, RolledSuccess: true, LogEntryID: 21,
	}, nil)
	m.skills.EXPECT().RollAbility(mock.Anything, int64(1), "luck").Return(nil, fmt.Errorf("%w: unknown ability", domain.ErrInvalidInput))

	rec := do(h, "POST", "/api/roll/ability/str", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ability":"str","die":20,"roll":14,"abilityScore":12,"target":16,
		"adjustment":0,"adjustedTarget":16,"success":true,"rolledSuccess":true,"logEntryId":21}`, rec.Body.String())

	rec = do(h, "POST", "/api/roll/ability/luck", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// ADR 0028 step 6: POST /api/roll/ability/{entryId}/adjust passes the entry
// id, adjustment and reason to the service and returns the recomputed check.
func TestAbilityCheck_AdjustWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.skills.EXPECT().AdjustAbility(mock.Anything, int64(1), int64(21), skillsvc.Adjustment{Value: -2, Reason: "ledge"}).Return(&skillsvc.AbilityCheckResult{
		Ability: skillsvc.AbilityDex, Die: 20, Roll: 14, AbilityScore: 15, Target: 15,
		Adjustment: -2, AdjustmentReason: "ledge", AdjustedTarget: 13, Success: false, RolledSuccess: true, LogEntryID: 21,
	}, nil)

	rec := do(h, "POST", "/api/roll/ability/21/adjust", `{"adjustment":-2,"reason":"ledge"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"ability":"dex","die":20,"roll":14,"abilityScore":15,"target":15,
		"adjustment":-2,"adjustmentReason":"ledge","adjustedTarget":13,"success":false,"rolledSuccess":true,
		"logEntryId":21}`, rec.Body.String())

	for _, c := range []struct{ name, path, body string }{
		{"adjustment required", "/api/roll/ability/21/adjust", `{"reason":"ledge"}`},
		{"no body", "/api/roll/ability/21/adjust", ""},
		{"bad entry id", "/api/roll/ability/dex/adjust", `{"adjustment":1}`},
	} {
		rec = do(h, "POST", c.path, c.body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, c.name)
	}
}

// ADR 0030 step 17: PUT /api/skills/points sets a thief's points and
// answers the skills; points refused are a 400 naming the field, and a body
// without points is a 400 before the service.
func TestSkills_SetPointsWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.skills.EXPECT().SetPoints(mock.Anything, int64(1), map[string]int{"open_locks": 45}).Return(map[string]int{"open_locks": 45}, nil).Once()
	m.skills.EXPECT().List(mock.Anything, int64(1)).Return(&skillsvc.Skills{}, nil).Once()
	m.skills.EXPECT().SetPoints(mock.Anything, int64(1), map[string]int{"open_locks": 96}).
		Return(nil, &domain.FieldError{Field: "skillPoints.open_locks", Message: "out_of_range"}).Once()

	rec := do(h, "PUT", "/api/skills/points", `{"points":{"open_locks":45}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"proficiencies":[],"bardSkills":[],"thiefSkills":[]}`, rec.Body.String())

	rec = do(h, "PUT", "/api/skills/points", `{"points":{"open_locks":96}}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"field":"skillPoints.open_locks"`)

	for _, body := range []string{`{}`, `{"points":{"open_locks":"x"}}`, `{"points":{},"armor":"none"}`} {
		rec = do(h, "PUT", "/api/skills/points", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}
