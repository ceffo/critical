package dualclasshdl_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/service/dualclasssvc"
	"github.com/ceffo/critical/api/internal/domain/service/dualclasssvc/mocks"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/dualclasshdl"
)

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/character/dual-class", strings.NewReader(body)))
	return rec
}

func handler(svc dualclasssvc.SVC) http.HandlerFunc {
	return render.Handle(dualclasshdl.Switch(func(*http.Request) int64 { return 3 }, svc))
}

func TestSwitch_AnswersWhatTheSwitchDid(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().Switch(mock.Anything, int64(3), "mage").
		Return(&dualclasssvc.Result{From: "fighter", To: "mage", LeftAtLevel: 6, XP: 21_000}, nil).Once()
	rec := post(handler(svc), `{"class":"mage"}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"from":"fighter","to":"mage","leftAtLevel":6,"xp":21000}`, rec.Body.String())
}

func TestSwitch_RefusesABadBodyBeforeTheService(t *testing.T) {
	for _, body := range []string{`{}`, `{"class":""}`, `{"class":"mage","xp":9}`, `not json`} {
		rec := post(handler(mocks.NewMockSVC(t)), body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}

// Each refusal of the service is a 409 with its own code, which the web words.
func TestSwitch_EachRefusalIsAConflictWithItsCode(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{dualclasssvc.ErrNotAllowed, "dualclass_not_allowed"},
		{dualclasssvc.ErrPrimeRequisiteTooLow, "prime_requisite_too_low"},
		{dualclasssvc.ErrLevelTooLow, "level_too_low"},
		{dualclasssvc.ErrClassAlreadyLeft, "class_already_left"},
		{dualclasssvc.ErrSameClass, "dualclass_same_class"},
		{dualclasssvc.ErrMinimumsNotMet, "dualclass_minimums_not_met"},
		{dualclasssvc.ErrAlignmentNotAllowed, "dualclass_alignment_not_allowed"},
	} {
		svc := mocks.NewMockSVC(t)
		svc.EXPECT().Switch(mock.Anything, int64(3), "mage").Return(nil, tc.err).Once()
		rec := post(handler(svc), `{"class":"mage"}`)
		require.Equal(t, http.StatusConflict, rec.Code, tc.code)
		assert.Contains(t, rec.Body.String(), `"code":"`+tc.code+`"`)
	}
}
