// Package dicehdl exposes the dice composer: POST /api/roll/dice (a formula
// rolled and logged on the server) and the favourites of the active
// character under /api/dice/favourites.
package dicehdl

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ceffo/critical/api/internal/domain/service/dicesvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/go-chi/chi/v5"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

var _ Router = (*chi.Mux)(nil)

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type diceSvc interface {
	Roll(ctx context.Context, characterID int64, formula string) (*dicesvc.Result, error)
	Favourites(ctx context.Context, characterID int64) ([]dicesvc.Favourite, error)
	AddFavourite(ctx context.Context, characterID int64, formula string) (*dicesvc.Favourite, error)
	DeleteFavourite(ctx context.Context, characterID, id int64) error
}

// RegisterRoutes wires the dice endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc dicesvc.SVC) {
	r.Post("/api/roll/dice", render.Handle(Roll(charID, svc)))
	r.Get("/api/dice/favourites", render.Handle(List(charID, svc)))
	r.Post("/api/dice/favourites", render.Handle(Add(charID, svc)))
	r.Delete("/api/dice/favourites/{id}", render.Handle(Delete(charID, svc)))
}

// GroupDTO is the dice of one group: subtotal is signed (negative for a
// subtracted group).
type GroupDTO struct {
	Sides    int   `json:"sides"`
	Rolls    []int `json:"rolls"`
	Negative bool  `json:"negative"`
	Subtotal int   `json:"subtotal"`
}

// RollDTO is the full breakdown of a free roll. formula is the normal form of
// what was sent.
type RollDTO struct {
	Formula    string     `json:"formula"`
	Groups     []GroupDTO `json:"groups"`
	Modifier   int        `json:"modifier"`
	Total      int        `json:"total"`
	LogEntryID int64      `json:"logEntryId"`
}

// FavouriteDTO is one saved formula.
type FavouriteDTO struct {
	ID      int64  `json:"id"`
	Formula string `json:"formula"`
}

// FavouritesDTO is the GET /api/dice/favourites answer.
type FavouritesDTO struct {
	Favourites []FavouriteDTO `json:"favourites"`
}

// RollToDTO maps a roll to its wire shape.
func RollToDTO(r *dicesvc.Result) RollDTO {
	out := RollDTO{
		Formula: r.Formula, Groups: make([]GroupDTO, 0, len(r.Groups)), Modifier: r.Modifier, Total: r.Total,
		LogEntryID: r.LogEntryID,
	}
	for _, g := range r.Groups {
		out.Groups = append(out.Groups, GroupDTO{Sides: g.Sides, Rolls: g.Rolls, Negative: g.Negative, Subtotal: g.Subtotal})
	}
	return out
}

func favouriteDTO(f dicesvc.Favourite) FavouriteDTO {
	return FavouriteDTO{ID: f.ID, Formula: f.Formula}
}

type formulaBody struct {
	Formula *string `json:"formula"`
}

func decodeFormula(r *http.Request) (string, error) {
	var b formulaBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return "", render.BadRequestError("invalid request body")
	}
	if b.Formula == nil {
		return "", render.BadRequestError("formula is required")
	}
	return *b.Formula, nil
}

// Roll rolls a formula; an invalid one is a 400 with the reason.
func Roll(charID CharacterIDFunc, svc diceSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		formula, err := decodeFormula(r)
		if err != nil {
			return err
		}
		res, err := svc.Roll(r.Context(), charID(r), formula)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RollToDTO(res))
		return nil
	}
}

// List returns the favourites.
func List(charID CharacterIDFunc, svc diceSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		return renderList(w, r, charID, svc)
	}
}

func renderList(w http.ResponseWriter, r *http.Request, charID CharacterIDFunc, svc diceSvc) error {
	favs, err := svc.Favourites(r.Context(), charID(r))
	if err != nil {
		return err
	}
	out := FavouritesDTO{Favourites: make([]FavouriteDTO, 0, len(favs))}
	for _, f := range favs {
		out.Favourites = append(out.Favourites, favouriteDTO(f))
	}
	render.OK(r.Context(), w, out)
	return nil
}

// Add saves a formula as a favourite.
func Add(charID CharacterIDFunc, svc diceSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		formula, err := decodeFormula(r)
		if err != nil {
			return err
		}
		f, err := svc.AddFavourite(r.Context(), charID(r), formula)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, favouriteDTO(*f))
		return nil
	}
}

// Delete removes a favourite and returns the remaining ones; an unknown id is
// a 404.
func Delete(charID CharacterIDFunc, svc diceSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil || id < 1 {
			return render.BadRequestError("invalid favourite id")
		}
		if err := svc.DeleteFavourite(r.Context(), charID(r), id); err != nil {
			return err
		}
		return renderList(w, r, charID, svc)
	}
}
