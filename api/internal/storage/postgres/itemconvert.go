package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
	"github.com/ceffo/critical/api/internal/storage/seedfile"
)

// ConvertItems is ADR 0049 step 18's boot step: every item whose abilities are NULL (not converted
// yet) gets the abilities itemsvc.ConvertLegacy reads in its old fields, and the review rows its
// reasons give, in one transaction. It then sets the count of every converted item whose counter
// is an ability's (per_day or charges) to its quantity, 1 (step 2: the counter is the ability's
// `left`, read from the converted ability, which equals the count at conversion; slice 3's
// cut-over). It runs at every start, after the migrations and the seed: it is idempotent (a
// converted row is never read again, a count of 1 is never rewritten; a row an older release wrote
// after a rollback is converted by the next start). behaviour, max_count and the effect fields are
// never written: nothing reads them any more, and the contract migration drops them.
//
// A seed character's item that the seed names with abilities lends the names of its abilities
// (the Power Gem's "Dispel Magic"), by ability id: the name is data of the seed row, not a branch.
//
// Then, in the same transaction, every scroll that names a spell (spell_id) and whose abilities
// hold no spell effect gets one on its with_item ability (ADR 0049 step 9): the spell, at the
// caster level levels gives it (DMG p.145, ruletablesvc.Tables.ScrollCasterLevel). spell_id stays
// as stored, so the previous release still runs; a rerun finds every list holding its effect and
// writes nothing. A scroll whose spell levels does not know, or that has no with_item ability, is
// left as it is and counted in BootConversion.ScrollsSkipped.
func (d *DB) ConvertItems(ctx context.Context, seedsDir string, levels itemsvc.ScrollLevels) (itemsvc.BootConversion, error) {
	return convertItems(ctx, d, d.q, seedsDir, levels)
}

// convertItems is ConvertItems over its ports: tx runs the transaction, q runs on the transaction
// tx carries.
func convertItems(ctx context.Context, tx domain.TxRunner, q storeq.Querier, seedsDir string, levels itemsvc.ScrollLevels) (out itemsvc.BootConversion, err error) {
	err = tx.InTx(ctx, func(ctx context.Context) error {
		if out, err = convertAbilities(ctx, q, seedsDir); err != nil {
			return err
		}
		if out.Quantities, err = applyQuantities(ctx, q); err != nil {
			return err
		}
		out.Scrolls, out.ScrollsSkipped, err = foldScrollSpells(ctx, q, levels)
		return err
	})
	return out, err
}

// convertAbilities writes the abilities and review rows of the items not converted yet.
func convertAbilities(ctx context.Context, q storeq.Querier, seedsDir string) (out itemsvc.BootConversion, err error) {
	rows, err := q.ListUnconvertedItems(ctx)
	if err != nil {
		return out, fmt.Errorf("list unconverted items: %w", err)
	}
	if len(rows) == 0 {
		return out, nil
	}
	seedOwnerID, seedNames, err := seedAbilityNames(ctx, q, seedsDir)
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		legacy := itemsvc.Item{
			ID: r.ID, Category: itemsvc.Category(r.Category), MagicKind: r.MagicKind,
			Behaviour: itemsvc.Behaviour(r.Behaviour), Count: int(r.Count), MaxCount: int(r.MaxCount),
			EffectDice: r.EffectDice, EffectSave: r.EffectSave,
		}
		c := itemsvc.ConvertLegacy(legacy)
		// Never store a list the editor would then refuse: a row the conversion cannot
		// express (an unknown category, effect dice that do not parse) stops the boot,
		// loudly and with the row, and the transaction leaves every row as it was.
		if err := c.Check(legacy); err != nil {
			return out, fmt.Errorf("convert item %s: %w", r.ID, err)
		}
		if r.CharacterID == seedOwnerID {
			itemsvc.NamedFrom(c.Abilities, seedNames[r.ID])
		}
		text, _, err := itemsvc.EncodeAbilities(c.Abilities)
		if err != nil {
			return out, fmt.Errorf("item %s: %w", r.ID, err)
		}
		if err := q.SetItemAbilities(ctx, storeq.SetItemAbilitiesParams{CharacterID: r.CharacterID, ID: r.ID, Abilities: text}); err != nil {
			return out, fmt.Errorf("convert item %s: %w", r.ID, err)
		}
		out.Converted++
		if len(c.Reasons) > 0 {
			out.Review++
		}
		for _, reason := range c.Reasons {
			if err := q.InsertReview(ctx, storeq.InsertReviewParams{CharacterID: r.CharacterID, ItemID: r.ID, Reason: reason}); err != nil {
				return out, fmt.Errorf("review item %s: %w", r.ID, err)
			}
		}
	}
	return out, nil
}

// applyQuantities sets to 1 the count of every converted item that holds a per_day or charges
// ability (ADR 0049 step 2), and returns how many it set. The ability's left is not touched.
func applyQuantities(ctx context.Context, q storeq.Querier) (int, error) {
	rows, err := q.ListCountedItemAbilities(ctx)
	if err != nil {
		return 0, fmt.Errorf("list counted items: %w", err)
	}
	n := 0
	for _, r := range rows {
		abilities, err := itemsvc.DecodeAbilities(r.Abilities.String, r.Abilities.Valid)
		if err != nil {
			return 0, fmt.Errorf("item %s: %w", r.ID, err)
		}
		if !itemsvc.OneCounterPerObject(abilities) {
			continue
		}
		if err := q.SetItemQuantity(ctx, storeq.SetItemQuantityParams{CharacterID: r.CharacterID, ID: r.ID, Count: 1}); err != nil {
			return 0, fmt.Errorf("set quantity of item %s: %w", r.ID, err)
		}
		n++
	}
	return n, nil
}

// foldScrollSpells adds the spell effect to each converted scroll that names a spell and has none
// (ADR 0049 step 9), and returns how many it changed and how many it could not (a spell levels
// does not know, a scroll with no with_item ability).
func foldScrollSpells(ctx context.Context, q storeq.Querier, levels itemsvc.ScrollLevels) (changed, skipped int, err error) {
	rows, err := q.ListScrollsWithSpell(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list scrolls: %w", err)
	}
	for _, r := range rows {
		abilities, err := itemsvc.DecodeAbilities(r.Abilities.String, r.Abilities.Valid)
		if err != nil {
			return 0, 0, fmt.Errorf("item %s: %w", r.ID, err)
		}
		if itemsvc.FirstSpell(abilities) != "" {
			continue
		}
		level, ok := 0, false
		if levels != nil {
			level, ok = levels(r.SpellID)
		}
		if !ok {
			skipped++
			continue
		}
		folded := itemsvc.WithScrollSpell(abilities, r.SpellID, level)
		if itemsvc.FirstSpell(folded) == "" {
			skipped++
			continue
		}
		text, _, err := itemsvc.EncodeAbilities(folded)
		if err != nil {
			return 0, 0, fmt.Errorf("item %s: %w", r.ID, err)
		}
		if err := q.SetScrollAbilities(ctx, storeq.SetScrollAbilitiesParams{CharacterID: r.CharacterID, ID: r.ID, Abilities: text, Was: r.Abilities.String}); err != nil {
			return 0, 0, fmt.Errorf("fold the spell of scroll %s: %w", r.ID, err)
		}
		changed++
	}
	return changed, skipped, nil
}

// seedAbilityNames is the seed character and, by item id, the abilities its seed row has; none when
// there is no seed character or no readable items seed (then no name is lent).
func seedAbilityNames(ctx context.Context, q storeq.Querier, seedsDir string) (int64, map[string][]itemsvc.Ability, error) {
	owner, ok, err := seedOwner(ctx, q, seedfile.Path(seedsDir, seedfile.CharacterFile))
	if err != nil || !ok {
		return 0, nil, err
	}
	items, err := seedfile.ReadItems(seedfile.Path(seedsDir, seedfile.ItemsFile))
	if errors.Is(err, seedfile.ErrUnreadable) {
		return 0, nil, nil
	}
	if err != nil {
		return 0, nil, err
	}
	names := make(map[string][]itemsvc.Ability, len(items))
	for _, it := range items {
		if len(it.AbilityList) > 0 {
			names[it.ID] = it.AbilityList
		}
	}
	return owner, names, nil
}
