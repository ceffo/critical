// Package localized holds a text in human languages (ADR 0023): one body per
// language code, never a field per language. Storage keeps one
// localized_texts row per language; the API serves the map as it is and the
// client picks its language, falling back to Fallback per field.
package localized

// Fallback is the language every text falls back to (ADR 0023 step 3): the
// books' language, which every catalogue text has.
const Fallback = "en"

// Text is a text's body by language code. An absent language has no entry,
// never an empty body; a text in no language is empty (nil or no entries).
type Text map[string]string

// Of is a text in the fallback language only; an empty s is an empty text.
func Of(s string) Text {
	if s == "" {
		return nil
	}
	return Text{Fallback: s}
}

// Default is the body in the fallback language ("" when it has none): what
// the rules code and the stat parser read, the books' own words.
func (t Text) Default() string { return t[Fallback] }

// Clean drops the empty bodies, so an absent language has no entry.
func (t Text) Clean() Text {
	var out Text
	for lang, body := range t {
		if body == "" {
			continue
		}
		if out == nil {
			out = Text{}
		}
		out[lang] = body
	}
	return out
}
