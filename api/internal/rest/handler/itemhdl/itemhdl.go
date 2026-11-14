// Package itemhdl exposes /api/items.
package itemhdl

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/render"
	"github.com/ceffo/critical/api/internal/rest/wire"
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
	UseAbility(ctx context.Context, characterID int64, itemID, abilityID string, asks itemsvc.UseAsks) (*itemsvc.AbilityUseResult, error)
	Restock(ctx context.Context, characterID int64, itemID string, n int) (*itemsvc.RestockResult, error)
	Create(ctx context.Context, characterID int64, in itemsvc.ItemInput) (*itemsvc.ChangeResult, error)
	Update(ctx context.Context, characterID int64, itemID string, p itemsvc.ItemPatch) (*itemsvc.ChangeResult, error)
	Delete(ctx context.Context, characterID int64, itemID string) (*itemsvc.ChangeResult, error)
	SetState(ctx context.Context, characterID int64, itemID string, state itemsvc.State, swap bool) (*itemsvc.StateResult, error)
	Reviews(ctx context.Context, characterID int64) ([]itemsvc.Review, error)
	Warnings(ctx context.Context, characterID int64, items []itemsvc.Item) ([]itemsvc.Warning, error)
	DismissReview(ctx context.Context, characterID int64, itemID string) error
	Catalogue() ruletablesvc.Equipment
}

// SpellAsker serves what the use of a spell effect asks of the player (prepsvc.SVC).
type SpellAsker interface {
	AsksFor(spellID string, level int, inventory []itemsvc.Item) (prepsvc.SpellAsks, error)
}

// RegisterRoutes wires item endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc itemsvc.SVC, asker SpellAsker) {
	r.Get("/api/items", render.Handle(List(charID, svc, asker)))
	r.Post("/api/items", render.Handle(Create(charID, svc, asker)))
	r.Patch("/api/items/{id}", render.Handle(Update(charID, svc, asker)))
	r.Delete("/api/items/{id}", render.Handle(Delete(charID, svc, asker)))
	r.Post("/api/items/{id}/use", render.Handle(Use(charID, svc)))
	r.Post("/api/items/{id}/abilities/{abilityId}/use", render.Handle(UseAbility(charID, svc)))
	r.Post("/api/items/{id}/restock", render.Handle(Restock(charID, svc)))
	r.Post("/api/items/{id}/state", render.Handle(SetState(charID, svc, asker)))
	r.Delete("/api/items/{id}/review", render.Handle(DismissReview(charID, svc, asker)))
	r.Get("/api/equipment/catalogue", render.Handle(Catalogue(svc)))
	r.Get("/api/equipment/rules", render.Handle(Rules()))
}

// Rules serves which categories and magic kinds have a quantity the player
// sets (ADR 0049 step 2): reference data, the same for every character, so the
// editor does not copy the rule.
func Rules() render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		render.OK(r.Context(), w, wire.EquipmentRulesToDTO(itemsvc.QuantityRules()))
		return nil
	}
}

// Catalogue serves the equipment rows a piece is picked from (ADR 0033 steps
// 21 and 24): the same for every character.
func Catalogue(svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		render.OK(r.Context(), w, wire.EquipmentCatalogueToDTO(svc.Catalogue()))
		return nil
	}
}

// UseResultDTO is the wire shape of POST /api/items/{id}/use, spend one (ADR
// 0049 step 2): the quantity left. Nothing is rolled: the dice fields are the
// journal's shape of a use with none. Same camelCase names as the item_use log
// payload.
type UseResultDTO struct {
	ItemID     string `json:"itemId"`
	Name       string `json:"name"`
	Type       string `json:"type"`
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

// Use spends one of a stock item's quantity (the inventory's minus) and logs it.
func Use(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		res, err := svc.UseItem(r.Context(), charID(r), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, UseResultDTO{
			ItemID: res.ItemID, Name: res.Name, Type: res.Type,
			Removed: res.Removed, Remaining: res.Remaining, Rolls: []int{},
			Notes: res.Notes, LogEntryID: res.LogEntryID,
		})
		return nil
	}
}

// AbilityUseResultDTO is the wire shape of POST
// /api/items/{id}/abilities/{abilityId}/use (ADR 0049 step 7): the limit as it
// stands after the use and each effect's result. Same camelCase names as the
// item_use log payload, which also carries the first dice effect in the
// journal's flat fields (dice, rolls, modifier, total, save).
type AbilityUseResultDTO struct {
	ItemID      string         `json:"itemId"`
	AbilityID   string         `json:"abilityId"`
	Name        string         `json:"name"`
	AbilityName localized.Text `json:"abilityName,omitempty"`
	Type        string         `json:"type"`
	Limit       string         `json:"limit"`
	// Left and Max are sent for per_day and charges only; Remaining is the
	// counter or the quantity left (0 for unlimited).
	Left       *int                   `json:"left,omitempty"`
	Max        *int                   `json:"max,omitempty"`
	Remaining  int                    `json:"remaining"`
	Removed    bool                   `json:"removed"`
	Effects    []itemsvc.EffectResult `json:"effects"`
	Notes      string                 `json:"notes,omitempty"`
	LogEntryID int64                  `json:"logEntryId"`
}

// UseAbilityRequest is the optional body of POST
// /api/items/{id}/abilities/{abilityId}/use (ADR 0049 steps 9 and 10): the
// player's answers for the ability's spell effects, one entry per spell effect
// that needs one, by the effect's index in the ability's list. The fields are
// the cast's own (casthdl's request: onSelf, targetAc, itemId, extraSlots,
// mode); an effect with no entry, or no body at all, takes the cast's defaults.
type UseAbilityRequest struct {
	Spells []SpellAskDTO `json:"spells,omitempty"`
}

// SpellAskDTO is the answers for one spell effect.
type SpellAskDTO struct {
	Effect     int    `json:"effect"`
	OnSelf     *bool  `json:"onSelf,omitempty"`
	TargetAC   *int   `json:"targetAc,omitempty"`
	ItemID     string `json:"itemId,omitempty"`
	ExtraSlots []int  `json:"extraSlots,omitempty"`
	Mode       string `json:"mode,omitempty"`
}

// asks is the domain's answers of a request.
func (b UseAbilityRequest) asks() itemsvc.UseAsks {
	var out itemsvc.UseAsks
	for _, a := range b.Spells {
		out.Spells = append(out.Spells, itemsvc.SpellAsks{
			Effect: a.Effect, OnSelf: a.OnSelf, TargetAC: a.TargetAC, ItemID: a.ItemID, ExtraSlots: a.ExtraSlots, Mode: a.Mode,
		})
	}
	return out
}

// UseAbility uses one ability of an item: pays its limit, runs its effects,
// logs the use (ADR 0049 step 7). The body is optional.
func UseAbility(charID CharacterIDFunc, svc itemSvc) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var req UseAbilityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			return render.BadRequestError("invalid JSON body")
		}
		res, err := svc.UseAbility(r.Context(), charID(r), chi.URLParam(r, "id"), chi.URLParam(r, "abilityId"), req.asks())
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, AbilityUseResultDTO{
			ItemID: res.ItemID, AbilityID: res.AbilityID, Name: res.Name, AbilityName: res.AbilityName, Type: res.Type,
			Limit: string(res.Limit), Left: res.Left, Max: res.Max, Remaining: res.Remaining, Removed: res.Removed,
			Effects: res.Effects, Notes: res.Notes, LogEntryID: res.LogEntryID,
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

// List returns every piece with its current count, its review reasons
// (ADR 0033 step 27), its class warnings (step 20) and, for each spell effect of
// its abilities, what the use of it asks of the player (ADR 0049 steps 9 and 10).
func List(charID CharacterIDFunc, svc itemSvc, asker SpellAsker) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		items, err := svc.List(r.Context(), charID(r))
		if err != nil {
			return err
		}
		reviews, err := svc.Reviews(r.Context(), charID(r))
		if err != nil {
			return err
		}
		warnings, err := svc.Warnings(r.Context(), charID(r), items)
		if err != nil {
			return err
		}
		out := wire.ItemsToDTO(items, warnings)
		for i := range out {
			if err := serveAsks(&out[i], items, asker); err != nil {
				return err
			}
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

// serveAsks fills the asks of each spell effect of item's abilities: the spell's cast inputs at the
// effect's level and the items of the inventory it can spend. The server decides, the web only
// collects the answers.
func serveAsks(item *wire.ItemDTO, inventory []itemsvc.Item, asker SpellAsker) error {
	if item.Abilities == nil || asker == nil {
		return nil
	}
	for a := range *item.Abilities {
		effects := (*item.Abilities)[a].Effects
		for e := range effects {
			if effects[e].Kind != string(itemsvc.EffectSpell) {
				continue
			}
			asks, err := asker.AsksFor(effects[e].SpellID, effects[e].Level, inventory)
			if err != nil {
				return err
			}
			effects[e].Asks = wire.SpellAsksToDTO(asks)
		}
	}
	return nil
}

// serveAllAsks is serveAsks for items answered by a change or a state move: the inventory the asks
// read (the items a spell can spend) is the character's list, read only when an item holds a spell
// effect.
func serveAllAsks(ctx context.Context, svc itemSvc, characterID int64, asker SpellAsker, out []wire.ItemDTO) error {
	var inventory []itemsvc.Item
	read := false
	for i := range out {
		if !holdsSpellEffect(out[i]) {
			continue
		}
		if !read {
			var err error
			if inventory, err = svc.List(ctx, characterID); err != nil {
				return err
			}
			read = true
		}
		if err := serveAsks(&out[i], inventory, asker); err != nil {
			return err
		}
	}
	return nil
}

// holdsSpellEffect reports whether an ability of the item has a spell effect.
func holdsSpellEffect(item wire.ItemDTO) bool {
	if item.Abilities == nil {
		return false
	}
	for _, a := range *item.Abilities {
		for _, e := range a.Effects {
			if e.Kind == string(itemsvc.EffectSpell) {
				return true
			}
		}
	}
	return false
}

// ChangeDTO is the wire shape of POST/PATCH/DELETE /api/items: the item as it
// is now (as it was, for a delete) and the log entry written.
type ChangeDTO struct {
	Item       wire.ItemDTO `json:"item"`
	LogEntryID int64        `json:"logEntryId"`
}

// ItemBody is the body of POST and PATCH /api/items. On a PATCH only the
// fields present change. maxCount is the daily number (daily items only).
// The fields of the piece's category count; the others are stored as sent
// (ADR 0033 step 1). On a POST, a catalogueCode picks the piece from that
// catalogue row (ADR 0033 step 24): only name, count, notes and the magic
// (magicToHit and magicDamage, magicalBonus) are read with it; equip equips
// the new piece in the same undo. A PATCH ignores both.
type ItemBody struct {
	CatalogueCode *string `json:"catalogueCode"`
	Equip         *bool   `json:"equip"`
	// Name is a language map: the languages typed (ADR 0023 step 11).
	Name      localized.Text `json:"name"`
	Category  *string        `json:"category"`
	MagicKind *string        `json:"magicKind"`
	Component *string        `json:"component"`
	// SpellID is the spell a magic scroll holds (ADR 0033 step 2); "" clears it.
	SpellID      *string   `json:"spellId"`
	Behaviour    *string   `json:"behaviour"`
	Count        *int      `json:"count"`
	MaxCount     *int      `json:"maxCount"`
	MagicalBonus *int      `json:"magicalBonus"`
	AmmoKind     *string   `json:"ammoKind"`
	EffectDice   *string   `json:"effectDice"`
	EffectSave   *string   `json:"effectSave"`
	Notes        *string   `json:"notes"`
	DamageTypes  *[]string `json:"damageTypes"`
	DamageSM     *string   `json:"damageSm"`
	DamageL      *string   `json:"damageL"`
	WeaponType   *string   `json:"weaponType"`
	WeaponKind   *string   `json:"weaponKind"`
	Size         *string   `json:"size"`
	Speed        *int      `json:"speed"`
	MagicToHit   *int      `json:"magicToHit"`
	MagicDamage  *int      `json:"magicDamage"`
	TwoHanded    *bool     `json:"twoHanded"`
	Fires        *string   `json:"fires"`
	// StrengthRating is a bow's Strength rating (ADR 0033 step 3): an
	// object sets it, null clears it, absent keeps it (a PATCH).
	StrengthRating ratingField `json:"strengthRating"`
	BaseAC         *int        `json:"baseAc"`
	SkillColumn    *string     `json:"skillColumn"`
	ArmorRow       *string     `json:"armorRow"`
	ShieldKind     *string     `json:"shieldKind"`
	ACMelee        *int        `json:"acMelee"`
	ACMissile      *int        `json:"acMissile"`
	// Powers is the whole power list, as served (ADR 0033 step 17, amending
	// ADR 0016 step 15): on a PATCH it replaces the stored one, absent keeps
	// it. A power sent without an id gets power_<n>. An armour's or a
	// shield's "+N" is the power of id "magic", one effect ac -N.
	Powers *[]wire.PowerDTO `json:"powers"`
	// Abilities is the whole ability list, as served (ADR 0049 step 4): on a
	// PATCH it replaces the stored one, absent keeps it. An ability sent
	// without an id gets ability_<n>. absent on a POST: not converted yet.
	Abilities *[]wire.AbilityDTO `json:"abilities"`
	// Accepted and ignored, so a piece served back as is still decodes: the
	// type label is no longer edited (ADR 0033 step 1), the state moves
	// through POST /api/items/{id}/state, the review is not the editor's.
	Type       json.RawMessage `json:"type"`
	State      json.RawMessage `json:"state"`
	EquipState json.RawMessage `json:"equipState"`
	Review     json.RawMessage `json:"review"`
	// Warnings are accepted and ignored like the review: a piece served back as is still decodes.
	Warnings json.RawMessage `json:"warnings"`
	// Specialized is accepted and ignored (ADR 0037 step 9): a weapon is specialized by its
	// type's pick (PUT /api/proficiencies), and a piece served back, or the editor of the release
	// before, still decodes.
	Specialized json.RawMessage `json:"specialized"`
}

// ratingField is the body's strengthRating: absent leaves the rating as it
// is, JSON null clears it, an object sets it.
type ratingField struct {
	present bool
	val     *wire.StrengthRatingDTO
}

func (f *ratingField) UnmarshalJSON(b []byte) error {
	f.present = true
	return json.Unmarshal(b, &f.val)
}

// rating is the rating sent; the zero value (none) for null or absent.
func (f ratingField) rating() itemsvc.StrengthRating {
	if f.val == nil {
		return itemsvc.StrengthRating{}
	}
	return itemsvc.StrengthRating{Score: f.val.Score, Exceptional: f.val.Exceptional}
}

// patch is the rating to apply: nil when absent, the zero value (a clear)
// for null, else the rating.
func (f ratingField) patch() *itemsvc.StrengthRating {
	if !f.present {
		return nil
	}
	r := f.rating()
	return &r
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

// change answers a create, edit or delete with the piece and its class warnings (step 20), which
// depend on the other pieces too (a shield and a two-handed weapon), so the character's pieces are
// read once for them.
func change(r *http.Request, w http.ResponseWriter, svc itemSvc, asker SpellAsker, charID int64, res *itemsvc.ChangeResult) error {
	warnings, err := svc.Warnings(r.Context(), charID, nil)
	if err != nil {
		return err
	}
	out := wire.ItemsToDTO([]itemsvc.Item{res.Item}, warnings)
	if err := serveAllAsks(r.Context(), svc, charID, asker, out); err != nil {
		return err
	}
	render.OK(r.Context(), w, ChangeDTO{Item: out[0], LogEntryID: res.LogEntryID})
	return nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// powers is the domain list of a body's powers (none: nil).
func powers(dtos *[]wire.PowerDTO) []itemsvc.Power {
	if dtos == nil {
		return nil
	}
	return wire.PowersFromDTO(*dtos)
}

// abilities is the domain list of a body's abilities (absent or null: nil).
func abilities(dtos *[]wire.AbilityDTO) []itemsvc.Ability {
	if dtos == nil {
		return nil
	}
	return wire.AbilitiesFromDTO(*dtos)
}

// Create adds an item and logs it.
func Create(charID CharacterIDFunc, svc itemSvc, asker SpellAsker) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		b, err := decodeBody(r)
		if err != nil {
			return err
		}
		res, err := svc.Create(r.Context(), charID(r), itemsvc.ItemInput{
			CatalogueCode: deref(b.CatalogueCode), Equip: deref(b.Equip),
			Name: b.Name, Category: itemsvc.Category(deref(b.Category)), MagicKind: deref(b.MagicKind),
			Component: deref(b.Component), SpellID: deref(b.SpellID), Behaviour: itemsvc.Behaviour(deref(b.Behaviour)),
			Count: deref(b.Count), MaxCount: deref(b.MaxCount), MagicBonus: deref(b.MagicalBonus),
			AmmoKind: deref(b.AmmoKind), EffectDice: deref(b.EffectDice), EffectSave: deref(b.EffectSave),
			Notes: deref(b.Notes), DamageTypes: deref(b.DamageTypes), DamageSM: deref(b.DamageSM),
			DamageL: deref(b.DamageL), WeaponType: deref(b.WeaponType), WeaponKind: deref(b.WeaponKind),
			Size: deref(b.Size), Speed: deref(b.Speed), MagicToHit: deref(b.MagicToHit),
			MagicDamage: deref(b.MagicDamage), TwoHanded: deref(b.TwoHanded), Fires: deref(b.Fires),
			StrengthRating: b.StrengthRating.rating(),
			BaseAC:         deref(b.BaseAC), SkillColumn: deref(b.SkillColumn),
			ArmorRow: deref(b.ArmorRow), ShieldKind: deref(b.ShieldKind), ACMelee: deref(b.ACMelee),
			ACMissile: deref(b.ACMissile), Powers: powers(b.Powers), Abilities: abilities(b.Abilities),
		})
		if err != nil {
			return err
		}
		return change(r, w, svc, asker, charID(r), res)
	}
}

// Update edits the fields present in the body and logs the change.
func Update(charID CharacterIDFunc, svc itemSvc, asker SpellAsker) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		b, err := decodeBody(r)
		if err != nil {
			return err
		}
		p := itemsvc.ItemPatch{
			Name: b.Name, MagicKind: b.MagicKind, Component: b.Component, SpellID: b.SpellID, Count: b.Count, MaxCount: b.MaxCount, MagicBonus: b.MagicalBonus,
			AmmoKind: b.AmmoKind, EffectDice: b.EffectDice, EffectSave: b.EffectSave, Notes: b.Notes,
			DamageTypes: b.DamageTypes, DamageSM: b.DamageSM, DamageL: b.DamageL, WeaponType: b.WeaponType,
			WeaponKind: b.WeaponKind, Size: b.Size, Speed: b.Speed, MagicToHit: b.MagicToHit,
			MagicDamage: b.MagicDamage, TwoHanded: b.TwoHanded, Fires: b.Fires,
			StrengthRating: b.StrengthRating.patch(),
			BaseAC:         b.BaseAC, SkillColumn: b.SkillColumn, ArmorRow: b.ArmorRow, ShieldKind: b.ShieldKind,
			ACMelee: b.ACMelee, ACMissile: b.ACMissile,
		}
		if b.Powers != nil {
			ps := powers(b.Powers)
			p.Powers = &ps
		}
		if b.Abilities != nil {
			as := abilities(b.Abilities)
			p.Abilities = &as
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
		return change(r, w, svc, asker, charID(r), res)
	}
}

// Delete removes an item and logs it.
func Delete(charID CharacterIDFunc, svc itemSvc, asker SpellAsker) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		res, err := svc.Delete(r.Context(), charID(r), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		return change(r, w, svc, asker, charID(r), res)
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
	Items      []wire.ItemDTO `json:"items"`
	LogEntryID int64          `json:"logEntryId"`
}

// SetState wears, takes off, wields or puts away a piece (ADR 0033 steps 6
// and 7).
func SetState(charID CharacterIDFunc, svc itemSvc, asker SpellAsker) render.HandlerFunc {
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
		warnings, err := svc.Warnings(r.Context(), charID(r), nil)
		if err != nil {
			return err
		}
		out := wire.ItemsToDTO(res.Items, warnings)
		if err := serveAllAsks(r.Context(), svc, charID(r), asker, out); err != nil {
			return err
		}
		render.OK(r.Context(), w, StateDTO{Items: out, LogEntryID: res.LogEntryID})
		return nil
	}
}

// DismissReview takes a piece off the review list (ADR 0033 step 27), not
// logged, and answers the list as GET /api/items does.
func DismissReview(charID CharacterIDFunc, svc itemSvc, asker SpellAsker) render.HandlerFunc {
	list := List(charID, svc, asker)
	return func(w http.ResponseWriter, r *http.Request) error {
		if err := svc.DismissReview(r.Context(), charID(r), chi.URLParam(r, "id")); err != nil {
			return err
		}
		return list(w, r)
	}
}
