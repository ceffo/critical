package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
)

// MaxSpellEffectLevel is the highest caster level a spell effect names (ADR
// 0049 step 10: 1 to 30, castsvc's MaxCasterLevel).
const MaxSpellEffectLevel = 30

// SpellAsks is what the player answers for one spell effect at Use (ADR 0049
// steps 9 and 10), the fields a cast from a slot takes (castsvc.CastRequest)
// by the same names. Absent is the cast's default: the spell's own target, the
// only item of its type left, the first split, the first mode. Effect is the
// index of the spell effect in the ability's list.
type SpellAsks struct {
	Effect int
	// OnSelf says whether a buff spell lands on the character (nil: default).
	OnSelf *bool
	// TargetAC is the target's AC, required for a spell with an attack roll.
	TargetAC *int
	// ItemID is the item a spell that consumes one spends.
	ItemID string
	// ExtraSlots is the split chosen for a spell that grants slots.
	ExtraSlots []int
	// Mode is the key of the cast-time mode picked.
	Mode string
}

// UseAsks are the answers a use carries: one SpellAsks per spell effect that
// needs one. A use of an ability with no spell effect carries none.
type UseAsks struct {
	Spells []SpellAsks
}

// forEffect is the asks for the spell effect at index (zero when none).
func (a UseAsks) forEffect(index int) SpellAsks {
	for _, s := range a.Spells {
		if s.Effect == index {
			return s
		}
	}
	return SpellAsks{Effect: index}
}

// check refuses an ask that names no spell effect of the ability, or names one
// twice, before anything is written.
func (a UseAsks) check(ab Ability) error {
	seen := map[int]bool{}
	for _, s := range a.Spells {
		if s.Effect < 0 || s.Effect >= len(ab.Effects) || ab.Effects[s.Effect].Kind != EffectSpell {
			return fmt.Errorf("%w: effect %d of %s is not a spell effect", domain.ErrInvalidInput, s.Effect, ab.ID)
		}
		if seen[s.Effect] {
			return fmt.Errorf("%w: effect %d is answered twice", domain.ErrInvalidInput, s.Effect)
		}
		seen[s.Effect] = true
	}
	return nil
}

// SpellCast is one spell effect to apply (ADR 0049 step 12): the spell, the
// fixed caster level, the player's answers and the HP the use's earlier
// effects already took (HPPending, so a heal reads the ledger as the entry will
// leave it).
type SpellCast struct {
	SpellID   string
	Level     int
	Asks      SpellAsks
	HPPending int
}

// SpellCastResult is what applying a spell effect produced. Cast is the cast
// breakdown JSON (castsvc's, as a cast from a slot logs it) the use's entry
// and answer carry; Snapshots are the states the spell wrote (buff list,
// slots, a spent item), which ride the use's entry; Healed and HPDelta are the
// HP a heal spell asks the ledger for on the character (before the cap).
type SpellCastResult struct {
	Name      string
	Cast      json.RawMessage
	Snapshots []actionlog.Snapshot
	Healed    bool
	HPDelta   int
}

// Caster applies a spell's effect at a fixed level inside the caller's
// transaction, recording nothing (castsvc): a spell effect of an ability goes
// through the one path a cast does (ADR 0049 step 12). castsvc imports
// itemsvc, so the dependency is inverted here and wired in the composition
// root, as the buff service is.
type Caster interface {
	CastFromItem(ctx context.Context, characterID int64, req SpellCast) (SpellCastResult, error)
}

// knownSpellEffects refuses an ability list holding a spell effect that names
// a spell the catalogue does not know (ADR 0049 step 10): field "abilities",
// code CodeUnknown.
func (s *service) knownSpellEffects(ctx context.Context, abilities []Ability) error {
	for _, a := range abilities {
		for _, e := range a.Effects {
			if e.Kind != EffectSpell {
				continue
			}
			if s.spells == nil {
				return fieldErr("abilities", CodeUnknown)
			}
			_, found, err := s.spells.Describe(ctx, e.SpellID)
			if err != nil {
				return fmt.Errorf("describe %s: %w", e.SpellID, err)
			}
			if !found {
				return fieldErr("abilities", CodeUnknown)
			}
		}
	}
	return nil
}

// runSpell applies a spell effect through the port (ADR 0049 step 12): the
// asks are the player's, the heal (if any) is capped against the ledger as the
// heal effects are, and the cast breakdown is the effect's result.
func (s *service) runSpell(ctx context.Context, characterID int64, hp *hpRun, e Effect, asks SpellAsks) (EffectResult, []actionlog.Snapshot, error) {
	res := EffectResult{Kind: e.Kind, Rolls: []int{}}
	if s.casts == nil {
		return EffectResult{}, nil, fmt.Errorf("a spell effect needs the cast service")
	}
	if err := s.readHP(ctx, characterID, hp); err != nil {
		return EffectResult{}, nil, err
	}
	cast, err := s.casts.CastFromItem(ctx, characterID, SpellCast{SpellID: e.SpellID, Level: e.Level, Asks: asks, HPPending: hp.applied})
	if err != nil {
		return EffectResult{}, nil, err
	}
	res.SpellID, res.Cast = e.SpellID, cast.Cast
	if cast.Healed {
		before := actionlog.CurrentHP(hp.max, hp.total)
		applied := actionlog.CapHeal(hp.total, max(0, cast.HPDelta))
		hp.total += applied
		hp.applied += applied
		res.Applied = &Applied{HPDelta: applied, HPBefore: before, HPAfter: actionlog.CurrentHP(hp.max, hp.total)}
	}
	return res, cast.Snapshots, nil
}

// addSnapshot joins snap to the use's snapshots: one per state key (an entry
// holds one), the state before the first and after the last.
func addSnapshot(snaps []actionlog.Snapshot, snap actionlog.Snapshot) []actionlog.Snapshot {
	i := slices.IndexFunc(snaps, func(s actionlog.Snapshot) bool { return s.Key == snap.Key })
	if i < 0 {
		return append(snaps, snap)
	}
	snaps[i] = *mergeSnapshots(&snaps[i], snap)
	return snaps
}
