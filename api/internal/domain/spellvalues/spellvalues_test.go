package spellvalues_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ceffo/critical/api/internal/domain/spellvalues"
)

// values are the marked parts of text, as "kind:text".
func values(lang, text string) []string {
	var out []string
	for _, s := range spellvalues.Mark(lang, text) {
		if s.Kind != spellvalues.Plain {
			out = append(out, string(s.Kind)+":"+s.Text)
		}
	}
	return out
}

func TestMark_FindsTheSixKindsInEnglishAndFrench(t *testing.T) {
	tests := []struct {
		name, lang, text string
		want             []string
	}{
		{"dice", "en", "Deals 1d12 damage.", []string{"dice:1d12"}},
		{"dice with a bonus", "en", "inflicts 3d6+2 points", []string{"dice:3d6+2"}},
		{"percentile die", "fr", "lancer d100 et", []string{"dice:d100"}},
		{"time per level", "en", "lasts 2 turns/level", []string{"time:2 turns/level"}},
		{"time", "en", "for 1 round", []string{"time:1 round"}},
		{"time of dice", "en", "for 1d4 rounds.", []string{"time:1d4 rounds"}},
		{"time per level, abbreviated", "en", "1 rd./level", []string{"time:1 rd./level"}},
		{"time per level in French", "fr", "dure 2 rounds/niveau", []string{"time:2 rounds/niveau"}},
		{"time in French", "fr", "pendant 3 tours", []string{"time:3 tours"}},
		{"a month is a time, not a distance", "fr", "pendant 2 mois", []string{"time:2 mois"}},
		{"ordinal level", "en", "a 15th level caster", []string{"level:15th level"}},
		{"hyphenated ordinal level", "en", "a 10th-level caster", []string{"level:10th-level"}},
		{"per level", "en", "increases per level", []string{"level:per level"}},
		{"every levels", "en", "one more every 2 levels", []string{"level:every 2 levels"}},
		{"level in French", "fr", "un lanceur du 12e niveau", []string{"level:12e niveau"}},
		{"level number in French", "fr", "au niveau 12", []string{"level:niveau 12"}},
		{"per level in French", "fr", "un de plus par niveau", []string{"level:par niveau"}},
		{"distance", "en", "within 10 feet of", []string{"distance:10 feet"}},
		{"hyphenated distance", "en", "a 60-foot arc", []string{"distance:60-foot"}},
		{"distance per level", "en", "range 10 yards + 10 yards/level", []string{"distance:10 yards", "distance:10 yards/level"}},
		{"square feet", "en", "covers 10 sq. ft. of floor", []string{"distance:10 sq. ft."}},
		{"metres", "fr", "dans un rayon de 3 m autour", []string{"distance:3 m"}},
		{"feet in French", "fr", "une vitesse de 10 pieds par round", []string{"distance:10 pieds par round"}},
		{"percent", "en", "a 10% chance", []string{"percent:10%"}},
		{"percent in French", "fr", "50 % de chances", []string{"percent:50 %"}},
		{"bonus", "en", "gains a +3 bonus", []string{"modifier:+3"}},
		{"penalty", "en", "with a -2 penalty.", []string{"modifier:-2"}},
		{"penalty in French", "fr", "une pénalité de -2", []string{"modifier:-2"}},
		{"several values keep their order", "en", "1d6 damage, +1 per level, 20% fail", []string{"dice:1d6", "modifier:+1", "level:per level", "percent:20%"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := values(tc.lang, tc.text)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("Mark(%q, %q) values = %q, want %q", tc.lang, tc.text, got, tc.want)
			}
		})
	}
}

func TestMark_LeavesNonValuesAlone(t *testing.T) {
	tests := []struct{ name, lang, text string }{
		{"a year", "en", "In the year 1372 the wizard fell."},
		{"an item count in words", "en", "three arrows and two feet of rope"},
		{"a plus in prose", "en", "Strength + Dexterity, or a + b"},
		{"a range of numbers", "en", "table result 7-9"},
		{"a thousands separator", "en", "worth 1,000 gp"},
		{"a multiplier", "en", "times x10 the amount"},
		{"a spell name with digits", "en", "Otiluke's Resilient Sphere of 7 Winds"},
		{"a saving throw in words", "en", "the target saves vs. spell for half"},
		{"a word starting like a unit", "fr", "3 mètrerie"},
		{"a language with no vocabulary", "de", "3 Runden"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := values(tc.lang, tc.text); len(got) != 0 {
				t.Errorf("Mark(%q, %q) = %q, want no value", tc.lang, tc.text, got)
			}
		})
	}
}

func TestMark_SpansConcatenateToTheText(t *testing.T) {
	text := "Dégâts de 1d6 + 2 pendant 3 tours/niveau à 10 pieds, 50% de chances. Éloigné."
	var b strings.Builder
	for _, s := range spellvalues.Mark("fr", text) {
		b.WriteString(s.Text)
	}
	if b.String() != text {
		t.Errorf("spans joined = %q, want %q", b.String(), text)
	}
}

type catalogueFile struct {
	Spells []struct {
		ID          string            `json:"id"`
		Description map[string]string `json:"description"`
	} `json:"spells"`
}

// lossless checks, for every description of a catalogue, that the marks drop nothing.
func lossless(t *testing.T, path string) (values int) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no catalogue at %s", path)
	}
	var c catalogueFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	for _, s := range c.Spells {
		for lang, text := range s.Description {
			var b strings.Builder
			for _, sp := range spellvalues.Mark(lang, text) {
				if sp.Text == "" {
					t.Errorf("%s/%s: an empty span", s.ID, lang)
				}
				if sp.Kind != spellvalues.Plain {
					values++
				}
				b.WriteString(sp.Text)
			}
			if b.String() != text {
				t.Errorf("%s/%s: the marked text differs from the original", s.ID, lang)
			}
		}
	}
	return values
}

func TestMark_TheFixtureCatalogueLosesNothing(t *testing.T) {
	lossless(t, filepath.Join("..", "..", "..", "testdata", "catalogue.json"))
}

// The real artifact is private: this runs where it is fetched (just catalogue) and skips elsewhere.
func TestMark_TheRealCatalogueLosesNothingAndHasValues(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", ".catalogue", "catalogue.json")
	if n := lossless(t, root); n < 1000 {
		t.Errorf("only %d values found in the real catalogue", n)
	}
}
