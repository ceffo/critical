// Package characterhdl exposes /api/character.
package characterhdl

import (
	"context"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character ID per-request.
type CharacterIDFunc func(r *http.Request) int64

type characterGetter interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// RegisterRoutes wires character endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc charactersvc.SVC) {
	r.Get("/api/character", GetCharacter(charID, svc))
}

// GetCharacter renders the character stats DTO.
func GetCharacter(charID CharacterIDFunc, svc characterGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := svc.GetCharacter(r.Context(), charID(r))
		if err != nil {
			render.Error(r.Context(), w, err)
			return
		}
		render.OK(r.Context(), w, toDTO(c))
	}
}

// CharacterDTO preserves the API contract previously emitted by domain.Character.
type CharacterDTO struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Str              int    `json:"str"`
	Dex              int    `json:"dex"`
	Con              int    `json:"con"`
	Int              int    `json:"int"`
	Wis              int    `json:"wis"`
	Cha              int    `json:"cha"`
	HPMax            int    `json:"hpMax"`
	Thac0Base        int    `json:"thac0Base"`
	Level            int    `json:"level"`
	XPTotal          int    `json:"xpTotal"`
	SaveParaPoison   int    `json:"saveParaPoison"`
	SaveRodStaffWand int    `json:"saveRodStaffWand"`
	SavePetriPoly    int    `json:"savePetriPoly"`
	SaveBreath       int    `json:"saveBreath"`
	SaveSpell        int    `json:"saveSpell"`
}

func toDTO(c *charactersvc.Character) CharacterDTO {
	if c == nil {
		return CharacterDTO{}
	}
	return CharacterDTO{
		ID: c.ID, Name: c.Name,
		Str: c.Str, Dex: c.Dex, Con: c.Con, Int: c.Int, Wis: c.Wis, Cha: c.Cha,
		HPMax: c.HPMax, Thac0Base: c.Thac0Base, Level: c.Level, XPTotal: c.XPTotal,
		SaveParaPoison: c.SaveParaPoison, SaveRodStaffWand: c.SaveRodStaffWand,
		SavePetriPoly: c.SavePetriPoly, SaveBreath: c.SaveBreath, SaveSpell: c.SaveSpell,
	}
}
