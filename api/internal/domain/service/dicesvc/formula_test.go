package dicesvc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_Valid(t *testing.T) {
	cases := []struct {
		in       string
		normal   string
		groups   []dicesvc.Group
		modifier int
	}{
		{"d20", "d20", []dicesvc.Group{{Sides: 20, Count: 1}}, 0},
		{"1d20", "d20", []dicesvc.Group{{Sides: 20, Count: 1}}, 0},
		{"2d6+3", "2d6+3", []dicesvc.Group{{Sides: 6, Count: 2}}, 3},
		{"2D6 + 3", "2d6+3", []dicesvc.Group{{Sides: 6, Count: 2}}, 3},
		{"d%", "d%", []dicesvc.Group{{Sides: 100, Count: 1}}, 0},
		{"d100", "d%", []dicesvc.Group{{Sides: 100, Count: 1}}, 0},
		{"2d%", "2d%", []dicesvc.Group{{Sides: 100, Count: 2}}, 0},
		{"d10+d%", "d10+d%", []dicesvc.Group{{Sides: 10, Count: 1}, {Sides: 100, Count: 1}}, 0},
		{"d6+d6+d6", "3d6", []dicesvc.Group{{Sides: 6, Count: 3}}, 0},
		{"d6+d4+d6", "2d6+d4", []dicesvc.Group{{Sides: 6, Count: 2}, {Sides: 4, Count: 1}}, 0},
		{"+d4", "d4", []dicesvc.Group{{Sides: 4, Count: 1}}, 0},
		{"d8-1", "d8-1", []dicesvc.Group{{Sides: 8, Count: 1}}, -1},
		{"3+d8+2-1", "d8+4", []dicesvc.Group{{Sides: 8, Count: 1}}, 4},
		{"d8-5", "d8-5", []dicesvc.Group{{Sides: 8, Count: 1}}, -5},
		{"d8+3-3", "d8", []dicesvc.Group{{Sides: 8, Count: 1}}, 0},
		{"2d6-d4", "2d6-d4", []dicesvc.Group{{Sides: 6, Count: 2}, {Sides: 4, Count: 1, Negative: true}}, 0},
		{"d6-d6", "d6-d6", []dicesvc.Group{{Sides: 6, Count: 1}, {Sides: 6, Count: 1, Negative: true}}, 0},
		{"0+d2", "d2", []dicesvc.Group{{Sides: 2, Count: 1}}, 0},
		{"100d6", "100d6", []dicesvc.Group{{Sides: 6, Count: 100}}, 0},
		{"d1000", "d1000", []dicesvc.Group{{Sides: 1000, Count: 1}}, 0},
		{"d6+10000", "d6+10000", []dicesvc.Group{{Sides: 6, Count: 1}}, 10000},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			f, err := dicesvc.Parse(c.in)
			require.NoError(t, err)
			assert.Equal(t, c.groups, f.Groups)
			assert.Equal(t, c.modifier, f.Modifier)
			assert.Equal(t, c.normal, f.String())
			again, err := dicesvc.Parse(f.String())
			require.NoError(t, err, "the normal form parses")
			assert.Equal(t, f, again, "the normal form is stable")
		})
	}
}

func TestParse_Invalid(t *testing.T) {
	cases := []struct{ name, in string }{
		{"empty", ""},
		{"blank", "   "},
		{"constant only", "5"},
		{"constant sum only", "3+4"},
		{"no die after sign", "d6+"},
		{"lone sign", "-"},
		{"double sign", "d6++1"},
		{"missing sides", "2d"},
		{"just d", "d"},
		{"zero dice", "0d6"},
		{"zero sides", "d0"},
		{"one side", "d1"},
		{"too many sides", "d1001"},
		{"too many dice in a term", "101d6"},
		{"too many dice in total", "60d6+60d4"},
		{"too many dice merged", "d6+" + strings.Repeat("d6+", 100) + "d6"},
		{"constant too large", "d6+10001"},
		{"constants add up too large", "d6+6000+6000"},
		{"negative constants add up too large", "d6-6000-6000"},
		{"formula too long", strings.Repeat("d6+", 100)},
		{"not a number", "d6+x"},
		{"unknown word", "2d6 plus 3"},
		{"two d in a term", "2d6d4"},
		{"d% with count text", "xd%"},
		{"percent in the middle", "d%6"},
		{"decimal", "d6+1.5"},
		{"multiplication", "2d6*2"},
		{"parentheses", "(d6)"},
		{"keep highest", "2d20kh1"},
		{"huge number", "d99999999999999"},
		{"negative sides", "d-6"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := dicesvc.Parse(c.in)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			var fe *domain.FieldError
			require.True(t, errors.As(err, &fe))
			assert.Equal(t, "formula", fe.Field)
			assert.NotEmpty(t, fe.Message)
		})
	}
}
