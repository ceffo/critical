// Package detailshdl exposes PATCH /api/character/details, the character's
// editable details.
package detailshdl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

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
// GET /api/character. age, height (centimetres) and weight (kilograms) are
// null when not set (ADR 0032 step 5); alignment and sex are codes (steps 2
// and 3), "" when missing.
type DetailsDTO struct {
	Alignment string   `json:"alignment"`
	Age       *int     `json:"age"`
	Sex       string   `json:"sex"`
	Height    *Height  `json:"height"`
	Weight    *Weight  `json:"weight"`
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
		out.Height = &h
	}
	if d.Weight > 0 {
		w := Weight(d.Weight)
		out.Weight = &w
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
	Height    field[Height]   `json:"height"`
	Weight    field[Weight]   `json:"weight"`
	Hair      field[string]   `json:"hair"`
	Eyes      field[string]   `json:"eyes"`
	Languages field[[]string] `json:"languages"`
}

func (b Body) patch() detailssvc.Patch {
	p := detailssvc.Patch{
		Alignment: b.Alignment.ptr(), Age: b.Age.ptr(), Sex: b.Sex.ptr(),
		Height: measure(b.Height), Weight: measure(b.Weight), Hair: b.Hair.ptr(), Eyes: b.Eyes.ptr(),
		Languages: b.Languages.ptr(),
	}
	if b.Age.null {
		p.Age, p.ClearAge = nil, true
	}
	p.ClearHeight, p.ClearWeight = b.Height.null, b.Weight.null
	if p.Languages != nil && b.Languages.null {
		empty := []string{}
		p.Languages = &empty
	}
	return p
}

// measure is the value to apply: nil when absent or null (null clears, see
// DetailsPatch.ClearHeight), else the number.
func measure[N ~float64](f field[N]) *float64 {
	if !f.present || f.null {
		return nil
	}
	v := float64(f.val)
	return &v
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
		d, err := svc.Update(r.Context(), charID(r), b.patch())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(d))
		return nil
	}
}
