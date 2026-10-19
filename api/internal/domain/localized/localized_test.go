package localized

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOf_EmptyIsNoText(t *testing.T) {
	assert.Empty(t, Of(""))
	assert.Equal(t, Text{Fallback: "Fireball"}, Of("Fireball"))
}

func TestDefault_IsTheFallbackBody(t *testing.T) {
	assert.Equal(t, "Fireball", Text{Fallback: "Fireball", "fr": "Boule de feu"}.Default())
	assert.Empty(t, Text{"fr": "Boule de feu"}.Default(), "no fallback body: empty, never another language")
	assert.Empty(t, Text(nil).Default())
}

func TestClean_DropsEmptyBodies(t *testing.T) {
	assert.Equal(t, Text{Fallback: "Armor"}, Text{Fallback: "Armor", "fr": ""}.Clean())
	assert.Nil(t, Text{"fr": ""}.Clean())
}

// ADR 0023 step 12: the one string recorded for a text is its English body,
// else its first language's in code order.
func TestAny_FallbackElseFirstLanguage(t *testing.T) {
	assert.Equal(t, "Rope", Text{"en": "Rope", "fr": "Corde"}.Any())
	assert.Equal(t, "Seil", Text{"fr": "Corde", "de": "Seil"}.Any())
	assert.Empty(t, Text(nil).Any())
}

// ADR 0023 step 11: an edit replaces the languages it sends, a blank body
// removes its language, the others are kept; bodies are trimmed.
func TestWith_EditsOnlyTheLanguagesSent(t *testing.T) {
	cur := Text{"en": "Rope", "fr": "Corde"}
	assert.Equal(t, Text{"en": "Rope", "fr": "Cordelette"}, cur.With(Text{"fr": " Cordelette "}))
	assert.Equal(t, Text{"fr": "Corde"}, cur.With(Text{"en": "  "}))
	assert.Equal(t, Text{"en": "Rope", "fr": "Corde", "de": "Seil"}, cur.With(Text{"de": "Seil"}))
	assert.Equal(t, Text{"en": "Rope", "fr": "Corde"}, cur, "the text edited is untouched")
	assert.Nil(t, cur.With(Text{"en": "", "fr": ""}))
}

// UILanguages are the web client's locales, so a name the player types in
// the UI language is accepted and no other code is (ADR 0023 step 11).
func TestUILanguages_AreTheWebLocales(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "..", "..", "web", "src", "i18n", "locales"))
	require.NoError(t, err)
	var locales []string
	for _, e := range entries {
		if e.IsDir() {
			locales = append(locales, e.Name())
		}
	}
	assert.Equal(t, locales, UILanguages)
	_, unknown := Text{"fr": "Corde", "en": ""}.UnknownLanguage()
	assert.False(t, unknown)
	lang, unknown := Text{"fr": "Corde", "de": "Seil", "es": "Cuerda"}.UnknownLanguage()
	assert.True(t, unknown)
	assert.Equal(t, "de", lang, "the first unknown code in code order")
}
