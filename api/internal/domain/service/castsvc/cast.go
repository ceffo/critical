package castsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// Cast spends the slot and applies its spell at the character's level, in
// one transaction with one log entry.
func (s *service) Cast(ctx context.Context, characterID int64, slotIndex int, req CastRequest) (CastResult, error) {
	c, err := s.Characters.GetCharacter(ctx, characterID)
	if err != nil {
		return CastResult{}, fmt.Errorf("get character: %w", err)
	}
	var res CastResult
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		before, err := s.Slots.GetSlots(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get slots: %w", err)
		}
		if slotIndex < 0 || slotIndex >= len(before) {
			return fmt.Errorf("%w: %d", spellsvc.ErrInvalidSlotIndex, slotIndex)
		}
		slot := before[slotIndex]
		if slot.Used {
			return fmt.Errorf("%w: %d", spellsvc.ErrSlotAlreadyUsed, slotIndex)
		}
		after := append([]spellsvc.SpellSlot(nil), before...)
		after[slotIndex].Used = true
		id := slot.SpellID
		if id == "" {
			id = spelleffectsvc.Slug(slot.PreparedSpellName)
		}
		name := slot.PreparedSpellName
		if slot.Extra && id == "" {
			// A blank Rary's slot: the player picks the spell now.
			var err error
			id, name, err = s.pickForExtra(ctx, slot.Level, req.SpellID)
			if err != nil {
				return err
			}
			after[slotIndex].SpellID, after[slotIndex].PreparedSpellName = id, name
		}
		var extra []int
		if id == RaryID {
			var err error
			if extra, err = raryLevels(req.ExtraSlots); err != nil {
				return err
			}
			for _, l := range extra {
				after = append(after, spellsvc.SpellSlot{Level: l, Extra: true})
			}
		}
		if err := s.Slots.SaveSlots(ctx, characterID, after); err != nil {
			return fmt.Errorf("save slots: %w", err)
		}
		slotSnap, err := prepsvc.SlotsSnapshot(before, after)
		if err != nil {
			return err
		}
		var snaps []actionlog.Snapshot
		res, snaps, err = s.resolve(ctx, characterID, effectReq{
			spellID: id, name: name, level: slot.Level,
			casterLevel: c.Level, source: SourceCast, onSelf: req.OnSelf, req: req,
		})
		if err != nil {
			return err
		}
		res.SlotIndex = &slotIndex
		res.ExtraSlots = extra
		res.Slots = after
		res.EntryID, err = s.record(ctx, characterID, res, append([]actionlog.Snapshot{slotSnap}, snaps...))
		return err
	})
	if err != nil {
		return CastResult{}, err
	}
	return res, nil
}

// Receive applies a spell another caster put on the character.
func (s *service) Receive(ctx context.Context, characterID int64, req ReceiveRequest) (CastResult, error) {
	id := strings.TrimSpace(req.SpellID)
	if id == "" {
		return CastResult{}, fmt.Errorf("%w: spell id is required", domain.ErrInvalidInput)
	}
	if req.CasterLevel < 1 || req.CasterLevel > MaxCasterLevel {
		return CastResult{}, fmt.Errorf("%w: caster level must be between 1 and %d", domain.ErrInvalidInput, MaxCasterLevel)
	}
	var res CastResult
	err := s.Tx.InTx(ctx, func(ctx context.Context) error {
		var snaps []actionlog.Snapshot
		var err error
		res, snaps, err = s.resolve(ctx, characterID, effectReq{
			spellID: id, casterLevel: req.CasterLevel, source: SourceReceived,
		})
		if err != nil {
			return err
		}
		res.EntryID, err = s.record(ctx, characterID, res, snaps)
		return err
	})
	if err != nil {
		return CastResult{}, err
	}
	return res, nil
}

type effectReq struct {
	spellID     string
	name        string // the slot's name, used when neither source knows the spell
	level       int
	casterLevel int
	source      Source
	onSelf      *bool
	req         CastRequest // the cast request (Chromatic Orb: gem, target AC)
}

// resolve looks the spell up and applies its effect; it returns the buffs
// snapshot when a buff went on the character.
func (s *service) resolve(ctx context.Context, characterID int64, r effectReq) (CastResult, []actionlog.Snapshot, error) {
	spell, found, err := s.Texts.Describe(ctx, r.spellID)
	if err != nil {
		return CastResult{}, nil, fmt.Errorf("describe %s: %w", r.spellID, err)
	}
	entry, hasEffect := s.Effects.Get(r.spellID)
	if !found && !hasEffect && r.source == SourceReceived {
		return CastResult{}, nil, fmt.Errorf("%w: spell %q", domain.ErrNotFound, r.spellID)
	}
	res := CastResult{
		Source: r.source, SpellID: r.spellID, CasterLevel: r.casterLevel,
		Name: firstNonEmpty(entry.Name, spell.Name, r.name), Level: r.level, Spell: spell,
	}
	if res.Level == 0 {
		res.Level = max(entry.Level, spell.Level)
	}
	if !hasEffect {
		res.TextOnly = TextOnlyNoEffect
		return res, nil, nil
	}
	if r.source == SourceReceived && entry.Target == spelleffectsvc.TargetSelf {
		return CastResult{}, nil, fmt.Errorf("%w: %s affects only its caster and cannot be cast on you", domain.ErrInvalidInput, entry.Name)
	}
	res.Kind, res.Confidence, res.Target, res.Note = entry.Kind, entry.Confidence, entry.Target, entry.Note
	if entry.Save != nil && entry.Save.Type != "none" {
		save := *entry.Save
		res.Save = &save
	}
	if entry.Confidence == spelleffectsvc.ConfidenceLow {
		res.TextOnly = TextOnlyLowConfidence
		return res, nil, nil
	}
	// Chromatic Orb (critical-qki.42): a gem is spent, an attack is rolled
	// and the gem decides the effect; the level damage table is not used.
	if r.spellID == OrbID && r.source == SourceCast {
		orb, snap, err := s.throwOrb(ctx, characterID, r.req)
		if err != nil {
			return CastResult{}, nil, err
		}
		res.Orb = orb
		return res, []actionlog.Snapshot{snap}, nil
	}
	switch entry.Kind {
	case spelleffectsvc.KindDamage:
		if entry.Damage == nil {
			res.TextOnly = TextOnlyUtility
			return res, nil, nil
		}
		res.Damage, err = s.rollDamage(entry.Damage, r.casterLevel)
		if err != nil {
			return CastResult{}, nil, fmt.Errorf("%s: %w", entry.ID, err)
		}
		return res, nil, nil
	case spelleffectsvc.KindBuff:
		buff, untracked, err := s.rollBuff(entry, r.casterLevel)
		if err != nil {
			return CastResult{}, nil, fmt.Errorf("%s: %w", entry.ID, err)
		}
		res.Buff, res.Untracked = buff, untracked
		if entry.Buff != nil && len(entry.Buff.AbilitySet) > 0 {
			res.TextOnly = TextOnlyByHand
			return res, nil, nil
		}
		// A spell someone casts on the Bard is on him by definition.
		if r.source == SourceCast && !appliesToSelf(entry.Target, r.onSelf, hostile(buff, untracked)) {
			res.TextOnly = TextOnlyNotOnSelf
			return res, nil, nil
		}
		added, snap, err := s.Buffs.Add(ctx, characterID, buff.Buff)
		if err != nil {
			return CastResult{}, nil, fmt.Errorf("add buff: %w", err)
		}
		buff.Buff, buff.Applied = added, true
		return res, []actionlog.Snapshot{snap}, nil
	default:
		res.TextOnly = TextOnlyUtility
		return res, nil, nil
	}
}

func (s *service) record(ctx context.Context, characterID int64, res CastResult, snaps []actionlog.Snapshot) (int64, error) {
	payload, err := json.Marshal(toRecord(res))
	if err != nil {
		return 0, fmt.Errorf("encode cast payload: %w", err)
	}
	id, err := s.Log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID,
		Kind:        actionlog.KindCast,
		Summary:     summary(res),
		Payload:     payload,
		Snapshots:   snaps,
	})
	if err != nil {
		return 0, fmt.Errorf("record cast: %w", err)
	}
	return id, nil
}

// summary is the English log text (the UI renders the payload instead).
func summary(res CastResult) string {
	var b strings.Builder
	if res.Source == SourceReceived {
		fmt.Fprintf(&b, "Received %s (caster level %d)", res.Name, res.CasterLevel)
	} else {
		fmt.Fprintf(&b, "Cast %s", res.Name)
	}
	switch {
	case res.Buff != nil && res.Buff.Applied:
		b.WriteString(": " + counters(res.Buff.Buff.Duration, res.Buff.Buff.Charges))
		if res.Buff.StrGain != nil {
			fmt.Fprintf(&b, ", STR +%d", res.Buff.StrGain.Total)
		}
	case res.Damage != nil:
		fmt.Fprintf(&b, ": %d %s damage", res.Damage.Total, res.Damage.Type)
	case res.Orb != nil:
		verdict := "missed"
		if res.Orb.Attack.Hit {
			verdict = "hit"
		}
		fmt.Fprintf(&b, ": %s, %s (d20 %d vs AC %d)", res.Orb.Gem, verdict, res.Orb.Attack.D20Roll, res.Orb.Attack.TargetAC)
	case len(res.ExtraSlots) > 0:
		fmt.Fprintf(&b, ": extra slots, levels %v", res.ExtraSlots)
	}
	return b.String()
}

func counters(duration, charges int) string {
	switch {
	case duration > 0 && charges > 0:
		return fmt.Sprintf("%d rd, %d ch", duration, charges)
	case duration > 0:
		return fmt.Sprintf("%d rd", duration)
	case charges > 0:
		return fmt.Sprintf("%d ch", charges)
	default:
		return "until dismissed"
	}
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
