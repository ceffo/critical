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
