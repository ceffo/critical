// Package detailshdl exposes PATCH /api/character/details, the character's
// editable details.
package detailshdl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/detailssvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Patch(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type updater interface {
	Update(ctx context.Context, characterID int64, p detailssvc.Patch) (charactersvc.Details, error)
}

// RegisterRoutes wires the details endpoint onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc detailssvc.SVC) {
	r.Patch("/api/character/details", render.Handle(Update(charID, svc)))
}

// DetailsDTO is the wire shape of the details, also embedded in
// GET /api/character. age, height_cm and weight_kg are null when not set (ADR
// 0032 step 5); alignment and sex are codes (steps 2 and 3), "" when missing.
// height and weight are the previous release's free-text keys, kept for one
// release so that a client of that release still works (ADR 0032 step 8):
// read as "180 cm" / "75.5 kg" ("" when not set), written as a text the
// migration's parser reads (an empty one clears). height_cm and weight_kg
// win when both are sent.
type DetailsDTO struct {
	Alignment string   `json:"alignment"`
	Age       *int     `json:"age"`
	Sex       string   `json:"sex"`
	Height    string   `json:"height"`
	Weight    string   `json:"weight"`
	HeightCm  *Height  `json:"height_cm"`
	WeightKg  *Weight  `json:"weight_kg"`
	Hair      string   `json:"hair"`
	Eyes      string   `json:"eyes"`
	Languages []string `json:"languages"`
}

// ToDTO maps the domain details to their wire shape.
func ToDTO(d charactersvc.Details) DetailsDTO {
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

// field is one optional body field: absent leaves the value unchanged, JSON
// null clears it, a value sets it.
type field[T any] struct {
	present bool
	null    bool
	val     T
}

func (f *field[T]) UnmarshalJSON(b []byte) error {
	f.present = true
	if string(b) == "null" {
		f.null = true
		return nil
	}
	return json.Unmarshal(b, &f.val)
}

// ptr is the value to apply: nil when absent, a pointer to the zero value
// when null (clearing), else the value.
func (f field[T]) ptr() *T {
	if !f.present {
		return nil
	}
	v := f.val
	if f.null {
		var zero T
		v = zero
	}
	return &v
}

// Body is the PATCH body: only the fields present change, and null clears a
// field. An age of 0 is not a clear, the service rejects it.
type Body struct {
	Alignment field[string]   `json:"alignment"`
	Age       field[int]      `json:"age"`
	Sex       field[string]   `json:"sex"`
	Height    field[string]   `json:"height"`
	Weight    field[string]   `json:"weight"`
	HeightCm  field[Height]   `json:"height_cm"`
	WeightKg  field[Weight]   `json:"weight_kg"`
	Hair      field[string]   `json:"hair"`
	Eyes      field[string]   `json:"eyes"`
	Languages field[[]string] `json:"languages"`
}

func (b Body) patch() (detailssvc.Patch, error) {
	p := detailssvc.Patch{
		Alignment: b.Alignment.ptr(), Age: b.Age.ptr(), Sex: b.Sex.ptr(),
		Hair: b.Hair.ptr(), Eyes: b.Eyes.ptr(), Languages: b.Languages.ptr(),
	}
	if b.Age.null {
		p.Age, p.ClearAge = nil, true
	}
	var err error
	if p.Height, p.ClearHeight, err = ResolveMeasure(charactersvc.FieldHeight,
		numeric(b.HeightCm), b.HeightCm.null, text(b.Height), charactersvc.ParseHeight); err != nil {
		return p, err
	}
	if p.Weight, p.ClearWeight, err = ResolveMeasure(charactersvc.FieldWeight,
		numeric(b.WeightKg), b.WeightKg.null, text(b.Weight), charactersvc.ParseWeight); err != nil {
		return p, err
	}
	if p.Languages != nil && b.Languages.null {
		empty := []string{}
		p.Languages = &empty
	}
	return p, nil
}

// numeric is the number of a height_cm / weight_kg field: nil when absent or
// null (null clears, see ResolveMeasure).
func numeric[N ~float64](f field[N]) *float64 {
	if !f.present || f.null {
		return nil
	}
	v := float64(f.val)
	return &v
}

// text is the legacy text key: nil when absent, "" when null.
func text(f field[string]) *string { return f.ptr() }

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

// Update applies the fields present in the body and returns the details.
func Update(charID CharacterIDFunc, svc updater) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b Body
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			if fe, ok := errors.AsType[*domain.FieldError](err); ok {
				return fe
			}
			return render.BadRequestError("invalid JSON body")
		}
		patch, err := b.patch()
		if err != nil {
			return err
		}
		d, err := svc.Update(r.Context(), charID(r), patch)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(d))
		return nil
	}
}
