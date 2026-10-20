package router_test

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/composition"
	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/profdescsvc/mocks"
	"github.com/ceffo/critical/api/internal/rest/router"
)

// ADR 0037 step 19.4: GET /api/proficiencies/{id}/description answers {id, description: {en, fr}},
// a language map, and a 404 not_found for an id the catalogue does not hold. It needs a session but
// no current character.
func TestProficiencyDescription_Step19_ServedAsALanguageMapFromTheCatalogue(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().Description(mock.Anything, "tumbling").
		Return(localized.Text{"en": "Tumble.", "fr": "Cabriole."}, nil).Once()
	svc.EXPECT().Description(mock.Anything, "swordplay").
		Return(nil, fmt.Errorf("proficiency %q: %w", "swordplay", domain.ErrNotFound)).Once()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := router.New(log, &composition.BackingServices{Auth: signedIn(t), ProficiencyDescriptions: svc}, router.Options{})

	rec := do(h, "GET", "/api/proficiencies/tumbling/description", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"id":"tumbling","description":{"en":"Tumble.","fr":"Cabriole."}}`, rec.Body.String())

	rec = do(h, "GET", "/api/proficiencies/swordplay/description", "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"not_found"`)
}
