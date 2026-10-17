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
		after[slotIndex].Used = true // ADR 0008 step 6: this slot and no other
		id := slot.SpellID
		if id == "" {
			id = spelleffectsvc.Slug(slot.PreparedSpellName)
		}
		name := slot.PreparedSpellName
		if slot.Extra && id == "" {
			// A blank extra slot: the player picks the spell now.
			var err error
			id, name, err = s.pickForExtra(ctx, slot.Level, req.SpellID)
			if err != nil {
				return err
			}
			after[slotIndex].SpellID, after[slotIndex].PreparedSpellName = id, name
		}
		// ADR 0008 step 14: an unknown mode is refused before anything is
		// written.
		mode, err := s.modeFor(id, req.Mode)
		if err != nil {
			return err
		}
		var extra []int
		// ADR 0008 step 9: a spell that grants slots adds them, blank.
		if entry, ok := s.Effects.Get(id); ok && entry.GrantSlots != nil {
			if extra, err = grantedLevels(entry, req.ExtraSlots); err != nil {
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
			casterLevel: c.Level, source: SourceCast, onSelf: req.OnSelf, req: req, extraSlots: extra, mode: mode,
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
	mode, err := s.modeFor(id, req.Mode)
	if err != nil {
		return CastResult{}, err
	}
	var res CastResult
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		var snaps []actionlog.Snapshot
		var err error
		res, snaps, err = s.resolve(ctx, characterID, effectReq{
			spellID: id, casterLevel: req.CasterLevel, source: SourceReceived, mode: mode,
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

// AddEffect puts a spell on the character as an effect from the spell
// lookup, at the character's level (ADR 0004 step 19): the buff a cast would
// put on him, rolled the same way, with no slot spent, no cast mechanics and
// no damage. A spell that adds no buff is refused before anything is rolled
// or written. It is one cast entry (action "effect") with the buff list's
// snapshots, so voiding it removes the buff.
func (s *service) AddEffect(ctx context.Context, characterID int64, req AddEffectRequest) (CastResult, error) {
	id := strings.TrimSpace(req.SpellID)
	if id == "" {
		return CastResult{}, fmt.Errorf("%w: spell id is required", domain.ErrInvalidInput)
	}
	entry, ok := s.Effects.Get(id)
	if !ok {
		_, found, err := s.Texts.Describe(ctx, id)
		if err != nil {
			return CastResult{}, fmt.Errorf("describe %s: %w", id, err)
		}
		if !found {
			return CastResult{}, fmt.Errorf("%w: spell %q", domain.ErrNotFound, id)
		}
	}
	if !entry.AddsBuff() {
		return CastResult{}, fmt.Errorf("%w: %s", spelleffectsvc.ErrNotAnEffect, id)
	}
	c, err := s.Characters.GetCharacter(ctx, characterID)
	if err != nil {
		return CastResult{}, fmt.Errorf("get character: %w", err)
	}
	if err := checkCasterLevel(c.Level); err != nil {
		return CastResult{}, err
	}
	mode, err := s.modeFor(id, "")
	if err != nil {
		return CastResult{}, err
	}
	var res CastResult
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		var snaps []actionlog.Snapshot
		var err error
		res, snaps, err = s.resolve(ctx, characterID, effectReq{
			spellID: id, casterLevel: c.Level, source: SourceEffect, mode: mode,
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

// AddsEffect reports whether AddEffect takes spellID: its effect entry adds
// a buff (ADR 0004 step 19).
func (s *service) AddsEffect(_ context.Context, spellID string) bool {
	e, ok := s.Effects.Get(spellID)
	return ok && e.AddsBuff()
}

type effectReq struct {
	spellID     string
	name        string // the slot's name, used when neither source knows the spell
	level       int
	casterLevel int
	source      Source
	onSelf      *bool
	req         CastRequest          // the cast request (item to spend, target AC)
	extraSlots  []int                // the extra slots this cast granted
	mode        *spelleffectsvc.Mode // the cast-time mode picked (nil: the spell has none)
}

// modeFor resolves the cast-time mode a cast of spell id names (ADR 0008
// step 14): the mode with that key, the first when none is named, nil for a
// spell without modes (or not in the catalogue) cast without one. A key the
// spell does not list is refused.
func (s *service) modeFor(id, key string) (*spelleffectsvc.Mode, error) {
	entry, ok := s.Effects.Get(id)
	if !ok {
		entry = spelleffectsvc.Entry{ID: id, Name: id}
	}
	return entry.ModeFor(key)
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
		Name: firstNonEmpty(entry.Name, spell.Name.Default(), r.name), Level: r.level, Spell: spell,
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
	if r.mode != nil {
		res.Mode = r.mode.Key
	}
	if entry.Save != nil && entry.Save.Type != "none" {
		save := *entry.Save
		res.Save = &save
	}
	if entry.Confidence == spelleffectsvc.ConfidenceLow {
		res.TextOnly = TextOnlyLowConfidence
		return res, nil, nil
	}
	// ADR 0008 step 16: the exposure damage is served on the result, for
	// the player to roll each round of exposure from.
	if res.Exposure, err = entry.ExposureAt(r.casterLevel); err != nil {
		return CastResult{}, nil, fmt.Errorf("%s: %w", entry.ID, err)
	}
	// The caster's own mechanics (spend an item, roll to hit) apply only to
	// a cast, not to a spell received from someone else nor to one added as
	// an effect.
	var snaps []actionlog.Snapshot
	if r.source == SourceCast {
		if snaps, err = s.castMechanics(ctx, characterID, entry, r.req, &res); err != nil {
			return CastResult{}, nil, err
		}
	}
	mechanics := len(r.extraSlots) > 0 || res.Consumed != nil || res.Attack != nil
	switch entry.Kind {
	case spelleffectsvc.KindDamage:
		damage := entry.CastDamage(r.mode)
		switch {
		case damage == nil && r.mode != nil:
			// ADR 0008 step 14: a mode without damage rolls nothing; the
			// slot is still spent.
			return res, snaps, nil
		case damage == nil && res.Exposure != nil:
			// ADR 0008 step 16: a cast rolls nothing, each exposure does.
			return res, snaps, nil
		case damage == nil:
			res.TextOnly = TextOnlyUtility
			return res, snaps, nil
		}
		res.Damage, err = s.rollDamage(entry, damage, r.casterLevel)
		if err != nil {
			return CastResult{}, nil, fmt.Errorf("%s: %w", entry.ID, err)
		}
		return res, snaps, nil
	case spelleffectsvc.KindBuff:
		buff, untracked, err := s.rollBuff(entry, r.casterLevel)
		if err != nil {
			return CastResult{}, nil, fmt.Errorf("%s: %w", entry.ID, err)
		}
		res.Buff, res.Untracked = buff, untracked
		// Low confidence returned above: what is left of a buff that adds
		// none is an ability set, applied by hand.
		if !entry.AddsBuff() {
			res.TextOnly = TextOnlyByHand
			return res, snaps, nil
		}
		// A spell someone casts on the Bard, or one added as an effect, is on
		// him by definition.
		if r.source == SourceCast {
			onSelf, err := appliesToSelf(entry, r.onSelf, r.casterLevel)
			if err != nil {
				return CastResult{}, nil, err
			}
			if !onSelf {
				res.TextOnly = TextOnlyNotOnSelf
				return res, snaps, nil
			}
		}
		added, snap, err := s.Buffs.Add(ctx, characterID, buff.Buff)
		if err != nil {
			return CastResult{}, nil, fmt.Errorf("add buff: %w", err)
		}
		buff.Buff, buff.Applied = added, true
		return res, append(snaps, snap), nil
	default:
		// A utility spell tracks nothing, unless casting it did something
		// (slots granted, an item spent, a to-hit rolled).
		if !mechanics {
			res.TextOnly = TextOnlyUtility
		}
		return res, snaps, nil
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
		Payload:     payload,
		Snapshots:   snaps,
	})
	if err != nil {
		return 0, fmt.Errorf("record cast: %w", err)
	}
	return id, nil
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// DamageAt reads the damage a cast of spellID without a mode rolls at
// casterLevel from the effect entry: the same dice rollDamage rolls, left
// unrolled.
func (s *service) DamageAt(_ context.Context, spellID string, casterLevel int) (*spelleffectsvc.DamageAt, error) {
	if err := checkCasterLevel(casterLevel); err != nil {
		return nil, err
	}
	e, ok := s.Effects.Get(spellID)
	if !ok {
		return nil, nil
	}
	return e.DamageAt(casterLevel)
}

// ModesAt reads the cast-time modes of spellID and the damage a cast in each
// rolls at casterLevel (ADR 0008 step 14).
func (s *service) ModesAt(_ context.Context, spellID string, casterLevel int) ([]spelleffectsvc.ModeAt, error) {
	if err := checkCasterLevel(casterLevel); err != nil {
		return nil, err
	}
	e, ok := s.Effects.Get(spellID)
	if !ok {
		return nil, nil
	}
	return e.ModesAt(casterLevel)
}

func checkCasterLevel(level int) error {
	if level < 1 || level > MaxCasterLevel {
		return fmt.Errorf("%w: caster level must be between 1 and %d", domain.ErrInvalidInput, MaxCasterLevel)
	}
	return nil
}
