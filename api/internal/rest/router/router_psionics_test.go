package router_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/psionics"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/psionicsvc"
)

// PUT /api/psionics/picks, a psionicist's known powers (ADR 0043 step 9): the
// whole list reaches the service as sent and the answer is the known powers
// as saved, in the derived character's `knownPowers` shape.
func TestPicks_Step9_PutWireShape(t *testing.T) {
	h, m := newRouter(t)
	want := []psionics.Pick{{Kind: psionics.PickDiscipline, ID: "telepathy"}, {Kind: psionics.PickPower, ID: "contact"}}
	m.psionics.EXPECT().SetKnownPowers(mock.Anything, int64(1), want).Return(&charactersvc.PowerKnowledge{
		Allowance: psionics.Allowance{Disciplines: 3, Sciences: 1, Devotions: 4, DefenceModes: 1},
		Used:      psionics.Used{Disciplines: 1, Devotions: 1},
		Primary:   "telepathy",
		Picks:     want,
	}, nil).Once()
	rec := do(h, "PUT", "/api/psionics/picks", `{"picks":[{"kind":"discipline","id":"telepathy"},{"kind":"power","id":"contact"}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"allowance":{"disciplines":3,"sciences":1,"devotions":4,"defenceModes":1},
		"used":{"disciplines":1,"sciences":0,"devotions":1,"defenceModes":0},
		"primary":"telepathy",
		"picks":[{"kind":"discipline","id":"telepathy"},{"kind":"power","id":"contact"}]}`, rec.Body.String())

	rec = do(h, "PUT", "/api/psionics/picks", `{"picks":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A list over the rules is a 409 with the rule's own code and the figures the
// web words; a character with no psionics is a 409 too; a malformed list is a
// 400 field error on "picks".
func TestPicks_Step9_PutRefusals(t *testing.T) {
	h, m := newRouter(t)
	body := `{"picks":[{"kind":"discipline","id":"telepathy"}]}`
	over := &psionics.Refusal{Code: psionics.CodeAllowanceExceeded, Slot: psionics.SlotDevotion, Count: 4, Limit: 3}
	m.psionics.EXPECT().SetKnownPowers(mock.Anything, int64(1), mock.Anything).
		Return(nil, over).Once()
	rec := do(h, "PUT", "/api/psionics/picks", body)
	require.Equal(t, http.StatusConflict, rec.Code)
	wantBody, err := json.Marshal(map[string]any{
		"code": "psionic_allowance_exceeded", "error": over.Error(),
		"power": "", "discipline": "", "slot": "devotion", "count": 4, "limit": 3,
	})
	require.NoError(t, err)
	assert.JSONEq(t, string(wantBody), rec.Body.String())

	m.psionics.EXPECT().SetKnownPowers(mock.Anything, int64(1), mock.Anything).Return(nil, psionicsvc.ErrNotPsionicist).Once()
	rec = do(h, "PUT", "/api/psionics/picks", body)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"not_a_psionicist"`)

	m.psionics.EXPECT().SetKnownPowers(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.FieldError{Field: "picks", Message: "unknown power"}).Once()
	rec = do(h, "PUT", "/api/psionics/picks", body)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `picks`)
}

// POST /api/psionics/powers/{id}/use, a power's check and its spend (ADR 0043
// steps 10 and 11): the power's id, the modifiers, the cost and the score
// reach the service as sent; the answer is the check's breakdown, the pool
// after the spend and the entry's id.
func TestUse_Step10_PostWireShape(t *testing.T) {
	h, m := newRouter(t)
	cost, score := 6, 14
	want := psionicsvc.UseRequest{
		Modifiers: []psionicsvc.Modifier{{Value: -2, Reason: "distance"}}, Cost: &cost, Score: &score,
	}
	m.psionics.EXPECT().Use(mock.Anything, int64(1), "mind-bar", want).Return(&psionicsvc.UseResult{
		Check: psionicsvc.CheckRecord{
			Power: "mind-bar", Ability: "int", AbilityScore: 15, PowerModifier: -4, Relearns: 1,
			Modifiers: want.Modifiers, Target: 10, Roll: 12, Outcome: psionics.OutcomeFailure, Cost: 6, Spent: 3,
		},
		PSP: actionlog.PSPState{Current: 20, Max: 40}, LogEntryID: 9,
	}, nil).Once()
	rec := do(h, "POST", "/api/psionics/powers/mind-bar/use", `{"modifiers":[{"value":-2,"reason":"distance"}],"cost":6,"score":14}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"check":{"power":"mind-bar","ability":"int","abilityScore":15,"powerModifier":-4,"relearns":1,
			"modifiers":[{"value":-2,"reason":"distance"}],"target":10,"d20":12,"outcome":"failure",
			"success":false,"cost":6,"spent":3},
		"psp":{"current":20,"max":40},
		"logEntryId":9}`, rec.Body.String())

	rec = do(h, "POST", "/api/psionics/powers/mind-bar/use", `{"modifiers":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// The use's refusals answer their own codes: 409 for the rules', 400 for a
// field error.
func TestUse_Step11_PostRefusals(t *testing.T) {
	h, m := newRouter(t)
	for err, want := range map[error]string{
		psionicsvc.ErrNotPsionicist: "not_a_psionicist",
		psionicsvc.ErrPowerNotKnown: "power_not_known",
		psionicsvc.ErrNeedsContact:  "needs_contact",
		psionicsvc.ErrNotEnoughPSP:  "not_enough_psp",
	} {
		m.psionics.EXPECT().Use(mock.Anything, int64(1), "contact", mock.Anything).Return(nil, err).Once()
		rec := do(h, "POST", "/api/psionics/powers/contact/use", `{}`)
		require.Equal(t, http.StatusConflict, rec.Code, want)
		assert.Contains(t, rec.Body.String(), `"code":"`+want+`"`)
	}
	m.psionics.EXPECT().Use(mock.Anything, int64(1), "contact", mock.Anything).
		Return(nil, &domain.FieldError{Field: "cost", Message: "give it"}).Once()
	rec := do(h, "POST", "/api/psionics/powers/contact/use", `{}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `cost`)
}

func TestStop_Step16_Post(t *testing.T) {
	h, m := newRouter(t)
	m.psionics.EXPECT().Stop(mock.Anything, int64(1), "lift").Return(nil, nil).Once()
	rec := do(h, "POST", "/api/psionics/powers/lift/stop", ``)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"powerId":"lift"}`, rec.Body.String())

	m.psionics.EXPECT().Stop(mock.Anything, int64(1), "lift").Return(nil, psionicsvc.ErrPowerNotMaintained).Once()
	rec = do(h, "POST", "/api/psionics/powers/lift/stop", ``)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"power_not_maintained"`)
}

// POST /api/psionics/powers/{id}/raise: the wire shape and the refusals'
// codes (ADR 0044 step 4).
func TestRaise_Step4_Post(t *testing.T) {
	h, m := newRouter(t)
	m.psionics.EXPECT().RaiseDefence(mock.Anything, int64(1), "mind-blank").Return(&psionicsvc.RaiseResult{
		Raise:      psionicsvc.RaiseRecord{Power: "mind-blank", Defence: buffsvc.DefenceRaised, Cost: 1},
		PSP:        actionlog.PSPState{Current: 19, Max: 40},
		LogEntryID: 4, BuffID: "b1",
	}, nil).Once()
	rec := do(h, "POST", "/api/psionics/powers/mind-blank/raise", ``)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"raise":{"power":"mind-blank","defence":"raised","cost":1},
		"psp":{"current":19,"max":40},"logEntryId":4,"buffId":"b1"}`, rec.Body.String())

	for err, want := range map[error]string{
		psionicsvc.ErrNotInCombat:         "not_in_combat",
		psionicsvc.ErrNotADefenceMode:     "not_a_defence_mode",
		psionicsvc.ErrNotEnoughPSP:        "not_enough_psp",
		psionicsvc.ErrNeedsContact:        "needs_contact",
		psionicsvc.ErrPowerNotKnown:       "power_not_known",
		psionicsvc.ErrNotPsionicist:       "not_a_psionicist",
		psionicsvc.ErrDefenceModeIsRaised: "defence_mode_is_raised",
	} {
		m.psionics.EXPECT().RaiseDefence(mock.Anything, int64(1), "x").Return(nil, err).Once()
		rec := do(h, "POST", "/api/psionics/powers/x/raise", ``)
		require.Equal(t, http.StatusConflict, rec.Code, want)
		assert.Contains(t, rec.Body.String(), `"code":"`+want+`"`)
	}
}

// POST /api/psionics/defend: the attack, the rolls and the keep reach the
// service as sent; a field error answers 400.
func TestDefend_Step8_Post(t *testing.T) {
	h, m := newRouter(t)
	keep := 1
	want := psionicsvc.DefendRequest{
		Attack: "psionic-blast", Keep: &keep,
		Rolls: []psionics.ContestSide{{Roll: 12, Success: true}, {Roll: 3, Success: false}},
	}
	m.psionics.EXPECT().Defend(mock.Anything, int64(1), want).Return(&psionicsvc.DefendResult{
		Defence: psionicsvc.DefenceRecord{Attack: "psionic-blast", Contests: []psionicsvc.ContestRecord{}}, LogEntryID: 8,
	}, nil).Once()
	rec := do(h, "POST", "/api/psionics/defend",
		`{"attack":"psionic-blast","rolls":[{"roll":12,"success":true},{"roll":3,"success":false}],"keep":1}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"defence":{"attack":"psionic-blast","contests":[]},"logEntryId":8}`, rec.Body.String())

	m.psionics.EXPECT().Defend(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.FieldError{Field: "rolls", Message: "one or 2 rolls"}).Once()
	rec = do(h, "POST", "/api/psionics/defend", `{"attack":"x","rolls":[]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `rolls`)

	rec = do(h, "POST", "/api/psionics/defend", `{"rolls":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// POST /api/psionics/powers/{id}/attack: the opponent, the DM's rolls and the
// modifiers reach the service as sent, the payload's wire shape is the
// journal's, and the refusals' codes are stable (ADR 0044 step 7).
func TestAttack_Step7_Post(t *testing.T) {
	h, m := newRouter(t)
	vs := 2
	want := psionicsvc.AttackRequest{
		Mode: "ego-whip",
		Opponent: psionics.Opponent{
			Target: "Orc", Mind: "closed", Level: 5, Defence: "thought-shield",
			Rolls: []psionics.ContestSide{{Roll: 10, Success: true}, {Roll: 3, Success: false}},
		},
		Modifiers: []psionicsvc.Modifier{{Value: -2, Reason: "medium range"}},
	}
	m.psionics.EXPECT().Attack(mock.Anything, int64(1), want).Return(&psionicsvc.AttackResult{
		Attack: psionicsvc.AttackRecord{
			Power: "ego-whip", Score: 13, Target: "Orc", Mind: "closed",
			Defence: "thought-shield", AttackVersusDefence: &vs,
			Checks: []psionicsvc.AttackCheck{
				{
					Check:   psionics.ContestCheck{Target: 15, Roll: 12, Outcome: "success"},
					Defence: &psionics.ContestSide{Roll: 10, Success: true}, Winner: "attacker",
				},
				{
					Check:  psionics.ContestCheck{Target: 15, Roll: 2, Outcome: "success"},
					Winner: "attacker", Effect: true,
				},
			},
			Cost: 4, Spent: 4, TangentsBefore: 2, TangentsAfter: 3, Contact: true, Broken: "Goblin",
		},
		PSP: actionlog.PSPState{Current: 16, Max: 40}, LogEntryID: 9,
	}, nil).Once()
	rec := do(h, "POST", "/api/psionics/powers/ego-whip/attack",
		`{"target":"Orc","mind":"closed","level":5,"defence":"thought-shield",
		"rolls":[{"roll":10,"success":true},{"roll":3,"success":false}],
		"modifiers":[{"value":-2,"reason":"medium range"}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"attack":{"power":"ego-whip","score":13,"target":"Orc","mind":"closed",
		"defence":"thought-shield","attackVersusDefence":2,
		"checks":[
		  {"check":{"target":15,"d20":12,"outcome":"success"},"defence":{"roll":10,"success":true},"winner":"attacker"},
		  {"check":{"target":15,"d20":2,"outcome":"success"},"winner":"attacker","effect":true}],
		"cost":4,"spent":4,"tangentsBefore":2,"tangentsAfter":3,"contact":true,"broken":"Goblin"},
		"psp":{"current":16,"max":40},"logEntryId":9}`, rec.Body.String())

	for err, want := range map[error]string{
		psionicsvc.ErrNotAnAttackMode: "not_an_attack_mode",
		psionicsvc.ErrNotEnoughPSP:    "not_enough_psp",
		psionicsvc.ErrNeedsContact:    "needs_contact",
		psionicsvc.ErrPowerNotKnown:   "power_not_known",
		psionicsvc.ErrNotPsionicist:   "not_a_psionicist",
	} {
		m.psionics.EXPECT().Attack(mock.Anything, int64(1), mock.Anything).Return(nil, err).Once()
		rec := do(h, "POST", "/api/psionics/powers/x/attack", `{"target":"Orc","mind":"open"}`)
		require.Equal(t, http.StatusConflict, rec.Code, want)
		assert.Contains(t, rec.Body.String(), `"code":"`+want+`"`)
	}

	m.psionics.EXPECT().Attack(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.FieldError{Field: "target", Message: "a target is 1 to 60 characters"}).Once()
	rec = do(h, "POST", "/api/psionics/powers/x/attack", `{"mind":"open"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `target`)

	rec = do(h, "POST", "/api/psionics/powers/x/attack", `{"target":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// POST /api/psionics/powers/{id}/contact: the opponent, the distance, the
// life order, the resisting flag and the modifiers reach the service as sent;
// the answer is the Use one with the contact lines (ADR 0044 step 9).
func TestContact_Step9_Post(t *testing.T) {
	h, m := newRouter(t)
	want := psionicsvc.ContactRequest{
		Power:     "contact",
		Opponent:  psionics.Opponent{Target: "Bird", Mind: "open", Level: 7},
		Distance:  "miles_10",
		LifeOrder: "bird",
		Resisting: true,
		Modifiers: []psionicsvc.Modifier{{Value: 1, Reason: "calm"}},
	}
	m.psionics.EXPECT().Contact(mock.Anything, int64(1), want).Return(&psionicsvc.UseResult{
		Check: psionicsvc.CheckRecord{
			Power: "contact", Ability: "wis", AbilityScore: 15, Target: 7, Roll: 10, Outcome: psionics.OutcomeSuccess,
			Success: true, Cost: 8, Spent: 8, Opponent: "Bird", Level: 7, Contact: true,
			ContactModifiers: []psionicsvc.ContactLine{
				{Kind: "distance", Code: "miles_10", Modifier: -3},
				{Kind: "life_order", Code: "bird", Modifier: -3},
				{Kind: "resistance", Code: "resisting", Modifier: -2},
			},
		},
		PSP: actionlog.PSPState{Current: 20, Max: 40}, LogEntryID: 9,
	}, nil).Once()
	rec := do(h, "POST", "/api/psionics/powers/contact/contact",
		`{"target":"Bird","mind":"open","level":7,"distance":"miles_10","lifeOrder":"bird","resisting":true,
		"modifiers":[{"value":1,"reason":"calm"}]}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"check":{"power":"contact","ability":"wis","abilityScore":15,"target":7,"d20":10,
		"outcome":"success","success":true,"cost":8,"spent":8,"opponent":"Bird","level":7,
		"contactModifiers":[{"kind":"distance","code":"miles_10","modifier":-3},
		  {"kind":"life_order","code":"bird","modifier":-3},{"kind":"resistance","code":"resisting","modifier":-2}],
		"contact":true},
		"psp":{"current":20,"max":40},"logEntryId":9}`, rec.Body.String())

	for err, want := range map[error]string{
		psionicsvc.ErrNotAContactPower: "not_a_contact_power",
		psionicsvc.ErrMindClosed:       "mind_closed",
		psionicsvc.ErrNotEnoughPSP:     "not_enough_psp",
		psionicsvc.ErrPowerNotKnown:    "power_not_known",
		psionicsvc.ErrNotPsionicist:    "not_a_psionicist",
	} {
		m.psionics.EXPECT().Contact(mock.Anything, int64(1), mock.Anything).Return(nil, err).Once()
		rec := do(h, "POST", "/api/psionics/powers/x/contact", `{"target":"Orc","mind":"open"}`)
		require.Equal(t, http.StatusConflict, rec.Code, want)
		assert.Contains(t, rec.Body.String(), `"code":"`+want+`"`)
	}

	m.psionics.EXPECT().Contact(mock.Anything, int64(1), mock.Anything).Return(nil, psionicsvc.ErrLevelOutOfTable).Once()
	rec = do(h, "POST", "/api/psionics/powers/x/contact", `{"target":"Orc","mind":"open","level":99}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"level_out_of_table"`)

	m.psionics.EXPECT().Contact(mock.Anything, int64(1), mock.Anything).
		Return(nil, &domain.FieldError{Field: "distance", Message: "unknown distance"}).Once()
	rec = do(h, "POST", "/api/psionics/powers/x/contact", `{"target":"Orc","mind":"open","distance":"far"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `distance`)

	// Q15: the DM's cost for a level past the table reaches the service.
	cost := 13
	m.psionics.EXPECT().Contact(mock.Anything, int64(1), psionicsvc.ContactRequest{
		Power: "contact", Opponent: psionics.Opponent{Target: "Orc", Mind: "open", Level: 21}, Cost: &cost,
		Distance: "sight", LifeOrder: "mammal",
	}).Return(&psionicsvc.UseResult{Check: psionicsvc.CheckRecord{Power: "contact", Cost: 13, Level: 21}}, nil).Once()
	rec = do(h, "POST", "/api/psionics/powers/contact/contact",
		`{"target":"Orc","mind":"open","level":21,"cost":13,"distance":"sight","lifeOrder":"mammal"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = do(h, "POST", "/api/psionics/powers/x/contact", `{"target":`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A power whose cost is contact names its target on the use route; its
// refusal is no_contact (ADR 0044 step 10).
func TestUse_Step10_PostTarget(t *testing.T) {
	h, m := newRouter(t)
	m.psionics.EXPECT().Use(mock.Anything, int64(1), "inflict_pain", psionicsvc.UseRequest{Target: "Orc"}).
		Return(&psionicsvc.UseResult{
			Check: psionicsvc.CheckRecord{
				Power: "inflict_pain", Target: 12, Roll: 5, Outcome: psionics.OutcomeSuccess,
				Success: true, Cost: 0, Opponent: "Orc",
			},
			PSP: actionlog.PSPState{Current: 30, Max: 40}, LogEntryID: 4,
		}, nil).Once()
	rec := do(h, "POST", "/api/psionics/powers/inflict_pain/use", `{"target":"Orc"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"opponent":"Orc"`)

	m.psionics.EXPECT().Use(mock.Anything, int64(1), "inflict_pain", mock.Anything).Return(nil, psionicsvc.ErrNoContact).Once()
	rec = do(h, "POST", "/api/psionics/powers/inflict_pain/use", `{"target":"Elf"}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"no_contact"`)

	// ADR 0044 step 9 (Q15): the contact power goes through its own route.
	m.psionics.EXPECT().Use(mock.Anything, int64(1), "contact", mock.Anything).Return(nil, psionicsvc.ErrUseContactRoute).Once()
	rec = do(h, "POST", "/api/psionics/powers/contact/use", `{"cost":13}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"use_contact_route"`)
}

// GET /api/character carries the power used this round in a psionicist's
// known powers (ADR 0043 step 12), omitted when the round is free; a sheet
// with no known powers never asks.
func TestCharacter_Step12_ServesThePowerUsedThisRound(t *testing.T) {
	for _, c := range []struct {
		name string
		used string
		want string
	}{
		{"a power was used", "lift", `"usedThisRound":"lift"`},
		{"the round is free", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			h, m := newRouter(t)
			m.char.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(&charactersvc.Character{
				ID: 1, Name: "Joran",
				Derived: charactersvc.Derived{Powers: &charactersvc.PowerKnowledge{Primary: "telepathy"}},
			}, nil).Once()
			m.psionics.EXPECT().PowerThisRound(mock.Anything, int64(1)).Return(c.used, nil).Once()
			rec := do(h, "GET", "/api/character", "")
			require.Equal(t, http.StatusOK, rec.Code)
			if c.want == "" {
				assert.NotContains(t, rec.Body.String(), "usedThisRound")
				return
			}
			assert.Contains(t, rec.Body.String(), c.want)
		})
	}
}

func TestContactPreview_Step9_PostAndCostTableGet(t *testing.T) {
	h, m := newRouter(t)
	want := psionicsvc.ContactRequest{
		Power:     "contact",
		Opponent:  psionics.Opponent{Target: "Bird", Mind: "open", Level: 7},
		Distance:  "sight",
		LifeOrder: "bird",
	}
	m.psionics.EXPECT().ContactPreview(mock.Anything, int64(1), want).Return(&psionicsvc.ContactPreview{
		Cost: 8, FromTable: true, Target: 11, Current: 20, Enough: true,
		Lines: []psionicsvc.ContactLine{{Kind: "distance", Code: "sight", Modifier: 0}},
	}, nil).Once()
	rec := do(h, "POST", "/api/psionics/powers/contact/contact/preview",
		`{"target":"Bird","mind":"open","level":7,"distance":"sight","lifeOrder":"bird"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"cost":8,"fromTable":true,"target":11,"lines":[{"kind":"distance","code":"sight","modifier":0}],
		"current":20,"enough":true}`, rec.Body.String())

	m.psionics.EXPECT().ContactPreview(mock.Anything, int64(1), mock.Anything).Return(nil, psionicsvc.ErrMindClosed).Once()
	rec = do(h, "POST", "/api/psionics/powers/contact/contact/preview", `{"target":"Orc","mind":"closed"}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"mind_closed"`)

	m.psionics.EXPECT().CostTable(mock.Anything, "contact_cost").Return(psionicsvc.CostTable{
		Table: "contact_cost", MaxLevel: 20,
		Bands:     []psionicsvc.CostBand{{MinLevel: 1, MaxLevel: 2, Cost: 2}},
		Modifiers: []psionicsvc.ContactLine{{Kind: "resistance", Code: "resisting", Modifier: -2}},
	}, nil).Once()
	rec = do(h, "GET", "/api/psionics/cost-tables/contact_cost", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"table":"contact_cost","maxLevel":20,"bands":[{"minLevel":1,"maxLevel":2,"cost":2}],
		"modifiers":[{"kind":"resistance","code":"resisting","modifier":-2}]}`, rec.Body.String())

	m.psionics.EXPECT().CostTable(mock.Anything, "nothing").Return(psionicsvc.CostTable{}, domain.ErrNotFound).Once()
	rec = do(h, "GET", "/api/psionics/cost-tables/nothing", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
