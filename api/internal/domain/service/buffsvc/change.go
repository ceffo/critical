package buffsvc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
)

// logPayload is the structured record of a buff entry, worded by the UI in
// the player's language.
type logPayload struct {
	// Action is activate, edit, absorb, dismiss, next_round, end_combat,
	// set_round, pass_time, save_template or delete_template. For pass_time,
	// Hours is what the player typed and Rounds the rounds that passed.
	Action      string   `json:"action"`
	Buff        string   `json:"buff,omitempty"`
	Rounds      int      `json:"rounds,omitempty"`
	Charges     int      `json:"charges,omitempty"`
	FromRounds  int      `json:"fromRounds,omitempty"`
	FromCharges int      `json:"fromCharges,omitempty"`
	Ended       bool     `json:"ended,omitempty"`
	FromRound   *int     `json:"fromRound,omitempty"`
	ToRound     *int     `json:"toRound,omitempty"`
	Expired     []string `json:"expired,omitempty"`
	// SpellID is the buff's spell (activate, edit, absorb, dismiss), when it
	// comes from one, so the UI names it in the player's language; Buff is the
	// stored name, used otherwise and by entries recorded before it existed.
	SpellID string `json:"spellId,omitempty"`
	// ExpiredSpellIDs runs parallel to Expired: the spell of each expired
	// buff, "" for a buff not from a spell. Omitted when none has one.
	ExpiredSpellIDs []string `json:"expiredSpellIds,omitempty"`
	// ProficiencyID is the buff's proficiency (edit, absorb, dismiss) when
	// a successful check put it on (ADR 0037 step 20.1), so the UI names it
	// in the player's language; ExpiredProficiencyIDs runs parallel to
	// Expired, "" for a buff not from a check, omitted when none is.
	ProficiencyID         string   `json:"proficiencyId,omitempty"`
	ExpiredProficiencyIDs []string `json:"expiredProficiencyIds,omitempty"`
	// Note is the buff's note (activate, save_template) or its new note (edit,
	// with NoteChanged); FromNote is the previous one.
	Note        string  `json:"note,omitempty"`
	FromNote    string  `json:"fromNote,omitempty"`
	NoteChanged bool    `json:"noteChanged,omitempty"`
	Hours       float64 `json:"hours,omitempty"`
	// Cause is how the player said a dismissed row ended, when its spell
	// has end effects (ADR 0004 step 21).
	Cause string `json:"cause,omitempty"`
	// EndEffects are the end effects the action applied, with their dice;
	// HPBefore and HPAfter the current HP around them (set with them).
	EndEffects []endEffectPayload `json:"endEffects,omitempty"`
	HPBefore   *int               `json:"hpBefore,omitempty"`
	HPAfter    *int               `json:"hpAfter,omitempty"`
	// PowerID is the row's psionic power (dismiss: a maintained power
	// stopped, ADR 0043 step 16), so the UI names it.
	PowerID string `json:"powerId,omitempty"`
	// Maintenance is what Next Round charged each maintained power, in the
	// list's order (ADR 0043 step 14); the entry's PSP delta is their sum,
	// negated. MaintenanceUnpaid are the powers whose cost the PSPs left
	// could not pay: they ended, and are among Expired too.
	Maintenance       []maintenanceCharge `json:"maintenance,omitempty"`
	MaintenanceUnpaid []string            `json:"maintenanceUnpaid,omitempty"`
	// PowerNames names those powers (and the ones a long rest ended) by id in
	// every language, as power_check does, so the journal words them.
	PowerNames map[string]localized.Text `json:"powerNames,omitempty"`
}

// maintenanceCharge is one maintained power's charge on Next Round.
type maintenanceCharge struct {
	PowerID string `json:"powerId"`
	Cost    int    `json:"cost"`
}

// change is what a use case decided: the new buff list, the new round when
// it changes, and the log entry describing it.
type change struct {
	kind    actionlog.Kind
	payload logPayload
	buffs   []BuffEffect
	round   *int
	// roundOnly: the buff list is neither snapshotted nor written (End combat,
	// Set round leave buffs untouched).
	roundOnly bool
	// clearInitiative: the round's initiative ends with this change (Next
	// Round, and any move to round 0: ADR 0013 steps 2 and 5). When there is
	// one, the entry snapshots it so voiding the entry brings it back.
	clearInitiative bool
	// ended are the rows the change ends by endCause, whose spells' end
	// effects it applies (ADR 0004 step 21; "": none); end is what they did.
	ended    []BuffEffect
	endCause spelleffectsvc.EndCause
	end      EndResult
	// pspDelta is the entry's PSP delta: maintenance charged (ADR 0043 step
	// 14), negative.
	pspDelta int
}

// apply runs decide on the current state and, in one transaction, records
// the action-log entry with before/after snapshots and writes the new state.
// The entry is recorded before the writes so it is stamped with the round
// the action happened in (Next Round belongs to the round it ends).
func (s *service) apply(ctx context.Context, characterID int64, decide func(buffs []BuffEffect, round int) (change, error)) (change, error) {
	var ch change
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		var err error
		ch, err = s.applyIn(ctx, characterID, decide)
		return err
	})
	if err != nil {
		return change{}, err
	}
	return ch, nil
}

// applyIn is apply inside the transaction ctx carries.
func (s *service) applyIn(ctx context.Context, characterID int64, decide func(buffs []BuffEffect, round int) (change, error)) (change, error) {
	var ch change
	err := func() error {
		buffs, err := s.dal.GetActiveBuffs(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get active buffs: %w", err)
		}
		round, err := s.dal.GetRoundNumber(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get round: %w", err)
		}
		ch, err = decide(append([]BuffEffect(nil), buffs...), round)
		if err != nil {
			return err
		}
		// ADR 0004 step 21: the end effects of the rows the change ends are
		// rolled now and their heal is this entry's HP delta, so the one
		// transaction (and the one undo) holds the end and the HP.
		rolls, heal, err := s.rollEndEffects(ch.ended, ch.endCause)
		if err != nil {
			return err
		}
		if len(rolls) > 0 {
			if ch.end, err = s.endResult(ctx, characterID, rolls, heal); err != nil {
				return err
			}
			ch.payload.EndEffects = endEffectPayloads(rolls)
			ch.payload.HPBefore, ch.payload.HPAfter = &ch.end.HPBefore, &ch.end.HPAfter
		}
		snaps, err := snapshots(buffs, ch.buffs, round, ch.round, ch.roundOnly)
		if err != nil {
			return err
		}
		var initiative *Initiative
		if ch.clearInitiative {
			if initiative, err = s.dal.GetInitiative(ctx, characterID); err != nil {
				return fmt.Errorf("get initiative: %w", err)
			}
			if initiative != nil {
				snap, err := initiativeSnapshot(initiative, nil)
				if err != nil {
					return err
				}
				snaps = append(snaps, snap)
			}
		}
		payload, err := json.Marshal(ch.payload)
		if err != nil {
			return fmt.Errorf("encode payload: %w", err)
		}
		if _, err := s.log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        ch.kind,
			HPDelta:     heal,
			PSPDelta:    ch.pspDelta,
			Payload:     payload,
			Snapshots:   snaps,
		}); err != nil {
			return fmt.Errorf("record %s: %w", ch.kind, err)
		}
		if !ch.roundOnly {
			if err := s.dal.SaveActiveBuffs(ctx, characterID, ch.buffs); err != nil {
				return fmt.Errorf("save active buffs: %w", err)
			}
		}
		if ch.round != nil {
			if err := s.dal.SaveRoundNumber(ctx, characterID, *ch.round); err != nil {
				return fmt.Errorf("save round: %w", err)
			}
		}
		if initiative != nil {
			if err := s.dal.SaveInitiative(ctx, characterID, nil); err != nil {
				return fmt.Errorf("clear initiative: %w", err)
			}
		}
		return nil
	}()
	if err != nil {
		return change{}, err
	}
	return ch, nil
}

func snapshots(before, after []BuffEffect, round int, newRound *int, roundOnly bool) ([]actionlog.Snapshot, error) {
	if roundOnly {
		snap, err := roundSnapshot(round, *newRound)
		if err != nil {
			return nil, err
		}
		return []actionlog.Snapshot{snap}, nil
	}
	snap, err := buffsSnapshot(before, after)
	if err != nil {
		return nil, err
	}
	snaps := []actionlog.Snapshot{snap}
	if newRound != nil {
		rs, err := roundSnapshot(round, *newRound)
		if err != nil {
			return nil, err
		}
		snaps = append(snaps, rs)
	}
	return snaps, nil
}

// expiredNames returns the names of the expired buffs and, parallel to them,
// their spell ids (nil when no expired buff comes from a spell) and their
// proficiency ids (nil when none comes from a proficiency check).
func expiredNames(expired []BuffEffect) (names, spellIDs, proficiencyIDs []string) {
	names = make([]string, len(expired))
	ids := make([]string, len(expired))
	profs := make([]string, len(expired))
	anySpell, anyProf := false, false
	for i, b := range expired {
		names[i] = b.Name
		ids[i] = b.SpellID
		profs[i] = b.ProficiencyID
		anySpell = anySpell || b.SpellID != ""
		anyProf = anyProf || b.ProficiencyID != ""
	}
	if anySpell {
		spellIDs = ids
	}
	if anyProf {
		proficiencyIDs = profs
	}
	return names, spellIDs, proficiencyIDs
}

// find returns the index of buffID or ErrNotFound.
func find(buffs []BuffEffect, buffID string) (int, error) {
	i := indexOf(buffs, buffID)
	if i < 0 {
		return -1, fmt.Errorf("%w: buff %q", domain.ErrNotFound, buffID)
	}
	return i, nil
}
