package spellprephdl_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc/mocks"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
	"github.com/ceffo/critical/api/internal/rest/handler/spellprephdl"
	"github.com/ceffo/critical/api/pkg/formula"
)

// The spellbook entry carries the items the service says the spell can spend,
// as id, name and count; the handler decides none of it.
func TestGetInfo_ServesTheSpendableItems(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().InfoFor(mock.Anything, int64(1), "").Return(prepsvc.Info{Spellbook: []prepsvc.SpellbookEntry{
		{ID: "alarm", Name: "Alarm", Level: 1},
		{
			ID: "chromatic_orb", Name: "Chromatic Orb", Level: 1,
			Cast:  prepsvc.CastInputs{ConsumeItemType: "gem", AttackRoll: true},
			Items: []itemsvc.Item{{ID: "ruby", Name: localized.Of("Ruby"), Component: "gem", Count: 2}},
		},
	}}, nil)

	h := spellprephdl.GetInfo(func(*http.Request) int64 { return 1 }, svc)
	rec := httptest.NewRecorder()
	require.NoError(t, h(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/spells/preparation", nil)))
	require.Equal(t, http.StatusOK, rec.Code)

	var got spellprephdl.InfoDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	spellbook := got.Spellbook
	require.Len(t, spellbook, 2)
	assert.Empty(t, spellbook[0].Items)
	assert.Equal(t, []spellprephdl.SpendableItemDTO{{ID: "ruby", Name: localized.Of("Ruby"), Count: 2}}, spellbook[1].Items)
}

// ADR 0008 step 14: the spellbook entry serves the cast-time modes, each with
// the damage a cast in it rolls (none for a mode without damage).
func TestGetInfo_ServesTheModes(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().InfoFor(mock.Anything, int64(1), "").Return(prepsvc.Info{Spellbook: []prepsvc.SpellbookEntry{{
		ID: "ice_storm", Name: "Ice Storm", Level: 4,
		Cast: prepsvc.CastInputs{Modes: []spelleffectsvc.ModeAt{
			{Key: "hail", Damage: &spelleffectsvc.DamageAt{
				CasterLevel: 15, Dice: formula.Sum{Dice: []formula.Dice{{Count: 3, Sides: 10}}},
				Instances: formula.Sum{Constant: 1}, Rounds: formula.Sum{Constant: 1}, Type: "cold",
			}},
			{Key: "sleet"},
		}},
	}}}, nil)

	h := spellprephdl.GetInfo(func(*http.Request) int64 { return 1 }, svc)
	rec := httptest.NewRecorder()
	require.NoError(t, h(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/spells/preparation", nil)))
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Spellbook []struct {
			Modes json.RawMessage `json:"modes"`
		} `json:"spellbook"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Spellbook, 1)
	assert.JSONEq(t, `[
		{"key":"hail","damage":{"caster_level":15,"type":"cold","dice":{"groups":[{"count":3,"sides":10}],"modifier":0},
			"instances":{"groups":[],"modifier":1},"rounds":{"groups":[],"modifier":1}}},
		{"key":"sleet"}]`, string(got.Spellbook[0].Modes))
}

// The spellbook entry serves the target choice of a buff's cast as the
// default and the alternate ("self", "other"); none when the service gives
// none.
func TestGetInfo_ServesTheCastTarget(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().InfoFor(mock.Anything, int64(1), "").Return(prepsvc.Info{Spellbook: []prepsvc.SpellbookEntry{
		{ID: "alarm", Name: "Alarm", Level: 1},
		{ID: "slow", Name: "Slow", Level: 3, Cast: prepsvc.CastInputs{Target: &spelleffectsvc.TargetChoice{
			Default: spelleffectsvc.CastOnOther, Alternate: spelleffectsvc.CastOnSelf,
		}}},
	}}, nil)

	h := spellprephdl.GetInfo(func(*http.Request) int64 { return 1 }, svc)
	rec := httptest.NewRecorder()
	require.NoError(t, h(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/spells/preparation", nil)))
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Spellbook []map[string]json.RawMessage `json:"spellbook"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Spellbook, 2)
	assert.NotContains(t, got.Spellbook[0], "cast_target", "no choice: the field is absent")
	assert.JSONEq(t, `{"default":"other","alternate":"self"}`, string(got.Spellbook[1]["cast_target"]))
}

// ADR 0030 step 20.5: every spellbook entry serves its origin, a level_grant
// one with its grant level; a priest's sphere spell (no origin) serves none.
func TestGetInfo_ServesTheOrigin(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().InfoFor(mock.Anything, int64(1), "").Return(prepsvc.Info{Spellbook: []prepsvc.SpellbookEntry{
		{ID: "sleep", Name: "Sleep", Level: 1, Origin: spellbook.OriginLearned},
		{ID: "fireball", Name: "Fireball", Level: 3, Origin: spellbook.OriginLevelGrant, GrantLevel: 5},
		{ID: "bless", Name: "Bless", Level: 1},
	}}, nil)

	h := spellprephdl.GetInfo(func(*http.Request) int64 { return 1 }, svc)
	rec := httptest.NewRecorder()
	require.NoError(t, h(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/spells/preparation", nil)))
	require.Equal(t, http.StatusOK, rec.Code)
	var got struct {
		Spellbook []json.RawMessage `json:"spellbook"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Len(t, got.Spellbook, 3)
	assert.JSONEq(t, `{"id":"sleep","name":"Sleep","level":1,"origin":"learned"}`, string(got.Spellbook[0]))
	assert.JSONEq(t, `{"id":"fireball","name":"Fireball","level":3,"origin":"level_grant","grant_level":5}`, string(got.Spellbook[1]))
	assert.JSONEq(t, `{"id":"bless","name":"Bless","level":1}`, string(got.Spellbook[2]))
}
