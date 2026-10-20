package buffsvc

import (
	"context"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// Edit corrects a buff's remaining rounds and/or charges and logs the change.
// Setting a counter the buff uses to 0 ends it.
func (s *service) Edit(ctx context.Context, characterID int64, buffID string, e BuffEdit) ([]BuffEffect, error) {
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, _ int) (change, error) {
		i, err := find(buffs, buffID)
		if err != nil {
			return change{}, err
		}
		before := buffs[i]
		next, after, ended, err := edit(buffs, i, e)
		if err != nil {
			return change{}, err
		}
		noteChanged := e.Note != nil && after.Note != before.Note
		return change{
			kind: actionlog.KindBuffChange,
			payload: logPayload{
				Action: "edit", Buff: before.Name, SpellID: before.SpellID, ProficiencyID: before.ProficiencyID,
				FromRounds: before.Duration, FromCharges: before.Charges,
				Rounds: after.Duration, Charges: after.Charges, Ended: ended,
				Note: after.Note, FromNote: before.Note, NoteChanged: noteChanged,
			},
			buffs: next,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return ch.buffs, nil
}
