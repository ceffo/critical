// Package itemhdl exposes /api/items.
package itemhdl

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/handler/buffhdl"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

type itemSvc interface {
	List(ctx context.Context, characterID int64) ([]itemsvc.Item, error)
	UseItem(ctx context.Context, characterID int64, itemID string) (*itemsvc.UseResult, error)
	Restock(ctx context.Context, characterID int64, itemID string, n int) (*itemsvc.RestockResult, error)
	Create(ctx context.Context, characterID int64, in itemsvc.ItemInput) (*itemsvc.ChangeResult, error)
	Update(ctx context.Context, characterID int64, itemID string, p itemsvc.ItemPatch) (*itemsvc.ChangeResult, error)
	Delete(ctx context.Context, characterID int64, itemID string) (*itemsvc.ChangeResult, error)
}

// RegisterRoutes wires item endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc itemsvc.SVC) {
	r.Get("/api/items", render.Handle(List(charID, svc)))
	r.Post("/api/items", render.Handle(Create(charID, svc)))
	r.Patch("/api/items/{id}", render.Handle(Update(charID, svc)))
	r.Delete("/api/items/{id}", render.Handle(Delete(charID, svc)))
	r.Post("/api/items/{id}/use", render.Handle(Use(charID, svc)))
	r.Post("/api/items/{id}/restock", render.Handle(Restock(charID, svc)))
}

// UseResultDTO is the wire shape of POST /api/items/{id}/use: the dice
// rolled (each die, the flat modifier, the total) and the count left. Same camelCase names as
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
	Modifier   int    `json:"modifier"`
	Total      int    `json:"total"`
	Save       string `json:"save"`
	Notes      string `json:"notes"`
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
			Modifier: res.Modifier, Save: res.Save, Notes: res.Notes, LogEntryID: res.LogEntryID,
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
func List(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		items, err := svc.List(r.Context(), charID(r))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, ToDTOs(items))
		return nil
	}
}

// ItemDTO is the wire shape of an item: behaviour is
// stock, charges or daily, type the free-text label.
type ItemDTO struct {
	ID string `json:"id"`
	// Name is a language map (ADR 0023 step 9).
	Name         localized.Text `json:"name"`
	Type         string         `json:"type"`
	Behaviour    string         `json:"behaviour"`
	MagicalBonus int            `json:"magicalBonus"`
	AmmoKind     string         `json:"ammoKind"`
	Count        int            `json:"count"`
	MaxCount     int            `json:"maxCount"`
	EffectDice   string         `json:"effectDice"`
	EffectSave   string         `json:"effectSave"`
	Notes        string         `json:"notes"`
	// Powers are the item's own powers (ADR 0016 steps 9 to 16), read only:
	// whether one is on is served with the effects list (GET /api/buffs).
	Powers []PowerDTO `json:"powers"`
}

// PowerDTO is one of an item's powers: its id, its trigger (minStr, omitted:
// always on) and its effects, in the buffs' effect keys.
type PowerDTO struct {
	ID      string                 `json:"id"`
	MinStr  *int                   `json:"minStr,omitempty"`
	Effects buffhdl.BuffEffectsDTO `json:"effects"`
}

// ToDTOs maps items to their wire shape.
func ToDTOs(items []itemsvc.Item) []ItemDTO {
	out := make([]ItemDTO, 0, len(items))
	for _, i := range items {
		powers := make([]PowerDTO, 0, len(i.Powers))
		for _, p := range i.Powers {
			powers = append(powers, PowerDTO{ID: p.ID, MinStr: p.MinStr, Effects: buffhdl.BuffEffectsDTO(p.Effects)})
		}
		out = append(out, ItemDTO{
			ID: i.ID, Name: i.Name, Type: i.Type, Behaviour: string(i.Behaviour),
			MagicalBonus: i.MagicBonus, AmmoKind: i.AmmoKind, Count: i.Count, MaxCount: i.MaxCount,
			EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Powers: powers,
		})
	}
	return out
}

// ChangeDTO is the wire shape of POST/PATCH/DELETE /api/items: the item as it
// is now (as it was, for a delete) and the log entry written.
type ChangeDTO struct {
	Item       ItemDTO `json:"item"`
	LogEntryID int64   `json:"logEntryId"`
}

// ItemBody is the body of POST and PATCH /api/items. On a PATCH only the
// fields present change. maxCount is the daily number (daily items only).
type ItemBody struct {
	// Name is a language map: the languages typed (ADR 0023 step 11).
	Name         localized.Text `json:"name"`
	Type         *string        `json:"type"`
	Behaviour    *string        `json:"behaviour"`
	Count        *int           `json:"count"`
	MaxCount     *int           `json:"maxCount"`
	MagicalBonus *int           `json:"magicalBonus"`
	AmmoKind     *string        `json:"ammoKind"`
	EffectDice   *string        `json:"effectDice"`
	EffectSave   *string        `json:"effectSave"`
	Notes        *string        `json:"notes"`
	// Powers is accepted and ignored (ADR 0016 step 15): powers come from
	// the seed and data migrations only, so an item served back as is still
	// decodes, but no body sets them.
	Powers json.RawMessage `json:"powers"`
}

func decodeBody(r *http.Request) (ItemBody, error) {
	var b ItemBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return b, render.BadRequestError("invalid JSON body")
	}
	return b, nil
}

func change(ctx context.Context, w http.ResponseWriter, res *itemsvc.ChangeResult) {
	render.OK(ctx, w, ChangeDTO{Item: ToDTOs([]itemsvc.Item{res.Item})[0], LogEntryID: res.LogEntryID})
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// Create adds an item and logs it.
func Create(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		b, err := decodeBody(r)
		if err != nil {
			return err
		}
		res, err := svc.Create(r.Context(), charID(r), itemsvc.ItemInput{
			Name: b.Name, Type: deref(b.Type), Behaviour: itemsvc.Behaviour(deref(b.Behaviour)),
			Count: deref(b.Count), MaxCount: deref(b.MaxCount), MagicBonus: deref(b.MagicalBonus),
			AmmoKind: deref(b.AmmoKind), EffectDice: deref(b.EffectDice), EffectSave: deref(b.EffectSave),
			Notes: deref(b.Notes),
		})
		if err != nil {
			return err
		}
		change(r.Context(), w, res)
		return nil
	}
}

// Update edits the fields present in the body and logs the change.
func Update(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		b, err := decodeBody(r)
		if err != nil {
			return err
		}
		p := itemsvc.ItemPatch{
			Name: b.Name, Type: b.Type, Count: b.Count, MaxCount: b.MaxCount, MagicBonus: b.MagicalBonus,
			AmmoKind: b.AmmoKind, EffectDice: b.EffectDice, EffectSave: b.EffectSave, Notes: b.Notes,
		}
		if b.Behaviour != nil {
			bh := itemsvc.Behaviour(*b.Behaviour)
			p.Behaviour = &bh
		}
		res, err := svc.Update(r.Context(), charID(r), chi.URLParam(r, "id"), p)
		if err != nil {
			return err
		}
		change(r.Context(), w, res)
		return nil
	}
}

// Delete removes an item and logs it.
func Delete(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		res, err := svc.Delete(r.Context(), charID(r), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		change(r.Context(), w, res)
		return nil
	}
}
