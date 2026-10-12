// Package itemhdl exposes /api/items.
package itemhdl

import (
	"context"
	"net/http"

	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
}

type itemSvc interface {
	List(ctx context.Context) ([]itemsvc.Item, error)
	ResetDaily(ctx context.Context) ([]itemsvc.Item, error)
}

// RegisterRoutes wires item endpoints onto r.
func RegisterRoutes(r Router, svc itemsvc.SVC) {
	r.Get("/api/items", render.Handle(List(svc)))
	r.Post("/api/items/reset", render.Handle(Reset(svc)))
}

// List returns every item with its current count.
func List(svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		items, err := svc.List(r.Context())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTOs(items))
		return nil
	}
}

// Reset refills the per-day items (long rest).
func Reset(svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		items, err := svc.ResetDaily(r.Context())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, toDTOs(items))
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

func toDTOs(items []itemsvc.Item) []ItemDTO {
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
