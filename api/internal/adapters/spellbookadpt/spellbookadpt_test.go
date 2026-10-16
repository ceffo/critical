package spellbookadpt_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
	"github.com/ceffo/critical/api/internal/domain/catalogue"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

var cat = catalogue.Catalogue{Spells: []catalogue.Spell{
	{ID: "mirror_image", Name: localized.Of("Mirror Image"), Class: "Wizard", Level: 2},
	{ID: "fireball", Name: localized.Of("Fireball"), Class: "Wizard", Level: 3},
	{ID: "cure_light_wounds", Name: localized.Of("Cure Light Wounds"), Class: "Priest", Level: 1},
}}

func TestParse(t *testing.T) {
	tests := []struct {
		name, book, wantErr string
	}{
		{"ok", `[{"id":"mirror_image","name":"Mirror Image","level":2}]`, ""},
		{"id not in the catalogue", `[{"id":"mirror","name":"Mirror Image","level":2}]`, "not a Wizard spell"},
		{"unknown spell", `[{"id":"shield","name":"Shield","level":1}]`, "not a Wizard spell"},
		{"priest spell is not a wizard spell", `[{"id":"cure_light_wounds","name":"Cure Light Wounds","level":1}]`, "not a Wizard spell"},
		{"name differs from the catalogue", `[{"id":"fireball","name":"Fire Ball","level":3}]`, `named "Fireball"`},
		{"wrong level", `[{"id":"fireball","name":"Fireball","level":2}]`, "is level 3"},
		{"duplicate", `[{"id":"fireball","name":"Fireball","level":3},{"id":"fireball","name":"Fireball","level":3}]`, "duplicate"},
		{"unknown field", `[{"id":"fireball","name":"Fireball","level":3,"extra":1}]`, "parse"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := spellbookadpt.Parse([]byte(tc.book), cat)
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNew_MissingFile(t *testing.T) {
	_, err := spellbookadpt.New(t.TempDir(), cat)
	require.Error(t, err)
}
