package router_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Every text of a spell payload is a language map (ADR 0023 step 6) holding
// exactly the languages the service returned: French and English for a spell
// that has both, English only for one without a French text (no "fr" key: the
// client falls back to English), and `{}` for a text in no language.
func TestSpells_TextsAreLanguageMaps(t *testing.T) {
	missile := spellsvc.Spell{
		ID: "magic_missile", Class: "wizard", Level: 1, Source: "PHB",
		Name:        localized.Text{"en": "Magic Missile", "fr": "Projectile magique"},
		Description: localized.Text{"en": "Fixture text for Magic Missile", "fr": "Texte de test pour Projectile magique"},
		Range:       localized.Of("60 yds. + 10 yds./level"),
	}
	armor := spellsvc.Spell{
		ID: "armor", Class: "wizard", Level: 1, Source: "PHB",
		Name:        localized.Of("Armor"),
		Description: localized.Of("Fixture text for Armor"),
	}
	h, m := newRouter(t)
	m.spell.EXPECT().Search(mock.Anything, spellsvc.Filter{Query: "armor"}, spellsvc.PageRequest{Page: 1, PageSize: spellsvc.DefaultPageSize}).
		Return(spellsvc.Page{Items: []spellsvc.Spell{missile, armor}, Total: 2, Page: 1, PageSize: spellsvc.DefaultPageSize}, nil)
	m.char.EXPECT().GetCharacter(mock.Anything, int64(1)).Return(nil, nil)
	m.cast.EXPECT().AddsEffect(mock.Anything, mock.Anything).Return(false)

	rec := do(h, "GET", "/api/spells?q=armor", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var page struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Items, 2)

	mm := page.Items[0]
	assert.Equal(t, map[string]any{"en": "Magic Missile", "fr": "Projectile magique"}, mm["name"])
	desc, ok := mm["description"].(map[string]any)
	require.True(t, ok, "description is a language map")
	assert.Contains(t, desc["en"], "Fixture text for Magic Missile")
	assert.Contains(t, desc["fr"], "Texte de test pour Projectile magique")
	assert.Equal(t, map[string]any{"en": "60 yds. + 10 yds./level"}, mm["range"])
	assert.Equal(t, "PHB", mm["source"])

	ar := page.Items[1]
	assert.Equal(t, map[string]any{"en": "Armor"}, ar["name"], "no French name: no fr key")
	adesc, ok := ar["description"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, adesc["en"])
	assert.NotContains(t, adesc, "fr")
	assert.Equal(t, map[string]any{}, ar["sphere"], "no sphere: an empty map")
}

// GET /api/spells/names serves each spell's id and name map, nothing else.
func TestSpellNames_AreIDAndLanguageMap(t *testing.T) {
	h, m := newRouter(t)
	m.spell.EXPECT().Names(mock.Anything).Return([]spellsvc.SpellName{
		{ID: "magic_missile", Name: localized.Text{"en": "Magic Missile", "fr": "Projectile magique"}},
		{ID: "armor", Name: localized.Of("Armor")},
	}, nil)

	rec := do(h, "GET", "/api/spells/names", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var names []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &names))
	byID := map[string]any{}
	for _, n := range names {
		assert.Len(t, n, 2, "id and name only")
		byID[n["id"].(string)] = n["name"]
	}
	assert.Equal(t, map[string]any{"en": "Magic Missile", "fr": "Projectile magique"}, byID["magic_missile"])
	assert.Equal(t, map[string]any{"en": "Armor"}, byID["armor"])
}
