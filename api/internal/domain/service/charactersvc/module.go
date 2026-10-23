package charactersvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// ArmorStateKey is the action-log state key of a thief's or a bard's skill
// armour as it was entered before ADR 0033 step 14: no move records it any
// more, but the entries already logged still pop and redo (they write the
// stored column back, which nothing reads).
const ArmorStateKey = "skill_armor"

// armorSnaps is the snapshotter of the skill armour (ADR 0018 step 8).
var armorSnaps = actionlog.NewSnapshotter[armorState](ArmorStateKey)

// armorState is the JSON encoding of a "skill_armor" snapshot (the entries'
// payload, {before, after}, is the web's to word). Entries already in the log
// must stay decodable: add fields, never rename.
type armorState struct {
	Armor string `json:"armor"`
}

// Module declares the codec that writes a thief's or a bard's skill armour
// back: undo writes the change's "before", redo its "after" (ADR 0019), inside
// the undo's transaction.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "charactersvc", Codecs: []actionlog.Codec{
		armorSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, st armorState) error {
			if err := s.owners.RestoreSkillArmor(ctx, characterID, st.Armor); err != nil {
				return fmt.Errorf("restore skill armor: %w", err)
			}
			return nil
		}, nil),
	}}
}
