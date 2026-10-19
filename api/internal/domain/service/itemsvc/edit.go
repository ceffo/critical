package itemsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// ItemInput is a new piece (ADR 0033 step 24), typed or picked. MaxCount is
// the daily number (daily items only; for the others it is stored as the
// starting count, informational). The fields of the other categories are
// ignored. A new piece is carried (step 6), unless Equip.
type ItemInput struct {
	// CatalogueCode picks the piece from the catalogue row of that code: the
	// row's fields are copied and only the name, count, notes and magic are
	// read from the input (picked). Empty: typed (homebrew).
	CatalogueCode string
	// Equip equips the new piece in the same transaction (ADR 0033 step 24's
	// "wear it now"): one equipment_state entry after the item_add, popped
	// with it by one undo; a body armour or shield worn in the way is taken
	// off (swap). A piece that cannot be equipped is refused (field equip).
	Equip bool
	// Name holds the languages the player typed (ADR 0023 step 11).
	Name      localized.Text
	Category  Category
	MagicKind string
	Component string
	// SpellID is the spell a scroll holds (a magic scroll only).
	SpellID    string
	Behaviour  Behaviour
	Count      int
	MaxCount   int
	EffectDice string
	EffectSave string
	Notes      string
	// Powers are the piece's powers as the editor sets them (ADR 0033 step
	// 17), picked or typed; one without an id gets power_<n>.
	Powers []Power
	// Ammunition.
	AmmoKind   string
	MagicBonus int
	// Weapon and ammunition.
	DamageTypes []string
	DamageSM    string
	DamageL     string
	// Weapon.
	WeaponType  string
	WeaponKind  string
	Size        string
	Speed       int
	MagicToHit  int
	MagicDamage int
	TwoHanded   bool
	Fires       string
	Specialized bool
	// Armour.
	BaseAC      int
	SkillColumn string
	ArmorRow    string
	// Shield.
	ShieldKind string
	ACMelee    int
	ACMissile  int
}

// ItemPatch is an edit: only the set (non-nil) fields change. The state is
// not edited here (SetState); the catalogue code is set by the conversion
// only, for now.
type ItemPatch struct {
	// Name replaces the bodies of the languages it holds and keeps the
	// others; a blank body removes its language; nil: unchanged (ADR 0023
	// step 11).
	Name      localized.Text
	Category  *Category
	MagicKind *string
	Component *string
	// SpellID names the spell a scroll holds; "" clears it. A piece that
	// stops being a magic scroll drops it unless the patch sets it.
	SpellID    *string
	Behaviour  *Behaviour
	Count      *int
	MaxCount   *int
	EffectDice *string
	EffectSave *string
	Notes      *string
	// Powers replaces the whole power list (ADR 0033 step 17); nil:
	// unchanged. One without an id gets power_<n>.
	Powers      *[]Power
	AmmoKind    *string
	MagicBonus  *int
	DamageTypes *[]string
	DamageSM    *string
	DamageL     *string
	WeaponType  *string
	WeaponKind  *string
	Size        *string
	Speed       *int
	MagicToHit  *int
	MagicDamage *int
	TwoHanded   *bool
	Fires       *string
	Specialized *bool
	BaseAC      *int
	SkillColumn *string
	ArmorRow    *string
	ShieldKind  *string
	ACMelee     *int
	ACMissile   *int
}

// ChangeResult is one add, edit or remove: the item as it is now (as it was,
// for a remove) and the log entry written.
type ChangeResult struct {
	Item       Item
	LogEntryID int64
}

// FieldChange is one edited field in an item_edit payload (values as text).
type FieldChange struct {
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// changeRecord is the item_add / item_edit / item_remove log payload (camelCase,
// the /api/items wire names). Changes only in item_edit. Add fields, never
// rename: entries logged before the equipment model have no category (the
// web reads them as before).
type changeRecord struct {
	ItemID    string `json:"itemId"`
	Name      string `json:"name"`
	Category  string `json:"category,omitempty"`
	MagicKind string `json:"magicKind,omitempty"`
	// CatalogueCode is the catalogue row a piece was picked from (ADR 0033
	// step 24), absent for a typed one and on entries logged before it.
	CatalogueCode string        `json:"catalogueCode,omitempty"`
	Component     string        `json:"component,omitempty"`
	Type          string        `json:"type"`
	Behaviour     string        `json:"behaviour"`
	Count         int           `json:"count"`
	MaxCount      int           `json:"maxCount"`
	MagicBonus    int           `json:"magicalBonus"`
	AmmoKind      string        `json:"ammoKind"`
	EffectDice    string        `json:"effectDice"`
	EffectSave    string        `json:"effectSave"`
	Notes         string        `json:"notes"`
	Changes       []FieldChange `json:"changes,omitempty"`
}

func record(i Item, changes []FieldChange) changeRecord {
	return changeRecord{
		ItemID: i.ID, Name: i.Name.Any(), Category: string(i.Category), MagicKind: i.MagicKind, CatalogueCode: i.CatalogueCode,
		Component: i.Component, Type: i.Type, Behaviour: string(i.Behaviour), Count: i.Count,
		MaxCount: i.MaxCount, MagicBonus: i.MagicBonus, AmmoKind: i.AmmoKind,
		EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Changes: changes,
	}
}

// slug turns a name into an id: lower-case letters and digits, "_" between.
func slug(name string) string {
	var b strings.Builder
	sep := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if sep && b.Len() > 0 {
				b.WriteByte('_')
			}
			sep = false
			b.WriteRune(r)
		} else {
			sep = true
		}
	}
	if b.Len() == 0 {
		return "item"
	}
	return b.String()
}

// logChange records one entry with the item's snapshot.
func (s *service) logChange(ctx context.Context, characterID int64, kind actionlog.Kind, rec changeRecord, before, after *Item) (int64, error) {
	snap, err := itemSnapshot(rec.ItemID, before, after)
	if err != nil {
		return 0, err
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return 0, fmt.Errorf("encode %s payload: %w", kind, err)
	}
	id, err := s.log.Record(ctx, actionlog.NewEntry{
		CharacterID: characterID, Kind: kind, Payload: payload,
		Snapshots: []actionlog.Snapshot{snap},
	})
	if err != nil {
		return 0, fmt.Errorf("record %s: %w", kind, err)
	}
	return id, nil
}

func (s *service) Create(ctx context.Context, characterID int64, in ItemInput) (*ChangeResult, error) {
	if err := nameLanguages(in.Name); err != nil {
		return nil, err
	}
	item, err := s.newPiece(in)
	if err != nil {
		return nil, err
	}
	if item.Behaviour != BehaviourDaily {
		item.MaxCount = item.Count
	}
	if err := item.Validate(); err != nil {
		return nil, err
	}
	if in.Equip && item.EquipState() == "" {
		return nil, fieldErr("equip", CodeNotAllowed)
	}
	if err := s.checkSpecialized(ctx, characterID, item); err != nil {
		return nil, err
	}
	if err := s.knownSpell(ctx, item); err != nil {
		return nil, err
	}
	var res *ChangeResult
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		all, err := s.dal.List(ctx, characterID)
		if err != nil {
			return err
		}
		taken, pos := map[string]bool{}, 0
		for _, o := range all {
			taken[o.ID] = true
			if o.Position >= pos {
				pos = o.Position + 1
			}
		}
		base := slug(item.Name.Any())
		item.ID = base
		for n := 2; taken[item.ID]; n++ {
			item.ID = base + "_" + strconv.Itoa(n)
		}
		item.Position = pos
		if err := s.dal.Insert(ctx, characterID, item); err != nil {
			return err
		}
		// A review row left by a removed piece of the same id is not this one's.
		if err := s.dal.DeleteReviews(ctx, characterID, item.ID); err != nil {
			return err
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindItemAdd, record(item, nil), nil, &item)
		if err != nil {
			return err
		}
		res = &ChangeResult{Item: item, LogEntryID: logID}
		if in.Equip {
			moved, err := s.setState(ctx, characterID, item.ID, item.EquipState(), true)
			if err != nil {
				return err
			}
			res.Item = moved.Items[0]
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// newPiece is the piece in describes: picked from the catalogue row of its
// code, or typed.
func (s *service) newPiece(in ItemInput) (Item, error) {
	if in.CatalogueCode != "" {
		return s.picked(in)
	}
	return clean(Item{
		Name: in.Name, Category: in.Category, MagicKind: in.MagicKind, Component: in.Component, SpellID: in.SpellID, State: StateCarried,
		Behaviour: in.Behaviour, Count: in.Count, MaxCount: in.MaxCount,
		EffectDice: in.EffectDice, EffectSave: in.EffectSave, Notes: in.Notes, Powers: withPowerIDs(in.Powers),
		AmmoKind: in.AmmoKind, MagicBonus: in.MagicBonus,
		DamageTypes: in.DamageTypes, DamageSM: in.DamageSM, DamageL: in.DamageL,
		WeaponType: in.WeaponType, WeaponKind: in.WeaponKind, Size: in.Size, Speed: in.Speed,
		MagicToHit: in.MagicToHit, MagicDamage: in.MagicDamage, TwoHanded: in.TwoHanded, Fires: in.Fires,
		Specialized: in.Specialized,
		BaseAC:      in.BaseAC, SkillColumn: in.SkillColumn, ArmorRow: in.ArmorRow,
		ShieldKind: in.ShieldKind, ACMelee: in.ACMelee, ACMissile: in.ACMissile,
	}), nil
}

// clean trims the typed text and lower-cases the codes and dice.
func clean(i Item) Item {
	low := func(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
	i.Name = i.Name.Trimmed()
	i.MagicKind, i.Component, i.SpellID = low(i.MagicKind), low(i.Component), low(i.SpellID)
	i.EffectDice = strings.TrimSpace(i.EffectDice)
	i.Notes = strings.TrimSpace(i.Notes)
	i.AmmoKind = low(i.AmmoKind)
	i.DamageSM, i.DamageL = low(i.DamageSM), low(i.DamageL)
	i.WeaponType, i.WeaponKind, i.Size, i.Fires = low(i.WeaponType), low(i.WeaponKind), low(i.Size), low(i.Fires)
	i.SkillColumn, i.ArmorRow, i.ShieldKind = low(i.SkillColumn), low(i.ArmorRow), low(i.ShieldKind)
	var types []string // none: nil
	for _, t := range i.DamageTypes {
		types = append(types, low(t))
	}
	i.DamageTypes = types
	return i
}

func (s *service) Update(ctx context.Context, characterID int64, itemID string, p ItemPatch) (*ChangeResult, error) {
	if err := nameLanguages(p.Name); err != nil {
		return nil, err
	}
	var res *ChangeResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.dal.Get(ctx, characterID, itemID)
		if err != nil {
			return err
		}
		item := patched(old, p)
		// Becoming daily needs an explicit daily number: the stored max of a stock or
		// charges item is informational.
		if item.Behaviour == BehaviourDaily && old.Behaviour != BehaviourDaily && p.MaxCount == nil {
			item.MaxCount = 0
		}
		// A non-daily item's max is informational: keep it from going below the count.
		if item.Behaviour != BehaviourDaily && p.MaxCount == nil && item.Count > item.MaxCount {
			item.MaxCount = item.Count
		}
		// Switching a weapon to melee drops what it fires unless the edit sets
		// it (which Validate then refuses).
		if p.WeaponKind != nil && item.WeaponKind == WeaponKindMelee && p.Fires == nil {
			item.Fires = ""
		}
		// A new category that cannot keep the piece's state puts it back in
		// the pack (ADR 0033 step 6); the snapshot holds the state, so a pop
		// gives it back.
		if item.Category != old.Category || item.MagicKind != old.MagicKind {
			free, err := s.slotFree(ctx, characterID, item)
			if err != nil {
				return err
			}
			if !Allows(item.Category, item.MagicKind, item.State) || !free {
				item.State = StateCarried
			}
		}
		if err := item.Validate(); err != nil {
			return err
		}
		if err := s.checkSpecialized(ctx, characterID, item); err != nil {
			return err
		}
		if item.SpellID != old.SpellID {
			if err := s.knownSpell(ctx, item); err != nil {
				return err
			}
		}
		changes := diff(old, item)
		if len(changes) == 0 {
			return fmt.Errorf("%w: nothing changed", domain.ErrInvalidInput)
		}
		if err := s.dal.Update(ctx, characterID, item); err != nil {
			return err
		}
		// A piece the player classified is settled: it leaves the review
		// list (ADR 0033 step 27).
		if settles(changes) {
			if err := s.dal.DeleteReviews(ctx, characterID, item.ID); err != nil {
				return err
			}
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindItemEdit, record(item, changes), &old, &item)
		if err != nil {
			return err
		}
		res = &ChangeResult{Item: item, LogEntryID: logID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// patched is old with the set fields of p, cleaned.
func patched(old Item, p ItemPatch) Item {
	item := old
	set := func(dst, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	setInt := func(dst, v *int) {
		if v != nil {
			*dst = *v
		}
	}
	setBool := func(dst, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	if p.Name != nil {
		item.Name = old.Name.With(p.Name)
	}
	if p.Category != nil {
		item.Category = *p.Category
		if item.Category != CategoryMagic && p.MagicKind == nil {
			item.MagicKind = ""
		}
	}
	if p.Behaviour != nil {
		item.Behaviour = *p.Behaviour
	}
	if p.DamageTypes != nil {
		item.DamageTypes = *p.DamageTypes
	}
	if p.Powers != nil {
		item.Powers = withPowerIDs(*p.Powers)
	} else if p.Category != nil && item.Category != CategoryArmour && item.Category != CategoryShield {
		// The "+N" is an armour's or a shield's (ADR 0033 step 17): a piece
		// that stops being one drops it, logged with the edit.
		item.Powers = slices.DeleteFunc(slices.Clone(item.Powers), func(pw Power) bool { return pw.ID == MagicPowerID })
		if len(item.Powers) == 0 {
			item.Powers = nil
		}
	}
	set(&item.MagicKind, p.MagicKind)
	set(&item.Component, p.Component)
	set(&item.SpellID, p.SpellID)
	// A piece that stops being a magic scroll holds no spell (logged with
	// the edit); one the patch names is refused by Validate.
	if p.SpellID == nil && !item.HoldsSpell() {
		item.SpellID = ""
	}
	set(&item.EffectDice, p.EffectDice)
	set(&item.EffectSave, p.EffectSave)
	set(&item.Notes, p.Notes)
	set(&item.AmmoKind, p.AmmoKind)
	set(&item.DamageSM, p.DamageSM)
	set(&item.DamageL, p.DamageL)
	set(&item.WeaponType, p.WeaponType)
	set(&item.WeaponKind, p.WeaponKind)
	set(&item.Size, p.Size)
	set(&item.Fires, p.Fires)
	set(&item.SkillColumn, p.SkillColumn)
	set(&item.ArmorRow, p.ArmorRow)
	set(&item.ShieldKind, p.ShieldKind)
	setInt(&item.Count, p.Count)
	setInt(&item.MaxCount, p.MaxCount)
	setInt(&item.MagicBonus, p.MagicBonus)
	setInt(&item.Speed, p.Speed)
	setInt(&item.MagicToHit, p.MagicToHit)
	setInt(&item.MagicDamage, p.MagicDamage)
	setInt(&item.BaseAC, p.BaseAC)
	setInt(&item.ACMelee, p.ACMelee)
	setInt(&item.ACMissile, p.ACMissile)
	setBool(&item.TwoHanded, p.TwoHanded)
	setBool(&item.Specialized, p.Specialized)
	return clean(item)
}

// classifying are the item_edit fields that settle a piece on the review
// list (ADR 0033 step 27: a category, a catalogue row, its category's codes).
var classifying = map[string]bool{
	"category": true, "magicKind": true, "catalogueCode": true, "weaponType": true, "weaponKind": true,
	"size": true, "damageTypes": true, "speed": true, "ammoKind": true, "fires": true,
	"skillColumn": true, "armorRow": true, "shieldKind": true,
}

func settles(changes []FieldChange) bool {
	for _, c := range changes {
		if classifying[c.Field] {
			return true
		}
	}
	return false
}

// checkSpecialized refuses a specialized weapon (ADR 0030 step 12) for a
// character whose class does not specialize: field "specialized", code
// CodeNotAllowed.
func (s *service) checkSpecialized(ctx context.Context, characterID int64, i Item) error {
	if i.Category != CategoryWeapon || !i.Specialized {
		return nil
	}
	c, err := s.chars.GetCharacter(ctx, characterID)
	if err != nil {
		return fmt.Errorf("get character: %w", err)
	}
	if !c.Derived.Specializes {
		return fieldErr("specialized", CodeNotAllowed)
	}
	return nil
}

// knownSpell refuses a scroll naming a spell the catalogue does not know
// (ADR 0033 step 2): field "spellId", code CodeUnknown. None is fine.
func (s *service) knownSpell(ctx context.Context, i Item) error {
	if i.SpellID == "" {
		return nil
	}
	if s.spells == nil {
		return fieldErr("spellId", CodeUnknown)
	}
	_, found, err := s.spells.Describe(ctx, i.SpellID)
	if err != nil {
		return fmt.Errorf("describe %s: %w", i.SpellID, err)
	}
	if !found {
		return fieldErr("spellId", CodeUnknown)
	}
	return nil
}

func (s *service) Delete(ctx context.Context, characterID int64, itemID string) (*ChangeResult, error) {
	var res *ChangeResult
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		old, err := s.dal.Get(ctx, characterID, itemID)
		if err != nil {
			return err
		}
		if err := s.dal.Delete(ctx, characterID, itemID); err != nil {
			return err
		}
		logID, err := s.logChange(ctx, characterID, actionlog.KindItemRemove, record(old, nil), &old, nil)
		if err != nil {
			return err
		}
		res = &ChangeResult{Item: old, LogEntryID: logID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// diff lists the fields that differ, in form order (wire names).
func diff(a, b Item) []FieldChange {
	var out []FieldChange
	add := func(field, x, y string) {
		if x != y {
			out = append(out, FieldChange{Field: field, Before: x, After: y})
		}
	}
	num := strconv.Itoa
	flag := strconv.FormatBool
	// One change per language whose name body differs, in code order.
	langs := maps.Clone(a.Name)
	if langs == nil {
		langs = localized.Text{}
	}
	maps.Copy(langs, b.Name)
	for _, lang := range langs.Languages() {
		add("name", a.Name[lang], b.Name[lang])
	}
	add("category", string(a.Category), string(b.Category))
	add("magicKind", a.MagicKind, b.MagicKind)
	add("catalogueCode", a.CatalogueCode, b.CatalogueCode)
	add("component", a.Component, b.Component)
	add("spellId", a.SpellID, b.SpellID)
	add("state", string(a.State), string(b.State))
	add("type", a.Type, b.Type)
	add("behaviour", string(a.Behaviour), string(b.Behaviour))
	add("count", num(a.Count), num(b.Count))
	add("maxCount", num(a.MaxCount), num(b.MaxCount))
	add("magicalBonus", num(a.MagicBonus), num(b.MagicBonus))
	add("ammoKind", a.AmmoKind, b.AmmoKind)
	add("effectDice", a.EffectDice, b.EffectDice)
	add("effectSave", a.EffectSave, b.EffectSave)
	add("notes", a.Notes, b.Notes)
	add("powers", powersText(a.Powers), powersText(b.Powers))
	add("damageTypes", strings.Join(a.DamageTypes, "/"), strings.Join(b.DamageTypes, "/"))
	add("damageSm", a.DamageSM, b.DamageSM)
	add("damageL", a.DamageL, b.DamageL)
	add("weaponType", a.WeaponType, b.WeaponType)
	add("weaponKind", a.WeaponKind, b.WeaponKind)
	add("size", a.Size, b.Size)
	add("speed", num(a.Speed), num(b.Speed))
	add("magicToHit", num(a.MagicToHit), num(b.MagicToHit))
	add("magicDamage", num(a.MagicDamage), num(b.MagicDamage))
	add("twoHanded", flag(a.TwoHanded), flag(b.TwoHanded))
	add("fires", a.Fires, b.Fires)
	add("specialized", flag(a.Specialized), flag(b.Specialized))
	add("baseAc", num(a.BaseAC), num(b.BaseAC))
	add("skillColumn", a.SkillColumn, b.SkillColumn)
	add("armorRow", a.ArmorRow, b.ArmorRow)
	add("shieldKind", a.ShieldKind, b.ShieldKind)
	add("acMelee", num(a.ACMelee), num(b.ACMelee))
	add("acMissile", num(a.ACMissile), num(b.ACMissile))
	return out
}

// powersText is a power list as an item_edit change value (ADR 0033 step 17):
// its stored JSON encoding ("[]" for none), which the web words.
func powersText(powers []Power) string {
	text, err := EncodePowers(powers)
	if err != nil {
		return ""
	}
	return text
}
