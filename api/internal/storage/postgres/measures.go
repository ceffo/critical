package postgres

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// mapMeasures is the data step after measuresMigration (ADR 0032 step 8): every character's stored
// free-text height and weight become the centimetres and kilograms they name; an unrecognised text
// leaves the number unset. The old columns are left as they are (the release before may still
// read them), so a second run gives the same numbers.
func mapMeasures(ctx context.Context, q storeq.Querier) error {
	rows, err := q.ListCharacterMeasures(ctx)
	if err != nil {
		return fmt.Errorf("list character measures: %w", err)
	}
	for _, r := range rows {
		// A text beyond the sanity bound (999 cm) is no figure: the parsers' flag, not the value, decides.
		h, ok := charactersvc.ParseHeight(r.Height)
		if !ok {
			h = 0
		}
		w, ok := charactersvc.ParseWeight(r.Weight)
		if !ok {
			w = 0
		}
		if h == 0 && w == 0 {
			continue
		}
		if err := q.SetCharacterMeasures(ctx, storeq.SetCharacterMeasuresParams{ID: r.ID, HeightCm: h, WeightKg: w}); err != nil {
			return fmt.Errorf("map the measures of character %d: %w", r.ID, err)
		}
	}
	return nil
}
