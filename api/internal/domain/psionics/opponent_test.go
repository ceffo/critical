package psionics_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/psionics"
)

// ADR 0044 step 2: the opponent is the request's. Its shape is checked
// against the catalogue, each refusal naming its field; the label is
// trimmed and kept as typed.
func TestOpponent_Step2_TheRequestsOpponentIsValidated(t *testing.T) {
	two := []psionics.ContestSide{{Roll: 12, Success: true}, {Roll: 20, Success: false}}
	ok := []psionics.Opponent{
		{Target: "  Grey Ogre ", Mind: psionics.MindClosed, Level: 7, Defence: "mode", Rolls: two},
		{Target: "Orc", Mind: psionics.MindOpen},
		{Target: strings.Repeat("é", psionics.MaxTargetLen), Mind: psionics.MindClosed},
	}
	for _, o := range ok {
		require.NoError(t, o.Validate(book()), o.Target)
	}
	o := ok[0]
	require.NoError(t, o.Validate(book()))
	assert.Equal(t, "Grey Ogre", o.Target, "the label trimmed, its case kept")

	refused := []struct {
		name  string
		o     psionics.Opponent
		field string
	}{
		{"no label", psionics.Opponent{Target: "   ", Mind: psionics.MindOpen}, "target"},
		{"a label over 60 characters", psionics.Opponent{Target: strings.Repeat("é", psionics.MaxTargetLen+1), Mind: psionics.MindOpen}, "target"},
		{"no mind", psionics.Opponent{Target: "Orc"}, "mind"},
		{"a mind not open or closed", psionics.Opponent{Target: "Orc", Mind: "ajar"}, "mind"},
		{"a negative level", psionics.Opponent{Target: "Orc", Mind: psionics.MindOpen, Level: -1}, "level"},
		{"a defence that is a devotion", psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed, Defence: "d1", Rolls: two}, "defence"},
		{"a defence not in the catalogue", psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed, Defence: "nope", Rolls: two}, "defence"},
		{"a defence with one roll", psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed, Defence: "mode", Rolls: two[:1]}, "rolls"},
		{"a defence with no roll", psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed, Defence: "mode"}, "rolls"},
		{"a roll of 21", psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed, Defence: "mode", Rolls: []psionics.ContestSide{{Roll: 21}, {Roll: 3}}}, "rolls"},
		{"a roll of 0", psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed, Defence: "mode", Rolls: []psionics.ContestSide{{Roll: 3}, {Roll: 0}}}, "rolls"},
		{"rolls with no defence", psionics.Opponent{Target: "Orc", Mind: psionics.MindClosed, Rolls: two}, "rolls"},
	}
	for _, c := range refused {
		t.Run(c.name, func(t *testing.T) {
			err := c.o.Validate(book())
			var fe *domain.FieldError
			require.True(t, errors.As(err, &fe), "a field error, got %v", err)
			assert.Equal(t, c.field, fe.Field)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
}
