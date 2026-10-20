package router_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
)

// ADR 0030 step 7.3: POST /api/character/levelup/roll passes the level (and
// the manual entry's die) and answers the full breakdown, with the levels
// still owed, each with what it grants (ADR 0037 step 15).
func TestLevelUp_RollAnswersTheFullBreakdown(t *testing.T) {
	h, m := newRouter(t)
	rolled, counted := 1, 2
	m.levelUp.EXPECT().Roll(mock.Anything, int64(1), levelupsvc.RollInput{Level: 9}).Return(&levelupsvc.Result{
		Level: 9, HitDie: 6, Rolled: &rolled, Counted: &counted, ConAdjustment: 2, Gain: 4,
		HPMaxBefore: 40, HPMaxAfter: 44,
		Pending: []charactersvc.PendingLevel{{Level: 10, Die: 6, Grants: skills.LevelGrants{SkillPoints: 30}}, {Level: 11, FixedGain: 2}},
	}, nil).Once()
	rec := do(h, "POST", "/api/character/levelup/roll", `{"level":9}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"level":9,"hitDie":6,"rolled":1,"counted":2,"conAdjustment":2,"gain":4,"fixed":false,
		"manual":false,"hpMaxBefore":40,"hpMaxAfter":44,
		"pending":[{"level":10,"die":6,"fixedGain":0,"grants":{"weaponSlots":0,"nonWeaponSlots":0,"skillPoints":30,"freeSpells":[]}},
			{"level":11,"die":0,"fixedGain":2,"grants":{"weaponSlots":0,"nonWeaponSlots":0,"skillPoints":0,"freeSpells":[]}}]}`, rec.Body.String())

	// A fixed gain: no die, nulls; nothing owed is an empty list.
	m.levelUp.EXPECT().Roll(mock.Anything, int64(1), levelupsvc.RollInput{Level: 16}).Return(&levelupsvc.Result{
		Level: 16, Gain: 2, Fixed: true, HPMaxBefore: 85, HPMaxAfter: 87,
	}, nil).Once()
	rec = do(h, "POST", "/api/character/levelup/roll", `{"level":16}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"level":16,"hitDie":0,"rolled":null,"counted":null,"conAdjustment":0,"gain":2,"fixed":true,
		"manual":false,"hpMaxBefore":85,"hpMaxAfter":87,"pending":[]}`, rec.Body.String())
}

// The manual entry sends the die rolled at the table; its field error names
// "die".
func TestLevelUp_TheManualEntryPassesTheDie(t *testing.T) {
	h, m := newRouter(t)
	die := 7
	m.levelUp.EXPECT().Roll(mock.Anything, int64(1), levelupsvc.RollInput{Level: 9, Die: &die}).
		Return(nil, &domain.FieldError{Field: "die", Message: levelupsvc.CodeOutOfRange}).Once()
	rec := do(h, "POST", "/api/character/levelup/roll", `{"level":9,"die":7}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"code":"invalid_input","error":"out_of_range","field":"die"}`, rec.Body.String())
}

// A body without the level, with an unknown field or not JSON is refused
// before the service.
func TestLevelUp_RefusesABadBody(t *testing.T) {
	h, _ := newRouter(t)
	for _, body := range []string{`{}`, `{"die":3}`, `{"level":9,"extra":1}`, `nope`} {
		rec := do(h, "POST", "/api/character/levelup/roll", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}
