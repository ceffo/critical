// Package bridgeadpt holds the adapters that satisfy one service's port from
// another service or store: the small translations the composition root used
// to define inline. Each type implements a port declared in the service that
// needs it (clean_architecture.md §5.5); none holds state of its own.
package bridgeadpt

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/classes"
	"github.com/ceffo/critical/api/internal/domain/service/actionlogsvc"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/combatsvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/levelupsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
	"github.com/ceffo/critical/api/internal/domain/spellbook"
)

// RoundStore is the buff store's round read.
type RoundStore interface {
	GetRoundNumber(ctx context.Context, characterID int64) (int, error)
}

// RoundReader satisfies actionlogsvc.Rounds from the buff adapter. It runs
// inside the recording transaction (the adapter follows the ctx transaction).
type RoundReader struct{ DAL RoundStore }

// GetRound is the character's current round number.
func (r RoundReader) GetRound(ctx context.Context, characterID int64) (int, error) {
	return r.DAL.GetRoundNumber(ctx, characterID)
}

// WornArmour satisfies charactersvc.Armour from the equipment's store (the
// item service reads the characters, so the character service reads the
// pieces through their DAL): the worn body armour's Table 29 column (ADR 0033
// step 14). It follows the ctx transaction.
type WornArmour struct{ DAL itemsvc.DAL }

// WornSkillColumn is the worn body armour's column, "" when none is worn.
func (a WornArmour) WornSkillColumn(ctx context.Context, characterID int64) (string, error) {
	pieces, err := a.DAL.List(ctx, characterID)
	if err != nil {
		return "", err
	}
	return itemsvc.WornSkillColumn(pieces), nil
}

// WeaponInHand is buffsvc's Weapons port: the attack form's weapon in hand
// (combatsvc.InHand), as the initiative reads it (ADR 0035 item 6).
type WeaponInHand struct {
	Forms   combatsvc.FormStore
	Weapons combatsvc.Weapons
}

// WeaponInHand is the weapon the attack form holds, false when none is.
func (w WeaponInHand) WeaponInHand(ctx context.Context, characterID int64) (buffsvc.HeldWeapon, bool, error) {
	held, ok, err := combatsvc.InHand(ctx, w.Forms, w.Weapons, characterID)
	if err != nil || !ok {
		return buffsvc.HeldWeapon{}, false, err
	}
	return buffsvc.HeldWeapon{
		ItemID: held.ID, Name: held.Name.Any(), Kind: buffsvc.WeaponKind(held.Kind),
		Speed: held.Speed, MagicToHit: held.MagicToHit, MagicDamage: held.MagicDamage,
	}, true, nil
}

// Claims reads a character's free-spell claims (the spellbook's level_grant
// entries).
type Claims interface {
	Claimed(ctx context.Context, characterID int64) ([]spellbook.FreeSpell, error)
}

// FreeSpells satisfies charactersvc.FreeSpells (ADR 0030 step 20.8): the
// levels taken are the active level_up entries of the log (an undone roll's
// is voided, a redone one's active again), the claims the spellbook's
// level_grant entries. Both follow the ctx transaction.
type FreeSpells struct {
	Log  actionlogsvc.SVC
	Book Claims
}

// LevelsTaken lists the levels the active level_up entries reached.
func (f FreeSpells) LevelsTaken(ctx context.Context, characterID int64) ([]charactersvc.TakenLevel, error) {
	// One entry per level taken at most: the class tables' levels of a
	// multi-class character's classes bound them (ADR 0042 step 7).
	entries, err := f.Log.ActiveEntries(ctx, characterID, actionlog.KindLevelUp, classes.MaxMultiClasses*ruletablesvc.MaxLevel)
	if err != nil {
		return nil, err
	}
	levels := make([]charactersvc.TakenLevel, 0, len(entries))
	for _, e := range entries {
		class, level, err := levelupsvc.LevelReached(e.Payload)
		if err != nil {
			return nil, fmt.Errorf("level_up entry %d: %w", e.ID, err)
		}
		levels = append(levels, charactersvc.TakenLevel{Class: class, Level: level})
	}
	return levels, nil
}

// Claimed lists the free spells the character claimed.
func (f FreeSpells) Claimed(ctx context.Context, characterID int64) ([]spellbook.FreeSpell, error) {
	return f.Book.Claimed(ctx, characterID)
}

// ProficiencyIDs tells rulesetsvc which proficiency ids the book knows.
type ProficiencyIDs struct{ Book *ruletablesvc.Proficiencies }

// Has reports whether the book lists id.
func (p ProficiencyIDs) Has(id string) bool {
	_, ok := p.Book.Get(id)
	return ok
}
