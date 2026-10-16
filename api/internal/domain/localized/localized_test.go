package localized

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
