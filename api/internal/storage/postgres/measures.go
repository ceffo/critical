package postgres

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/text/transform"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// The parsing of the free-text height and weight stored before they were numbers (ADR 0032 step
// 8.1): a number, with or without a recognised unit (cm, m, feet-inches, kg, lb), en and fr,
// stored metric. A bare height under 3 reads as metres and otherwise as centimetres, a bare weight
// as kilograms. Only this migration reads them; the app never shows or accepts the text.

var (
	numberPat = `(\d+(?:\.\d+)?)`
	// 5'11", 5 ft 11 in, 6 feet 2 inches, 5 pieds 11 pouces, 5', 5ft.
	feetInchesRe = regexp.MustCompile(`^` + numberPat + `\s*(?:'|ft|feet|foot|pieds?)\s*(?:` + numberPat + `\s*(?:"|''|in|inch|inches|pouces?)?)?$`)
	// 1m80, 1 m 80, 1m80cm.
	metresCmRe = regexp.MustCompile(`^(\d)\s*m\s*(\d{1,2})\s*(?:cm)?$`)
	heightRe   = regexp.MustCompile(`^` + numberPat + `\s*(cm|centimetres?|centimeters?|m|metres?|meters?|in|inch|inches|pouces?|ft|feet|pieds?)?$`)
	weightRe   = regexp.MustCompile(`^` + numberPat + `\s*(kg|kgs|kilos?|kilograms?|kilogrammes?|lb|lbs|pounds?)?$`)
)

// measureText is s lower-cased, without accents, with typographic quotes plain and a decimal comma a
// dot.
func measureText(s string) string {
	plain, _, err := transform.String(accentStripper, s)
	if err != nil {
		plain = s
	}
	plain = strings.ToLower(strings.TrimSpace(plain))
	plain = strings.NewReplacer("’", "'", "′", "'", "″", `"`, "”", `"`, "“", `"`, "‘", "'").Replace(plain)
	return regexp.MustCompile(`(\d),(\d)`).ReplaceAllString(plain, "$1.$2")
}

func num(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// tenth rounds to the stored step.
func tenth(v float64) float64 { return math.Round(v*10) / 10 }

// parseHeight is the height in centimetres a stored text names, false when it names none (or one
// beyond the sanity bound of a height).
func parseHeight(raw string) (float64, bool) {
	s := measureText(raw)
	cm := 0.0
	switch {
	case feetInchesRe.MatchString(s):
		m := feetInchesRe.FindStringSubmatch(s)
		inches := num(m[1])*12 + num(m[2])
		cm = ruletablesvc.CentimetresPerInch * inches
	case metresCmRe.MatchString(s):
		m := metresCmRe.FindStringSubmatch(s)
		cm = num(m[1])*100 + num(m[2])
	case heightRe.MatchString(s):
		m := heightRe.FindStringSubmatch(s)
		v := num(m[1])
		switch m[2] {
		case "cm", "centimetre", "centimetres", "centimeter", "centimeters":
			cm = v
		case "m", "metre", "metres", "meter", "meters":
			cm = v * 100
		case "in", "inch", "inches", "pouce", "pouces":
			cm = v * ruletablesvc.CentimetresPerInch
		case "ft", "feet", "pied", "pieds":
			cm = v * 12 * ruletablesvc.CentimetresPerInch
		default:
			if v < 3 {
				cm = v * 100
			} else {
				cm = v
			}
		}
	default:
		return 0, false
	}
	cm = tenth(cm)
	return cm, cm > 0 && cm <= charactersvc.MaxHeightCm
}

// parseWeight is the weight in kilograms a stored text names, false when it names none.
func parseWeight(raw string) (float64, bool) {
	m := weightRe.FindStringSubmatch(measureText(raw))
	if m == nil {
		return 0, false
	}
	kg := num(m[1])
	switch m[2] {
	case "lb", "lbs", "pound", "pounds":
		kg *= ruletablesvc.KilogramsPerPound
	}
	kg = tenth(kg)
	return kg, kg > 0 && kg <= charactersvc.MaxWeightKg
}

// mapMeasures is the data step after measuresMigration (ADR 0032 step 8): every character's stored
// free-text height and weight become the centimetres and kilograms they name; an unrecognised text
// leaves the number unset. The old columns are left as they are (the release before may still
// read them), so a second run gives the same numbers.
func mapMeasures(ctx context.Context, q storeq.Querier) error {
	rows, err := q.ListCharacterMeasures(ctx)
	if err != nil {
		return fmt.Errorf("list character measures: %w", err)
	}
	for _, r := range rows {
		h, _ := parseHeight(r.Height)
		w, _ := parseWeight(r.Weight)
		if h == 0 && w == 0 {
			continue
		}
		if err := q.SetCharacterMeasures(ctx, storeq.SetCharacterMeasuresParams{ID: r.ID, HeightCm: h, WeightKg: w}); err != nil {
			return fmt.Errorf("map the measures of character %d: %w", r.ID, err)
		}
	}
	return nil
}
