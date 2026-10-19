package ruletablesvc

import (
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/localized"
)

// The language catalogue of catalogue format 8 (ADR 0021 step 14, ADR 0032
// step 4): the languages of the PHB racial lists plus Common (and any
// campaign language the data adds), and each race's list with its own
// language. Codes and language-map names (ADR 0023); a character's languages
// are codes of it.

// CommonLanguage is the code of Common, the native language of the races with
// none of their own (ADR 0032 step 4.1).
const CommonLanguage = "common"

// Language is one catalogue language: a code and its name by language.
type Language struct {
	Code string
	Name localized.Text
}

// RaceLanguageRow is one language of a race's PHB list; Native marks the
// race's own language (exactly one row per race).
type RaceLanguageRow struct {
	Race     string
	Language string
	Native   bool
}

// Languages is the validated language catalogue. The zero value and nil are
// the empty catalogue of a format before 8: no language is known and no race
// has a native one.
type Languages struct {
	list   []Language
	known  map[string]bool
	native map[string]string
	lists  map[string][]string
}

// raceCodes are the six PHB races (ADR 0030 step 1), the races a
// race_languages table covers.
var raceCodes = []string{"human", "dwarf", "elf", "gnome", "half_elf", "halfling"}

// NewLanguages validates what the schema cannot express (ADR 0021 step 14.3):
// a language code once and with an English name, a race's list naming only
// catalogue languages once each, every race with exactly one native language
// that belongs to its own list. A violation names the language or the race.
func NewLanguages(list []Language, rows []RaceLanguageRow) (*Languages, error) {
	out := &Languages{
		list: slices.Clone(list), known: make(map[string]bool, len(list)),
		native: map[string]string{}, lists: map[string][]string{},
	}
	for _, l := range list {
		switch {
		case out.known[l.Code]:
			return nil, fmt.Errorf("languages: code %q twice", l.Code)
		case l.Name.Default() == "":
			return nil, fmt.Errorf("languages: %q has no English name", l.Code)
		}
		out.known[l.Code] = true
	}
	seen := map[string]bool{}
	for _, r := range rows {
		switch {
		case !slices.Contains(raceCodes, r.Race):
			return nil, fmt.Errorf("race_languages: unknown race %q", r.Race)
		case !out.known[r.Language]:
			return nil, fmt.Errorf("race_languages: %s names the unknown language %q", r.Race, r.Language)
		case seen[r.Race+" "+r.Language]:
			return nil, fmt.Errorf("race_languages: %s lists %q twice", r.Race, r.Language)
		}
		seen[r.Race+" "+r.Language] = true
		out.lists[r.Race] = append(out.lists[r.Race], r.Language)
		if r.Native {
			if _, dup := out.native[r.Race]; dup {
				return nil, fmt.Errorf("race_languages: %s has two native languages", r.Race)
			}
			out.native[r.Race] = r.Language
		}
	}
	for _, race := range raceCodes {
		if _, ok := out.native[race]; !ok {
			return nil, fmt.Errorf("race_languages: %s has no native language", race)
		}
	}
	return out, nil
}

// All are the catalogue's languages, in the artifact's order.
func (l *Languages) All() []Language {
	if l == nil {
		return nil
	}
	return slices.Clone(l.list)
}

// Has reports whether code is a catalogue language.
func (l *Languages) Has(code string) bool { return l != nil && l.known[code] }

// Native is the race's own language (ADR 0032 step 4.1), "" without a
// catalogue.
func (l *Languages) Native(race string) string {
	if l == nil {
		return ""
	}
	return l.native[race]
}

// RaceList is the race's PHB list, native language included, in the artifact's
// order; empty without a catalogue.
func (l *Languages) RaceList(race string) []string {
	if l == nil {
		return []string{}
	}
	return append([]string{}, l.lists[race]...)
}

// languagesByIntelligence is PHB Table 4's "# of Lang." column, from score 1
// up (ADR 0032 step 4.2).
var languagesByIntelligence = []int{0, 1, 1, 1, 1, 1, 1, 1, 2, 2, 2, 3, 3, 4, 4, 5, 6, 7, 8, 9, 10, 11, 12, 15, 20}

// LanguageLimit is the number of additional languages a character of
// Intelligence score may know (PHB Table 4, ADR 0032 step 4.2), the native
// language not counted. A score under 1 is read as 1 and one over 25 as 25.
func LanguageLimit(score int) int {
	return languagesByIntelligence[min(max(score, 1), len(languagesByIntelligence))-1]
}

// WithLanguages returns a copy of t carrying the format-8 language catalogue.
func (t *Tables) WithLanguages(l *Languages) *Tables {
	out := *t
	out.languages = l
	return &out
}

// Languages returns the language catalogue, false before format 8.
func (t *Tables) Languages() (*Languages, bool) { return t.languages, t.languages != nil }
