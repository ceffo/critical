// Package castsvc makes casting a spell do something. Cast spends a prepared
// slot and resolves the spell's effect from the spell-effects catalog at the
// Bard's level: a buff on the Bard (rolled rounds, charges and ability gain),
// a damage roll for the DM, or only the spell text. Receive applies a spell a
// party member cast on the Bard, at that caster's level, and spends no slot.
// AddEffect puts a buff spell on the Bard at his level from the spell lookup:
// no slot, no damage.
//
// Each cast or receive is one action-log entry with before/after snapshots of
// the slot list (prepsvc.StateKey) and the buff list (buffsvc.StateKeyBuffs),
// written in one transaction; voiding it gives the slot back and removes the
// buff through the codecs of prepsvc's and buffsvc's modules.
package castsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spelleffectsvc"
	"github.com/ceffo/critical/api/internal/domain/service/spellsvc"
	"github.com/ceffo/critical/api/internal/domain/spellstats"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// Cast spends slot slotIndex and applies its spell at the character's
	// level.
	Cast(ctx context.Context, characterID int64, slotIndex int, req CastRequest) (CastResult, error)
	// Receive applies a spell someone else cast on the character, at their
	// caster level. No slot is used.
	Receive(ctx context.Context, characterID int64, req ReceiveRequest) (CastResult, error)
	// AddEffect puts a spell that gets a row on the character as an effect,
	// at the character's level, with no slot spent and no damage rolled (ADR
	// 0004 steps 19 and 20). A spell that gets no row is refused.
	AddEffect(ctx context.Context, characterID int64, req AddEffectRequest) (CastResult, error)
	// AddsEffect reports whether AddEffect takes spellID, whose catalogue
	// duration is duration (ADR 0004 step 20: the one rule, RowOf).
	AddsEffect(spellID string, duration spellstats.Duration) bool
	// DamageAt is the damage a cast of spellID rolls at casterLevel (1 to
	// MaxCasterLevel), evaluated down to the dice left to roll; nil when a
	// cast rolls none (no effect entry, not a damage spell, low confidence).
	DamageAt(ctx context.Context, spellID string, casterLevel int) (*spelleffectsvc.DamageAt, error)
	// ModesAt is the cast-time modes of spellID, each with the damage a cast
	// in it rolls at casterLevel; nil when the spell has none.
	ModesAt(ctx context.Context, spellID string, casterLevel int) ([]spelleffectsvc.ModeAt, error)
	// ExposureAt is the exposure damage of spellID at casterLevel (ADR 0008
	// step 16); nil when it has none.
	ExposureAt(ctx context.Context, spellID string, casterLevel int) (*spelleffectsvc.ExposureAt, error)
	// Expose rolls one round of exposure to a spell's area and logs it; on
	// the Bard, the damage comes off his HP in the same entry.
	Expose(ctx context.Context, characterID int64, req ExposureRequest) (ExposureResult, error)
}

// Slots is the spell slot store (spellsvc's DAL satisfies it).
type Slots interface {
	GetSlots(ctx context.Context, characterID int64) ([]spellsvc.SpellSlot, error)
	SaveSlots(ctx context.Context, characterID int64, slots []spellsvc.SpellSlot) error
}

// Texts finds a spell of the spell database by id (snake_case slug of its
// English name), preferring the Wizard version of a name both lists share.
type Texts interface {
	Describe(ctx context.Context, spellID string) (spellsvc.Spell, bool, error)
}

// Buffs puts a buff on the Bard inside the caller's transaction and returns
// the snapshot to record (buffsvc satisfies it).
type Buffs interface {
	Add(ctx context.Context, characterID int64, buff buffsvc.BuffEffect) (buffsvc.BuffEffect, actionlog.Snapshot, error)
}

// Initiative corrects the round's initiative by the spell cast (ADR 0035
// item 6.2), inside the cast's transaction, as its last entry (buffsvc). It
// does nothing out of combat.
type Initiative interface {
	CorrectInitiative(ctx context.Context, characterID int64, cause buffsvc.InitiativeModifier) (*buffsvc.Initiative, error)
}

// Items is the inventory (itemsvc): a spell that consumes an item lists the
// items and spends one. Use joins the caller's transaction.
type Items interface {
	List(ctx context.Context, characterID int64) ([]itemsvc.Item, error)
	Use(ctx context.Context, characterID int64, itemID string) (itemsvc.Item, error)
}

// Attacks rolls the to-hit of a thrown spell (combatsvc).
type Attacks interface {
	RollSpellToHit(ctx context.Context, characterID int64, targetAC int) (*combatsvc.ToHitRoll, error)
}

// Characters supplies the caster level of the Bard (charactersvc).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Dice is the dice port.
type Dice interface {
	RollDie(sides int) int
}

// Log records the cast (actionlogsvc). HPTotal is the ledger's sum of HP
// deltas, read around an exposure the Bard takes to report the HP the
// ledger applied (the floor of ADR 0011 may cut it).
type Log interface {
	actionlog.Recorder
	HPTotal(ctx context.Context, characterID int64) (int, error)
}

// Deps are the collaborators; all are required.
type Deps struct {
	Slots      Slots
	Effects    spelleffectsvc.Catalog
	Texts      Texts
	Buffs      Buffs
	Initiative Initiative
	Items      Items
	Attacks    Attacks
	Characters Characters
	Dice       Dice
	Log        Log
	Tx         domain.TxRunner
}

type service struct{ Deps }

// New constructs the service.
func New(d Deps) (SVC, error) {
	if d.Slots == nil || d.Effects == nil || d.Texts == nil || d.Buffs == nil || d.Initiative == nil || d.Items == nil || d.Attacks == nil ||
		d.Characters == nil || d.Dice == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("castsvc: nil dependency")
	}
	return &service{d}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module declares no codec: the slots, buffs and items a cast changes are
// encoded by prepsvc's, buffsvc's and itemsvc's helpers and restored by
// their codecs.
func (s *service) Module() domain.Module { return domain.Module{Name: "castsvc"} }
