package wire

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
)

// DetailsDTO is the wire shape of the details, also embedded in
// GET /api/character. age, height_cm and weight_kg are null when not set (ADR
// 0032 step 5); alignment and sex are codes (steps 2 and 3), "" when missing.
// height and weight are the previous release's free-text keys, kept for one
// release so that a client of that release still works (ADR 0032 step 8):
// read as "180 cm" / "75.5 kg" ("" when not set), written as a text the
// migration's parser reads (an empty one clears). height_cm and weight_kg
// win when both are sent.
type DetailsDTO struct {
	Alignment string  `json:"alignment"`
	Age       *int    `json:"age"`
	Sex       string  `json:"sex"`
	Height    string  `json:"height"`
	Weight    string  `json:"weight"`
	HeightCm  *Height `json:"height_cm"`
	WeightKg  *Weight `json:"weight_kg"`
	// HeightNull and WeightNull are set when the body names height_cm / weight_kg as JSON null (a
	// clear), which pointers alone cannot tell from an omitted key. Read by the creation only.
	HeightNull bool     `json:"-"`
	WeightNull bool     `json:"-"`
	Hair       string   `json:"hair"`
	Eyes       string   `json:"eyes"`
	Languages  []string `json:"languages"`
}

// UnmarshalJSON reads the details strictly (unknown keys refused, as the bodies are) and notes a
// height_cm or weight_kg sent as null.
func (d *DetailsDTO) UnmarshalJSON(b []byte) error {
	type plain DetailsDTO
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode((*plain)(d)); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	d.HeightNull = string(raw["height_cm"]) == "null"
	d.WeightNull = string(raw["weight_kg"]) == "null"
	return nil
}

// DetailsToDTO maps the domain details to their wire shape.
func DetailsToDTO(d charactersvc.Details) DetailsDTO {
	d = d.Coded()
	out := DetailsDTO{
		Alignment: d.Alignment, Sex: d.Sex,
		Hair: d.Hair, Eyes: d.Eyes, Languages: append([]string{}, d.Languages...),
	}
	if d.Age > 0 {
		age := d.Age
		out.Age = &age
	}
	if d.Height > 0 {
		h := Height(d.Height)
		out.HeightCm = &h
		out.Height = charactersvc.FormatHeight(d.Height)
	}
	if d.Weight > 0 {
		w := Weight(d.Weight)
		out.WeightKg = &w
		out.Weight = charactersvc.FormatWeight(d.Weight)
	}
	return out
}

// Height is a height in centimetres on the wire: a JSON number, anything else
// (a string, a boolean) being refused as the field error not_a_number (ADR
// 0032 step 5.2).
type Height float64

// Weight is a weight in kilograms on the wire, read like Height.
type Weight float64

// UnmarshalJSON reads a JSON number.
func (h *Height) UnmarshalJSON(b []byte) error {
	v, err := number(charactersvc.FieldHeight, b)
	*h = Height(v)
	return err
}

// UnmarshalJSON reads a JSON number.
func (w *Weight) UnmarshalJSON(b []byte) error {
	v, err := number(charactersvc.FieldWeight, b)
	*w = Weight(v)
	return err
}

func number(field string, b []byte) (float64, error) {
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return 0, &domain.FieldError{Field: field, Message: charactersvc.CodeNotANumber}
	}
	return v, nil
}

// ResolveMeasure is the height or weight a request names, in the number key
// (num, or nullNum to clear it) or in the previous release's text key (txt):
// the number wins; a text is parsed with parse, an empty one clears and one
// that names no figure is refused as not_a_number (ADR 0032 step 8).
func ResolveMeasure(name string, num *float64, nullNum bool, txt *string, parse func(string) (float64, bool)) (v *float64, clear bool, err error) {
	switch {
	case num != nil:
		return num, false, nil
	case nullNum:
		return nil, true, nil
	case txt == nil:
		return nil, false, nil
	case strings.TrimSpace(*txt) == "":
		return nil, true, nil
	}
	f, ok := parse(*txt)
	if !ok {
		return nil, false, &domain.FieldError{Field: name, Message: charactersvc.CodeNotANumber}
	}
	return &f, false, nil
}
