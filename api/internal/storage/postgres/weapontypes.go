package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// catalogueWeaponTypesMigration is the migration whose data step gives the stored catalogue
// weapons the type their catalogue row now names (retypeCatalogueWeapons), once, right after it.
const catalogueWeaponTypesMigration = "041_catalogue_weapon_types.sql"

// retypedFrom is the catch-all type a catalogue row named before it got a type of its own (ADR
// 0030 step 23: the staff sling, other_missile until catalogue format 22).
const retypedFrom = "other_missile"

// typesOfFormat22 are the weapon types catalogue format 22 added (ADR 0021 step 30): the rows of
// these types are the ones that were other_missile before.
var typesOfFormat22 = []string{"staff_sling"}

// retypeCatalogueWeapons is the data step after migration 041 (ADR 0030 step 23): every weapon
// piece of the code of a catalogue row of a type format 22 added, still typed other_missile, takes
// the row's type, as a pick of the row would type it now (ADR 0033 step 23). It reads the
// catalogue's rows, never a weapon code: a piece the player typed otherwise, a homebrew piece, the
// blowgun (still other_missile) and a piece of an older type the player set to other_missile are
// left as they are. The owner of a retyped piece keeps its proficiency (carryRetypedPicks), as
// migration 026 did (ADR 0037 step 16.2): no attack changes. A second run changes nothing. No
// catalogue equipment (before format 7) changes nothing.
func retypeCatalogueWeapons(ctx context.Context, q storeq.Querier, catalogue []ruletablesvc.Weapon) error {
	retyped := map[int64][]string{} // the owner of a retyped piece: the types its pieces took
	for _, w := range catalogue {
		if !slices.Contains(typesOfFormat22, w.WeaponType) {
			continue
		}
		owners, err := q.RetypeCatalogueWeapons(ctx, storeq.RetypeCatalogueWeaponsParams{
			ToType: w.WeaponType, CatalogueCode: w.Code, FromType: retypedFrom,
		})
		if err != nil {
			return fmt.Errorf("retype the %s pieces: %w", w.Code, err)
		}
		for _, id := range owners {
			if !slices.Contains(retyped[id], w.WeaponType) {
				retyped[id] = append(retyped[id], w.WeaponType)
			}
		}
	}
	if len(retyped) == 0 {
		return nil
	}
	return carryRetypedPicks(ctx, q, retyped)
}

// carryRetypedPicks gives each owner of a retyped piece the weapon pick of the piece's new type
// when it had the other_missile pick that covered the piece (ADR 0030 step 23): the pick moves,
// slots and all (a specialization stays one: other_missile and staff_sling cost the same), when no
// piece of the owner is still other_missile; otherwise the other_missile pick stays for those
// pieces and the new type gets a plain pick (1 slot), as migration 026 granted it. An owner
// without the other_missile pick was not proficient and stays so; one that already has the new
// type's pick keeps its picks. Nothing is refused: an extra slot shows the slots_over_budget
// warning (ADR 0037 step 16.4).
func carryRetypedPicks(ctx context.Context, q storeq.Querier, retyped map[int64][]string) error {
	rows, err := q.CharacterWeaponTypes(ctx)
	if err != nil {
		return fmt.Errorf("list weapon types: %w", err)
	}
	keepsFrom := map[int64]bool{} // the owner still has a piece typed other_missile
	for _, r := range rows {
		if r.WeaponType == retypedFrom {
			keepsFrom[r.CharacterID] = true
		}
	}
	chars, err := q.CharacterPicks(ctx)
	if err != nil {
		return fmt.Errorf("list picks: %w", err)
	}
	for _, c := range chars {
		types := retyped[c.ID]
		if len(types) == 0 {
			continue
		}
		picks, err := characterdal.DecodePicks(c.ProficiencyPicks)
		if err != nil {
			return fmt.Errorf("character %d: %w", c.ID, err)
		}
		if !carryPick(&picks, types, keepsFrom[c.ID], c.ID) {
			continue
		}
		raw, err := characterdal.EncodePicks(picks)
		if err != nil {
			return err
		}
		if err := q.SetCharacterPicks(ctx, storeq.SetCharacterPicksParams{ProficiencyPicks: raw, ID: c.ID}); err != nil {
			return fmt.Errorf("character %d picks: %w", c.ID, err)
		}
	}
	return nil
}

// carryPick gives picks a pick of each of types, carried from its other_missile pick (moved to
// the first type when keepFrom is false), and reports whether it changed picks.
func carryPick(picks *skills.Picks, types []string, keepFrom bool, characterID int64) bool {
	from := slices.IndexFunc(picks.Weapon, func(p skills.WeaponPick) bool { return p.Type == retypedFrom })
	if from < 0 {
		return false
	}
	specialized := picks.Weapon[from].Slots > skills.WeaponPickSlots
	changed := false
	for _, t := range types {
		if slices.ContainsFunc(picks.Weapon, func(p skills.WeaponPick) bool { return p.Type == t }) {
			continue
		}
		changed = true
		if !keepFrom {
			picks.Weapon[from].Type = t
			keepFrom = true // moved: any further type gets a plain pick
			continue
		}
		if specialized {
			slog.Warn("migration 041: the other_missile specialization stays with the other missile pieces; the retyped piece gets a plain pick",
				"character", characterID, "type", t)
		}
		picks.Weapon = append(picks.Weapon, skills.WeaponPick{Type: t, Slots: skills.WeaponPickSlots})
	}
	return changed
}
