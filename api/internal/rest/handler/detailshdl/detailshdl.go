// Package detailshdl exposes PATCH /api/character/details, the character's
// editable free-text details.
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
// GET /api/character. age is null when not set.
type DetailsDTO struct {
	Race      string   `json:"race"`
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
	out := DetailsDTO{
		Race: d.Race, Alignment: d.Alignment, Sex: d.Sex, Height: d.Height, Weight: d.Weight,
		Hair: d.Hair, Eyes: d.Eyes, Languages: append([]string{}, d.Languages...),
	}
	if d.Age > 0 {
		age := d.Age
		out.Age = &age
	}
	return out
}

// Body is the PATCH body: only the fields present change. age null or 0
// clears the age.
type Body struct {
	Race      *string   `json:"race"`
	Alignment *string   `json:"alignment"`
	Age       *int      `json:"age"`
	Sex       *string   `json:"sex"`
	Height    *string   `json:"height"`
	Weight    *string   `json:"weight"`
	Hair      *string   `json:"hair"`
	Eyes      *string   `json:"eyes"`
	Languages *[]string `json:"languages"`
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
		d, err := svc.Update(r.Context(), charID(r), detailssvc.Patch(b))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTO(d))
		return nil
	}
}
