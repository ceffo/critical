package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/ceffo/critical/api/internal/adapters/characterdal"
	"github.com/ceffo/critical/api/internal/adapters/ruletablesadpt"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/skills"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// picksMigration adds characters.proficiency_picks. Right after it the run turns every stored
// proficiencies row into a pick (pickProficiencies, ADR 0037 step 16).
const picksMigration = "025_proficiency_picks.sql"

// pickProficiencies is ADR 0037 step 16.1, the data step after migration 025: each character's
// proficiencies rows, in sheet order, become its non-weapon picks, each at its catalogue cost for
// the character's class (step 4: Table 37's slots, one more outside the class's Table 38
// groups), so every check target stays what it was (no extra slot). A row whose id the catalogue
// does not hold is kept as a pick of 1 slot, never dropped: the derived warnings report it
// (proficiency_unknown) and the player may remove it. Nothing is refused: a character left over
// budget sees the slots_over_budget warning (step 16.4). The rows stay in their table, read by
// nothing. book is the catalogue (seeds/proficiencies.json).
func pickProficiencies(ctx context.Context, q storeq.Querier, book *ruletablesvc.Proficiencies) error {
	rows, err := q.ProficiencyRows(ctx)
	if err != nil {
		return fmt.Errorf("list proficiencies: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	chars, err := q.CharacterClasses(ctx)
	if err != nil {
		return fmt.Errorf("list characters: %w", err)
	}
	class := make(map[int64]string, len(chars))
	for _, c := range chars {
		class[c.ID] = c.Class
	}
	picks := map[int64]*skills.Picks{}
	var order []int64
	for _, r := range rows {
		p, ok := picks[r.CharacterID]
		if !ok {
			p = &skills.Picks{}
			picks[r.CharacterID] = p
			order = append(order, r.CharacterID)
		}
		slots := 1
		if row, known := book.Get(r.ID); known {
			slots = ruletablesvc.NonweaponCost(row, book.GroupsOf(class[r.CharacterID]))
		}
		p.Nonweapon = append(p.Nonweapon, skills.Pick{ID: r.ID, Slots: slots})
	}
	for _, id := range order {
		raw, err := characterdal.EncodePicks(*picks[id])
		if err != nil {
			return err
		}
		if err := q.SetCharacterPicks(ctx, storeq.SetCharacterPicksParams{ProficiencyPicks: raw, ID: id}); err != nil {
			return fmt.Errorf("character %d picks: %w", id, err)
		}
	}
	return nil
}

// pickProficienciesFrom reads the catalogue of seedsDir (its classes checked against defs), then
// runs pickProficiencies.
func pickProficienciesFrom(ctx context.Context, q storeq.Querier, seedsDir string, defs *classes.Registry) error {
	book, err := ruletablesadpt.ReadProficiencies(seedsDir, defs)
	if err != nil {
		return err
	}
	return pickProficiencies(ctx, q, book)
}

// weaponPicksMigration documents the weapon pool of characters.proficiency_picks. Right after it
// the run grants every character its owned weapon types (grantOwnedWeaponTypes, ADR 0037 step
// 16.2).
const weaponPicksMigration = "026_weapon_proficiency_picks.sql"

// grantOwnedWeaponTypes is ADR 0037 step 16.2, the data step after migration 026: each character
// gets one weapon pick (1 slot) per distinct weapon type among its weapons, in any state, so no
// attack changes when the non-proficiency penalty (step 8) lands. A weapon without a type gives
// none, nor does a stored item type that is no weapon type (items.weapon_type has no check). The
// picks follow the weapon types' display order, after any weapon pick already stored,
// which is kept: a run on picks that have a type adds nothing for it. Nothing is refused: the
// class's list (step 7) and the budget (step 10) are not checked, a type outside the list reads as
// not allowed and an overspend shows the slots_over_budget warning (step 16.4).
func grantOwnedWeaponTypes(ctx context.Context, q storeq.Querier) error {
	rows, err := q.CharacterWeaponTypes(ctx)
	if err != nil {
		return fmt.Errorf("list weapon types: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	owned := map[int64]map[string]bool{}
	for _, r := range rows {
		if owned[r.CharacterID] == nil {
			owned[r.CharacterID] = map[string]bool{}
		}
		owned[r.CharacterID][r.WeaponType] = true
	}
	chars, err := q.CharacterPicks(ctx)
	if err != nil {
		return fmt.Errorf("list picks: %w", err)
	}
	for _, c := range chars {
		types := owned[c.ID]
		if len(types) == 0 {
			continue
		}
		picks, err := characterdal.DecodePicks(c.ProficiencyPicks)
		if err != nil {
			return fmt.Errorf("character %d: %w", c.ID, err)
		}
		had := map[string]bool{}
		for _, p := range picks.Weapon {
			had[p.Type] = true
		}
		added := false
		for _, t := range ruletablesvc.WeaponTypes {
			if types[t] && !had[t] {
				picks.Weapon = append(picks.Weapon, skills.WeaponPick{Type: t, Slots: skills.WeaponPickSlots})
				added = true
			}
		}
		if !added {
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

// specializationMigration documents items.specialized as unused. Right after it the run turns
// each fighter's specialized weapon type into its specialization pick (specializeFlaggedWeapons,
// ADR 0037 step 16.3).
const specializationMigration = "027_weapon_specialization_picks.sql"

// specializeFlaggedWeapons is ADR 0037 step 16.3, the data step after migration 027: a fighter
// (a class that specializes) with a weapon flagged specialized gets that weapon type's pick as
// the specialization (step 9's slots, ruletablesvc.SpecializationSlots), the pick added when
// missing; with several specialized types, the first in list order, the others plain picks
// (logged), each added at one slot when missing so no attack loses its proficiency. Another class's flag is dropped (ADR 0030 step 12 refused it on every save),
// as is a flag on a type that is no weapon type, both logged; a flag on a weapon without a
// weapon type is not read at all (the query skips it), so it is dropped silently. A character
// whose picks already hold a specialization is left as it is, so a rerun changes nothing. Nothing is refused (step
// 16.4): an overspend shows the slots_over_budget warning. The flags stay in the column, read by
// nothing.
func specializeFlaggedWeapons(ctx context.Context, q storeq.Querier, defs *classes.Registry) error {
	rows, err := q.SpecializedWeaponTypes(ctx)
	if err != nil {
		return fmt.Errorf("list specialized weapons: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	flagged := map[int64][]string{}
	for _, r := range rows {
		if !slices.Contains(ruletablesvc.WeaponTypes, r.WeaponType) {
			slog.Warn("migration 027: dropped the specialization of a type that is no weapon type",
				"character", r.CharacterID, "type", r.WeaponType)
			continue
		}
		if !slices.Contains(flagged[r.CharacterID], r.WeaponType) {
			flagged[r.CharacterID] = append(flagged[r.CharacterID], r.WeaponType)
		}
	}
	classes, err := q.CharacterClasses(ctx)
	if err != nil {
		return fmt.Errorf("list classes: %w", err)
	}
	classOf := map[int64]string{}
	for _, c := range classes {
		classOf[c.ID] = c.Class
	}
	chars, err := q.CharacterPicks(ctx)
	if err != nil {
		return fmt.Errorf("list picks: %w", err)
	}
	for _, c := range chars {
		types := flagged[c.ID]
		if len(types) == 0 {
			continue
		}
		def, _ := defs.Class(classOf[c.ID]) // an undefined class lists no feature
		if !ruletablesvc.Specializes(def) {
			slog.Warn("migration 027: dropped the specialization of a class that does not specialize",
				"character", c.ID, "class", classOf[c.ID], "types", types)
			continue
		}
		picks, err := characterdal.DecodePicks(c.ProficiencyPicks)
		if err != nil {
			return fmt.Errorf("character %d: %w", c.ID, err)
		}
		if slices.ContainsFunc(picks.Weapon, skills.WeaponPick.IsSpecialization) {
			continue
		}
		first := types[0]
		if len(types) > 1 {
			slog.Warn("migration 027: specialized in several weapon types; the first is the specialization, the others plain picks",
				"character", c.ID, "specialization", first, "plain", types[1:])
		}
		slots := ruletablesvc.SpecializationSlots(first)
		if i := slices.IndexFunc(picks.Weapon, func(p skills.WeaponPick) bool { return p.Type == first }); i >= 0 {
			picks.Weapon[i].Slots = slots
		} else {
			picks.Weapon = append(picks.Weapon, skills.WeaponPick{Type: first, Slots: slots})
		}
		// The other flagged types are plain picks, added when missing: no attack loses its
		// proficiency the day the rule lands.
		for _, t := range types[1:] {
			if !slices.ContainsFunc(picks.Weapon, func(p skills.WeaponPick) bool { return p.Type == t }) {
				picks.Weapon = append(picks.Weapon, skills.WeaponPick{Type: t, Slots: skills.WeaponPickSlots})
			}
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
