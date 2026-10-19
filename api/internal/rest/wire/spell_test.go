package wire_test

import (
	"encoding/json"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/rest/wire"
)

func TestSpellToDTO_MarksTheValuesOfTheDescriptionAndTheStatsPerLanguage(t *testing.T) {
	dto := wire.SpellToDTO(spellsvc.Spell{
		ID:          "x",
		Description: localized.Text{"en": "Deals 1d6 damage.", "fr": "Inflige 1d6 dégâts."},
		Duration:    localized.Text{"en": "2 rounds/level"},
		Range:       localized.Text{"en": "Touch"},
	}, 0)

	raw, err := json.Marshal(dto.Marked)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"description":{"en":[{"text":"Deals "},{"text":"1d6","kind":"dice"},{"text":" damage."}],` +
		`"fr":[{"text":"Inflige "},{"text":"1d6","kind":"dice"},{"text":" dégâts."}]},` +
		`"duration":{"en":[{"text":"2 rounds/level","kind":"time"}]}}`
	if string(raw) != want {
		t.Errorf("marked = %s\nwant     %s", raw, want)
	}
}

func TestSpellToDTO_ASpellWithoutValuesServesAnEmptyMarkedObject(t *testing.T) {
	dto := wire.SpellToDTO(spellsvc.Spell{ID: "x", Description: localized.Text{"en": "Nothing here."}}, 0)
	raw, _ := json.Marshal(dto.Marked)
	if string(raw) != "{}" {
		t.Errorf("marked = %s, want {}", raw)
	}
}
