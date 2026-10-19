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
	"github.com/ceffo/critical/api/internal/rest/wire"
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
	Alignment field[string]      `json:"alignment"`
	Age       field[int]         `json:"age"`
	Sex       field[string]      `json:"sex"`
	Height    field[string]      `json:"height"`
	Weight    field[string]      `json:"weight"`
	HeightCm  field[wire.Height] `json:"height_cm"`
	WeightKg  field[wire.Weight] `json:"weight_kg"`
	Hair      field[string]      `json:"hair"`
	Eyes      field[string]      `json:"eyes"`
	Languages field[[]string]    `json:"languages"`
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
	if p.Height, p.ClearHeight, err = wire.ResolveMeasure(charactersvc.FieldHeight,
		numeric(b.HeightCm), b.HeightCm.null, text(b.Height), charactersvc.ParseHeight); err != nil {
		return p, err
	}
	if p.Weight, p.ClearWeight, err = wire.ResolveMeasure(charactersvc.FieldWeight,
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
// null (null clears, see wire.ResolveMeasure).
func numeric[N ~float64](f field[N]) *float64 {
	if !f.present || f.null {
		return nil
	}
	v := float64(f.val)
	return &v
}

// text is the legacy text key: nil when absent, "" when null.
func text(f field[string]) *string { return f.ptr() }

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
		render.OK(r.Context(), w, wire.DetailsToDTO(d))
		return nil
	}
}
