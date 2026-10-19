package itemsvc

import (
	"context"
	"encoding/json"
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

// StateKey is the action-log state key of one item's count.
func StateKey(itemID string) string { return StateKeyItems + actionlog.KeySeparator + itemID }

// snapCount is itemsvc's JSON encoding of one item count inside a snapshot.
// Entries already in the log must stay decodable: add fields, never rename.
// Item is the whole item as it was (a use's "before"), so a pop can put back
// an item the use removed. Removed marks an "after" in which the item is gone.
type snapCount struct {
	ID      string    `json:"id"`
	Count   int       `json:"count"`
	Item    *snapItem `json:"item,omitempty"`
	Removed bool      `json:"removed,omitempty"`
	// Absent marks a "before" in which the item did not exist yet (an item
	// added by hand): restoring it deletes the item.
	Absent bool `json:"absent,omitempty"`
	// Replace marks a snapshot whose Item is the whole truth (edit, remove):
	// restoring overwrites every field, or inserts the item when it is gone.
	Replace bool `json:"replace,omitempty"`
	// Model is snapshotModel on every snapshot written by the one equipment
	// model; 0 on one written before, whose id may have been renamed by the
	// conversion (ADR 0033 step 26.3).
	Model int `json:"model,omitempty"`
}

// snapshotModel marks the snapshots of the one equipment model.
const snapshotModel = 2

// snapItem is the whole item inside a snapshot (add fields, never rename).
type snapItem struct {
	Position int `json:"position"`
	// Name is the item's name by language (ADR 0023 step 11); a snapshot
	// taken before names were localized holds a plain string.
	Name       localized.Snap `json:"name"`
	Type       string         `json:"type"`
	Behaviour  string         `json:"behaviour"`
	MagicBonus int            `json:"magicalBonus"`
	AmmoKind   string         `json:"ammoKind"`
	MaxCount   int            `json:"maxCount"`
	EffectDice string         `json:"effectDice"`
	EffectSave string         `json:"effectSave"`
	Notes      string         `json:"notes"`
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
		Position: i.Position, Name: localized.Snap{Text: i.Name}, Type: i.Type, Behaviour: string(i.Behaviour),
		MagicBonus: i.MagicBonus, AmmoKind: i.AmmoKind, MaxCount: i.MaxCount,
		EffectDice: i.EffectDice, EffectSave: i.EffectSave, Notes: i.Notes, Powers: &powers,
		Piece: &snapPiece{
			Category: string(i.Category), MagicKind: i.MagicKind, CatalogueCode: i.CatalogueCode,
			State: string(i.State), DamageTypes: i.DamageTypes, DamageSM: i.DamageSM, DamageL: i.DamageL,
			WeaponType: i.WeaponType, WeaponKind: i.WeaponKind, Size: i.Size, Speed: i.Speed,
			MagicToHit: i.MagicToHit, MagicDamage: i.MagicDamage, TwoHanded: i.TwoHanded, Fires: i.Fires,
			Specialized: i.Specialized, BaseAC: i.BaseAC, SkillColumn: i.SkillColumn, ArmorRow: i.ArmorRow,
			ShieldKind: i.ShieldKind, ACMelee: i.ACMelee, ACMissile: i.ACMissile,
		},
	}
}

func (c snapCount) item() Item {
	it := c.Item
	out := Item{
		ID: c.ID, Position: it.Position, Name: it.Name.Text, Type: it.Type, Behaviour: Behaviour(it.Behaviour),
		MagicBonus: it.MagicBonus, AmmoKind: it.AmmoKind, Count: c.Count, MaxCount: it.MaxCount,
		EffectDice: it.EffectDice, EffectSave: it.EffectSave, Notes: it.Notes,
	}
	if it.Powers != nil {
		out.Powers = fromJSONPowers(*it.Powers)
	}
	if p := it.Piece; p != nil {
		out.Category, out.MagicKind, out.CatalogueCode = Category(p.Category), p.MagicKind, p.CatalogueCode
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
// predates localized names (a plain string), that string is the fallback
// body and the other languages the item has now are kept (ADR 0023 step
// 11); when it predates the equipment model, the piece keeps its category,
// state and the fields of its category, or is converted when it is gone (ADR
// 0033 step 26.2). An item that is gone has nothing else to keep (ADR 0016
// Consequences).
func (s *service) keepCurrent(ctx context.Context, characterID int64, c snapCount) (Item, error) {
	item := c.item()
	if c.Item.Powers != nil && !c.Item.Name.Plain && c.Item.Piece != nil {
		return item, nil
	}
	cur, err := s.dal.Get(ctx, characterID, c.ID)
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
	if c.Item.Name.Plain {
		item.Name = c.Item.Name.Restored(cur.Name)
	}
	if c.Item.Piece == nil {
		item = withPiece(item, &cur)
	}
	return item, nil
}

// UseSnapshot is the snapshot of one Use: after is the item Use returned,
// the "before" count is one more (Use always spends exactly one). When the
// use removed the item (a charges item spent to 0), "before" carries the
// whole item, so popping the use brings it back with 1 charge.
func UseSnapshot(after Item) (actionlog.Snapshot, error) {
	before := snapCount{ID: after.ID, Count: after.Count + 1, Model: snapshotModel}
	if after.Removed() {
		before.Item = toSnapItem(after)
	}
	return encodeSnapshot(after.ID, before, snapCount{ID: after.ID, Count: after.Count, Removed: after.Removed(), Model: snapshotModel})
}

// CountSnapshot is the snapshot of one item's count going from before to
// after (a long rest refilling a daily item, a restock).
func CountSnapshot(itemID string, before, after int) (actionlog.Snapshot, error) {
	return encodeSnapshot(itemID, snapCount{ID: itemID, Count: before, Model: snapshotModel},
		snapCount{ID: itemID, Count: after, Model: snapshotModel})
}

// itemSnapshot is the snapshot of an item going from before to after by hand
// (nil = it did not exist / no longer exists). Restoring "before" undoes it:
// a nil before deletes the item, any other before overwrites it or puts it
// back.
func itemSnapshot(itemID string, before, after *Item) (actionlog.Snapshot, error) {
	return encodeSnapshot(itemID, wholeItem(itemID, before), wholeItem(itemID, after))
}

func wholeItem(id string, i *Item) snapCount {
	if i == nil {
		return snapCount{ID: id, Absent: true, Removed: true, Model: snapshotModel}
	}
	return snapCount{ID: id, Count: i.Count, Item: toSnapItem(*i), Replace: true, Model: snapshotModel}
}

func encodeSnapshot(itemID string, before, after snapCount) (actionlog.Snapshot, error) {
	b, err := json.Marshal([]snapCount{before})
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode items snapshot: %w", err)
	}
	a, err := json.Marshal([]snapCount{after})
	if err != nil {
		return actionlog.Snapshot{}, fmt.Errorf("encode items snapshot: %w", err)
	}
	return actionlog.Snapshot{Key: StateKey(itemID), Before: b, After: a}, nil
}

// restoreFunc adapts a function to actionlog.Restorer.
type restoreFunc func(ctx context.Context, characterID int64, state json.RawMessage) error

func (f restoreFunc) Restore(ctx context.Context, characterID int64, state json.RawMessage) error {
	return f(ctx, characterID, state)
}

// Restorers writes the counts of an items snapshot back (registered for the
// StateKeyItems family, so it serves every "items:<id>" key). An item that is
// gone (a charges item its use removed) is inserted back whole when the
// snapshot carries it; a state in which the item is gone (Absent or Removed)
// deletes it. It runs inside the void's transaction (ctx carries
// it). The restored item is the entry's character's.
func (s *service) Restorers() map[string]actionlog.Restorer {
	return map[string]actionlog.Restorer{
		StateKeyWeapons: restoreFunc(s.restoreLegacyWeapon),
		StateKeyItems: restoreFunc(func(ctx context.Context, characterID int64, state json.RawMessage) error {
			var counts []snapCount
			if err := json.Unmarshal(state, &counts); err != nil {
				return fmt.Errorf("decode items snapshot: %w", err)
			}
			for _, c := range counts {
				if c.ID == "" || c.Count < 0 {
					return fmt.Errorf("decode items snapshot: invalid count %+v", c)
				}
			}
			for _, c := range counts {
				if c.Model == 0 {
					// An items snapshot of before the model: its item may
					// have been renamed by the conversion (step 26.3).
					id, err := s.dal.RenamedID(ctx, characterID, c.ID)
					if err != nil {
						return fmt.Errorf("restore item %s: %w", c.ID, err)
					}
					c.ID = id
				}
				var err error
				switch {
				case c.Absent, c.Removed:
					// Gone in this state: an add's "before", a remove's
					// "after", the "after" of a use that spent the last
					// charge (written back by a redo, ADR 0019 step 1).
					err = s.dal.Delete(ctx, characterID, c.ID)
				case c.Replace && c.Item != nil:
					var item Item
					if item, err = s.keepCurrent(ctx, characterID, c); err != nil {
						break
					}
					err = s.dal.Update(ctx, characterID, item)
					if errors.Is(err, ErrItemNotFound) {
						err = s.dal.Insert(ctx, characterID, item)
					}
				default:
					// The item is inserted only when it is gone, so there
					// are no stored powers to keep: a snapshot from before
					// powers brings it back without them (ADR 0016
					// Consequences).
					err = s.dal.SetCount(ctx, characterID, c.ID, c.Count)
					if errors.Is(err, ErrItemNotFound) && c.Item != nil {
						item := c.item()
						if c.Item.Piece == nil {
							item = withPiece(item, nil)
						}
						err = s.dal.Insert(ctx, characterID, item)
					}
				}
				if err != nil {
					return fmt.Errorf("restore item %s: %w", c.ID, err)
				}
			}
			return nil
		}),
	}
}

// StateKeyWeapons is the state-key family of the weapons of before the one
// equipment model ("weapons:<id>"). No longer written; its Restorer puts an
// old weapon entry's snapshot back into the piece of that id (ADR 0033 step
// 26.2).
const StateKeyWeapons = "weapons"

// snapWeapon is a "weapons:<id>" snapshot as written before the model.
type snapWeapon struct {
	ID          string         `json:"id"`
	Absent      bool           `json:"absent,omitempty"`
	Position    int            `json:"position"`
	Name        localized.Snap `json:"name"`
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

// restoreLegacyWeapon writes a "weapons:<id>" snapshot back: an absent weapon
// deletes the piece; otherwise the piece of that id takes the weapon's stats
// and keeps its place, category fields and state, or, when it is gone, the
// weapon comes back converted (ADR 0033 step 26.4), last in the list.
func (s *service) restoreLegacyWeapon(ctx context.Context, characterID int64, state json.RawMessage) error {
	var c snapWeapon
	if err := json.Unmarshal(state, &c); err != nil {
		return fmt.Errorf("decode weapons snapshot: %w", err)
	}
	if c.ID == "" {
		return fmt.Errorf("decode weapons snapshot: missing id")
	}
	if c.Absent {
		if err := s.dal.Delete(ctx, characterID, c.ID); err != nil {
			return fmt.Errorf("restore weapon %s: %w", c.ID, err)
		}
		return nil
	}
	w := LegacyWeapon{
		ID: c.ID, Name: c.Name.Text, DamageSM: c.DamageSM, DamageL: c.DamageL, MagicToHit: c.MagicToHit,
		MagicDamage: c.MagicDamage, Kind: c.Kind, TwoHanded: c.TwoHanded, AmmoKind: c.AmmoKind, Type: c.Type,
		Specialized: c.Specialized,
	}
	cur, err := s.dal.Get(ctx, characterID, c.ID)
	switch {
	case errors.Is(err, ErrItemNotFound):
		piece, _ := ConvertWeapon(w, s.catalogue)
		all, err := s.dal.List(ctx, characterID)
		if err != nil {
			return fmt.Errorf("restore weapon %s: %w", c.ID, err)
		}
		for _, o := range all {
			if o.Position >= piece.Position {
				piece.Position = o.Position + 1
			}
		}
		if err := s.dal.Insert(ctx, characterID, piece); err != nil {
			return fmt.Errorf("restore weapon %s: %w", c.ID, err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("restore weapon %s: %w", c.ID, err)
	}
	piece := cur
	piece.Name = c.Name.Restored(cur.Name)
	fires, _ := ammoKind(w.AmmoKind)
	piece.DamageSM, piece.DamageL, piece.MagicToHit, piece.MagicDamage = w.DamageSM, w.DamageL, w.MagicToHit, w.MagicDamage
	piece.WeaponKind, piece.TwoHanded, piece.Fires, piece.WeaponType = w.Kind, w.TwoHanded, fires, w.Type
	piece.Specialized = w.Specialized
	if err := s.dal.Update(ctx, characterID, piece); err != nil {
		return fmt.Errorf("restore weapon %s: %w", c.ID, err)
	}
	return nil
}
