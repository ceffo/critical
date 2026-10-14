package weaponsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// FieldChange is one edited field in a weapon_edit payload (values as text).
type FieldChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// changeRecord is the weapon_add / weapon_edit / weapon_remove log payload
// (snake_case, the /api/weapons wire names). Changes only in weapon_edit.
// Add fields, never rename.
type changeRecord struct {
	WeaponID    string        `json:"weapon_id"`
	Name        string        `json:"name"`
	DamageSM    string        `json:"damage_sm"`
	DamageL     string        `json:"damage_l"`
	MagicToHit  int           `json:"magic_to_hit"`
	MagicDamage int           `json:"magic_damage"`
	Kind        string        `json:"kind"`
	TwoHanded   bool          `json:"two_handed"`
	AmmoKind    string        `json:"ammo_kind"`
	Changes     []FieldChange `json:"changes,omitempty"`
}

func record(w Weapon, changes []FieldChange) changeRecord {
	return changeRecord{
		WeaponID: w.ID, Name: w.Name, DamageSM: w.DamageSM, DamageL: w.DamageL,
		MagicToHit: w.MagicToHit, MagicDamage: w.MagicDamage, Kind: w.Kind,
		TwoHanded: w.TwoHanded, AmmoKind: w.AmmoKind, Changes: changes,
	}
}

// slug turns a name into an id: lower-case letters and digits, "_" between.
func slug(name string) string {
	var b strings.Builder
	sep := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if sep && b.Len() > 0 {
				b.WriteByte('_')
			}
			sep = false
			b.WriteRune(r)
		} else {
			sep = true
		}
	}
	if b.Len() == 0 {
		return "weapon"
	}
	return b.String()
}

func (s *service) logChange(ctx context.Context, characterID int64, kind actionlog.Kind, summary string, rec changeRecord, before, after *Weapon) (int64, error) {
	snap, err := weaponSnapshot(rec.WeaponID, before, after)
	if err != nil {
		return 0, err
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return 0, fmt.Errorf("encode %s payload: %w", kind, err)
	}
	id, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: kind, Summary: summary, Payload: payload,
		Snapshots: []actionlog.Snapshot{snap},
	})
	if err != nil {
		return 0, fmt.Errorf("record %s: %w", kind, err)
	}
	return id, nil
}

func clean(w Weapon) Weapon {
	w.Name = strings.TrimSpace(w.Name)
	w.DamageSM = strings.ToLower(strings.TrimSpace(w.DamageSM))
	w.DamageL = strings.ToLower(strings.TrimSpace(w.DamageL))
	w.Kind = strings.TrimSpace(w.Kind)
	w.AmmoKind = strings.ToLower(strings.TrimSpace(w.AmmoKind))
	return w
}

func (s *service) Create(ctx context.Context, characterID int64, in WeaponInput) (*ChangeResult, error) {
	w := clean(Weapon{
		Name: in.Name, DamageSM: in.DamageSM, DamageL: in.DamageL, MagicToHit: in.MagicToHit,
		MagicDamage: in.MagicDamage, Kind: in.Kind, TwoHanded: in.TwoHanded, AmmoKind: in.AmmoKind,
	})
	if err := w.Validate(); err != nil {
		return nil, err
	}
	var res *ChangeResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		all, err := s.dal.List(ctx)
		if err != nil {
			return err
		}
		taken, pos := map[string]bool{}, 0
		for _, o := range all {
			taken[o.ID] = true
			if o.Position >= pos {
				pos = o.Position + 1
			}
		}
		base := slug(w.Name)
		w.ID = base
		for n := 2; taken[w.ID]; n++ {
			w.ID = fmt.Sprintf("%s_%d", base, n)
		}
		w.Position = pos
		if err := s.dal.Insert(ctx, w); err != nil {
			return err
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindWeaponAdd,
			fmt.Sprintf("Added weapon %s", w.Name), record(w, nil), nil, &w)
		if err != nil {
			return err
		}
		res = &ChangeResult{Weapon: w, LogEntryID: logID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *service) Update(ctx context.Context, characterID int64, id string, p WeaponPatch) (*ChangeResult, error) {
	var res *ChangeResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.dal.Get(ctx, id)
		if err != nil {
			return err
		}
		w := old
		if p.Name != nil {
			w.Name = *p.Name
		}
		if p.DamageSM != nil {
			w.DamageSM = *p.DamageSM
		}
		if p.DamageL != nil {
			w.DamageL = *p.DamageL
		}
		if p.MagicToHit != nil {
			w.MagicToHit = *p.MagicToHit
		}
		if p.MagicDamage != nil {
			w.MagicDamage = *p.MagicDamage
		}
		if p.Kind != nil {
			w.Kind = *p.Kind
		}
		if p.TwoHanded != nil {
			w.TwoHanded = *p.TwoHanded
		}
		if p.AmmoKind != nil {
			w.AmmoKind = *p.AmmoKind
		}
		w = clean(w)
		// Switching to melee drops the ammo kind unless the edit sets one
		// (which Validate then refuses).
		if p.Kind != nil && w.Kind == KindMelee && p.AmmoKind == nil {
			w.AmmoKind = ""
		}
		if err := w.Validate(); err != nil {
			return err
		}
		changes := diff(old, w)
		if len(changes) == 0 {
			return fmt.Errorf("%w: nothing changed", domain.ErrInvalidInput)
		}
		if err := s.dal.Update(ctx, w); err != nil {
			return err
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindWeaponEdit,
			fmt.Sprintf("Edited weapon %s: %s", w.Name, describe(changes)), record(w, changes), &old, &w)
		if err != nil {
			return err
		}
		res = &ChangeResult{Weapon: w, LogEntryID: logID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *service) Delete(ctx context.Context, characterID int64, id string) (*ChangeResult, error) {
	var res *ChangeResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.dal.Get(ctx, id)
		if err != nil {
			return err
		}
		if err := s.dal.Delete(ctx, id); err != nil {
			return err
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindWeaponRemove,
			fmt.Sprintf("Removed weapon %s", old.Name), record(old, nil), &old, nil)
		if err != nil {
			return err
		}
		res = &ChangeResult{Weapon: old, LogEntryID: logID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// diff lists the fields that differ, in form order.
func diff(a, b Weapon) []FieldChange {
	var out []FieldChange
	add := func(field, x, y string) {
		if x != y {
			out = append(out, FieldChange{Field: field, Before: x, After: y})
		}
	}
	add("name", a.Name, b.Name)
	add("damage_sm", a.DamageSM, b.DamageSM)
	add("damage_l", a.DamageL, b.DamageL)
	add("magic_to_hit", fmt.Sprint(a.MagicToHit), fmt.Sprint(b.MagicToHit))
	add("magic_damage", fmt.Sprint(a.MagicDamage), fmt.Sprint(b.MagicDamage))
	add("kind", a.Kind, b.Kind)
	add("two_handed", fmt.Sprint(a.TwoHanded), fmt.Sprint(b.TwoHanded))
	add("ammo_kind", a.AmmoKind, b.AmmoKind)
	return out
}

func describe(changes []FieldChange) string {
	parts := make([]string, len(changes))
	for i, c := range changes {
		parts[i] = fmt.Sprintf("%s %s → %s", c.Field, c.Before, c.After)
	}
	return strings.Join(parts, ", ")
}
