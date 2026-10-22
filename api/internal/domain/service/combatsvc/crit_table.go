package combatsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/ruleset"
)

// The crit table is the ruleset's (ADR 0041 step 2.1): these are its types,
// named here because the attack's breakdown speaks of them.
type (
	// CritRow is one inclusive D100 range of the crit table and its multiplier.
	CritRow = ruleset.CritRow
	// CritTable is the whole table; Placeholder while it is the unedited default.
	CritTable = ruleset.CritTable
)

// loadCritTable reads the crit table of the character's ruleset at the moment
// of the roll (ADR 0041 steps 3 and 4). The outcome keeps the row it used, so
// editing the ruleset later never rewrites a logged attack.
func (s *service) loadCritTable(ctx context.Context, characterID int64) (CritTable, error) {
	rs, err := s.rulesets.ForCharacter(ctx, characterID)
	if err != nil {
		return CritTable{}, fmt.Errorf("load crit table: %w", err)
	}
	return rs.Crit, nil
}
