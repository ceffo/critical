// Package itemhdl exposes /api/items.
package itemhdl

import (
	"context"
	"encoding/json"
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
	Restock(ctx context.Context, characterID int64, itemID string, n int) (*itemsvc.RestockResult, error)
}

// RegisterRoutes wires item endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc itemsvc.SVC) {
	r.Get("/api/items", render.Handle(List(svc)))
	r.Post("/api/items/{id}/use", render.Handle(Use(charID, svc)))
	r.Post("/api/items/{id}/restock", render.Handle(Restock(charID, svc)))
}

// UseResultDTO is the wire shape of POST /api/items/{id}/use: the dice
// rolled with their breakdown and the count left. Same camelCase names as
// the item_use log payload.
type UseResultDTO struct {
	ItemID     string `json:"itemId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Behaviour  string `json:"behaviour"`
	Removed    bool   `json:"removed"`
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
			ItemID: res.ItemID, Name: res.Name, Type: res.Type, Behaviour: string(res.Behaviour),
			Removed: res.Removed, Remaining: res.Remaining,
			MaxCount: res.MaxCount, Dice: res.Dice, Rolls: res.Rolls, Total: res.Total,
			Save: res.Save, Notes: res.Notes, Breakdown: res.Breakdown, LogEntryID: res.LogEntryID,
		})
		return nil
	}
}

// RestockRequest is the body of POST /api/items/{id}/restock.
type RestockRequest struct {
	Count *int `json:"count"`
}

// RestockResultDTO is the wire shape of a restock, the same camelCase names
// as the item_restock log payload.
type RestockResultDTO struct {
	ItemID     string `json:"itemId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Added      int    `json:"added"`
	Count      int    `json:"count"`
	LogEntryID int64  `json:"logEntryId"`
}

// Restock adds count to a stock item and logs it.
func Restock(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req RestockRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		if req.Count == nil {
			return render.BadRequestError("count is required")
		}
		res, err := svc.Restock(r.Context(), charID(r), chi.URLParam(r, "id"), *req.Count)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, RestockResultDTO{
			ItemID: res.ItemID, Name: res.Name, Type: res.Type, Added: res.Added,
			Count: res.Count, LogEntryID: res.LogEntryID,
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

// ItemDTO is the wire shape of an item (critical-qki.13): behaviour is
// stock, charges or daily, type the free-text label.
type ItemDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Behaviour    string `json:"behaviour"`
	MagicalBonus int    `json:"magicalBonus"`
	AmmoKind     string `json:"ammoKind"`
	Count        int    `json:"count"`
	MaxCount     int    `json:"maxCount"`
	EffectDice   string `json:"effectDice"`
	EffectSave   string `json:"effectSave"`
	Notes        string `json:"notes"`
}

// ToDTOs maps items to their wire shape.
func ToDTOs(items []itemsvc.Item) []ItemDTO {
	out := make([]ItemDTO, 0, len(items))
	for _, i := range items {
		out = append(out, ItemDTO{
			ID: i.ID, Name: i.Name, Type: i.Type, Behaviour: string(i.Behaviour),
			MagicalBonus: i.MagicBonus, AmmoKind: i.AmmoKind, Count: i.Count, MaxCount: i.MaxCount,
			EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes,
		})
	}
	return out
}
