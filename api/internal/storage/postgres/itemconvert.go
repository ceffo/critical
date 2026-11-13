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
// reasons give, in one transaction. It runs at every start, after the migrations and the seed: it
// is idempotent (a converted row is never read again; a row an older release wrote after a
// rollback is converted by the next start). Only the abilities column is written: count,
// behaviour, max_count and the effect fields keep their old meaning (slice 2's ruling), so the
// previous release still runs on the migrated database.
//
// A seed character's item that the seed names with abilities lends the names of its abilities
// (the Power Gem's "Dispel Magic"), by ability id: the name is data of the seed row, not a branch.
func (d *DB) ConvertItems(ctx context.Context, seedsDir string) (itemsvc.BootConversion, error) {
	return convertItems(ctx, d, d.q, seedsDir)
}

// convertItems is ConvertItems over its ports: tx runs the transaction, q runs on the transaction
// tx carries.
func convertItems(ctx context.Context, tx domain.TxRunner, q storeq.Querier, seedsDir string) (out itemsvc.BootConversion, err error) {
	err = tx.InTx(ctx, func(ctx context.Context) error {
		rows, err := q.ListUnconvertedItems(ctx)
		if err != nil {
			return fmt.Errorf("list unconverted items: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		seedOwnerID, seedNames, err := seedAbilityNames(ctx, q, seedsDir)
		if err != nil {
			return err
		}
		for _, r := range rows {
			c := itemsvc.ConvertLegacy(itemsvc.Item{
				ID: r.ID, Category: itemsvc.Category(r.Category), MagicKind: r.MagicKind,
				Behaviour: itemsvc.Behaviour(r.Behaviour), Count: int(r.Count), MaxCount: int(r.MaxCount),
				EffectDice: r.EffectDice, EffectSave: r.EffectSave,
			})
			if r.CharacterID == seedOwnerID {
				itemsvc.NamedFrom(c.Abilities, seedNames[r.ID])
			}
			text, _, err := itemsvc.EncodeAbilities(c.Abilities)
			if err != nil {
				return fmt.Errorf("item %s: %w", r.ID, err)
			}
			if err := q.SetItemAbilities(ctx, storeq.SetItemAbilitiesParams{CharacterID: r.CharacterID, ID: r.ID, Abilities: text}); err != nil {
				return fmt.Errorf("convert item %s: %w", r.ID, err)
			}
			out.Converted++
			if len(c.Reasons) > 0 {
				out.Review++
			}
			for _, reason := range c.Reasons {
				if err := q.InsertReview(ctx, storeq.InsertReviewParams{CharacterID: r.CharacterID, ItemID: r.ID, Reason: reason}); err != nil {
					return fmt.Errorf("review item %s: %w", r.ID, err)
				}
			}
		}
		return nil
	})
	return out, err
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
