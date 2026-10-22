// Package combatsvc owns attack resolution and saving throws. It has no DAL of
// its own — it consumes other services and infra adapters through narrow ports
// declared here (clean_architecture.md §4.5).
package combatsvc

import (
	"context"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/ruleset"
	"github.com/ceffo/critical/api/internal/domain/service/buffsvc"
	"github.com/ceffo/critical/api/internal/domain/service/charactersvc"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
	"github.com/ceffo/critical/api/internal/domain/service/ruletablesvc"
)

// SVC is the driver port.
type SVC interface {
	domain.Service
	// ResolveAttack rolls one attack, spends the ammunition of a missile
	// weapon and records the attack in the action log, all in one
	// transaction.
	ResolveAttack(ctx context.Context, characterID int64, req AttackRequest) (*RollResult, error)
	// ResolveSavingThrow rolls a save and records it in the action log.
	ResolveSavingThrow(ctx context.Context, characterID int64, category SaveCategory, opts SaveOptions) (*SaveBreakdown, error)
	// AdjustSave sets the adjustment of the latest save entry and returns
	// the recomputed save (same d20); recorded as an audited amend.
	AdjustSave(ctx context.Context, characterID, entryID int64, adj SaveAdjustment) (*SaveBreakdown, error)
	// AdjustAttackAC corrects the target AC of the latest attack entry and
	// returns the recomputed attack (same d20, d100 and damage dice, no ammo
	// spent); recorded as an audited amend.
	AdjustAttackAC(ctx context.Context, characterID, entryID int64, targetAC int) (*RollResult, error)
	// SetAttackVerdict records the GM's hit or miss on the latest attack
	// when its target AC is unknown (no crit); an audited amend.
	SetAttackVerdict(ctx context.Context, characterID, entryID int64, hit bool) (*RollResult, error)
	// ClearAttackAC makes the target AC of the latest attack unknown again
	// (ADR 0012 step 8).
	ClearAttackAC(ctx context.Context, characterID, entryID int64) (*RollResult, error)
	// RollSpellToHit rolls a to-hit (no weapon, no damage, no crit) for a
	// thrown spell; it logs nothing, the spell's own cast entry does.
	RollSpellToHit(ctx context.Context, characterID int64, targetAC int) (*ToHitRoll, error)
	ListWeapons(ctx context.Context, characterID int64) ([]Weapon, error)
	// GetAttackForm returns the attack form's last choices, resolved
	// against the catalog, with the preview of the effective THAC0.
	GetAttackForm(ctx context.Context, characterID int64) (*AttackFormView, error)
	// UpdateAttackForm saves the set fields of the patch and returns the
	// form as GetAttackForm does. A *domain.FieldError for invalid input.
	UpdateAttackForm(ctx context.Context, characterID int64, p AttackFormPatch) (*AttackFormView, error)
}

// Characters is the narrow inter-service port (just the read we need).
type Characters interface {
	GetCharacter(ctx context.Context, id int64) (*charactersvc.Character, error)
}

// Buffs is the narrow inter-service port. Aggregate takes the sheet's STR
// and the weapon kind of the roll (none for a save or a spell's to-hit) and
// returns the totals of the buffs and the item powers that are on, with the
// effective STR.
type Buffs interface {
	Aggregate(ctx context.Context, characterID int64, str buffsvc.BaseStrength, kind buffsvc.WeaponKind) (buffsvc.BuffModifiers, error)
	// GetRound is the current round (ADR 0005), for the attack rate's count
	// this round (ADR 0030 step 11.4).
	GetRound(ctx context.Context, characterID int64) (int, error)
}

// Dice is the dice adapter port.
type Dice interface {
	RollDie(sides int) int
	RollD20() int
	RollD100() int
}

// Weapons is the character's weapons: its weapon pieces (ADR 0033 step 3),
// wielded or not, as the attack reads them; ItemWeapons builds it over the
// equipment.
type Weapons interface {
	// Get returns the weapon, or an error wrapping ErrWeaponNotFound.
	Get(ctx context.Context, characterID int64, id string) (Weapon, error)
	// List returns the weapons in list order.
	List(ctx context.Context, characterID int64) ([]Weapon, error)
}

// Items is the character's equipment (itemsvc, ADR 0033): Use spends
// ammunition, List lists the pieces the attack form offers as ammunition,
// Wield wields the weapon the form picks (step 8). Use and Wield join the
// ctx transaction.
type Items interface {
	Use(ctx context.Context, characterID int64, itemID string) (itemsvc.Item, error)
	List(ctx context.Context, characterID int64) ([]itemsvc.Item, error)
	Wield(ctx context.Context, characterID int64, itemID string) (*itemsvc.StateResult, error)
}

// FormStore keeps the attack form of each character. Get's ok is false when
// none was saved. Both join the ctx transaction.
type FormStore interface {
	Get(ctx context.Context, characterID int64) (form AttackForm, ok bool, err error)
	Put(ctx context.Context, characterID int64, form AttackForm) error
}

// Rules is the PHB rule-table lookup (ruletablesvc.Lookup).
type Rules interface {
	StrengthMods(str, exceptional int) (hit, dmg int, err error)
	DexMods(dex int) (missile, ac int, err error)
	// AttackRateFor is ADR 0030 step 11 (ruletablesvc.Tables.AttackRateFor).
	AttackRateFor(class string, level int, kind, weaponType string, specialized bool) (ruletablesvc.AttackRate, bool, error)
}

// Log is the slice of the action log attacks and saves are recorded into;
// AmendLatest corrects the latest save (adjustment) or attack (target AC).
type Log interface {
	actionlog.Recorder
	actionlog.LatestAmender
}

// Deps are the service's collaborators.
type Deps struct {
	Chars   Characters
	Buffs   Buffs
	Dice    Dice
	Weapons Weapons
	// Rulesets gives a character its ruleset: the crit table is read from it
	// (ADR 0041 step 3).
	Rulesets ruleset.Resolver
	Items    Items
	Forms    FormStore
	Rules    Rules
	Log      Log
	Tx       domain.TxRunner
}

type service struct {
	chars    Characters
	buffs    Buffs
	dice     Dice
	weapons  Weapons
	rulesets ruleset.Resolver
	items    Items
	forms    FormStore
	rules    Rules
	log      Log
	tx       domain.TxRunner
}

// New constructs the service.
func New(d Deps) (SVC, error) {
	return NewWithConfig(DefaultConfig(), d)
}

// NewWithConfig is here for shape consistency.
func NewWithConfig(cfg *Config, d Deps) (SVC, error) {
	if cfg == nil {
		return nil, fmt.Errorf("combatsvc: nil config")
	}
	if d.Chars == nil || d.Buffs == nil || d.Dice == nil || d.Weapons == nil || d.Rulesets == nil ||
		d.Items == nil || d.Forms == nil || d.Rules == nil || d.Log == nil || d.Tx == nil {
		return nil, fmt.Errorf("combatsvc: nil dependency")
	}
	return &service{
		chars: d.Chars, buffs: d.Buffs, dice: d.Dice, weapons: d.Weapons, rulesets: d.Rulesets,
		items: d.Items, forms: d.Forms, rules: d.Rules, log: d.Log, tx: d.Tx,
	}, nil
}

func (s *service) Health(_ context.Context) error { return nil }
func (s *service) Close(_ context.Context) error  { return nil }

// Module declares the save entries' payload view (SavePayloadView) and no
// codec: the ammunition an attack spends is encoded by itemsvc's helper and
// restored by its codec.
func (s *service) Module() domain.Module {
	return domain.Module{Name: "combatsvc", Views: actionlog.PayloadViews{actionlog.KindSave: SavePayloadView}}
}
