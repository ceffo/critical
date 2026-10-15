package router_test

import (
	"net/http"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDetails_PatchMapsBodyToPatchAndAnswersWithDetails(t *testing.T) {
	h, m := newRouter(t)
	race, age := "Half-elf", 112
	langs := []string{"Elfique"}
	m.details.EXPECT().Update(mock.Anything, int64(1), detailssvc.Patch{Race: &race, Age: &age, Languages: &langs}).
		Return(charactersvc.Details{Race: "Half-elf", Age: 112, Languages: []string{"Elfique"}}, nil)
	rec := do(h, "PATCH", "/api/character/details", `{"race":"Half-elf","age":112,"languages":["Elfique"]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"race":"Half-elf","alignment":"","age":112,"sex":"","height":"","weight":"","hair":"","eyes":"","languages":["Elfique"]}`, rec.Body.String())
}

func TestDetails_UnsetAgeIsNull(t *testing.T) {
	h, m := newRouter(t)
	m.details.EXPECT().Update(mock.Anything, int64(1), mock.Anything).Return(charactersvc.Details{}, nil)
	rec := do(h, "PATCH", "/api/character/details", `{"age":0}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"race":"","alignment":"","age":null,"sex":"","height":"","weight":"","hair":"","eyes":"","languages":[]}`, rec.Body.String())
}

func TestDetails_FieldErrorNamesTheField(t *testing.T) {
	h, m := newRouter(t)
	m.details.EXPECT().Update(mock.Anything, int64(1), mock.Anything).
		Return(charactersvc.Details{}, &domain.FieldError{Field: "age", Message: detailssvc.CodeOutOfRange})
	rec := do(h, "PATCH", "/api/character/details", `{"age":-1}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"out_of_range","field":"age"}`, rec.Body.String())
}

func TestDetails_RejectsUnknownFieldAndBadJSON(t *testing.T) {
	h, _ := newRouter(t)
	for _, b := range []string{`{"name":"x"}`, `nope`} {
		assert.Equal(t, http.StatusBadRequest, do(h, "PATCH", "/api/character/details", b).Code, b)
	}
}
