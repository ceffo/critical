package weaponsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// StateKeyWeapons is the state-key family of weapons. Each weapon has its
// own key, StateKey(id) = "weapons:<id>".
const StateKeyWeapons = "weapons"

// StateKey is the action-log state key of one weapon.
func StateKey(id string) string { return StateKeyWeapons + actionlog.KeySeparator + id }

// snapWeapon is the JSON encoding of one weapon inside a snapshot. Entries
// already in the log must stay decodable: add fields, never rename. Absent
// marks a "before" in which the weapon did not exist (restoring deletes it),
// otherwise the weapon is the whole truth: restoring overwrites every field,
// or inserts the weapon when it is gone.
type snapWeapon struct {
	ID          string `json:"id"`
	Absent      bool   `json:"absent,omitempty"`
	Position    int    `json:"position"`
	Name        string `json:"name"`
	DamageSM    string `json:"damageSm"`
	DamageL     string `json:"damageL"`
	MagicToHit  int    `json:"magicToHit"`
	MagicDamage int    `json:"magicDamage"`
	Kind        string `json:"kind"`
	TwoHanded   bool   `json:"twoHanded"`
	AmmoKind    string `json:"ammoKind"`
}

func toSnap(id string, w *Weapon) snapWeapon {
	if w == nil {
		return snapWeapon{ID: id, Absent: true}
	}
	return snapWeapon{
		ID: w.ID, Position: w.Position, Name: w.Name, DamageSM: w.DamageSM, DamageL: w.DamageL,
		MagicToHit: w.MagicToHit, MagicDamage: w.MagicDamage, Kind: w.Kind, TwoHanded: w.TwoHanded,
		AmmoKind: w.AmmoKind,
	}
}

func (c snapWeapon) weapon() Weapon {
	return Weapon{
		ID: c.ID, Position: c.Position, Name: c.Name, DamageSM: c.DamageSM, DamageL: c.DamageL,
		MagicToHit: c.MagicToHit, MagicDamage: c.MagicDamage, Kind: c.Kind, TwoHanded: c.TwoHanded,
		AmmoKind: c.AmmoKind,
	}
}

// weaponSnapshot is the snapshot of a weapon going from before to after by
// hand (nil = it did not exist / no longer exists).
func weaponSnapshot(id string, before, after *Weapon) (actionlog.Snapshot, error) {
	b, err := json.Marshal(toSnap(id, before))
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode weapons snapshot: %w", err)
	}
	a, err := json.Marshal(toSnap(id, after))
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode weapons snapshot: %w", err)
	}
	return actionlog.Snapshot{Key: StateKey(id), Before: b, After: a}, nil
}

type restoreFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restoreFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

// Restorers writes a weapons snapshot back (registered for the
// StateKeyWeapons family). It runs inside the void's transaction.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKeyWeapons: restoreFunc(func(ctx context.Context, _ int64, state json.RawMessage) error {
			var c snapWeapon
			if err := json.Unmarshal(state, &c); err != nil {
				return fmt.Errorf("decode weapons snapshot: %w", err)
			}
			if c.ID == "" {
				return fmt.Errorf("decode weapons snapshot: missing id")
			}
			var err error
			if c.Absent {
				err = s.dal.Delete(ctx, c.ID)
			} else {
				err = s.dal.Update(ctx, c.weapon())
				if errors.Is(err, ErrWeaponNotFound) {
					err = s.dal.Insert(ctx, c.weapon())
				}
			}
			if err != nil {
				return fmt.Errorf("restore weapon %s: %w", c.ID, err)
			}
			return nil
		}),
	}
}
