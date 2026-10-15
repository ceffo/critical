package combatsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/itemsvc"
)

// AttackForm is the attack form's last choices for one character, kept on
// the server so a reload, another tab or another device finds them as left.
type AttackForm struct {
	WeaponID string
	// PreviousWeaponID is the weapon held before the current one, for the
	// one-click switch back; empty when there is none.
	PreviousWeaponID string
	AmmoItemID       string
	// TargetAC is nil while the target's AC is unknown, the default (ADR
	// 0012 step 1).
	TargetAC   *int
	TargetSize TargetSize
	// Situational is kept with its label even at 0, so the reason typed
	// stays while the value is stepped through 0. The label is stored as
	// typed, untrimmed.
	Situational Situational
}

// MaxSituational bounds the situational modifier (±MaxSituational).
const MaxSituational = 20

// DefaultAttackForm is the form of a character who never changed it.
func DefaultAttackForm() AttackForm {
	return AttackForm{TargetSize: SizeSmallMedium}
}

// AttackFormPatch changes the set fields of the form; nil leaves a field as
// it is. A new WeaponID makes the weapon held until then the previous one.
// An empty AmmoItemID clears the choice (the first arrow left is offered).
// UnknownTargetAC forgets the target AC (it is unknown again); it cannot
// come with a TargetAC.
type AttackFormPatch struct {
	WeaponID        *string
	AmmoItemID      *string
	TargetAC        *int
	UnknownTargetAC bool
	TargetSize      *TargetSize
	Situational     *Situational
}

// AmmoChoice is one ammunition the weapon in hand can fire.
type AmmoChoice struct {
	ItemID string
	Name   string
	Magic  int
	Count  int
}

// AttackPreview is what the current choices give before rolling (ADR 0002
// step 12): every to-hit modifier, the effective THAC0 and the roll needed
// against the target AC, and the flat damage bonus.
type AttackPreview struct {
	BaseThac0      int
	Modifiers      []Modifier
	EffectiveThac0 int
	// ToHit is the sum of the modifiers (BaseThac0 - EffectiveThac0).
	ToHit int
	// RollNeeded is the d20 that hits the target AC (step 2); nil while the
	// AC is unknown.
	RollNeeded *int
	// DamageBonus is the sum of the flat damage terms: weapon or buff "+N",
	// magic ammunition, Strength and buff damage (ADR 0003).
	DamageBonus int
}

// AttackFormView is the form as the panel shows it: the stored choices with
// the weapon and ammunition resolved against the current catalog, the
// ammunition the weapon can fire and the preview.
type AttackFormView struct {
	Form AttackForm
	// AmmoFromDefault: no ammunition was chosen (or it is gone), so
	// AmmoItemID is the first one left of the weapon's kind. An attack with
	// it should make it the choice (UpdateAttackForm) so it is kept once
	// spent.
	AmmoFromDefault bool
	// AmmoChoices are the items of the weapon's ammo kind with some left,
	// plus the chosen one even when spent; empty for a weapon without ammo.
	AmmoChoices []AmmoChoice
	// Preview is nil when there is no weapon at all.
	Preview *AttackPreview
}

// GetAttackForm returns the character's attack form with its preview.
func (s *service) GetAttackForm(ctx context.Context, characterID int64) (*AttackFormView, error) {
	var view *AttackFormView
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		form, err := s.storedForm(ctx, characterID)
		if err != nil {
			return err
		}
		view, err = s.formView(ctx, characterID, form)
		return err
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// UpdateAttackForm validates and saves the set fields of p, then returns
// the form with its preview. Switching weapon never attacks: it only moves
// the weapon held until then to PreviousWeaponID.
func (s *service) UpdateAttackForm(ctx context.Context, characterID int64, p AttackFormPatch) (*AttackFormView, error) {
	if p.TargetAC != nil && p.UnknownTargetAC {
		return nil, domain.FieldErrorf("targetAc", "the target AC cannot be both given and unknown")
	}
	var view *AttackFormView
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		form, err := s.storedForm(ctx, characterID)
		if err != nil {
			return err
		}
		weapons, err := s.weapons.List(ctx)
		if err != nil {
			return fmt.Errorf("list weapons: %w", err)
		}
		held, _ := resolveWeapon(weapons, form.WeaponID)

		if p.WeaponID != nil {
			next, ok := findWeapon(weapons, *p.WeaponID)
			if !ok {
				return domain.FieldErrorf("weaponId", "unknown weapon %q", *p.WeaponID)
			}
			if held != nil && held.ID != next.ID {
				form.PreviousWeaponID = held.ID
			}
			form.WeaponID = next.ID
			held = &next
		}
		if p.AmmoItemID != nil {
			id := strings.TrimSpace(*p.AmmoItemID)
			if id != "" {
				if err := s.checkAmmo(ctx, held, id); err != nil {
					return err
				}
			}
			form.AmmoItemID = id
		}
		switch {
		case p.UnknownTargetAC:
			form.TargetAC = nil
		case p.TargetAC != nil:
			if ac := *p.TargetAC; ac < -MaxTargetAC || ac > MaxTargetAC {
				return domain.FieldErrorf("targetAc", "target AC must be between -%d and %d", MaxTargetAC, MaxTargetAC)
			}
			ac := *p.TargetAC
			form.TargetAC = &ac
		}
		if p.TargetSize != nil {
			if !p.TargetSize.Valid() {
				return domain.FieldErrorf("targetSize", "target size must be %q or %q", SizeSmallMedium, SizeLarge)
			}
			form.TargetSize = *p.TargetSize
		}
		if sit := p.Situational; sit != nil {
			if sit.Value < -MaxSituational || sit.Value > MaxSituational {
				return domain.FieldErrorf("situational", "situational modifier must be between -%d and %d", MaxSituational, MaxSituational)
			}
			// The label is kept as typed (the reason input sends every keystroke, so a
			// trailing space is the start of the next word); the attack trims it.
			form.Situational = Situational{Value: sit.Value, Label: sit.Label}
		}
		if err := s.forms.Put(ctx, characterID, form); err != nil {
			return fmt.Errorf("save attack form: %w", err)
		}
		view, err = s.formView(ctx, characterID, form)
		return err
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

// checkAmmo refuses an ammunition the weapon in hand cannot fire.
func (s *service) checkAmmo(ctx context.Context, weapon *Weapon, itemID string) error {
	items, err := s.items.List(ctx)
	if err != nil {
		return fmt.Errorf("list items: %w", err)
	}
	item, ok := findItem(items, itemID)
	switch {
	case !ok:
		return domain.FieldErrorf("ammoItemId", "unknown item %q", itemID)
	case weapon == nil || !weapon.FiresAmmo():
		return domain.FieldErrorf("ammoItemId", "the weapon in hand fires no ammunition")
	case item.AmmoKind != weapon.AmmoKind:
		return domain.FieldErrorf("ammoItemId", "%s cannot fire %s", weapon.Name, item.Name)
	}
	return nil
}

func (s *service) storedForm(ctx context.Context, characterID int64) (AttackForm, error) {
	form, ok, err := s.forms.Get(ctx, characterID)
	if err != nil {
		return AttackForm{}, fmt.Errorf("get attack form: %w", err)
	}
	if !ok {
		return DefaultAttackForm(), nil
	}
	return form, nil
}

// formView resolves the stored form against the catalog and computes the
// preview:
//   - the weapon is the stored one, or the first of the list when it is
//     unset or gone;
//   - the previous weapon is dropped when gone or the same as the held one;
//   - for a weapon that fires ammunition, the chosen item stays chosen while
//     it exists and is of the weapon's kind, even spent (never switched
//     behind the player's back); otherwise the first of the kind with some
//     left is offered (AmmoFromDefault). A weapon without ammunition shows
//     none, the stored choice is kept for when a bow is back in hand.
func (s *service) formView(ctx context.Context, characterID int64, form AttackForm) (*AttackFormView, error) {
	weapons, err := s.weapons.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list weapons: %w", err)
	}
	view := &AttackFormView{Form: form, AmmoChoices: []AmmoChoice{}}
	held, ok := resolveWeapon(weapons, form.WeaponID)
	if !ok {
		view.Form.WeaponID, view.Form.PreviousWeaponID, view.Form.AmmoItemID = "", "", ""
		return view, nil
	}
	view.Form.WeaponID = held.ID
	if prev, found := findWeapon(weapons, form.PreviousWeaponID); !found || prev.ID == held.ID {
		view.Form.PreviousWeaponID = ""
	}

	var ammo *itemsvc.Item
	view.Form.AmmoItemID = ""
	if held.FiresAmmo() {
		items, err := s.items.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list items: %w", err)
		}
		var firstLeft *itemsvc.Item
		for i := range items {
			it := &items[i]
			if it.AmmoKind != held.AmmoKind || (it.Count == 0 && it.ID != form.AmmoItemID) {
				continue
			}
			view.AmmoChoices = append(view.AmmoChoices, AmmoChoice{ItemID: it.ID, Name: it.Name, Magic: it.MagicBonus, Count: it.Count})
			if it.ID == form.AmmoItemID {
				ammo = it
			}
			if firstLeft == nil && it.Count > 0 {
				firstLeft = it
			}
		}
		if ammo == nil && firstLeft != nil {
			ammo = firstLeft
			view.AmmoFromDefault = true
		}
		if ammo != nil {
			view.Form.AmmoItemID = ammo.ID
		}
	}

	preview, err := s.preview(ctx, characterID, *held, ammo, view.Form)
	if err != nil {
		return nil, err
	}
	view.Preview = preview
	return view, nil
}

// preview computes the effective THAC0 of the current choices with the
// same steps as an attack (ADR 0002 step 12): nothing is rolled or spent.
func (s *service) preview(ctx context.Context, characterID int64, weapon Weapon, ammo *itemsvc.Item, form AttackForm) (*AttackPreview, error) {
	char, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, fmt.Errorf("get character: %w", err)
	}
	mods, err := s.buffs.Aggregate(ctx, characterID, char.Str)
	if err != nil {
		return nil, fmt.Errorf("aggregate buffs: %w", err)
	}
	// Trimmed like ResolveAttack does, so the preview lists the same modifier.
	sit := Situational{Value: form.Situational.Value, Label: strings.TrimSpace(form.Situational.Label)}
	am, err := s.attackModifiers(char, weapon, ammo, mods, &sit)
	if err != nil {
		return nil, err
	}
	toHit := am.toHit()
	eff := char.Thac0Base - toHit
	p := &AttackPreview{
		BaseThac0:      char.Thac0Base,
		Modifiers:      am.modifiers,
		EffectiveThac0: eff,
		ToHit:          toHit,
		DamageBonus:    am.weaponDmg + am.ammoDmg + am.strDmg + mods.Damage,
	}
	if form.TargetAC != nil {
		needed := eff - *form.TargetAC
		p.RollNeeded = &needed
	}
	return p, nil
}

// resolveWeapon is the weapon with id, else the first of the list; ok is
// false for an empty list.
func resolveWeapon(weapons []Weapon, id string) (*Weapon, bool) {
	if w, ok := findWeapon(weapons, id); ok {
		return &w, true
	}
	if len(weapons) == 0 {
		return nil, false
	}
	w := weapons[0]
	return &w, true
}

func findWeapon(weapons []Weapon, id string) (Weapon, bool) {
	for _, w := range weapons {
		if id != "" && w.ID == id {
			return w, true
		}
	}
	return Weapon{}, false
}

func findItem(items []itemsvc.Item, id string) (itemsvc.Item, bool) {
	for _, it := range items {
		if it.ID == id {
			return it, true
		}
	}
	return itemsvc.Item{}, false
}
