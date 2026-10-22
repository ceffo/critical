package castsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
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
		// ADR 0030 step 14: a caster with a chance of spell failure rolls one
		// d100, after every refusal of the request (above, and checkCast for
		// the effect's); at or under it the spell fails: the slot is spent and
		// nothing else happens.
		var failure *SpellFailure
		if chance := c.Derived.SpellFailure; chance > 0 {
			if err := s.checkCast(ctx, characterID, id, req, c.Derived.RealizedLevel); err != nil {
				return err
			}
			roll := s.Dice.RollDie(100)
			failure = &SpellFailure{Roll: roll, Chance: chance, Failed: roll <= chance}
		}
		if failure != nil && failure.Failed {
			if err := s.Slots.SaveSlots(ctx, characterID, after); err != nil {
				return fmt.Errorf("save slots: %w", err)
			}
			slotSnap, err := prepsvc.SlotsSnapshot(before, after)
			if err != nil {
				return err
			}
			res = CastResult{
				Source: SourceCast, SpellID: id, Name: name, Level: slot.Level, CasterLevel: c.Derived.RealizedLevel,
				SlotIndex: &slotIndex, Slots: after, Failure: failure,
			}
			res.EntryID, err = s.record(ctx, characterID, res, []actionlog.Snapshot{slotSnap})
			if err != nil {
				return err
			}
			// A failed cast still took its casting time.
			return s.correctInitiative(ctx, characterID, id, name, c.Derived.RealizedLevel)
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
			casterLevel: c.Derived.RealizedLevel, class: c.Class, source: SourceCast, onSelf: req.OnSelf, req: req, extraSlots: extra, mode: mode,
		})
		if err != nil {
			return err
		}
		res.SlotIndex = &slotIndex
		res.Failure = failure
		res.ExtraSlots = extra
		res.Slots = after
		res.EntryID, err = s.record(ctx, characterID, res, append([]actionlog.Snapshot{slotSnap}, snaps...))
		if err != nil {
			return err
		}
		return s.correctInitiative(ctx, characterID, id, name, c.Derived.RealizedLevel)
	})
	if err != nil {
		return CastResult{}, err
	}
	return res, nil
}

// correctInitiative is ADR 0035 item 6.2 for a cast: the round's
// initiative becomes the same d10 plus the spell's casting time at the
// caster level (buffsvc.SpellInitiative). It is the cast transaction's last
// entry. A spell the catalogue does not know reads as a special casting
// time. A spell received or added as an effect is not cast by the
// character and corrects nothing.
func (s *service) correctInitiative(ctx context.Context, characterID int64, spellID, name string, casterLevel int) error {
	spell, found, err := s.Texts.Describe(ctx, spellID)
	if err != nil {
		return fmt.Errorf("describe %s: %w", spellID, err)
	}
	ct := spellstats.CastingTime{Kind: spellstats.KindSpecial}
	if found {
		ct = spell.Stats.CastingTime
	}
	if _, err := s.Initiative.CorrectInitiative(ctx, characterID, buffsvc.SpellInitiative(spellID, name, ct, casterLevel)); err != nil {
		return fmt.Errorf("correct initiative: %w", err)
	}
	return nil
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
	// The spell lands on the character: his class group picks a by-group
	// effect (ADR 0030 step 2), whoever cast it.
	c, err := s.Characters.GetCharacter(ctx, characterID)
	if err != nil {
		return CastResult{}, fmt.Errorf("get character: %w", err)
	}
	var res CastResult
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		var snaps []actionlog.Snapshot
		var err error
		res, snaps, err = s.resolve(ctx, characterID, effectReq{
			spellID: id, casterLevel: req.CasterLevel, class: c.Class, source: SourceReceived, mode: mode,
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
// lookup, at the character's level (ADR 0004 step 19): the row a cast would
// put on him (step 20), rolled the same way, with no slot spent, no cast
// mechanics and no damage. A spell that gets no row is refused before
// anything is rolled or written. It is one cast entry (action "effect") with
// the buff list's snapshots, so voiding it removes the row.
func (s *service) AddEffect(ctx context.Context, characterID int64, req AddEffectRequest) (CastResult, error) {
	id := strings.TrimSpace(req.SpellID)
	if id == "" {
		return CastResult{}, fmt.Errorf("%w: spell id is required", domain.ErrInvalidInput)
	}
	spell, found, err := s.Texts.Describe(ctx, id)
	if err != nil {
		return CastResult{}, fmt.Errorf("describe %s: %w", id, err)
	}
	entry, ok := s.Effects.Get(id)
	if !ok && !found {
		return CastResult{}, fmt.Errorf("%w: spell %q", domain.ErrNotFound, id)
	}
	// ADR 0004 step 20: the one rule the lookup's offer reads too.
	if !spelleffectsvc.RowOf(entryRef(entry, ok), spell.Stats.Duration).Any() {
		return CastResult{}, fmt.Errorf("%w: %s", spelleffectsvc.ErrNotAnEffect, id)
	}
	c, err := s.Characters.GetCharacter(ctx, characterID)
	if err != nil {
		return CastResult{}, fmt.Errorf("get character: %w", err)
	}
	if err := checkCasterLevel(c.Derived.RealizedLevel); err != nil {
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
			spellID: id, casterLevel: c.Derived.RealizedLevel, class: c.Class, source: SourceEffect, mode: mode,
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

// AddsEffect reports whether AddEffect takes spellID: the spell gets a row
// (ADR 0004 step 20, RowOf), from its effect entry or, without one, its
// catalogue duration.
func (s *service) AddsEffect(spellID string, duration spellstats.Duration) bool {
	e, ok := s.Effects.Get(spellID)
	return spelleffectsvc.RowOf(entryRef(e, ok), duration).Any()
}

// entryRef is e when the effects catalogue has it, nil otherwise.
func entryRef(e spelleffectsvc.Entry, ok bool) *spelleffectsvc.Entry {
	if !ok {
		return nil
	}
	return &e
}

type effectReq struct {
	spellID     string
	name        string // the slot's name, used when neither source knows the spell
	level       int
	casterLevel int
	// class is the class of the character the effect is about (the caster
	// of a cast, the receiver of a received spell): its group picks a
	// by-group effect (ADR 0030 step 2).
	class      string
	source     Source
	onSelf     *bool
	req        CastRequest          // the cast request (item to spend, target AC)
	extraSlots []int                // the extra slots this cast granted
	mode       *spelleffectsvc.Mode // the cast-time mode picked (nil: the spell has none)
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
// snapshot when a row went on the buff list (ADR 0004 steps 19 and 20).
func (s *service) resolve(ctx context.Context, characterID int64, r effectReq) (CastResult, []actionlog.Snapshot, error) {
	spell, found, err := s.Texts.Describe(ctx, r.spellID)
	if err != nil {
		return CastResult{}, nil, fmt.Errorf("describe %s: %w", r.spellID, err)
	}
	entry, hasEffect := s.Effects.Get(r.spellID)
	if !found && !hasEffect && r.source == SourceReceived {
		return CastResult{}, nil, fmt.Errorf("%w: spell %q", domain.ErrNotFound, r.spellID)
	}
	res, snaps, err := s.applyEffect(ctx, characterID, r, spell, entry, hasEffect)
	if err != nil {
		return CastResult{}, nil, err
	}
	snap, err := s.trackRow(ctx, characterID, r, entryRef(entry, hasEffect), &res)
	if err != nil {
		return CastResult{}, nil, err
	}
	if snap != nil {
		snaps = append(snaps, *snap)
	}
	return res, snaps, nil
}

// trackRow is ADR 0004 step 20: a spell that lasts a number of rounds and
// put no buff on the character gets a row with no effects, whether or not
// the engine models what it does. Its rounds are the ones the buff rolled
// when there is one (a buff cast on someone else), else rolled now at the
// caster level. A row the character casts on someone else or on an area is
// marked OnOther (step 20.5); a spell received or added as an effect is on
// him. The row goes on in the caller's transaction and its snapshot joins
// the cast's entry, so voiding the cast removes it.
func (s *service) trackRow(ctx context.Context, characterID int64, r effectReq, entry *spelleffectsvc.Entry, res *CastResult) (*actionlog.Snapshot, error) {
	if res.Buff != nil && res.Buff.Applied {
		return nil, nil
	}
	row := spelleffectsvc.RowOf(entry, res.Spell.Stats.Duration)
	if row.Rounds == nil {
		return nil, nil
	}
	var roll Roll
	if res.Buff != nil && res.Buff.Duration != nil {
		roll = *res.Buff.Duration
	} else {
		var err error
		if roll, err = s.roll(row.Rounds, r.casterLevel); err != nil {
			return nil, fmt.Errorf("%s: %w", r.spellID, err)
		}
	}
	if roll.Total <= 0 { // step 20.2
		return nil, nil
	}
	onSelf := true
	if r.source == SourceCast {
		// A buff that was not applied was cast on someone else.
		castOnOther := res.Buff != nil && row.Effects
		onSelf = !castOnOther && spelleffectsvc.RowOnSelf(entry, r.onSelf)
	}
	added, snap, err := s.Buffs.Add(ctx, characterID, buffsvc.BuffEffect{
		Name: res.Name, SpellID: r.spellID, Duration: roll.Total, OnOther: !onSelf,
	})
	if err != nil {
		return nil, fmt.Errorf("add row: %w", err)
	}
	switch {
	case res.Buff == nil:
		res.Buff = &AppliedBuff{Buff: added, Duration: &roll}
	case onSelf:
		// An ability set applied by hand: the row carries none of it.
		res.Buff.Buff = added
	default:
		// The rolls stay to tell the one it landed on; the row is his.
		res.Buff.Buff.ID = added.ID
	}
	res.Buff.Applied, res.Buff.OnOther = onSelf, !onSelf
	return &snap, nil
}

// applyEffect applies the spell's effect entry: its buff, damage and cast
// mechanics. The row of a spell that only lasts is trackRow's.
func (s *service) applyEffect(ctx context.Context, characterID int64, r effectReq, spell spellsvc.Spell, entry spelleffectsvc.Entry, hasEffect bool) (CastResult, []actionlog.Snapshot, error) {
	var err error
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
	res.Kind, res.Confidence, res.Target = entry.Kind, entry.Confidence, entry.Target
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
	// ADR 0004 step 19: an added effect rolls nothing but its row.
	if r.source == SourceEffect && entry.Kind != spelleffectsvc.KindBuff {
		res.TextOnly = TextOnlyUtility
		return res, snaps, nil
	}
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
		buff, untracked, err := s.rollBuff(entry, r.casterLevel, r.class)
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
	payload, err := json.Marshal(toBreakdown(res))
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
