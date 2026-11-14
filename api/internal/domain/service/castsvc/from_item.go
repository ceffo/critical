package castsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/prepsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
)

// CastFromItem applies the spell of an item ability's spell effect at the
// fixed caster level of the effect (ADR 0049 steps 9, 10 and 12), inside the
// use's transaction and recording nothing: the use's entry records what comes
// back. It is the effect of a cast from a slot with none of the slot's
// surroundings: no slot spent, no preparation, no spell failure, no
// casting-time initiative (the use corrects it by the item, step 15), no class
// warning. The cast's own mechanics stay: the target AC of an attack roll, the
// item a spell spends, the slots a spell grants, the mode, the buff's target.
// Every refusal of the asks comes before anything is written or rolled
// (checkCast, the cast's own).
func (s *service) CastFromItem(ctx context.Context, characterID int64, req itemsvc.SpellCast) (itemsvc.SpellCastResult, error) {
	id := strings.TrimSpace(req.SpellID)
	if id == "" {
		return itemsvc.SpellCastResult{}, fmt.Errorf("%w: spell id is required", domain.ErrInvalidInput)
	}
	if err := checkCasterLevel(req.Level); err != nil {
		return itemsvc.SpellCastResult{}, err
	}
	spell, found, err := s.Texts.Describe(ctx, id)
	if err != nil {
		return itemsvc.SpellCastResult{}, fmt.Errorf("describe %s: %w", id, err)
	}
	if _, has := s.Effects.Get(id); !has && !found {
		return itemsvc.SpellCastResult{}, fmt.Errorf("%w: spell %q", domain.ErrNotFound, id)
	}
	a := req.Asks
	cast := CastRequest{OnSelf: a.OnSelf, ExtraSlots: a.ExtraSlots, ItemID: a.ItemID, TargetAC: a.TargetAC, Mode: a.Mode}
	mode, err := s.modeFor(id, a.Mode)
	if err != nil {
		return itemsvc.SpellCastResult{}, err
	}
	if err := s.checkCast(ctx, characterID, id, cast, req.Level); err != nil {
		return itemsvc.SpellCastResult{}, err
	}
	c, err := s.Characters.View(ctx, characterID)
	if err != nil {
		return itemsvc.SpellCastResult{}, fmt.Errorf("get character view: %w", err)
	}
	var snaps []actionlog.Snapshot
	var extra []int
	// ADR 0008 step 9: a spell that grants slots adds them, blank.
	if entry, ok := s.Effects.Get(id); ok && entry.GrantSlots != nil {
		if extra, err = grantedLevels(entry, a.ExtraSlots); err != nil {
			return itemsvc.SpellCastResult{}, err
		}
		before, err := s.Slots.GetSlots(ctx, characterID)
		if err != nil {
			return itemsvc.SpellCastResult{}, fmt.Errorf("get slots: %w", err)
		}
		after := append([]spellsvc.SpellSlot(nil), before...)
		for _, l := range extra {
			after = append(after, spellsvc.SpellSlot{Level: l, Extra: true})
		}
		if err := s.Slots.SaveSlots(ctx, characterID, after); err != nil {
			return itemsvc.SpellCastResult{}, fmt.Errorf("save slots: %w", err)
		}
		snap, err := prepsvc.SlotsSnapshot(before, after)
		if err != nil {
			return itemsvc.SpellCastResult{}, err
		}
		snaps = append(snaps, snap)
	}
	res, effectSnaps, err := s.resolve(ctx, characterID, effectReq{
		spellID: id, level: spell.Level, casterLevel: req.Level, groups: c.Groups(), source: SourceItem,
		onSelf: a.OnSelf, req: cast, extraSlots: extra, mode: mode, hpMax: c.HPMax, hpPending: req.HPPending,
	})
	if err != nil {
		return itemsvc.SpellCastResult{}, err
	}
	res.ExtraSlots = extra
	payload, err := json.Marshal(toBreakdown(res))
	if err != nil {
		return itemsvc.SpellCastResult{}, fmt.Errorf("encode cast payload: %w", err)
	}
	out := itemsvc.SpellCastResult{Name: res.Name, Cast: payload, Snapshots: append(snaps, effectSnaps...)}
	if h := res.Heal; h != nil && !h.OnOther {
		out.Healed, out.HPDelta = true, h.HPDelta
	}
	return out, nil
}
