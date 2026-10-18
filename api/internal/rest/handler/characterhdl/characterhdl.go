// Package characterhdl exposes /api/character (the current character's
// sheet) and /api/characters (the user's characters and which one is current,
// ADR 0029 step 7).
package characterhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/detailshdl"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character ID per-request.
type CharacterIDFunc func(r *http.Request) int64

type characterGetter interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// RegisterRoutes wires the current character's endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc charactersvc.SVC) {
	r.Get("/api/character", render.Handle(GetCharacter(charID, svc)))
}

// RegisterUserRoutes wires the user's characters onto r: they need a
// session, not a current character (a new player has none yet).
func RegisterUserRoutes(r Router, svc charactersvc.SVC) {
	r.Get("/api/characters", render.Handle(List(svc)))
	r.Put("/api/characters/current", render.Handle(SetCurrent(svc)))
}

type ownedLister interface {
	List(ctx context.Context, owner uuid.UUID) ([]charactersvc.Summary, error)
}

type currentSetter interface {
	ownedLister
	SetCurrent(ctx context.Context, owner uuid.UUID, id int64) error
}

// SummaryDTO is a character in its owner's list.
type SummaryDTO struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Class string `json:"class"`
	Level int    `json:"level"`
}

// CharactersDTO is the user's characters, not retired, in creation order, and
// the id of the current one (null: none).
type CharactersDTO struct {
	Characters []SummaryDTO `json:"characters"`
	CurrentID  *int64       `json:"currentId"`
}

// user is the request's user; the gate lets no request without one reach a
// data route.
func user(r *http.Request) (*authsvc.User, error) {
	p := mw.PrincipalFrom(r.Context())
	if p == nil {
		return nil, authsvc.ErrUnauthenticated
	}
	return &p.User, nil
}

func renderCharacters(w http.ResponseWriter, r *http.Request, svc ownedLister, owner uuid.UUID, current int64) error {
	list, err := svc.List(r.Context(), owner)
	if err != nil {
		return err
	}
	out := CharactersDTO{Characters: make([]SummaryDTO, 0, len(list))}
	for _, c := range list {
		out.Characters = append(out.Characters, SummaryDTO{ID: c.ID, Name: c.Name, Class: c.Class, Level: c.Level})
		if c.ID == current {
			id := c.ID
			out.CurrentID = &id
		}
	}
	render.OK(r.Context(), w, out)
	return nil
}

// List is GET /api/characters: the user's own characters (ADR 0029 steps
// 7.1 to 7.3).
func List(svc ownedLister) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		u, err := user(r)
		if err != nil {
			return err
		}
		return renderCharacters(w, r, svc, u.ID, u.CurrentCharacterID)
	}
}

// CurrentBody is the body of PUT /api/characters/current.
type CurrentBody struct {
	ID *int64 `json:"id"`
}

// SetCurrent is PUT /api/characters/current {id}: the user's character id
// becomes current (ADR 0029 step 7.2); another user's character is 404, as an
// unknown id (step 7.1). It answers with the list, as GET /api/characters.
func SetCurrent(svc currentSetter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		u, err := user(r)
		if err != nil {
			return err
		}
		var b CurrentBody
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&b); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		if b.ID == nil {
			return render.BadRequestError("id is required")
		}
		if err := svc.SetCurrent(r.Context(), u.ID, *b.ID); err != nil {
			return err
		}
		return renderCharacters(w, r, svc, u.ID, *b.ID)
	}
}

// GetCharacter renders the character stats DTO.
func GetCharacter(charID CharacterIDFunc, svc characterGetter) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		c, err := svc.GetCharacter(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTO(c))
		return nil
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
	Class            string `json:"class"`
	detailshdl.DetailsDTO
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
		Class: c.Class, DetailsDTO: detailshdl.ToDTO(c.Details),
	}
}
