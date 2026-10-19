package itemsvc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ceffo/critical/api/internal/domain/actionlog"
	"github.com/ceffo/critical/api/internal/domain/localized"
)

// StateKeyItems is the action-log state-key family for item counts. Each
// item has its own key, StateKey(id) = "items:<id>", so voiding a use is
// only blocked by later uses of the same item. Services that spend items (an
// attack firing an arrow) record a snapshot with UseSnapshot; the Restorer
// registered for the family writes the "before" counts back on void.
const StateKeyItems = "items"

// The snapshotters of itemsvc's state keys (ADR 0018 step 8): an item, and a
// weapon of before the one equipment model (no longer written). Either may be
// absent: not created yet, or gone.
var (
	itemSnaps   = actionlog.NewSnapshotter[itemState](StateKeyItems).MayBeAbsent()
	weaponSnaps = actionlog.NewSnapshotter[weaponState](StateKeyWeapons).MayBeAbsent()
)

// StateKey is the action-log state key of one item's count.
func StateKey(itemID string) string { return StateKeyItems + actionlog.KeySeparator + itemID }

// itemState is the state of an "items:<id>" snapshot (itemSnaps), the
// envelope's id naming the item. Entries already in the log must stay
// decodable: add fields, never rename. Count is the item's count; Item is
// the whole item as it was (a use's "before" when the use removed it, so a
// pop can put it back; every state of an edit or a remove). Replace marks a
// state whose Item is the whole truth (edit, remove): restoring overwrites
// every field, or inserts the item when it is gone; otherwise only the count
// is written back, and the item is inserted whole only when it is gone.
type itemState struct {
	Count   int       `json:"count"`
	Item    *snapItem `json:"item,omitempty"`
	Replace bool      `json:"replace,omitempty"`
}

// snapItem is the whole item inside a snapshot (add fields, never rename).
type snapItem struct {
	Position int `json:"position"`
	// Name is the item's name by language (ADR 0023 step 11). NamePartial
	// marks a snapshot taken before names were localized: it knew only the
	// fallback body, so restoring it replaces that body and keeps the item's
	// other languages.
	Name        localized.Text `json:"name"`
	NamePartial bool           `json:"namePartial,omitempty"`
	Type        string         `json:"type"`
	Behaviour   string         `json:"behaviour"`
	MagicBonus  int            `json:"magicalBonus"`
	AmmoKind    string         `json:"ammoKind"`
	MaxCount    int            `json:"maxCount"`
	EffectDice  string         `json:"effectDice"`
	EffectSave  string         `json:"effectSave"`
	Notes       string         `json:"notes"`
	// Powers are the item's own powers (ADR 0016 step 15), always written
	// ([] for none); nil in a snapshot taken before they existed, which then
	// leaves the stored powers as they are (see keepCurrent).
	Powers *[]jsonPower `json:"powers,omitempty"`
	// Piece holds the fields of the one equipment model (ADR 0033 steps 1
	// to 6); nil in a snapshot taken before it, whose item then keeps the
	// piece's own (or is converted, when it is gone: step 26.2).
	Piece *snapPiece `json:"piece,omitempty"`
}

// snapPiece is the part of a piece the items of before the model did not
// have (add fields, never rename).
type snapPiece struct {
	Category      string   `json:"category"`
	MagicKind     string   `json:"magicKind,omitempty"`
	CatalogueCode string   `json:"catalogueCode,omitempty"`
	Component     string   `json:"component,omitempty"`
	State         string   `json:"state"`
	DamageTypes   []string `json:"damageTypes,omitempty"`
	DamageSM      string   `json:"damageSm,omitempty"`
	DamageL       string   `json:"damageL,omitempty"`
	WeaponType    string   `json:"weaponType,omitempty"`
	WeaponKind    string   `json:"weaponKind,omitempty"`
	Size          string   `json:"size,omitempty"`
	Speed         int      `json:"speed,omitempty"`
	MagicToHit    int      `json:"magicToHit,omitempty"`
	MagicDamage   int      `json:"magicDamage,omitempty"`
	TwoHanded     bool     `json:"twoHanded,omitempty"`
	Fires         string   `json:"fires,omitempty"`
	Specialized   bool     `json:"specialized,omitempty"`
	BaseAC        int      `json:"baseAc,omitempty"`
	SkillColumn   string   `json:"skillColumn,omitempty"`
	ArmorRow      string   `json:"armorRow,omitempty"`
	ShieldKind    string   `json:"shieldKind,omitempty"`
	ACMelee       int      `json:"acMelee,omitempty"`
	ACMissile     int      `json:"acMissile,omitempty"`
}

func toSnapItem(i Item) *snapItem {
	powers := toJSONPowers(i.Powers)
	return &snapItem{
		Position: i.Position, Name: i.Name, Type: i.Type, Behaviour: string(i.Behaviour),
		MagicBonus: i.MagicBonus, AmmoKind: i.AmmoKind, MaxCount: i.MaxCount,
		EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Powers: &powers,
		Piece: &snapPiece{
			Category: string(i.Category), MagicKind: i.MagicKind, CatalogueCode: i.CatalogueCode,
			Component: i.Component, State: string(i.State), DamageTypes: i.DamageTypes, DamageSM: i.DamageSM, DamageL: i.DamageL,
			WeaponType: i.WeaponType, WeaponKind: i.WeaponKind, Size: i.Size, Speed: i.Speed,
			MagicToHit: i.MagicToHit, MagicDamage: i.MagicDamage, TwoHanded: i.TwoHanded, Fires: i.Fires,
			Specialized: i.Specialized, BaseAC: i.BaseAC, SkillColumn: i.SkillColumn, ArmorRow: i.ArmorRow,
			ShieldKind: i.ShieldKind, ACMelee: i.ACMelee, ACMissile: i.ACMissile,
		},
	}
}

func (c itemState) item(id string) Item {
	it := c.Item
	out := Item{
		ID: id, Position: it.Position, Name: it.Name, Type: it.Type, Behaviour: Behaviour(it.Behaviour),
		MagicBonus: it.MagicBonus, AmmoKind: it.AmmoKind, Count: c.Count, MaxCount: it.MaxCount,
		EffectDice: it.EffectDice, EffectSave: it.EffectSave, Notes: it.Notes,
	}
	if it.Powers != nil {
		out.Powers = fromJSONPowers(*it.Powers)
	}
	if p := it.Piece; p != nil {
		out.Category, out.MagicKind, out.CatalogueCode = Category(p.Category), p.MagicKind, p.CatalogueCode
		out.Component = p.Component
		out.State, out.DamageTypes, out.DamageSM, out.DamageL = State(p.State), p.DamageTypes, p.DamageSM, p.DamageL
		out.WeaponType, out.WeaponKind, out.Size, out.Speed = p.WeaponType, p.WeaponKind, p.Size, p.Speed
		out.MagicToHit, out.MagicDamage, out.TwoHanded, out.Fires = p.MagicToHit, p.MagicDamage, p.TwoHanded, p.Fires
		out.Specialized, out.BaseAC, out.SkillColumn, out.ArmorRow = p.Specialized, p.BaseAC, p.SkillColumn, p.ArmorRow
		out.ShieldKind, out.ACMelee, out.ACMissile = p.ShieldKind, p.ACMelee, p.ACMissile
	}
	return out
}

// withPiece is item, a snapshot's item of before the model, with the
// model's fields of cur (the piece of that id now), or converted when there
// is none (ADR 0033 step 26.2).
func withPiece(item Item, cur *Item) Item {
	if cur == nil {
		piece, _ := ConvertItem(item)
		return piece
	}
	out := *cur
	out.ID, out.Position, out.Name, out.Type, out.Behaviour = item.ID, item.Position, item.Name, item.Type, item.Behaviour
	out.MagicBonus, out.AmmoKind, out.Count, out.MaxCount = item.MagicBonus, item.AmmoKind, item.Count, item.MaxCount
	out.EffectDice, out.EffectSave, out.Notes, out.Powers = item.EffectDice, item.EffectSave, item.Notes, item.Powers
	if cur.Category == CategoryAmmunition {
		// The piece's ammo kind is the conversion's reading of the stored one.
		if k, ok := ammoKind(item.AmmoKind); ok && k != "" {
			out.AmmoKind = k
		}
	}
	return out
}

// keepCurrent is the item a Replace snapshot writes back: when the snapshot
// predates powers (no "powers" key), the item keeps the powers it has now,
// so popping an old edit never strips the ones a migration gave it; when it
// predates localized names (NamePartial), its fallback body is restored and
// the other languages the item has now are kept (ADR 0023 step
// 11); when it predates the equipment model, the piece keeps its category,
// state and the fields of its category, or is converted when it is gone (ADR
// 0033 step 26.2). An item that is gone has nothing else to keep (ADR 0016
// Consequences).
func (s *service) keepCurrent(ctx context.Context, characterID int64, id string, c itemState) (Item, error) {
	item := c.item(id)
	if c.Item.Powers != nil && !c.Item.NamePartial && c.Item.Piece != nil {
		return item, nil
	}
	cur, err := s.dal.Get(ctx, characterID, id)
	switch {
	case errors.Is(err, ErrItemNotFound):
		if c.Item.Piece == nil {
			return withPiece(item, nil), nil
		}
		return item, nil
	case err != nil:
		return Item{}, err
	}
	if c.Item.Powers == nil {
		item.Powers = cur.Powers
	}
	if c.Item.NamePartial {
		item.Name = cur.Name.With(c.Item.Name)
	}
	if c.Item.Piece == nil {
		item = withPiece(item, &cur)
	}
	return item, nil
}

// UseSnapshot is the snapshot of one Use: after is the item Use returned,
// the "before" count is one more (Use always spends exactly one). When the
// use removed the item (a charges item spent to 0), "before" carries the
// whole item, so popping the use brings it back with 1 charge, and "after"
// is absent.
func UseSnapshot(after Item) (actionlog.Snapshot, error) {
	before := itemState{Count: after.Count + 1}
	if after.Removed() {
		before.Item = toSnapItem(after)
		return itemSnaps.Snapshot(after.ID, &before, nil)
	}
	return itemSnaps.Snapshot(after.ID, &before, &itemState{Count: after.Count})
}

// CountSnapshot is the snapshot of one item's count going from before to
// after (a long rest refilling a daily item, a restock).
func CountSnapshot(itemID string, before, after int) (actionlog.Snapshot, error) {
	return itemSnaps.Snapshot(itemID, &itemState{Count: before}, &itemState{Count: after})
}

// itemSnapshot is the snapshot of an item going from before to after by hand
// (nil = it did not exist / no longer exists). Restoring "before" undoes it:
// a nil before deletes the item, any other before overwrites it or puts it
// back.
func itemSnapshot(itemID string, before, after *Item) (actionlog.Snapshot, error) {
	return itemSnaps.Snapshot(itemID, wholeItem(before), wholeItem(after))
}

func wholeItem(i *Item) *itemState {
	if i == nil {
		return nil
	}
	return &itemState{Count: i.Count, Item: toSnapItem(*i), Replace: true}
}

// Restorers writes an item or a weapon of before the model back (registered
// for the StateKeyItems and StateKeyWeapons families, so they serve every
// "items:<id>" and "weapons:<id>" key). An absent state deletes the thing.
// An item that is gone (a charges item its use removed) is inserted back
// whole when the state carries it. It runs inside the void's transaction (ctx
// carries it). The restored item is the entry's character's.
func (s *service) Restorers() map[string]actionlog.Restorer {
	remove := func(ctx context.Context, characterID int64, id string) error {
		if id == "" {
			return errors.New("decode items snapshot: missing id")
		}
		return s.dal.Delete(ctx, characterID, id)
	}
	return map[string]actionlog.Restorer{
		StateKeyWeapons: weaponSnaps.Restorer(s.restoreLegacyWeapon, remove),
		StateKeyItems:   itemSnaps.Restorer(s.restoreItem, remove),
	}
}

func (s *service) restoreItem(ctx context.Context, characterID int64, id string, c itemState) error {
	if id == "" || c.Count < 0 {
		return fmt.Errorf("decode items snapshot: invalid item %q count %d", id, c.Count)
	}
	if c.Replace && c.Item != nil {
		item, err := s.keepCurrent(ctx, characterID, id, c)
		if err != nil {
			return err
		}
		err = s.dal.Update(ctx, characterID, item)
		if errors.Is(err, ErrItemNotFound) {
			err = s.dal.Insert(ctx, characterID, item)
		}
		return err
	}
	// The item is inserted only when it is gone, so there are no stored
	// powers to keep: a snapshot from before powers brings it back without
	// them (ADR 0016 Consequences).
	err := s.dal.SetCount(ctx, characterID, id, c.Count)
	if errors.Is(err, ErrItemNotFound) && c.Item != nil {
		item := c.item(id)
		if c.Item.Piece == nil {
			item = withPiece(item, nil)
		}
		err = s.dal.Insert(ctx, characterID, item)
	}
	return err
}

// StateKeyWeapons is the state-key family of the weapons of before the one
// equipment model ("weapons:<id>"). No longer written; its Restorer puts an
// old weapon entry's snapshot back into the piece of that id (ADR 0033 step
// 26.2).
const StateKeyWeapons = "weapons"

// weaponState is the state of a "weapons:<id>" snapshot as written before
// the model (the envelope's id naming the weapon). NamePartial marks one
// taken before names were localized (see snapItem).
type weaponState struct {
	Position    int            `json:"position"`
	Name        localized.Text `json:"name"`
	NamePartial bool           `json:"namePartial,omitempty"`
	DamageSM    string         `json:"damageSm"`
	DamageL     string         `json:"damageL"`
	MagicToHit  int            `json:"magicToHit"`
	MagicDamage int            `json:"magicDamage"`
	Kind        string         `json:"kind"`
	TwoHanded   bool           `json:"twoHanded"`
	AmmoKind    string         `json:"ammoKind"`
	Type        string         `json:"weaponType"`
	Specialized bool           `json:"specialized,omitempty"`
}

// restoreLegacyWeapon writes a "weapons:<id>" snapshot back: the piece of
// that id takes the weapon's stats and keeps its place, category fields and
// state, or, when it is gone, the weapon comes back converted (ADR 0033 step
// 26.4), last in the list. An absent weapon deletes the piece (Restorers).
func (s *service) restoreLegacyWeapon(ctx context.Context, characterID int64, id string, c weaponState) error {
	if id == "" {
		return errors.New("decode weapons snapshot: missing id")
	}
	w := LegacyWeapon{
		ID: id, Name: c.Name, DamageSM: c.DamageSM, DamageL: c.DamageL, MagicToHit: c.MagicToHit,
		MagicDamage: c.MagicDamage, Kind: c.Kind, TwoHanded: c.TwoHanded, AmmoKind: c.AmmoKind, Type: c.Type,
		Specialized: c.Specialized,
	}
	cur, err := s.dal.Get(ctx, characterID, id)
	switch {
	case errors.Is(err, ErrItemNotFound):
		piece, _ := ConvertWeapon(w, s.catalogue)
		all, err := s.dal.List(ctx, characterID)
		if err != nil {
			return err
		}
		for _, o := range all {
			if o.Position >= piece.Position {
				piece.Position = o.Position + 1
			}
		}
		return s.dal.Insert(ctx, characterID, piece)
	case err != nil:
		return err
	}
	piece := cur
	piece.Name = c.Name
	if c.NamePartial {
		piece.Name = cur.Name.With(c.Name)
	}
	fires, _ := ammoKind(w.AmmoKind)
	piece.DamageSM, piece.DamageL, piece.MagicToHit, piece.MagicDamage = w.DamageSM, w.DamageL, w.MagicToHit, w.MagicDamage
	piece.WeaponKind, piece.TwoHanded, piece.Fires, piece.WeaponType = w.Kind, w.TwoHanded, fires, w.Type
	piece.Specialized = w.Specialized
	return s.dal.Update(ctx, characterID, piece)
}
