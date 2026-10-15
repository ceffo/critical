// Package castsvc makes casting a spell do something. Cast spends a prepared
// slot and resolves the spell's effect from the spell-effects catalog at the
// Bard's level: a buff on the Bard (rolled rounds, charges and ability gain),
// a damage roll for the DM, or only the spell text. Receive applies a spell a
// party member cast on the Bard, at that caster's level, and spends no slot.
//
// Each cast or receive is one action-log entry with before/after snapshots of
// the slot list (prepsvc.StateKey) and the buff list (buffsvc.StateKeyBuffs),
// written in one transaction; voiding it gives the slot back and removes the
// buff through the Restorers prepsvc and buffsvc register.
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

// Items is the inventory (itemsvc): a spell that consumes an item lists the
// items and spends one. Use joins the caller's transaction.
type Items interface {
	List(ctx context.Context) ([]itemsvc.Item, error)
	Use(ctx context.Context, itemID string) (itemsvc.Item, error)
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

// Log records the cast (actionlogsvc).
type Log interface {
	actionlog.Recorder
}

// Deps are the collaborators; all are required.
type Deps struct {
	Slots      Slots
	Effects    spelleffectsvc.Catalog
	Texts      Texts
	Buffs      Buffs
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
	if d.Slots == nil || d.Effects == nil || d.Texts == nil || d.Buffs == nil || d.Items == nil || d.Attacks == nil ||
		d.Characters == nil || d.Dice == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("castsvc: nil dependency")
	}
	return &service{d}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }
