package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// The mapping of the free-text languages stored before they were catalogue codes (ADR 0032 step
// 8.1). The spellings are data, en and fr, matched ignoring case, accents and punctuation (see
// normalizeFree), and cover the languages of the PHB racial lists plus Common: the codes of the
// catalogue artifact (ADR 0021 step 14; a test holds the two together). Only this migration reads
// them; the app never shows or accepts them.
var languageSpellings = map[string]string{
	"common": "common", "commun": "common", "commune": "common", "langue commune": "common",
	"dwarvish": "dwarvish", "dwarven": "dwarvish", "dwarf": "dwarvish", "dwarfish": "dwarvish",
	"nain": "dwarvish", "nanique": "dwarvish",
	"elvish": "elvish", "elven": "elvish", "elf": "elvish", "elfique": "elvish", "elfe": "elvish",
	"gnomish": "gnomish", "gnome": "gnomish", "gnomique": "gnomish",
	"halfling": "halfling", "halflings": "halfling", "halfelin": "halfling", "halfelins": "halfling",
	"goblin": "goblin", "gobelin": "goblin",
	"hobgoblin": "hobgoblin", "hobgobelin": "hobgoblin",
	"kobold": "kobold", "kobolde": "kobold",
	"orcish": "orcish", "orc": "orcish", "orcs": "orcish", "orque": "orcish", "orques": "orcish",
	"gnoll":                       "gnoll",
	"burrowers":                   "burrowers",
	"burrowing mammals":           "burrowers",
	"speech of burrowing mammals": "burrowers",
	"fouisseurs":                  "burrowers",
	"parler des fouisseurs":       "burrowers",
	"mammiferes fouisseurs":       "burrowers",
}

// languageSeparators split a stored list: a comma, a semicolon, or the word "and" / "et" between
// spaces.
var languageSeparators = regexp.MustCompile(`(?i)[,;]|\s(?:and|et)\s`)

// splitLanguageItems are the items of a stored language text: split at commas, semicolons and the
// words "and" / "et" (ADR 0032 step 8.1).
func splitLanguageItems(raw string) []string {
	var out []string
	for _, item := range languageSeparators.Split(raw, -1) {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// mapLanguage is the code a language text names, "" when it is not recognised.
func mapLanguage(raw string) string { return languageSpellings[normalizeFree(raw)] }

// mapLanguageList maps the stored list: each item, and each part of an item naming several, to its
// code, each code once in the order met. The unrecognised items are returned apart.
func mapLanguageList(stored []string) (codes, dropped []string) {
	codes = []string{}
	for _, text := range stored {
		for _, item := range splitLanguageItems(text) {
			code := mapLanguage(item)
			switch {
			case code == "":
				dropped = append(dropped, item)
			case !slices.Contains(codes, code):
				codes = append(codes, code)
			}
		}
	}
	return codes, dropped
}

// mapLanguages is the data step after languagesMigration (ADR 0032 step 8): every character's
// stored languages become the codes their words name; an unrecognised word is dropped and logged
// (the character and the word), the recognised ones of the same list staying. A code maps to
// itself, so a second run changes nothing, and a character whose list holds nothing to change is
// left as it is.
func mapLanguages(ctx context.Context, q storeq.Querier) error {
	rows, err := q.ListCharacterLanguages(ctx)
	if err != nil {
		return fmt.Errorf("list character languages: %w", err)
	}
	for _, r := range rows {
		var stored []string
		if err := json.Unmarshal([]byte(r.Languages), &stored); err != nil {
			return fmt.Errorf("decode the languages of character %d: %w", r.ID, err)
		}
		codes, dropped := mapLanguageList(stored)
		for _, word := range dropped {
			slog.Warn("migration 014: dropped an unrecognised language", "character", r.ID, "language", word)
		}
		if slices.Equal(codes, stored) {
			continue
		}
		raw, err := json.Marshal(codes)
		if err != nil {
			return fmt.Errorf("encode the languages of character %d: %w", r.ID, err)
		}
		if err := q.SetCharacterLanguages(ctx, storeq.SetCharacterLanguagesParams{ID: r.ID, Languages: string(raw)}); err != nil {
			return fmt.Errorf("map the languages of character %d: %w", r.ID, err)
		}
	}
	return nil
}
