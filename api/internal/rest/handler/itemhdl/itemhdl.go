// Package itemhdl exposes /api/items.
package itemhdl

import (
	"context"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/go-chi/chi/v5"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type itemSvc interface {
	List(ctx context.Context) ([]itemsvc.Item, error)
	UseItem(ctx context.Context, characterID int64, itemID string) (*itemsvc.UseResult, error)
}

// RegisterRoutes wires item endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc itemsvc.SVC) {
	r.Get("/api/items", render.Handle(List(svc)))
	r.Post("/api/items/{id}/use", render.Handle(Use(charID, svc)))
}

// UseResultDTO is the wire shape of POST /api/items/{id}/use: the dice
// rolled with their breakdown and the count left. Same camelCase names as
// the item_use log payload.
type UseResultDTO struct {
	ItemID     string `json:"itemId"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Remaining  int    `json:"remaining"`
	MaxCount   int    `json:"maxCount"`
	Dice       string `json:"dice"`
	Rolls      []int  `json:"rolls"`
	Total      int    `json:"total"`
	Save       string `json:"save"`
	Notes      string `json:"notes"`
	Breakdown  string `json:"breakdown"`
	LogEntryID int64  `json:"logEntryId"`
}

// Use spends one count of the item, rolls its effect and logs the use.
func Use(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		res, err := svc.UseItem(r.Context(), charID(r), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, UseResultDTO{
			ItemID: res.ItemID, Name: res.Name, Kind: string(res.Kind), Remaining: res.Remaining,
			MaxCount: res.MaxCount, Dice: res.Dice, Rolls: res.Rolls, Total: res.Total,
			Save: res.Save, Notes: res.Notes, Breakdown: res.Breakdown, LogEntryID: res.LogEntryID,
		})
		return nil
	}
}

// Compile-time check that *chi.Mux satisfies our small Router interface.
var _ Router = (*chi.Mux)(nil)

// List returns every item with its current count.
func List(svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		items, err := svc.List(r.Context())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTOs(items))
		return nil
	}
}

// ItemDTO is the wire shape of an item.
type ItemDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	MagicBonus int    `json:"magic_bonus"`
	AmmoKind   string `json:"ammo_kind"`
	Count      int    `json:"count"`
	MaxCount   int    `json:"max_count"`
	PerDay     bool   `json:"per_day"`
	EffectDice string `json:"effect_dice"`
	EffectSave string `json:"effect_save"`
	Notes      string `json:"notes"`
}

// ToDTOs maps items to their wire shape.
func ToDTOs(items []itemsvc.Item) []ItemDTO {
	out := make([]ItemDTO, 0, len(items))
	for _, i := range items {
		out = append(out, ItemDTO{
			ID: i.ID, Name: i.Name, Kind: string(i.Kind), MagicBonus: i.MagicBonus,
			AmmoKind: i.AmmoKind, Count: i.Count, MaxCount: i.MaxCount, PerDay: i.PerDay,
			EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes,
		})
	}
	return out
}
