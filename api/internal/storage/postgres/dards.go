package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	storeq "github.com/ceffo/critical/api/internal/storage/postgres/queries"
)

// dardsMigration is the migration whose data step renames the darts weapon's French name. The
// seed offers a name's language once and never rewrites it (ADR 0023 step 10), so databases
// seeded before the seed said "Dards" are renamed by renameDarts, once, right after it.
const dardsMigration = "030_dards_french_name.sql"

// The seed's French name for the darts weapon, Fléchettes the one it gave before.
const (
	dartsID        = "darts"
	dartsOldFrench = "Fléchettes"
	dartsNewFrench = "Dards"
)

// renameDarts renames the seed character's darts weapon in French, from "Fléchettes" to "Dards",
// when its French body still reads "Fléchettes" (ADR 0023 step 10's one-off data step). A French
// name the player changed or removed, the English name, every other row and field are untouched,
// and a second run finds no "Fléchettes" and writes nothing. No seed character or no darts
// (removed, or a database seeded afterwards) changes nothing.
func renameDarts(ctx context.Context, q storeq.Querier, characterPath string) error {
	owner, ok, err := seedOwner(ctx, q, characterPath)
	if err != nil || !ok {
		return err
	}
	key, err := weaponName.nameKey(ctx, q, owner, dartsID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read the darts name key: %w", err)
	}
	rows, err := q.TextLanguages(ctx, key)
	if err != nil {
		return fmt.Errorf("read name %d: %w", key, err)
	}
	for _, r := range rows {
		if r.Lang == "fr" && r.Body == dartsOldFrench {
			if err := q.UpsertText(ctx, storeq.UpsertTextParams{TextID: key, Lang: "fr", Body: dartsNewFrench}); err != nil {
				return fmt.Errorf("rename the darts in French: %w", err)
			}
		}
	}
	return nil
}
