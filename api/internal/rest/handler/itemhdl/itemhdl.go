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
	SetState(ctx context.Context, characterID int64, itemID string, state itemsvc.State, swap bool) (*itemsvc.StateResult, error)
	Reviews(ctx context.Context, characterID int64) ([]itemsvc.Review, error)
	DismissReview(ctx context.Context, characterID int64, itemID string) error
}

// RegisterRoutes wires item endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc itemsvc.SVC) {
	r.Get("/api/items", render.Handle(List(charID, svc)))
	r.Post("/api/items", render.Handle(Create(charID, svc)))
	r.Patch("/api/items/{id}", render.Handle(Update(charID, svc)))
	r.Delete("/api/items/{id}", render.Handle(Delete(charID, svc)))
	r.Post("/api/items/{id}/use", render.Handle(Use(charID, svc)))
	r.Post("/api/items/{id}/restock", render.Handle(Restock(charID, svc)))
	r.Post("/api/items/{id}/state", render.Handle(SetState(charID, svc)))
	r.Delete("/api/items/{id}/review", render.Handle(DismissReview(charID, svc)))
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

// List returns every piece with its current count and its review reasons
// (ADR 0033 step 27).
func List(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		items, err := svc.List(r.Context(), charID(r))
		if err != nil {
			return err
		}
		reviews, err := svc.Reviews(r.Context(), charID(r))
		if err != nil {
			return err
		}
		out := ToDTOs(items)
		for i := range out {
			for _, rv := range reviews {
				if rv.ItemID == out[i].ID {
					out[i].Review = append(out[i].Review, rv.Reason)
				}
			}
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// ItemDTO is the wire shape of a piece of equipment (ADR 0033 step 1): the
// shared fields, then the fields of each category, zero (or empty) for the
// others. Codes go on the wire, the web names them (ADR 0023): category,
// magicKind, state (carried, worn, wielded), behaviour (stock, charges,
// daily), component (the spell component it is, empty for none; ADR 0033
// step 1.1). type is the free-text label of before the model, read only.
type ItemDTO struct {
	ID string `json:"id"`
	// Name is a language map (ADR 0023 step 9).
	Name          localized.Text `json:"name"`
	Category      string         `json:"category"`
	MagicKind     string         `json:"magicKind"`
	CatalogueCode string         `json:"catalogueCode"`
	Component     string         `json:"component"`
	State         string         `json:"state"`
	// EquipState is the state Equip moves the piece to (worn or wielded),
	// empty for a piece that is carried only (ADR 0033 step 6).
	EquipState   string `json:"equipState"`
	Type         string `json:"type"`
	Behaviour    string `json:"behaviour"`
	MagicalBonus int    `json:"magicalBonus"`
	AmmoKind     string `json:"ammoKind"`
	Count        int    `json:"count"`
	MaxCount     int    `json:"maxCount"`
	EffectDice   string `json:"effectDice"`
	EffectSave   string `json:"effectSave"`
	Notes        string `json:"notes"`
	// Powers are the item's own powers (ADR 0016 steps 9 to 16), read only:
	// whether one is on is served with the effects list (GET /api/buffs).
	Powers []PowerDTO `json:"powers"`
	// Weapon and ammunition.
	DamageTypes []string `json:"damageTypes"`
	DamageSM    string   `json:"damageSm"`
	DamageL     string   `json:"damageL"`
	// Weapon.
	WeaponType  string `json:"weaponType"`
	WeaponKind  string `json:"weaponKind"`
	Size        string `json:"size"`
	Speed       int    `json:"speed"`
	MagicToHit  int    `json:"magicToHit"`
	MagicDamage int    `json:"magicDamage"`
	TwoHanded   bool   `json:"twoHanded"`
	Fires       string `json:"fires"`
	Specialized bool   `json:"specialized"`
	// Armour.
	BaseAC      int    `json:"baseAc"`
	SkillColumn string `json:"skillColumn"`
	ArmorRow    string `json:"armorRow"`
	// Shield.
	ShieldKind string `json:"shieldKind"`
	ACMelee    int    `json:"acMelee"`
	ACMissile  int    `json:"acMissile"`
	// Review is the piece's review reasons (ADR 0033 step 27), empty when it
	// is not on the review list. Served by the list only.
	Review []string `json:"review"`
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
		damageTypes := i.DamageTypes
		if damageTypes == nil {
			damageTypes = []string{}
		}
		out = append(out, ItemDTO{
			ID: i.ID, Name: i.Name, Category: string(i.Category), MagicKind: i.MagicKind,
			CatalogueCode: i.CatalogueCode, Component: i.Component, State: string(i.State), EquipState: string(i.EquipState()),
			Type: i.Type, Behaviour: string(i.Behaviour),
			MagicalBonus: i.MagicBonus, AmmoKind: i.AmmoKind, Count: i.Count, MaxCount: i.MaxCount,
			EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Powers: powers,
			DamageTypes: damageTypes, DamageSM: i.DamageSM, DamageL: i.DamageL,
			WeaponType: i.WeaponType, WeaponKind: i.WeaponKind, Size: i.Size, Speed: i.Speed,
			MagicToHit: i.MagicToHit, MagicDamage: i.MagicDamage, TwoHanded: i.TwoHanded, Fires: i.Fires,
			Specialized: i.Specialized, BaseAC: i.BaseAC, SkillColumn: i.SkillColumn, ArmorRow: i.ArmorRow,
			ShieldKind: i.ShieldKind, ACMelee: i.ACMelee, ACMissile: i.ACMissile, Review: []string{},
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
// The fields of the piece's category count; the others are stored as sent
// (ADR 0033 step 1).
type ItemBody struct {
	// Name is a language map: the languages typed (ADR 0023 step 11).
	Name         localized.Text `json:"name"`
	Category     *string        `json:"category"`
	MagicKind    *string        `json:"magicKind"`
	Component    *string        `json:"component"`
	Behaviour    *string        `json:"behaviour"`
	Count        *int           `json:"count"`
	MaxCount     *int           `json:"maxCount"`
	MagicalBonus *int           `json:"magicalBonus"`
	AmmoKind     *string        `json:"ammoKind"`
	EffectDice   *string        `json:"effectDice"`
	EffectSave   *string        `json:"effectSave"`
	Notes        *string        `json:"notes"`
	DamageTypes  *[]string      `json:"damageTypes"`
	DamageSM     *string        `json:"damageSm"`
	DamageL      *string        `json:"damageL"`
	WeaponType   *string        `json:"weaponType"`
	WeaponKind   *string        `json:"weaponKind"`
	Size         *string        `json:"size"`
	Speed        *int           `json:"speed"`
	MagicToHit   *int           `json:"magicToHit"`
	MagicDamage  *int           `json:"magicDamage"`
	TwoHanded    *bool          `json:"twoHanded"`
	Fires        *string        `json:"fires"`
	Specialized  *bool          `json:"specialized"`
	BaseAC       *int           `json:"baseAc"`
	SkillColumn  *string        `json:"skillColumn"`
	ArmorRow     *string        `json:"armorRow"`
	ShieldKind   *string        `json:"shieldKind"`
	ACMelee      *int           `json:"acMelee"`
	ACMissile    *int           `json:"acMissile"`
	// Accepted and ignored, so a piece served back as is still decodes: the
	// powers come from the seed and data migrations only (ADR 0016 step 15),
	// the type label is no longer edited (ADR 0033 step 1), the state moves
	// through POST /api/items/{id}/state, the catalogue code and the review
	// are not the editor's.
	Powers        json.RawMessage `json:"powers"`
	Type          json.RawMessage `json:"type"`
	State         json.RawMessage `json:"state"`
	EquipState    json.RawMessage `json:"equipState"`
	CatalogueCode json.RawMessage `json:"catalogueCode"`
	Review        json.RawMessage `json:"review"`
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
			Name: b.Name, Category: itemsvc.Category(deref(b.Category)), MagicKind: deref(b.MagicKind),
			Component: deref(b.Component), Behaviour: itemsvc.Behaviour(deref(b.Behaviour)),
			Count: deref(b.Count), MaxCount: deref(b.MaxCount), MagicBonus: deref(b.MagicalBonus),
			AmmoKind: deref(b.AmmoKind), EffectDice: deref(b.EffectDice), EffectSave: deref(b.EffectSave),
			Notes: deref(b.Notes), DamageTypes: deref(b.DamageTypes), DamageSM: deref(b.DamageSM),
			DamageL: deref(b.DamageL), WeaponType: deref(b.WeaponType), WeaponKind: deref(b.WeaponKind),
			Size: deref(b.Size), Speed: deref(b.Speed), MagicToHit: deref(b.MagicToHit),
			MagicDamage: deref(b.MagicDamage), TwoHanded: deref(b.TwoHanded), Fires: deref(b.Fires),
			Specialized: deref(b.Specialized), BaseAC: deref(b.BaseAC), SkillColumn: deref(b.SkillColumn),
			ArmorRow: deref(b.ArmorRow), ShieldKind: deref(b.ShieldKind), ACMelee: deref(b.ACMelee),
			ACMissile: deref(b.ACMissile),
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
			Name: b.Name, MagicKind: b.MagicKind, Component: b.Component, Count: b.Count, MaxCount: b.MaxCount, MagicBonus: b.MagicalBonus,
			AmmoKind: b.AmmoKind, EffectDice: b.EffectDice, EffectSave: b.EffectSave, Notes: b.Notes,
			DamageTypes: b.DamageTypes, DamageSM: b.DamageSM, DamageL: b.DamageL, WeaponType: b.WeaponType,
			WeaponKind: b.WeaponKind, Size: b.Size, Speed: b.Speed, MagicToHit: b.MagicToHit,
			MagicDamage: b.MagicDamage, TwoHanded: b.TwoHanded, Fires: b.Fires, Specialized: b.Specialized,
			BaseAC: b.BaseAC, SkillColumn: b.SkillColumn, ArmorRow: b.ArmorRow, ShieldKind: b.ShieldKind,
			ACMelee: b.ACMelee, ACMissile: b.ACMissile,
		}
		if b.Behaviour != nil {
			bh := itemsvc.Behaviour(*b.Behaviour)
			p.Behaviour = &bh
		}
		if b.Category != nil {
			c := itemsvc.Category(*b.Category)
			p.Category = &c
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

// StateRequest is the body of POST /api/items/{id}/state: the state to move
// the piece to (carried, worn, wielded) and, for a body armour or a shield,
// swap to take off the one worn in the way (ADR 0033 step 6).
type StateRequest struct {
	State string `json:"state"`
	Swap  bool   `json:"swap"`
}

// StateDTO is the wire shape of a state change: the pieces as they are now
// (the one moved first) and the log entry written (0: nothing changed, none
// written).
type StateDTO struct {
	Items      []ItemDTO `json:"items"`
	LogEntryID int64     `json:"logEntryId"`
}

// SetState wears, takes off, wields or puts away a piece (ADR 0033 steps 6
// and 7).
func SetState(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req StateRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.SetState(r.Context(), charID(r), chi.URLParam(r, "id"), itemsvc.State(req.State), req.Swap)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, StateDTO{Items: ToDTOs(res.Items), LogEntryID: res.LogEntryID})
		return nil
	}
}

// DismissReview takes a piece off the review list (ADR 0033 step 27), not
// logged, and answers the list as GET /api/items does.
func DismissReview(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	list := List(charID, svc)
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := svc.DismissReview(r.Context(), charID(r), chi.URLParam(r, "id")); err != nil {
			return err
		}
		return list(w, r)
	}
}
