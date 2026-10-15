package buffsvc

import (
	"context"
	"strings"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/google/uuid"
)

// Activate appends buff to the active list and logs it. Assigns a UUID when
// ID is empty, and stamps StartRound with the current round.
func (s *service) Activate(ctx context.Context, characterID int64, buff BuffEffect) ([]BuffEffect, error) {
	buff.Name = strings.TrimSpace(buff.Name)
	buff.Note = strings.TrimSpace(buff.Note)
	if err := validateBuff(buff); err != nil {
		return nil, err
	}
	if buff.ID == "" {
		buff.ID = uuid.New().String()
	}
	ch, err := s.apply(ctx, characterID, func(buffs []BuffEffect, round int) (change, error) {
		// ADR 0004 step 15: a buff starts in the round it is activated.
		buff.StartRound = round
		return change{
			kind:    actionlog.KindBuffChange,
			payload: logPayload{Action: "activate", Buff: buff.Name, Rounds: buff.Duration, Charges: buff.Charges, Note: buff.Note},
			buffs:   append(buffs, buff),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return ch.buffs, nil
}
