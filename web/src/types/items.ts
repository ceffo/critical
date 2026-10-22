// Mirrors api/internal/rest/handler/itemhdl (ItemDTO, PowerDTO, UseResultDTO, RestockResultDTO, ChangeDTO,
// ItemBody, StateRequest, StateDTO). An item is any piece of equipment (ADR 0033 step 1).

import type { BuffEffects } from "./buffs";
import type { WeaponType } from "./combat";
import type { LocalizedText } from "./localized";

/**
 * How an item's count behaves: stock is used and restocked (stays at 0),
 * charges is used only (gone at 0), daily is used only and refilled by a long rest.
 */
export type ItemBehaviour = "stock" | "charges" | "daily";

/** A piece's category (ADR 0033 step 2), in display order (itemsvc.Categories). */
export const CATEGORIES = [
  "weapon",
  "ammunition",
  "armour",
  "shield",
  "magic",
  "gear",
] as const;
export type Category = (typeof CATEGORIES)[number];

/** A magic piece's kind (ADR 0033 step 2, itemsvc.MagicKinds). */
export const MAGIC_KINDS = [
  "potion",
  "scroll",
  "ring",
  "rod",
  "staff",
  "wand",
  "misc",
] as const;
export type MagicKind = (typeof MAGIC_KINDS)[number];

/** Ammunition kinds (ADR 0033 step 4, itemsvc.AmmoKinds). */
export const AMMO_KINDS = [
  "arrow",
  "quarrel",
  "bullet",
  "needle",
  "blowgun_dart",
] as const;

/**
 * The spell components a piece may be (ADR 0033 step 1.1, itemsvc.Components): a spell that spends
 * one (consume_item, the Chromatic Orb's gem) spends a piece of that code, whatever its category.
 */
export const COMPONENTS = ["gem"] as const;
export type Component = (typeof COMPONENTS)[number];

/** PHB Table 29 columns an armour reads (ADR 0033 step 5, itemsvc.SkillColumns). */
export const SKILL_COLUMNS = [
  "none",
  "leather",
  "padded_studded",
  "elven_chain",
  "metal",
  "none_allowed",
] as const;

/** PHB Table 52 rows (ADR 0033 step 5, ruletablesvc.ArmorRows). */
export const ARMOR_ROWS = [
  "banded",
  "brigandine",
  "chain",
  "field_plate",
  "full_plate",
  "leather",
  "plate",
  "ring",
  "scale",
  "splint",
  "studded_leather",
] as const;

/** Shield kinds (ADR 0033 step 5, itemsvc.ShieldKinds). */
export const SHIELD_KINDS = ["buckler", "small", "medium", "body"] as const;

/** Weapon sizes and damage types (ADR 0033 step 3). */
export const SIZES = ["s", "m", "l"] as const;
export const DAMAGE_TYPES = ["b", "p", "s"] as const;

/**
 * Where a piece is (ADR 0033 step 6). The web says Equipped for worn and wielded alike (ADR 0035
 * item 3); the codes stay on the wire.
 */
export type ItemState = "carried" | "worn" | "wielded";

/**
 * A class warning (ADR 0033 step 20, itemsvc.Warn*): the code the web words; nothing is refused.
 * weapon_type_missing: a wielded weapon with no weapon type, so no proficiency is judged (ADR 0037
 * step 8). dual_class_old_ability: a piece only a dual-class's old class allows, before the new class
 * passes it (ADR 0042 step 13).
 */
export type ClassWarningCode =
  | "armour_not_allowed"
  | "weapon_not_allowed"
  | "hands"
  | "weapon_type_missing"
  | "dual_class_old_ability";

/** Why a piece is on the review list (ADR 0033 step 27, itemsvc.Reason*). */
export type ReviewReason =
  "weapon_unmatched" | "uncategorised" | "ammo_kind_unknown" | "powers_off";

export interface Item {
  id: string;
  /** A language map (ADR 0023 step 9): pick with `localized`. */
  name: LocalizedText;
  category: Category;
  /** A magic piece's kind; empty for the other categories. */
  magicKind: MagicKind | "";
  /** The catalogue row it was picked from or matched to; empty for homebrew. */
  catalogueCode: string;
  /** The spell component it is (ADR 0033 step 1.1); empty for none. */
  component: Component | "";
  /** The catalogue id of the spell a magic scroll holds (ADR 0033 step 2); empty for none. */
  spellId: string;
  state: ItemState;
  /** The state Equip moves the piece to (server rule, ADR 0033 step 6); empty: carried only. */
  equipState: ItemState | "";
  /** The free-text label of before the equipment model, read only (not shown, read by nothing). */
  type: string;
  behaviour: ItemBehaviour;
  /** Magical bonus (+N): to hit and damage when fired as ammo. */
  magicalBonus: number;
  /** Launcher kind the item feeds ("arrow"); empty when it is not ammunition. */
  ammoKind: string;
  count: number;
  /** A daily item's refill value. */
  maxCount: number;
  effectDice: string;
  effectSave: string;
  notes: string;
  /**
   * The item's own powers (ADR 0016 steps 9 to 16), set by the editor (ADR 0033 step 17; an armour's
   * or a shield's "+N" is the power of id "magic", ac -N). Whether one is on is served with the
   * effects list (GET /api/buffs); only while the piece is equipped.
   */
  powers: ItemPowerDef[];
  /** Weapon and ammunition: one or two of b, p, s, and the damage S-M / L (NdM). */
  damageTypes: string[];
  damageSm: string;
  damageL: string;
  /** Weapon (ADR 0033 step 3). */
  weaponType: WeaponType;
  weaponKind: "melee" | "missile" | "";
  size: string;
  speed: number;
  magicToHit: number;
  magicDamage: number;
  twoHanded: boolean;
  /** The ammo kind a launcher fires; empty for melee and hurled weapons. */
  fires: string;
  /**
   * The fighter's weapon specialization (ADR 0030 step 12), read-only: derived from the
   * character's specialization pick of the weapon type (ADR 0037 step 9), never sent.
   */
  specialized: boolean;
  /** ADR 0037 step 8: the to-hit penalty an attack with the weapon takes for its type having no
   * weapon pick (0: none), read-only. */
  nonproficientPenalty: number;
  /** Armour (ADR 0033 step 5). */
  baseAc: number;
  skillColumn: string;
  armorRow: string;
  /** Shield (ADR 0033 step 5). */
  shieldKind: string;
  acMelee: number;
  acMissile: number;
  /** The review reasons (ADR 0033 step 27); empty when the piece is settled. */
  review: ReviewReason[];
  /** The class warnings of the piece (ADR 0033 step 20); empty for none. Served by the list. */
  warnings: ClassWarningCode[];
}

/** One of an item's powers: its trigger (minStr; absent: always on) and its effects. */
export interface ItemPowerDef {
  id: string;
  minStr?: number;
  effects: BuffEffects;
}

/** POST /api/items/{id}/use (itemhdl UseResultDTO); the item_use log payload has the same shape. */
export interface UseResult {
  itemId: string;
  name: string;
  /** Absent in entries logged before the item type was recorded. */
  type?: string;
  behaviour?: ItemBehaviour;
  /** The use spent a charges item's last charge: the item is gone until the use is popped. */
  removed?: boolean;
  remaining: number;
  maxCount: number;
  /** Effect dice ("6d6"); empty when the item rolls nothing. */
  dice: string;
  rolls: number[];
  /** The flat "+K" of the dice ("1d8+2"); absent on uses logged before it was recorded. */
  modifier?: number;
  total: number;
  /** Save category the effect allows; empty when none. */
  save: string;
  notes: string;
  logEntryId?: number;
}

/** POST /api/items/{id}/restock (itemhdl RestockResultDTO); the item_restock log payload has the same shape. */
export interface RestockResult {
  itemId: string;
  name: string;
  type: string;
  added: number;
  count: number;
  logEntryId?: number;
}

/** Body of POST /api/items and PATCH /api/items/{id} (itemhdl ItemBody); a PATCH changes only the fields present. */
export interface ItemBody {
  /** The languages typed (ADR 0023 step 11): the others are kept. */
  name?: LocalizedText;
  category?: Category;
  magicKind?: MagicKind | "";
  component?: Component | "";
  /** The spell a magic scroll holds (a catalogue id); "" clears it. */
  spellId?: string;
  behaviour?: ItemBehaviour;
  count?: number;
  /** The daily number (daily items only). */
  maxCount?: number;
  magicalBonus?: number;
  ammoKind?: string;
  effectDice?: string;
  effectSave?: string;
  notes?: string;
  damageTypes?: string[];
  damageSm?: string;
  damageL?: string;
  weaponType?: WeaponType;
  weaponKind?: "melee" | "missile" | "";
  size?: string;
  speed?: number;
  magicToHit?: number;
  magicDamage?: number;
  twoHanded?: boolean;
  fires?: string;
  baseAc?: number;
  skillColumn?: string;
  armorRow?: string;
  shieldKind?: string;
  acMelee?: number;
  acMissile?: number;
  /**
   * The whole power list (ADR 0033 step 17): a PATCH's replaces the stored one, absent keeps it. A
   * power sent with an empty id gets one. Refused on field `powers` (validated as the seed's).
   */
  powers?: ItemPowerDef[];
  /**
   * POST only: pick the piece from this catalogue row (ADR 0033 step 24). The server copies the row
   * and reads only name, count, notes and the magic (magicToHit and magicDamage, magicalBonus) with
   * it; an unknown code is refused (field `catalogueCode`, code `unknown`).
   */
  catalogueCode?: string;
  /** POST only: equip the new piece in the same undo (a worn armour or shield in the way comes off). */
  equip?: boolean;
}

/** The categories a piece is picked from the catalogue in (ADR 0033 step 24). */
export const PICKED_CATEGORIES = [
  "weapon",
  "ammunition",
  "armour",
  "shield",
] as const;
export type PickedCategory = (typeof PICKED_CATEGORIES)[number];

/** A catalogue weapon (wire.CatalogueWeaponDTO). Damage as printed ("1d4+1", "1"); none for a launcher. */
export interface CatalogueWeapon {
  code: string;
  name: LocalizedText;
  weaponType: WeaponType;
  weaponKind: "melee" | "missile";
  size: string;
  damageTypes: string[];
  speed: number;
  damageSm: string;
  damageL: string;
  twoHanded: boolean;
  fires: string[];
}

/** A catalogue ammunition row (wire.CatalogueAmmunitionDTO). */
export interface CatalogueAmmunition {
  code: string;
  name: LocalizedText;
  ammoKind: string;
  damageTypes: string[];
  damageSm: string;
  damageL: string;
}

/** A catalogue body armour (wire.CatalogueArmourDTO). */
export interface CatalogueArmour {
  code: string;
  name: LocalizedText;
  baseAc: number;
  skillColumn: string;
  armorRow: string;
}

/** A catalogue shield (wire.CatalogueShieldDTO). */
export interface CatalogueShield {
  code: string;
  name: LocalizedText;
  shieldKind: string;
  acMelee: number;
  acMissile: number;
}

/** GET /api/equipment/catalogue (wire.EquipmentCatalogueDTO): every list empty before catalogue format 7. */
export interface EquipmentCatalogue {
  weapons: CatalogueWeapon[];
  ammunition: CatalogueAmmunition[];
  armour: CatalogueArmour[];
  shields: CatalogueShield[];
}

/** POST /api/items/{id}/state (itemhdl StateRequest): swap takes off the armour or shield worn. */
export interface StateBody {
  state: ItemState;
  swap?: boolean;
}

/** POST /api/items/{id}/state (itemhdl StateDTO): the pieces changed, the moved one first. */
export interface StateChange {
  items: Item[];
  /** 0: the piece already was in that state, nothing logged. */
  logEntryId: number;
}

/** One piece's move in an equipment_state payload (itemsvc stateMove). */
export interface StateMove {
  itemId: string;
  /** The name recorded at write time (ADR 0023 step 12). */
  name: string;
  category: Category;
  before: ItemState;
  after: ItemState;
}

/** Payload of equipment_state entries (itemsvc stateRecord, ADR 0033 step 7). */
export interface EquipmentStateRecord extends StateMove {
  /** The pieces a swap took off. */
  off?: StateMove[];
}

/** POST/PATCH/DELETE /api/items (itemhdl ChangeDTO): the item as it is now (as it was, for a delete). */
export interface ItemChange {
  item: Item;
  logEntryId: number;
}

/** One edited field in an item_edit log payload (values as text). */
export interface FieldChange {
  field: string;
  before: string;
  after: string;
}

/** Payload of item_add, item_edit and item_remove log entries (itemsvc changeRecord). */
export interface ItemChangeRecord {
  itemId: string;
  name: string;
  /** Absent in entries logged before the equipment model (ADR 0033). */
  category?: Category;
  magicKind?: MagicKind;
  type: string;
  behaviour: ItemBehaviour;
  count: number;
  maxCount: number;
  magicalBonus: number;
  ammoKind: string;
  effectDice: string;
  effectSave: string;
  notes: string;
  /** Only in item_edit. */
  changes?: FieldChange[];
}
