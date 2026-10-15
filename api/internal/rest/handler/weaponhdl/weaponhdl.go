// Package weaponhdl exposes the weapon editor: POST /api/weapons, PATCH and
// DELETE /api/weapons/{id}. The list (GET /api/weapons) is combathdl's.
package weaponhdl

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ceffo/critical/api/internal/domain/service/weaponsvc"
	"github.com/ceffo/critical/api/internal/render"
)

// Router is the chi subset we need.
type Router interface {
	Post(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

// CharacterIDFunc resolves the active character.
type CharacterIDFunc func(r *http.Request) int64

// RegisterRoutes wires the weapon editor endpoints onto r.
func RegisterRoutes(r Router, charID CharacterIDFunc, svc weaponsvc.SVC) {
	r.Post("/api/weapons", render.Handle(Create(charID, svc)))
	r.Patch("/api/weapons/{id}", render.Handle(Update(charID, svc)))
	r.Delete("/api/weapons/{id}", render.Handle(Delete(charID, svc)))
}

// WeaponDTO is the wire shape of a weapon, the same as GET /api/weapons.
type WeaponDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DamageSM    string `json:"damage_sm"`
	DamageL     string `json:"damage_l"`
	MagicToHit  int    `json:"magic_to_hit"`
	MagicDamage int    `json:"magic_damage"`
	Kind        string `json:"kind"`
	TwoHanded   bool   `json:"two_handed"`
	AmmoKind    string `json:"ammo_kind"`
}

// ChangeDTO is the wire shape of POST/PATCH/DELETE /api/weapons: the weapon
// as it is now (as it was, for a delete) and the log entry written.
type ChangeDTO struct {
	Weapon     WeaponDTO `json:"weapon"`
	LogEntryID int64     `json:"logEntryId"`
}

// WeaponBody is the body of POST and PATCH. On a PATCH only the present
// fields change.
type WeaponBody struct {
	Name        *string `json:"name"`
	DamageSM    *string `json:"damage_sm"`
	DamageL     *string `json:"damage_l"`
	MagicToHit  *int    `json:"magic_to_hit"`
	MagicDamage *int    `json:"magic_damage"`
	Kind        *string `json:"kind"`
	TwoHanded   *bool   `json:"two_handed"`
	AmmoKind    *string `json:"ammo_kind"`
}

func decodeBody(r *http.Request) (WeaponBody, error) {
	var b WeaponBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return b, render.BadRequestError("invalid JSON body")
	}
	return b, nil
}

func change(w http.ResponseWriter, r *http.Request, res *weaponsvc.ChangeResult) {
	x := res.Weapon
	render.OK(r.Context(), w, ChangeDTO{
		Weapon: WeaponDTO{
			ID: x.ID, Name: x.Name, DamageSM: x.DamageSM, DamageL: x.DamageL, MagicToHit: x.MagicToHit,
			MagicDamage: x.MagicDamage, Kind: x.Kind, TwoHanded: x.TwoHanded, AmmoKind: x.AmmoKind,
		},
		LogEntryID: res.LogEntryID,
	})
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// Create adds a weapon and logs it.
func Create(charID CharacterIDFunc, svc weaponsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		b, err := decodeBody(r)
		if err != nil {
			return err
		}
		res, err := svc.Create(r.Context(), charID(r), weaponsvc.WeaponInput{
			Name: deref(b.Name), DamageSM: deref(b.DamageSM), DamageL: deref(b.DamageL),
			MagicToHit: deref(b.MagicToHit), MagicDamage: deref(b.MagicDamage), Kind: deref(b.Kind),
			TwoHanded: deref(b.TwoHanded), AmmoKind: deref(b.AmmoKind),
		})
		if err != nil {
			return err
		}
		change(w, r, res)
		return nil
	}
}

// Update edits the fields present in the body and logs the change.
func Update(charID CharacterIDFunc, svc weaponsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		b, err := decodeBody(r)
		if err != nil {
			return err
		}
		res, err := svc.Update(r.Context(), charID(r), chi.URLParam(r, "id"), weaponsvc.WeaponPatch{
			Name: b.Name, DamageSM: b.DamageSM, DamageL: b.DamageL, MagicToHit: b.MagicToHit,
			MagicDamage: b.MagicDamage, Kind: b.Kind, TwoHanded: b.TwoHanded, AmmoKind: b.AmmoKind,
		})
		if err != nil {
			return err
		}
		change(w, r, res)
		return nil
	}
}

// Delete removes a weapon and logs it.
func Delete(charID CharacterIDFunc, svc weaponsvc.SVC) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		res, err := svc.Delete(r.Context(), charID(r), chi.URLParam(r, "id"))
		if err != nil {
			return err
		}
		change(w, r, res)
		return nil
	}
}

var _ Router = (*chi.Mux)(nil)
