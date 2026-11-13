package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// weaponCodesMigration documents characters.proficiency_picks with weapon picks by catalogue
// code. Right after it the run turns each type pick into the character's owned weapons of the type
// (pickOwnedWeapons, ADR 0037 step 21.8).
const weaponCodesMigration = "042_weapon_picks_by_code.sql"

// weaponRedoKey is the column's key of the type picks left to redo (ADR 0037 step 21.8.3), as
// characterdal stores it.
const weaponRedoKey = "weapon_redo"

// redoPick is one type pick left to redo, as the column stores it.
type redoPick struct {
	Type  string `json:"type"`
	Slots int    `json:"slots"`
}

// pickOwnedWeapons is ADR 0037 step 21.8, the data step after migration 042. For each character
// and each stored type pick, the owned weapons of the type are its weapon items, in any state,
// whose catalogue code names a row of catalogue of the pick's type, in list order:
//  1. a plain type pick becomes one plain pick per distinct code among them, after any code pick
//     already there, never a duplicate;
//  2. a specialization (a type pick spending more than a plain one) makes the first owned weapon
//     the specialization, at the same slots, the other codes plain picks;
//  3. a type pick with no owned weapon of the type (none, or only pieces without a code) gives no
//     pick: its type and slots are listed under weapon_redo for the player to redo, its slots back
//     to spend;
//  4. a type pick whose type is no weapon type is listed to redo the same way;
//  5. nothing is refused and the class lists and the budget are not checked (a code the class may
//     not pick is kept, served allowed false; one type turned into several picks may leave the
//     character over budget, warned). Each changed character's result is logged;
//  6. the action log is not rewritten, and the item specialization flags are not read again.
//
// A character whose weapon picks are already codes (or who has none) is left as it is, so a
// rerun changes nothing. No catalogue equipment (before format 7) owns no weapon: every type pick
// is listed to redo.
func pickOwnedWeapons(ctx context.Context, q storeq.Querier, catalogue []ruletablesvc.Weapon) error {
	typeOf := make(map[string]string, len(catalogue))
	for _, w := range catalogue {
		typeOf[w.Code] = w.WeaponType
	}
	rows, err := q.CharacterWeaponCodes(ctx)
	if err != nil {
		return fmt.Errorf("list weapon codes: %w", err)
	}
	owned := map[int64][]string{} // each character's coded weapons, in list order, each code once
	for _, r := range rows {
		if !slices.Contains(owned[r.CharacterID], r.CatalogueCode) {
			owned[r.CharacterID] = append(owned[r.CharacterID], r.CatalogueCode)
		}
	}
	chars, err := q.CharacterPicks(ctx)
	if err != nil {
		return fmt.Errorf("list picks: %w", err)
	}
	for _, c := range chars {
		picks, err := decodeTypePicks(c.ProficiencyPicks)
		if err != nil {
			return fmt.Errorf("character %d: %w", c.ID, err)
		}
		if !slices.ContainsFunc(picks.Weapon, func(p typePick) bool { return p.Type != "" }) {
			continue
		}
		redo := expandTypePicks(&picks, owned[c.ID], typeOf)
		if err := picks.addRedo(redo); err != nil {
			return fmt.Errorf("character %d: %w", c.ID, err)
		}
		raw, err := picks.encode()
		if err != nil {
			return err
		}
		if err := q.SetCharacterPicks(ctx, storeq.SetCharacterPicksParams{ProficiencyPicks: raw, ID: c.ID}); err != nil {
			return fmt.Errorf("character %d picks: %w", c.ID, err)
		}
		codes := make([]string, 0, len(picks.Weapon))
		for _, p := range picks.Weapon {
			codes = append(codes, p.Code)
		}
		slog.Info("migration 042: weapon type picks turned into the owned weapons' codes",
			"character", c.ID, "codes", codes, "to_redo", redo)
	}
	return nil
}

// expandTypePicks replaces the type picks of picks by picks of the owned codes of their types
// (steps 21.8.1 and 2), the code picks already there first, and returns the type picks left to
// redo (steps 21.8.3 and 4), in order.
func expandTypePicks(picks *typePicks, owned []string, typeOf map[string]string) []redoPick {
	out := make([]typePick, 0, len(picks.Weapon))
	for _, p := range picks.Weapon {
		if p.Type == "" {
			out = append(out, p)
		}
	}
	index := func(code string) int { return slices.IndexFunc(out, func(p typePick) bool { return p.Code == code }) }
	var redo []redoPick
	for _, p := range picks.Weapon {
		if p.Type == "" {
			continue
		}
		var codes []string
		if slices.Contains(ruletablesvc.WeaponTypes, p.Type) {
			for _, code := range owned {
				if typeOf[code] == p.Type {
					codes = append(codes, code)
				}
			}
		}
		if len(codes) == 0 {
			redo = append(redo, redoPick{Type: p.Type, Slots: p.Slots})
			continue
		}
		for i, code := range codes {
			slots := skills.WeaponPickSlots
			if i == 0 {
				slots = p.Slots // the specialization goes to the first owned weapon
			}
			if j := index(code); j >= 0 {
				out[j].Slots = max(out[j].Slots, slots)
				continue
			}
			out = append(out, typePick{Code: code, Slots: slots})
		}
	}
	picks.Weapon = out
	return redo
}

// addRedo appends redo to the column's list of type picks left to redo (kept when there is none).
func (p *typePicks) addRedo(redo []redoPick) error {
	if len(redo) == 0 {
		return nil
	}
	var list []redoPick
	if raw, ok := p.other[weaponRedoKey]; ok {
		if err := json.Unmarshal(raw, &list); err != nil {
			return fmt.Errorf("decode weapon redo: %w", err)
		}
	}
	raw, err := json.Marshal(append(list, redo...))
	if err != nil {
		return fmt.Errorf("encode weapon redo: %w", err)
	}
	p.other[weaponRedoKey] = raw
	return nil
}
