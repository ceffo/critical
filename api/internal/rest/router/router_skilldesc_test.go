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
	"github.com/ceffo/critical/api/internal/domain/service/skilldescsvc/mocks"
	"github.com/ceffo/critical/api/internal/rest/router"
)

// ADR 0037 step 19.6: GET /api/skill-sets/{set}/skills/{skill}/description answers {set, skill,
// description: {en, fr}}, a language map, the text of that set for that skill, and a 404
// not_found for a pair the catalogue does not hold. It needs a session but no current character.
func TestSkillDescription_Step19_6_ServedAsALanguageMapPerSet(t *testing.T) {
	svc := mocks.NewMockSVC(t)
	svc.EXPECT().Description(mock.Anything, "thief", "pick_pockets").
		Return(localized.Text{"en": "Thief's.", "fr": "Du voleur."}, nil).Once()
	svc.EXPECT().Description(mock.Anything, "bard", "pick_pockets").
		Return(localized.Text{"en": "Bard's.", "fr": "Du barde."}, nil).Once()
	svc.EXPECT().Description(mock.Anything, "thief", "swordplay").
		Return(nil, fmt.Errorf("skill %q: %w", "swordplay", domain.ErrNotFound)).Once()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := router.New(log, &composition.BackingServices{Auth: signedIn(t), SkillDescriptions: svc}, router.Options{})

	rec := do(h, "GET", "/api/skill-sets/thief/skills/pick_pockets/description", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"set":"thief","skill":"pick_pockets","description":{"en":"Thief's.","fr":"Du voleur."}}`, rec.Body.String())

	rec = do(h, "GET", "/api/skill-sets/bard/skills/pick_pockets/description", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"set":"bard","skill":"pick_pockets","description":{"en":"Bard's.","fr":"Du barde."}}`, rec.Body.String())

	rec = do(h, "GET", "/api/skill-sets/thief/skills/swordplay/description", "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"not_found"`)
}
