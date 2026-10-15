package spellbookadpt_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ceffo/critical/api/internal/adapters/spellbookadpt"
)

const catalog = `[{"name":"Mirror Image","class":"Wizard","level":2},{"name":"Fireball","class":"Wizard","level":3},{"name":"Cure Light Wounds","class":"Priest","level":1}]`

func TestParse(t *testing.T) {
	tests := []struct {
		name, book, wantErr string
	}{
		{"ok", `[{"id":"mirror_image","name":"Mirror Image","level":2,"sheet":"Images miroir"}]`, ""},
		{"id is not the slug", `[{"id":"mirror","name":"Mirror Image","level":2,"sheet":"x"}]`, "not the slug"},
		{"unknown spell", `[{"id":"shield","name":"Shield","level":1,"sheet":"x"}]`, "not a Wizard spell"},
		{"priest spell is not a wizard spell", `[{"id":"cure_light_wounds","name":"Cure Light Wounds","level":1,"sheet":"x"}]`, "not a Wizard spell"},
		{"wrong level", `[{"id":"fireball","name":"Fireball","level":2,"sheet":"x"}]`, "is level 3"},
		{"duplicate", `[{"id":"fireball","name":"Fireball","level":3,"sheet":"x"},{"id":"fireball","name":"Fireball","level":3,"sheet":"x"}]`, "duplicate"},
		{"unknown field", `[{"id":"fireball","name":"Fireball","level":3,"extra":1}]`, "parse"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := spellbookadpt.Parse([]byte(tc.book), []byte(catalog))
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNew_MissingFile(t *testing.T) {
	_, err := spellbookadpt.New(t.TempDir())
	require.Error(t, err)
}
