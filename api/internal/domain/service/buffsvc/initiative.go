package buffsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// initiativeTerm is buffsvc's JSON encoding of one initiative modifier, in
// the entries' payloads and the initiative snapshots. Add fields, never
// rename them: entries already in the log must stay decodable.
type initiativeTerm struct {
	Source    InitiativeSource `json:"source"`
	Value     int              `json:"value"`
	ItemID    string           `json:"itemId,omitempty"`
	Name      string           `json:"name,omitempty"`
	SpellID   string           `json:"spellId,omitempty"`
	MagicKind string           `json:"magicKind,omitempty"`
	Timing    CastTiming       `json:"timing,omitempty"`
}

func toTerm(m InitiativeModifier) initiativeTerm { return initiativeTerm(m) }

func fromTerm(t initiativeTerm) InitiativeModifier { return InitiativeModifier(t) }

func toTerms(ms []InitiativeModifier) []initiativeTerm {
	if len(ms) == 0 {
		return nil
	}
	out := make([]initiativeTerm, len(ms))
	for i, m := range ms {
		out[i] = toTerm(m)
	}
	return out
}

func fromTerms(ts []initiativeTerm) []InitiativeModifier {
	if len(ts) == 0 {
		return nil
	}
	out := make([]InitiativeModifier, len(ts))
	for i, t := range ts {
		out[i] = fromTerm(t)
	}
	return out
}

// initiativePayload is the initiative entry's structured record, worded by
// the UI: the round it was rolled for, the d10, the modifiers added to it
// and the total (ADR 0035 item 6.3). Entries recorded before the modifiers
// carry only {round, d10}: their total is the die. Total is always written
// now (a pointer only so those entries decode as they are).
type initiativePayload struct {
	Round     int              `json:"round"`
	D10       int              `json:"d10"`
	Modifiers []initiativeTerm `json:"modifiers,omitempty"`
	Total     *int             `json:"total,omitempty"`
}

// initiativeCorrectPayload is the record of a correction of the round's
// initiative by a spell cast or an item used (ADR 0035 item 6.2): the same
// d10, the total before and after, the new modifiers and the cause among
// them.
type initiativeCorrectPayload struct {
	Round     int              `json:"round"`
	D10       int              `json:"d10"`
	Before    int              `json:"before"`
	After     int              `json:"after"`
	Modifiers []initiativeTerm `json:"modifiers"`
	Cause     initiativeTerm   `json:"cause"`
}

// snapInitiative is buffsvc's JSON encoding of the initiative in action-log
// snapshots (StateKeyInitiative); no initiative is absent. EntryID is
// absent from the "after" of the entry that rolled or corrected it (its id
// is not known before it is recorded): a redo writes that "after" back
// through initiativeSnaps' replay, which knows the entry (ADR 0019 step 1).
// Modifiers are absent from snapshots recorded before them (a plain die).
type snapInitiative struct {
	D10       int              `json:"d10"`
	Modifiers []initiativeTerm `json:"modifiers,omitempty"`
	EntryID   int64            `json:"entryId,omitempty"`
}

func toSnapInitiative(i *Initiative) *snapInitiative {
	if i == nil {
		return nil
	}
	return &snapInitiative{D10: i.D10, Modifiers: toTerms(i.Modifiers), EntryID: i.LogEntryID}
}

// initiativeSnapshot is the snapshot of the initiative going from before to
// after (nil: none).
func initiativeSnapshot(before, after *Initiative) (actionlog.Snapshot, error) {
	return initiativeSnaps.Snapshot("", toSnapInitiative(before), toSnapInitiative(after))
}

// segmentsPerRound is the length of a round in segments (PHB p.94: a
// casting time of a round or more is not an initiative modifier).
const segmentsPerRound = 10

// WeaponSpeed is the weapon in hand's term of the initiative (ADR 0035 item
// 6, ADR 0033 step 15.1, DMG "Magical Weapon Speeds"): its speed factor
// lowered by its plus (the lesser of its to-hit and damage magic), or by the
// "weapon becomes +N" weaponBuff when higher (ADR 0002 step 6), never below
// 0. A negative magic (a cursed weapon) does not slow it.
func WeaponSpeed(w HeldWeapon, weaponBuff int) int {
	plus := max(min(w.MagicToHit, w.MagicDamage), weaponBuff, 0)
	return max(0, w.Speed-plus)
}

// SpellInitiative is the term of a spell cast that round (ADR 0035 item 6.2
// with ADR 0033 step 15.1's rules): its casting time in segments at the
// caster level; a casting time of a round or more adds nothing and reads
// "end of round"; a special one adds nothing. A dice part of the casting
// time is not counted.
func SpellInitiative(spellID, name string, ct spellstats.CastingTime, casterLevel int) InitiativeModifier {
	m := InitiativeModifier{Source: InitiativeSpell, SpellID: spellID, Name: name}
	if ct.Kind != spellstats.KindTimed {
		m.Timing = TimingSpecial
		return m
	}
	v := ct.At(casterLevel)
	if v.Unit != "segment" || v.Amount >= segmentsPerRound {
		m.Timing = TimingEndOfRound
		return m
	}
	m.Value = max(0, int(v.Amount))
	return m
}

// ItemInitiative is the term of a magic item used that round (ADR 0035 item
// 6.2): its magic kind's PHB Table 56 modifier (a scroll has none: 0).
func ItemInitiative(itemID, name, magicKind string, modifier int) InitiativeModifier {
	return InitiativeModifier{Source: InitiativeItem, Value: modifier, ItemID: itemID, Name: name, MagicKind: magicKind}
}

// initiativeEffects are the initiative effects of the buffs that apply and
// the powers that are on (ADR 0033 step 11), each its own modifier, buffs
// first in list order then the powers, gated as the aggregate gates them.
// An effect with a weapon-kind condition does not count: it conditions an
// attack (ADR 0004 step 18), and the initiative is none.
func initiativeEffects(buffs []BuffEffect, powers []ItemPower, base BaseStrength) []InitiativeModifier {
	str := effectiveStrength(buffs, base).Score
	var out []InitiativeModifier
	counts := func(e BuffEffects) bool {
		return e.Initiative != nil && *e.Initiative != 0 && e.WeaponKind == WeaponNone
	}
	for _, b := range buffs {
		if meetsRequirements(b.Effects, str) && counts(b.Effects) {
			out = append(out, InitiativeModifier{Source: InitiativeEffect, Value: *b.Effects.Initiative, Name: b.Name, SpellID: b.SpellID})
		}
	}
	for _, p := range powers {
		if p.On(str) && counts(p.Effects) {
			out = append(out, InitiativeModifier{Source: InitiativeEffect, Value: *p.Effects.Initiative, ItemID: p.ItemID, Name: p.ItemName})
		}
	}
	return out
}

// effectModifiers keeps the effect terms of ms (a correction replaces the
// weapon, spell or item term and keeps the effects recorded at the roll).
func effectModifiers(ms []InitiativeModifier) []InitiativeModifier {
	var out []InitiativeModifier
	for _, m := range ms {
		if m.Source == InitiativeEffect {
			out = append(out, m)
		}
	}
	return out
}

// initiativeModifiers are the terms Next Round adds to the die (ADR 0035
// item 6): the weapon in hand's speed (none with no weapon in hand), then
// the initiative effects of buffs, the round's buff list after the advance.
func (s *service) initiativeModifiers(ctx context.Context, characterID int64, buffs []BuffEffect) ([]InitiativeModifier, error) {
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}
	base := BaseStrengthOf(c)
	powers, err := s.items.ItemPowers(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get item powers: %w", err)
	}
	held, ok, err := s.weapons.WeaponInHand(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("weapon in hand: %w", err)
	}
	var out []InitiativeModifier
	if ok {
		buff := aggregate(buffs, powers, base, held.Kind).WeaponBuff
		out = append(out, InitiativeModifier{
			Source: InitiativeWeapon, Value: WeaponSpeed(held, buff), ItemID: held.ItemID, Name: held.Name,
		})
	}
	return append(out, initiativeEffects(buffs, powers, base)...), nil
}

// rollInitiative rolls round's initiative and records it as its own entry,
// stamped with that round, whose snapshot has the stored initiative as
// "before", then stores it with the entry's id. It must run inside a
// transaction, as the last entry that transaction records.
//
// ADR 0013 step 1: one d10 through the dice port. ADR 0035 item 6 (amending
// step 3): the modifiers are added to it; the lowest total acts first,
// which only the display says.
func (s *service) rollInitiative(ctx context.Context, characterID int64, round int, mods []InitiativeModifier) (Initiative, error) {
	before, err := s.dal.GetInitiative(ctx, characterID)
	if err != nil {
		return Initiative{}, fmt.Errorf("get initiative: %w", err)
	}
	d10 := s.dice.RollDie(InitiativeDie)
	out := Initiative{D10: d10, Modifiers: mods}
	snap, err := initiativeSnapshot(before, &out)
	if err != nil {
		return Initiative{}, err
	}
	total := out.Total()
	payload, err := json.Marshal(initiativePayload{Round: round, D10: d10, Modifiers: toTerms(mods), Total: &total})
	if err != nil {
		return Initiative{}, fmt.Errorf("encode payload: %w", err)
	}
	// ADR 0013 step 2: its own entry, stamped with the round it is for.
	out.LogEntryID, err = s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID,
		Round:       round,
		Kind:        actionlog.KindInitiative,
		Payload:     payload,
		Snapshots:   []actionlog.Snapshot{snap},
	})
	if err != nil {
		return Initiative{}, fmt.Errorf("record %s: %w", actionlog.KindInitiative, err)
	}
	if err := s.dal.SaveInitiative(ctx, characterID, &out); err != nil {
		return Initiative{}, fmt.Errorf("save initiative: %w", err)
	}
	return out, nil
}

// GetInitiative returns the round's initiative, nil when there is none.
func (s *service) GetInitiative(ctx context.Context, characterID int64) (*Initiative, error) {
	return s.dal.GetInitiative(ctx, characterID)
}

// ErrNoInitiative is the conflict for rerolling out of combat; it also wraps
// domain.ErrConflict.
var ErrNoInitiative = errors.New("no initiative to reroll")

// ErrInitiativeNotOnTop is the conflict for rerolling once another
// transaction is newer than the one that recorded the round's initiative; it
// also wraps domain.ErrConflict.
var ErrInitiativeNotOnTop = errors.New("the initiative is no longer the newest action")

// RerollInitiative is ADR 0013 step 4 as amended by ADR 0018 step 5: a new
// d10 recorded as its own transaction, whose initiative snapshot keeps the
// roll it replaces as "before", so undo gives that roll back. Nothing is
// voided. The modifiers of the replaced initiative are kept: only the die
// changes (ADR 0035 item 6.4). It is allowed only when the newest
// transaction is the one that recorded the round's current initiative (the
// Next Round, a previous reroll, or the cast or item use that corrected it):
// each ends with the initiative's entry, so it is the newest one exactly
// when that entry is the newest non-voided entry. Refused, with nothing
// rolled, otherwise or when there is no initiative.
func (s *service) RerollInitiative(ctx context.Context, characterID int64) (Initiative, error) {
	var out Initiative
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		round, err := s.dal.GetRoundNumber(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get round: %w", err)
		}
		current, err := s.dal.GetInitiative(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get initiative: %w", err)
		}
		// ADR 0013 step 5: out of combat there is nothing to reroll.
		if current == nil || round == 0 {
			return fmt.Errorf("%w: %w", domain.ErrConflict, ErrNoInitiative)
		}
		top, ok, err := s.log.TopID(ctx, characterID)
		if err != nil {
			return fmt.Errorf("top of the log: %w", err)
		}
		if !ok || top != current.LogEntryID {
			return fmt.Errorf("%w: %w: the initiative is #%d, the newest entry #%d",
				domain.ErrConflict, ErrInitiativeNotOnTop, current.LogEntryID, top)
		}
		out, err = s.rollInitiative(ctx, characterID, round, current.Modifiers)
		return err
	})
	if err != nil {
		return Initiative{}, err
	}
	return out, nil
}

// CorrectInitiative is ADR 0035 item 6.2: a spell cast or a magic item used
// in a round replaces that round's initiative by the same d10 plus the
// cause's term (SpellInitiative, ItemInitiative) and the effects recorded at
// the roll; the weapon term, or an earlier correction's cause, goes. No die
// is rolled. It records an initiative_correct entry stamped with the round,
// whose snapshot keeps the replaced initiative as "before" (undo gives it
// back), and stores the new one with that entry's id. It runs inside the
// caller's transaction (ctx) and must be the last entry the caller records,
// so a reroll stays allowed right after it. Out of combat or with no
// initiative it does nothing and returns nil.
func (s *service) CorrectInitiative(ctx context.Context, characterID int64, cause InitiativeModifier) (*Initiative, error) {
	if cause.Source != InitiativeSpell && cause.Source != InitiativeItem {
		return nil, fmt.Errorf("%w: an initiative correction comes from a spell or an item, not %q", domain.ErrInvalidInput, cause.Source)
	}
	round, err := s.dal.GetRoundNumber(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get round: %w", err)
	}
	current, err := s.dal.GetInitiative(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get initiative: %w", err)
	}
	if current == nil || round == 0 {
		return nil, nil
	}
	next := Initiative{D10: current.D10, Modifiers: append([]InitiativeModifier{cause}, effectModifiers(current.Modifiers)...)}
	snap, err := initiativeSnapshot(current, &next)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(initiativeCorrectPayload{
		Round: round, D10: next.D10, Before: current.Total(), After: next.Total(),
		Modifiers: toTerms(next.Modifiers), Cause: toTerm(cause),
	})
	if err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}
	next.LogEntryID, err = s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID,
		Round:       round,
		Kind:        actionlog.KindInitiativeCorrect,
		Payload:     payload,
		Snapshots:   []actionlog.Snapshot{snap},
	})
	if err != nil {
		return nil, fmt.Errorf("record %s: %w", actionlog.KindInitiativeCorrect, err)
	}
	if err := s.dal.SaveInitiative(ctx, characterID, &next); err != nil {
		return nil, fmt.Errorf("save initiative: %w", err)
	}
	return &next, nil
}
