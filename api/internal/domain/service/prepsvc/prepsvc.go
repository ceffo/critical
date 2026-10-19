// Package prepsvc owns the daily spell preparation: the Bard picks, from the
// spellbook, the spells to have ready today, within the PHB Table 32 limit per
// spell level for the character's level. The preparation is stored as the
// spell slot list (spellsvc owns casting and the used flags), changed and
// recorded in the action log in one transaction, and voidable: prepsvc is the
// Restorer of the StateKey snapshots.
package prepsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// StateKey is the action-log snapshot key of the preparation (the slot list).
const StateKey = "spell_slots"

// Service-local sentinels.
var (
	// ErrNotInSpellbook: a requested spell id is not in the Bard's spellbook.
	ErrNotInSpellbook = errors.New("spell is not in the spellbook")
	// ErrOverLimit: more spells of one level than the PHB table allows.
	ErrOverLimit = errors.New("too many spells for this level")
)

// SpellbookEntry is one spell the Bard knows. ID is the snake_case slug of the
// English name (the i18n and spell-effects key).
type SpellbookEntry struct {
	ID    string
	Name  string
	Level int
	// Cast is what casting the spell asks of the player, from the spell
	// effects catalog (Info fills it).
	Cast CastInputs
	// Damage is the damage a cast rolls at the character's level, from the
	// spell effects catalog (Info fills it; nil: the cast rolls none).
	Damage *spelleffectsvc.DamageAt
	// Exposure is the exposure damage at the character's level (Info fills
	// it; nil: none; ADR 0008 step 16).
	Exposure *spelleffectsvc.ExposureAt
	// Items are the inventory items the spell can spend right now (Info
	// fills it; empty: the spell consumes none, or none is left).
	Items []itemsvc.Item
}

// CastInputs are the choices a spell's cast mechanics need from the player
// (spelleffectsvc.Entry): how to split the extra slots it grants (SlotSplits,
// the first is the default), the type of the inventory item it spends, and
// whether it rolls a to-hit (a target AC is then required), and the
// cast-time modes to pick from, each with the damage a cast in it rolls at the
// character's level (the first is the default; ADR 0008 step 14), and the
// choice of target a buff's cast offers (nil: none; the cast lands where the
// spell does by default).
type CastInputs struct {
	SlotSplits      [][]int
	ConsumeItemType string
	AttackRoll      bool
	Modes           []spelleffectsvc.ModeAt
	Target          *spelleffectsvc.TargetChoice
}

// LevelLimit is the number of spells of one level that may be prepared.
type LevelLimit struct {
	Level int
	Max   int
}

// Info is what the preparation editor needs: the spellbook and the limits.
type Info struct {
	Spellbook []SpellbookEntry
	Limits    []LevelLimit
}

// SVC is the driver port.
type SVC interface {
	domain.Service
	actionlog.Restorer

	// Info returns the spellbook and the per-level limits for the character.
	Info(ctx context.Context, characterID int64) (Info, error)
	// Prepare replaces today's preparation with the given spell ids
	// (duplicates allowed) and returns the new slot list.
	Prepare(ctx context.Context, characterID int64, spellIDs []string) ([]spellsvc.SpellSlot, error)
}

// Spellbook is the read port over the spells a character knows, each in
// its own spellbook, in the order of the sheet.
type Spellbook interface {
	All(ctx context.Context, characterID int64) ([]SpellbookEntry, error)
}

// Characters supplies the character's level and spell slot limits, both
// derived by the rules (ADR 0030 steps 3 and 8; charactersvc.SVC satisfies
// it).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Items is the inventory (itemsvc.SVC satisfies it).
type Items interface {
	List(ctx context.Context, characterID int64) ([]itemsvc.Item, error)
}

// Slots is the spell slot store (spellsvc's DAL satisfies it).
type Slots interface {
	GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	SaveSlots(ctx context.Context, characterID int64, slots []spellsvc.SpellSlot) error
}

// Log records the preparation in the action log (actionlogsvc satisfies it).
type Log interface {
	Record(ctx context.Context, e actionlog.NewEntry) (int64, error)
}

// Deps are the collaborators of the service; all are required.
type Deps struct {
	Slots      Slots
	Spellbook  Spellbook
	Effects    spelleffectsvc.Catalog
	Items      Items
	Characters Characters
	Log        Log
	Tx         domain.TxRunner
}

type service struct{ Deps }

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.Slots == nil || d.Spellbook == nil || d.Effects == nil || d.Items == nil || d.Characters == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("prepsvc: nil dependency")
	}
	return &service{d}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

func (s *service) limits(ctx context.Context, characterID int64) (map[int]int, error) {
	_, limits, err := s.levelLimits(ctx, characterID)
	return limits, err
}

// levelLimits returns the character's realized level and its preparation
// limits, the slots per day of that level (ADR 0008 step 2 as amended by
// ADR 0030 steps 7 and 8).
func (s *service) levelLimits(ctx context.Context, characterID int64) (int, map[int]int, error) {
	c, err := s.Characters.GetCharacter(ctx, characterID)
	if err != nil {
		return 0, nil, fmt.Errorf("get character: %w", err)
	}
	return c.Derived.RealizedLevel, c.Derived.SpellSlots, nil
}

func (s *service) Info(ctx context.Context, characterID int64) (Info, error) {
	level, limits, err := s.levelLimits(ctx, characterID)
	if err != nil {
		return Info{}, err
	}
	book, err := s.Spellbook.All(ctx, characterID)
	if err != nil {
		return Info{}, fmt.Errorf("read spellbook: %w", err)
	}
	out := Info{Spellbook: append([]SpellbookEntry(nil), book...)}
	inventory, err := s.Items.List(ctx, characterID)
	if err != nil {
		return Info{}, fmt.Errorf("list items: %w", err)
	}
	for i, e := range out.Spellbook {
		out.Spellbook[i].Cast = s.castInputs(e.ID)
		if t := out.Spellbook[i].Cast.ConsumeItemType; t != "" {
			for _, it := range inventory {
				if it.Spendable(t) {
					out.Spellbook[i].Items = append(out.Spellbook[i].Items, it)
				}
			}
		}
		if fx, ok := s.Effects.Get(e.ID); ok {
			if out.Spellbook[i].Damage, err = fx.DamageAt(level); err != nil {
				return Info{}, err
			}
			if out.Spellbook[i].Cast.Modes, err = fx.ModesAt(level); err != nil {
				return Info{}, err
			}
			if out.Spellbook[i].Exposure, err = fx.ExposureAt(level); err != nil {
				return Info{}, err
			}
			if out.Spellbook[i].Cast.Target, err = fx.TargetChoiceAt(level); err != nil {
				return Info{}, err
			}
		}
	}
	for level, max := range limits {
		out.Limits = append(out.Limits, LevelLimit{Level: level, Max: max})
	}
	sort.Slice(out.Limits, func(i, j int) bool { return out.Limits[i].Level < out.Limits[j].Level })
	return out, nil
}

// castInputs reads the cast mechanics of spell id from the catalog (ADR 0008
// steps 9 and 10).
func (s *service) castInputs(id string) CastInputs {
	e, ok := s.Effects.Get(id)
	if !ok {
		return CastInputs{}
	}
	out := CastInputs{AttackRoll: e.AttackRoll}
	if e.GrantSlots != nil {
		out.SlotSplits = e.GrantSlots.SlotSplits()
	}
	if e.ConsumeItem != nil {
		out.ConsumeItemType = e.ConsumeItem.Type
	}
	return out
}

// Prepare applies ADR 0008 steps 1 to 5.
func (s *service) Prepare(ctx context.Context, characterID int64, spellIDs []string) ([]spellsvc.SpellSlot, error) {
	limits, err := s.limits(ctx, characterID)
	if err != nil {
		return nil, err
	}
	book, err := s.Spellbook.All(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("read spellbook: %w", err)
	}
	known := make(map[string]SpellbookEntry, len(book))
	for _, e := range book {
		known[e.ID] = e
	}
	picked := make([]SpellbookEntry, 0, len(spellIDs))
	perLevel := map[int]int{}
	for _, id := range spellIDs {
		e, ok := known[id]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrNotInSpellbook, id)
		}
		picked = append(picked, e)
		perLevel[e.Level]++
	}
	for level, n := range perLevel {
		if n > limits[level] {
			return nil, fmt.Errorf("%w: %d at level %d, the limit is %d", ErrOverLimit, n, level, limits[level])
		}
	}
	// Stable by level so the request order is kept within a level.
	sort.SliceStable(picked, func(i, j int) bool { return picked[i].Level < picked[j].Level })

	var out []spellsvc.SpellSlot
	err = s.Tx.InTx(ctx, func(ctx context.Context) error {
		before, err := s.Slots.GetSlots(ctx, characterID)
		if err != nil {
			return fmt.Errorf("get slots: %w", err)
		}
		out = carryUsed(before, picked)
		if err := s.Slots.SaveSlots(ctx, characterID, out); err != nil {
			return fmt.Errorf("save slots: %w", err)
		}
		snap, err := SlotsSnapshot(before, out)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(prepRecordOf(picked))
		if err != nil {
			return fmt.Errorf("encode preparation payload: %w", err)
		}
		_, err = s.Log.Record(ctx, actionlog.NewEntry{
			CharacterID: characterID,
			Kind:        actionlog.KindSpellPrep,
			Payload:     payload,
			Snapshots:   []actionlog.Snapshot{snap},
		})
		if err != nil {
			return fmt.Errorf("record preparation: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Restore writes a snapshotted slot list back (void of a preparation).
func (s *service) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return slotsSnaps.Restorer(func(ctx context.Context, characterID int64, _ string, slots []spellsvc.SpellSlot) error {
		return s.Slots.SaveSlots(ctx, characterID, slots)
	}, nil).Restore(ctx, characterID, state)
}

// carryUsed builds the new slot list; a spell that was already prepared keeps
// its used flag so editing the preparation mid-day does not refill casts.
func carryUsed(before []spellsvc.SpellSlot, picked []SpellbookEntry) []spellsvc.SpellSlot {
	usedLeft := map[string]int{}
	for _, b := range before {
		if b.Used && !b.Extra {
			usedLeft[b.SpellID]++
		}
	}
	out := make([]spellsvc.SpellSlot, 0, len(picked))
	for _, e := range picked {
		used := usedLeft[e.ID] > 0
		if used {
			usedLeft[e.ID]--
		}
		out = append(out, spellsvc.SpellSlot{Level: e.Level, SpellID: e.ID, PreparedSpellName: e.Name, Used: used})
	}
	// Extra slots (granted by a spell) are not part of the preparation:
	// keep them (ADR 0008 step 9).
	for _, b := range before {
		if b.Extra {
			out = append(out, b)
		}
	}
	return out
}

// prepRecord is the spell_prep entry's payload: the spells prepared, each
// once with its number of copies, in preparation order. The UI words it.
// Add fields, never rename.
type prepRecord struct {
	Spells []prepSpell `json:"spells"`
}

type prepSpell struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func prepRecordOf(picked []SpellbookEntry) prepRecord {
	rec := prepRecord{Spells: []prepSpell{}}
	at := map[string]int{}
	for _, e := range picked {
		if i, ok := at[e.ID]; ok {
			rec.Spells[i].Count++
			continue
		}
		at[e.ID] = len(rec.Spells)
		rec.Spells = append(rec.Spells, prepSpell{ID: e.ID, Name: e.Name, Count: 1})
	}
	return rec
}

// slotsSnaps is the snapshotter of the slot list (ADR 0018 step 8); its state
// is spellsvc.SpellSlot encoded without JSON tags.
var slotsSnaps = actionlog.NewSnapshotter[[]spellsvc.SpellSlot](StateKey)

// SlotsSnapshot is the StateKey snapshot of a slot-list change, in the
// encoding Restore reads. Other services that change the slot list (a spell
// cast) record it so voiding their entry restores the slots.
func SlotsSnapshot(before, after []spellsvc.SpellSlot) (actionlog.Snapshot, error) {
	if before == nil {
		before = []spellsvc.SpellSlot{}
	}
	if after == nil {
		after = []spellsvc.SpellSlot{}
	}
	return slotsSnaps.Snapshot("", &before, &after)
}
