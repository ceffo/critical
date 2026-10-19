package router_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
)

func TestDetails_PatchMapsBodyToPatchAndAnswersWithDetails(t *testing.T) {
	h, m := newRouter(t)
	hair, age := "red", 112
	langs := []string{"Elfique"}
	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{Hair: &hair, Age: &age, Languages: &langs}).
		Return(charactersvc.Details{Hair: "red", Age: 112, Languages: []string{"Elfique"}}, nil)
	rec := do(h, "PATCH", "/api/character/details", `{"hair":"red","age":112,"languages":["Elfique"]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"alignment":"","age":112,"sex":"","height":"","weight":"","height_cm":null,"weight_kg":null,"hair":"red","eyes":"","languages":["Elfique"]}`, rec.Body.String())
}

func TestDetails_NullClearsAndAbsentLeavesAlone(t *testing.T) {
	h, m := newRouter(t)
	empty, none := "", []string{}
	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{ClearAge: true, Hair: &empty, Languages: &none}).
		Return(charactersvc.Details{}, nil)
	rec := do(h, "PATCH", "/api/character/details", `{"age":null,"hair":null,"languages":null}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"alignment":"","age":null,"sex":"","height":"","weight":"","height_cm":null,"weight_kg":null,"hair":"","eyes":"","languages":[]}`, rec.Body.String())
}

// ADR 0032 step 5: height (cm) and weight (kg) are JSON numbers, null clears, a value that is no
// number is a not_a_number refusal on its field before any service runs.
func TestDetails_Step5_HeightAndWeightAreNumbers(t *testing.T) {
	h, m := newRouter(t)
	height, weight := 172.5, 61.0
	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{Height: &height, ClearWeight: true}).
		Return(charactersvc.Details{Height: 172.5}, nil)
	rec := do(h, "PATCH", "/api/character/details", `{"height_cm":172.5,"weight_kg":null}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"alignment":"","age":null,"sex":"","height":"172.5 cm","weight":"","height_cm":172.5,"weight_kg":null,"hair":"","eyes":"","languages":[]}`, rec.Body.String())

	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{Weight: &weight}).Return(charactersvc.Details{Weight: 61}, nil)
	require.Equal(t, http.StatusOK, do(h, "PATCH", "/api/character/details", `{"weight_kg":61}`).Code)

	for body, field := range map[string]string{`{"height_cm":"tall"}`: "height", `{"weight_kg":[1]}`: "weight", `{"height":"tall"}`: "height", `{"weight":"heavy"}`: "weight"} {
		rec := do(h, "PATCH", "/api/character/details", body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.JSONEq(t, `{"error":"not_a_number","field":"`+field+`","code":"invalid_input"}`, rec.Body.String(), body)
	}
}

// ADR 0032 step 8: the previous release's clients still send and read the free-text keys; the
// text is parsed like the migration's, an empty one clears, one with no figure is not_a_number, and
// height_cm wins over height.
func TestDetails_Step8_5_LegacyTextKeysStillWork(t *testing.T) {
	h, m := newRouter(t)
	height, weight := 180.0, 74.8
	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{Height: &height, Weight: &weight}).
		Return(charactersvc.Details{Height: 180, Weight: 74.8}, nil).Once()
	rec := do(h, "PATCH", "/api/character/details", `{"height":"1m80","weight":"165 lb"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"alignment":"","age":null,"sex":"","height":"180 cm","weight":"74.8 kg","height_cm":180,"weight_kg":74.8,"hair":"","eyes":"","languages":[]}`, rec.Body.String())

	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{ClearHeight: true, ClearWeight: true}).
		Return(charactersvc.Details{}, nil).Once()
	require.Equal(t, http.StatusOK, do(h, "PATCH", "/api/character/details", `{"height":"","weight":""}`).Code)

	other := 170.0
	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{Height: &other}).
		Return(charactersvc.Details{Height: 170}, nil).Once()
	require.Equal(t, http.StatusOK, do(h, "PATCH", "/api/character/details", `{"height":"1m80","height_cm":170}`).Code)
}

func TestDetails_OnlyPresentFieldsReachTheService(t *testing.T) {
	h, m := newRouter(t)
	eyes := "green"
	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{Eyes: &eyes}).Return(charactersvc.Details{Eyes: "green"}, nil)
	require.Equal(t, http.StatusOK, do(h, "PATCH", "/api/character/details", `{"eyes":"green"}`).Code)
}

func TestDetails_ZeroAgeIsPassedOnToBeRejected(t *testing.T) {
	h, m := newRouter(t)
	zero := 0
	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{Age: &zero}).
		Return(charactersvc.Details{}, &domain.FieldError{Field: "age", Message: detailssvc.CodeOutOfRange})
	assert.Equal(t, http.StatusBadRequest, do(h, "PATCH", "/api/character/details", `{"age":0}`).Code)
}

func TestDetails_FieldErrorNamesTheField(t *testing.T) {
	h, m := newRouter(t)
	m.details.EXPECT().Update(mock.Anything, int64(1), mock.Anything).
		Return(charactersvc.Details{}, &domain.FieldError{Field: "age", Message: detailssvc.CodeOutOfRange})
	rec := do(h, "PATCH", "/api/character/details", `{"age":-1}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"out_of_range","field":"age","code":"invalid_input"}`, rec.Body.String())
}

func TestDetails_RejectsUnknownFieldAndBadJSON(t *testing.T) {
	h, _ := newRouter(t)
	for _, b := range []string{`{"name":"x"}`, `nope`} {
		assert.Equal(t, http.StatusBadRequest, do(h, "PATCH", "/api/character/details", b).Code, b)
	}
}
