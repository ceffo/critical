package render_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// A login that this server does not offer is a missing route: 404 not_found, never a 500.
func TestError_LoginDisabledIs404(t *testing.T) {
	for name, err := range map[string]error{
		"dev login":    authsvc.ErrDevLoginDisabled,
		"google login": authsvc.ErrGoogleLoginDisabled,
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			render.Error(t.Context(), rec, err)
			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Contains(t, rec.Body.String(), `"code":"not_found"`)
		})
	}
}
