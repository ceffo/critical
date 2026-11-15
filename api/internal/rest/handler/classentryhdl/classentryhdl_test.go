package classentryhdl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/classentrysvc"
	"github.com/ceffo/critical/api/internal/domain/service/classentrysvc/mocks"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/classentryhdl"
)

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/character/class-entry/class", strings.NewReader(body)))
	return rec
}

func handler(svc classentrysvc.SVC) http.HandlerFunc {
	return render.Handle(classentryhdl.Switch(func(*http.Request) int64 { return 3 }, svc))
}

// ADR 0050 step 15: the entry's index (0 included) and the class go to the service.
func TestSwitch_Step15_AnswersWhatTheSwitchDid(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().Switch(mock.Anything, int64(3), 0, "transmuter").
		Return(&classentrysvc.Result{Entry: 0, From: "mage", To: "transmuter"}, nil).Once()
	rec := post(handler(svc), `{"entry":0,"class":"transmuter"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"entry":0,"from":"mage","to":"transmuter"}`, rec.Body.String())
}

func TestSwitch_Step15_RefusesABadBodyBeforeTheService(t *testing.T) {
	for _, body := range []string{`{}`, `{"class":"transmuter"}`, `{"entry":0}`, `{"entry":0,"class":""}`, `{"entry":0,"class":"mage","xp":9}`, `not json`} {
		rec := post(handler(mocks.NewMockSVC(t)), body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}

// Each refusal of the service is a 409 with its own code, which the web words.
func TestSwitch_Step15_EachRefusalIsAConflictWithItsCode(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{classentrysvc.ErrNotAllowed, "class_switch_not_allowed"},
		{classentrysvc.ErrMinimumsNotMet, "class_switch_minimums_not_met"},
		{classentrysvc.ErrAlignmentNotAllowed, "class_switch_alignment_not_allowed"},
		{classentrysvc.ErrMultiClassNotAllowed, "multiclass_class_not_allowed"},
	} {
		svc := mocks.NewMockSVC(t)
		svc.EXPECT().Switch(mock.Anything, int64(3), 1, "mage").Return(nil, tc.err).Once()
		rec := post(handler(svc), `{"entry":1,"class":"mage"}`)
		require.Equal(t, http.StatusConflict, rec.Code, tc.code)
		assert.Contains(t, rec.Body.String(), `"code":"`+tc.code+`"`)
	}
}

func get(h http.HandlerFunc, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/character/class-entry/targets"+query, nil))
	return rec
}

// ADR 0050 step 15.6: the picker's list, the refusal as its code and the minimums against the score.
func TestTargets_Step15_AnswersWhatThePickerShows(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().Targets(mock.Anything, int64(3), 0).Return(&classentrysvc.TargetList{Entry: 0, From: "mage", Targets: []classentrysvc.Target{
		{
			Class: "abjurer", Schools: []string{"abjuration"}, OpposedSchools: []string{"alteration"}, RaceWarning: true,
			Minimums: []classentrysvc.Minimum{{Ability: "wis", Min: 15, Score: 8}}, Refusal: classentrysvc.ErrMinimumsNotMet,
		},
		{Class: "mage"},
	}}, nil).Once()
	rec := get(render.Handle(classentryhdl.Targets(func(*http.Request) int64 { return 3 }, svc)), "?entry=0")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"entry":0,"from":"mage","targets":[
	 {"class":"abjurer","schools":["abjuration"],"opposedSchools":["alteration"],"raceWarning":true,"refusal":"class_switch_minimums_not_met","minimums":[{"ability":"wis","min":15,"score":8,"met":false}]},
	 {"class":"mage","schools":null,"opposedSchools":null,"raceWarning":false,"refusal":"","minimums":[]}]}`, rec.Body.String())
}

func TestTargets_Step15_RefusesABadEntryBeforeTheService(t *testing.T) {
	for _, q := range []string{"", "?entry=", "?entry=x"} {
		rec := get(render.Handle(classentryhdl.Targets(func(*http.Request) int64 { return 3 }, mocks.NewMockSVC(t))), q)
		assert.Equal(t, http.StatusBadRequest, rec.Code, q)
	}
}
