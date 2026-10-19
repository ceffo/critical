package postgres

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// The mapping of the free-text alignment and sex stored before they were codes (ADR 0032 step 8.1).
// The spellings are data, en and fr, matched ignoring case, accents and punctuation (see
// normalizeFree). Only this migration reads them; the app never shows or accepts them.

// alignmentSpellings maps a normalised spelling to an alignment code: the nine names in en and fr,
// the usual abbreviations, and "neutral" / "neutre" alone as true neutral.
var alignmentSpellings = map[string]string{
	"lawful good": "lawful_good", "neutral good": "neutral_good", "chaotic good": "chaotic_good",
	"lawful neutral": "lawful_neutral", "true neutral": "true_neutral", "neutral": "true_neutral",
	"chaotic neutral": "chaotic_neutral", "lawful evil": "lawful_evil", "neutral evil": "neutral_evil",
	"chaotic evil": "chaotic_evil",
	"lg":           "lawful_good", "ng": "neutral_good", "cg": "chaotic_good",
	"ln": "lawful_neutral", "tn": "true_neutral", "n": "true_neutral", "cn": "chaotic_neutral",
	"le": "lawful_evil", "ne": "neutral_evil", "ce": "chaotic_evil",

	"loyal bon": "lawful_good", "neutre bon": "neutral_good", "chaotique bon": "chaotic_good",
	"loyal neutre": "lawful_neutral", "neutre strict": "true_neutral", "neutre absolu": "true_neutral",
	"vrai neutre": "true_neutral", "neutre": "true_neutral", "chaotique neutre": "chaotic_neutral",
	"loyal mauvais": "lawful_evil", "neutre mauvais": "neutral_evil", "chaotique mauvais": "chaotic_evil",
	"loyal mal": "lawful_evil", "neutre mal": "neutral_evil", "chaotique mal": "chaotic_evil",
	"lb": "lawful_good", "nb": "neutral_good", "cb": "chaotic_good",
	"lm": "lawful_evil", "nm": "neutral_evil", "cm": "chaotic_evil",
}

// sexSpellings maps a normalised spelling to a sex code: male and female in en and fr, and the
// initials "m" / "f" / "h".
var sexSpellings = map[string]string{
	"male": "male", "man": "male", "masculin": "male", "homme": "male", "garcon": "male",
	"m": "male", "h": "male",
	"female": "female", "woman": "female", "feminin": "female", "femme": "female", "fille": "female",
	"f": "female",
}

var accentStripper = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// normalizeFree is s lower-cased, without accents, every run of non-letters and non-digits (a
// space, a dash, an underscore, a dot) read as one space, trimmed.
func normalizeFree(s string) string {
	plain, _, err := transform.String(accentStripper, s)
	if err != nil {
		plain = s
	}
	return strings.Join(strings.FieldsFunc(strings.ToLower(plain), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}), " ")
}

// mapSpelling is the code a free text names in table, "" when it is not recognised. A run of
// separated initials ("L.G.") is also read joined.
func mapSpelling(table map[string]string, raw string) string {
	key := normalizeFree(raw)
	if code, ok := table[key]; ok {
		return code
	}
	return table[strings.ReplaceAll(key, " ", "")]
}

// mapAlignment is the alignment code of a stored text, "" when unrecognised (cleared).
func mapAlignment(raw string) string { return mapSpelling(alignmentSpellings, raw) }

// mapSex is the sex code of a stored text, "" when unrecognised (cleared).
func mapSex(raw string) string { return mapSpelling(sexSpellings, raw) }

// mapIdentity is the data step after identityMigration (ADR 0032 step 8): every character's
// alignment and sex become the code their free text names, or are cleared when it names none, and
// the free-text race detail is emptied. A code maps to itself, so a second run changes nothing.
func mapIdentity(ctx context.Context, q storeq.Querier) error {
	rows, err := q.ListCharacterIdentity(ctx)
	if err != nil {
		return fmt.Errorf("list character identities: %w", err)
	}
	for _, r := range rows {
		if err := q.SetCharacterIdentity(ctx, storeq.SetCharacterIdentityParams{
			ID: r.ID, Alignment: mapAlignment(r.Alignment), Sex: mapSex(r.Sex),
		}); err != nil {
			return fmt.Errorf("map the identity of character %d: %w", r.ID, err)
		}
	}
	return nil
}

// identitySeedCharacter gives the seed character stored before alignment and sex were required
// the ones its seed names: the data step after identityMigration, run after mapIdentity. A value
// the character already holds is kept; no seed character or a seed without them changes nothing.
func identitySeedCharacter(ctx context.Context, q storeq.Querier, characterPath string) error {
	c, found, err := seedfile.ReadCharacter(characterPath)
	if err != nil || !found || (c.Alignment == "" && c.Sex == "") {
		return err
	}
	owner, ok, err := seedOwner(ctx, q, characterPath)
	if err != nil || !ok {
		return err
	}
	if err := q.SetIdentityIfUnset(ctx, storeq.SetIdentityIfUnsetParams{ID: owner, Alignment: c.Alignment, Sex: c.Sex}); err != nil {
		return fmt.Errorf("set the identity of character %d: %w", owner, err)
	}
	return nil
}
