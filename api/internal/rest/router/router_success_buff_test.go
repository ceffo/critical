package router_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/acsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/skillsvc"
)

// ADR 0037 step 20.4 on the wire: a check whose success put a buff on answers it (row id, stored
// name, the proficiency it is named from, effects keyed like the buff API, rounds) and whether it
// renewed one, as its entry's payload records it; the adjustment's answer the same; a check that
// put none omits both. The buff list and the AC rows serve the buff's proficiencyId (step 20.1),
// so the web names it in the player's language.
func TestSkillCheck_Step20_EntryCarriesBuffSnapshots(t *testing.T) {
	ac := -4
	tumbling := buffsvc.BuffEffect{
		ID: "row-1", Name: "tumbling", ProficiencyID: "tumbling", Duration: 1, StartRound: 3,
		Effects: buffsvc.BuffEffects{AC: &ac},
	}
	h, m := newRouter(t)
	m.skills.EXPECT().RollInSet(mock.Anything, int64(1), "", "tumbling", skillsvc.Adjustment{}).Return(&skillsvc.CheckResult{
		ID: "tumbling", Kind: skillsvc.KindProficiency, Die: 20, Roll: 12, Target: 17, AdjustedTarget: 17,
		Success: true, RolledSuccess: true, Ability: skillsvc.AbilityDex, AbilityScore: 17, LogEntryID: 12,
		Buff: &skillsvc.AppliedBuff{Buff: tumbling, Renewed: true},
	}, nil).Once()
	m.skills.EXPECT().Adjust(mock.Anything, int64(1), int64(12), skillsvc.Adjustment{Value: -6}).Return(&skillsvc.CheckResult{
		ID: "tumbling", Kind: skillsvc.KindProficiency, Die: 20, Roll: 12, Target: 17, Adjustment: -6, AdjustedTarget: 11,
		Success: false, RolledSuccess: true, Ability: skillsvc.AbilityDex, AbilityScore: 17, LogEntryID: 12,
	}, nil).Once()

	rec := do(h, "POST", "/api/roll/skill/tumbling", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"id":"tumbling","kind":"proficiency","die":20,"roll":12,"target":17,
		"adjustment":0,"adjustedTarget":17,"success":true,"rolledSuccess":true,
		"ability":"dex","abilityScore":17,"modifier":0,"extraSlots":0,"logEntryId":12,
		"buff":{"id":"row-1","name":"tumbling","proficiencyId":"tumbling","effects":{"ac":-4},"rounds":1},
		"renewed":true}`, rec.Body.String())

	rec = do(h, "POST", "/api/roll/skill/12/adjust", `{"adjustment":-6}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"buff"`)
	assert.NotContains(t, rec.Body.String(), `"renewed"`)

	m.buffs.EXPECT().ListStatus(mock.Anything, int64(1)).Return([]buffsvc.BuffStatus{{BuffEffect: tumbling, Applies: true}}, nil).Once()
	m.buffs.EXPECT().ListPowers(mock.Anything, int64(1)).Return(nil, nil).Once()
	rec = do(h, "GET", "/api/buffs", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"buffs":[{"id":"row-1","name":"tumbling","charges":0,"duration":1,"effects":{"ac":-4},"note":"",
		"startRound":3,"chargesTotal":0,"proficiencyId":"tumbling","applies":true}],"powers":[]}`, rec.Body.String())

	m.ac.EXPECT().Get(mock.Anything, int64(1)).Return(&acsvc.AC{
		Value: 6, Rows: []acsvc.Row{{Term: acsvc.TermBase, Value: 10}, {Term: acsvc.TermBuffs, Buff: "tumbling", ProficiencyID: "tumbling", Value: -4}},
	}, nil).Once()
	rec = do(h, "GET", "/api/ac", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `{"term":"buffs","buff":"tumbling","proficiencyId":"tumbling","value":-4}`)
}
