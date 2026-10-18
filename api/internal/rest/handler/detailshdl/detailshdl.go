// Package detailshdl exposes PATCH /api/character/details, the character's
// editable details.
package detailshdl

import (
	"context"
	"encoding/json"
	"net/http"

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
// GET /api/character. age is null when not set; alignment and sex are codes
// (ADR 0032 steps 2 and 3), "" when missing.
type DetailsDTO struct {
	Alignment string   `json:"alignment"`
	Age       *int     `json:"age"`
	Sex       string   `json:"sex"`
	Height    string   `json:"height"`
	Weight    string   `json:"weight"`
	Hair      string   `json:"hair"`
	Eyes      string   `json:"eyes"`
	Languages []string `json:"languages"`
}

// ToDTO maps the domain details to their wire shape.
func ToDTO(d charactersvc.Details) DetailsDTO {
	d = d.Coded()
	out := DetailsDTO{
		Alignment: d.Alignment, Sex: d.Sex, Height: d.Height, Weight: d.Weight,
		Hair: d.Hair, Eyes: d.Eyes, Languages: append([]string{}, d.Languages...),
	}
	if d.Age > 0 {
		age := d.Age
		out.Age = &age
	}
	return out
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
	Hair      field[string]   `json:"hair"`
	Eyes      field[string]   `json:"eyes"`
	Languages field[[]string] `json:"languages"`
}

func (b Body) patch() detailssvc.Patch {
	p := detailssvc.Patch{
		Alignment: b.Alignment.ptr(), Age: b.Age.ptr(), Sex: b.Sex.ptr(),
		Height: b.Height.ptr(), Weight: b.Weight.ptr(), Hair: b.Hair.ptr(), Eyes: b.Eyes.ptr(),
		Languages: b.Languages.ptr(),
	}
	if b.Age.null {
		p.Age, p.ClearAge = nil, true
	}
	if p.Languages != nil && b.Languages.null {
		empty := []string{}
		p.Languages = &empty
	}
	return p
}

// Update applies the fields present in the body and returns the details.
func Update(charID CharacterIDFunc, svc updater) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var b Body
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
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
