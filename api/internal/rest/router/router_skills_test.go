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
				Last: &skillsvc.LastCheck{EntryID: 7, Roll: 18, Target: 16},
			},
		},
		BardSkills: []skillsvc.BardSkill{{ID: "climb_walls", Percent: 85}},
	}, nil)
	rec := do(h, "GET", "/api/skills", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"proficiencies":[{"id":"musical_instrument","ability":"dex","abilityScore":17,"modifier":-1,"target":16,
			"last":{"entryId":7,"roll":18,"target":16,"success":false}}],
		"bardSkills":[{"id":"climb_walls","percent":85,"last":null}]}`, rec.Body.String())
}

func TestSkills_RollWireShape(t *testing.T) {
	h, m := newRouter(t)
	m.skills.EXPECT().Roll(mock.Anything, int64(1), "tumbling").Return(&skillsvc.CheckResult{
		ID: "tumbling", Kind: skillsvc.KindProficiency, Die: 20, Roll: 20, Target: 17, Success: false,
		Ability: skillsvc.AbilityDex, AbilityScore: 17, Modifier: 0, LogEntryID: 12,
	}, nil)
	m.skills.EXPECT().Roll(mock.Anything, int64(1), "climb_walls").Return(&skillsvc.CheckResult{
		ID: "climb_walls", Kind: skillsvc.KindBardSkill, Die: 100, Roll: 40, Target: 85, Success: true, LogEntryID: 13,
	}, nil)
	m.skills.EXPECT().Roll(mock.Anything, int64(1), "juggling").Return(nil, fmt.Errorf("%w: unknown skill", domain.ErrNotFound))

	rec := do(h, "POST", "/api/roll/skill/tumbling", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"tumbling","kind":"proficiency","die":20,"roll":20,"target":17,"success":false,
		"ability":"dex","abilityScore":17,"modifier":0,"logEntryId":12}`, rec.Body.String())

	rec = do(h, "POST", "/api/roll/skill/climb_walls", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"climb_walls","kind":"bardSkill","die":100,"roll":40,"target":85,"success":true,
		"logEntryId":13}`, rec.Body.String())

	rec = do(h, "POST", "/api/roll/skill/juggling", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
