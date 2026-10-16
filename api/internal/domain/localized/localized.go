// Package localized holds a text in human languages (ADR 0023): one body per
// language code, never a field per language. Storage keeps one
// localized_texts row per language; the API serves the map as it is and the
// client picks its language, falling back to Fallback per field.
package localized

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Fallback is the language every text falls back to (ADR 0023 step 3): the
// books' language, which every catalogue text has.
const Fallback = "en"

// UILanguages are the language codes the player can read and write texts in:
// the web client's locales (web/src/i18n/locales), in code order. A text the
// player sends is refused in any other code (ADR 0023 step 11).
var UILanguages = []string{"en", "fr"}

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

// Languages are the text's language codes in code order.
func (t Text) Languages() []string {
	out := make([]string, 0, len(t))
	for lang, body := range t {
		if body != "" {
			out = append(out, lang)
		}
	}
	slices.Sort(out)
	return out
}

// Any is the body to record when a text must be one string (a log payload, an
// error message, an id slug): the fallback body, else the body of the first
// language in code order (a name the player typed in another language), ""
// for a text in none (ADR 0023 step 12).
func (t Text) Any() string {
	if b := t[Fallback]; b != "" {
		return b
	}
	if langs := t.Languages(); len(langs) > 0 {
		return t[langs[0]]
	}
	return ""
}

// UnknownLanguage is the first code in t, in code order, that is not one of
// UILanguages, blank bodies included; ok is false when every code is known.
func (t Text) UnknownLanguage() (lang string, ok bool) {
	codes := slices.Sorted(maps.Keys(t))
	for _, c := range codes {
		if !slices.Contains(UILanguages, c) {
			return c, true
		}
	}
	return "", false
}

// Trimmed is the text with every body trimmed and the empty ones dropped.
func (t Text) Trimmed() Text {
	out := make(Text, len(t))
	for lang, body := range t {
		out[lang] = strings.TrimSpace(body)
	}
	return out.Clean()
}

// With is t edited by patch (ADR 0023 step 11): each language patch sends
// takes its trimmed body, a blank body removes the language, and every other
// language keeps its body.
func (t Text) With(patch Text) Text {
	out := make(Text, len(t)+len(patch))
	for lang, body := range t {
		out[lang] = body
	}
	for lang, body := range patch {
		out[lang] = strings.TrimSpace(body)
	}
	return out.Clean()
}

// Equal reports whether both texts have the same body in every language.
func (t Text) Equal(o Text) bool {
	return maps.Equal(t.Clean(), o.Clean())
}

// Snap is a text inside a stored snapshot (an action-log state): the
// language map, or the plain string of a snapshot written before the text
// was localized (Plain), read as its fallback-language body. Restoring a
// Plain one replaces that body only (ADR 0023 step 11).
type Snap struct {
	Text  Text
	Plain bool
}

// MarshalJSON writes the language map.
func (n Snap) MarshalJSON() ([]byte, error) { return json.Marshal(n.Text) }

// UnmarshalJSON reads a language map or a plain string.
func (n *Snap) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*n = Snap{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*n = Snap{Text: Of(s), Plain: true}
		return nil
	}
	var t Text
	if err := json.Unmarshal(b, &t); err != nil {
		return fmt.Errorf("decode localized text: %w", err)
	}
	*n = Snap{Text: t}
	return nil
}

// Restored is the text a snapshot puts back over cur: the whole snapshot
// text, or for a Plain one cur with its fallback body replaced.
func (n Snap) Restored(cur Text) Text {
	if n.Plain {
		return cur.With(n.Text)
	}
	return n.Text
}
