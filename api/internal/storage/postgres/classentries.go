package postgres

import (
	"context"
	"fmt"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// classEntriesMigration creates character_classes (ADR 0042 step 1). Right after it the run gives
// every stored character its one entry (fillClassEntries).
const classEntriesMigration = "034_character_classes.sql"

// fillClassEntries is the data step after classEntriesMigration: every character becomes one
// active entry with its class, its realized level (the sheet's level when none was taken in the
// app, ADR 0030 step 7.5) and its starting XP, so a single-class character derives as it did. A
// character that has an entry already is left as it is, so a rerun changes nothing.
func fillClassEntries(ctx context.Context, q storeq.Querier) error {
	if err := q.FillClassEntries(ctx); err != nil {
		return fmt.Errorf("give the stored characters their class entry: %w", err)
	}
	return nil
}
