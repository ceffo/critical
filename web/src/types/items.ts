// Mirrors api/internal/rest/handler/itemhdl (ItemDTO, PowerDTO, UseResultDTO, RestockResultDTO, ChangeDTO,
// ItemBody, StateRequest, StateDTO). An item is any piece of equipment (ADR 0033 step 1).

import type { BuffEffects } from "./buffs";
import type { WeaponType } from "./combat";
import type { LocalizedText } from "./localized";
import type {
  CastBreakdown,
  CastTarget,
  SpellMode,
  SpendableItem,
} from "./spells";

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
 * weapon_code_missing: a wielded weapon with no catalogue code, so no pick can name it and its
 * attacks take the non-proficiency penalty (ADR 0037 step 21.4). dual_class_old_ability: a piece only a dual-class's old class allows, before the new class
 * passes it (ADR 0042 step 13). bow_too_strong: a wielded bow built for exceptional Strength (18/01
 * or more) for a character without it (ADR 0002 step 4.1, a bend bars roll the app does not make).
 */
export type ClassWarningCode =
  | "armour_not_allowed"
  | "weapon_not_allowed"
  | "hands"
  | "weapon_code_missing"
  | "dual_class_old_ability"
  | "bow_too_strong";

/**
 * A bow's Strength rating (ADR 0033 step 3): the Strength a strength bow is built for, a score and
 * at 18 its exceptional percentile (1 to 100, 100 for 18/00; 0 for none).
 */
export interface StrengthRating {
  score: number;
  exceptional: number;
}

/** Why a piece is on the review list (ADR 0033 step 27, itemsvc.Reason*). */
export type ReviewReason =
  | "weapon_unmatched"
  | "uncategorised"
  | "ammo_kind_unknown"
  | "powers_off"
  | "weapon_with_use"
  | "quantity_above_one"
  | "charges_total"
  | "ammo_not_stock"
  | "limit_clamped";

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
  /**
   * Whether the piece has a stock the player moves, spend one and restock (server rule: ammunition,
   * gear, a potion or a scroll; ADR 0049 step 2). The web filters and hides controls by it, never by
   * the category. Whether an ability can be used is on the ability.
   */
  usable: boolean;
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
  /**
   * What the item does when used, each under a usage limit (ADR 0049 step 4). Absent: the piece is
   * not converted to abilities yet (ADR 0049 step 18); an empty list: converted with none. Nothing
   * reads them yet: Use comes with a later slice.
   */
  abilities?: ItemAbility[];
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
  /** A bow's Strength rating (ADR 0033 step 3); null for an ordinary bow and every other piece. */
  strengthRating: StrengthRating | null;
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

/** How often an item ability may be used (ADR 0049 step 5). */
export type AbilityLimitKind =
  "unlimited" | "per_day" | "charges" | "with_item";

/**
 * An ability's usage limit: `max` and `left` for per_day (max 1 to 99) and charges (max 1 to 100),
 * `cost` for charges (1 to max, default 1 when sent as 0); none of them for unlimited and with_item.
 */
export interface ItemAbilityLimit {
  kind: AbilityLimitKind;
  max?: number;
  left?: number;
  cost?: number;
}

/**
 * An effect of an ability (ADR 0049 step 10). `dice`: dice rolled and shown, applied to nothing.
 * `heal`: hit points regained through the HP ledger, by `dice` or `full` (the gap to max HP), never both.
 * `buff`: a timed buff put on the buff list when used: `name` (absent: the ability's, else the item's),
 * `rounds` (0: until dismissed) and `buff`, its modifiers in the buffs' effect keys.
 * `spell`: a spell cast at a fixed caster level through the cast service (`spellId`, `level` 1 to 30).
 */
export interface ItemAbilityEffect {
  kind: "dice" | "heal" | "buff" | "spell";
  /** NdM or NdM+K; absent for a full heal. */
  dice?: string;
  /** A heal that fills the gap to max HP (no dice). */
  full?: boolean;
  /** A save category (the item's `effectSave` values), absent for none; a dice effect only. */
  save?: string;
  label?: string;
  /** A buff effect's own name; absent: the ability's, else the item's. */
  name?: string;
  /** A buff effect's rounds; 0 or absent: until dismissed. */
  rounds?: number;
  /** A buff effect's modifiers. */
  buff?: BuffEffects;
  /** A spell effect's spell (a catalogue id) and the caster level it is cast at. */
  spellId?: string;
  level?: number;
  /** What the use of a spell effect asks of the player, served on the item list (absent elsewhere). */
  asks?: SpellAsks;
}

/**
 * Mirrors wire.SpellAsksDTO: what casting a spell asks of the player, by the spellbook's own field
 * names, read at the effect's caster level. Every field is absent when the cast asks nothing.
 */
export interface SpellAsks {
  slot_splits?: number[][];
  consume_item_type?: string;
  attack_roll?: boolean;
  modes?: SpellMode[];
  cast_target?: CastTarget;
  items?: SpendableItem[];
}

/**
 * The answers for one spell effect of the ability used (itemhdl SpellAskDTO), by the cast request's
 * own names; `effect` is the index of the spell effect in the ability's list.
 */
export interface SpellAnswer {
  effect: number;
  onSelf?: boolean;
  targetAc?: number;
  itemId?: string;
  extraSlots?: number[];
  mode?: string;
}

/** The body of POST /api/items/{id}/abilities/{abilityId}/use: absent or empty takes the cast's defaults. */
export interface UseAbilityBody {
  spells?: SpellAnswer[];
}

/**
 * One of an item's abilities (ADR 0049 step 4): an id (a slug, unique within the item; sent empty it
 * gets ability_<n>), an optional name (a language map; none means the item's own) and note, one usage
 * limit and its effects (empty is allowed).
 */
export interface ItemAbility {
  id: string;
  name?: LocalizedText;
  note?: string;
  limit: ItemAbilityLimit;
  effects: ItemAbilityEffect[];
  /**
   * Served, never sent: whether a use is paid now, and why not (ADR 0049 steps 5 and 14). The web
   * derives neither.
   */
  usable?: boolean;
  reason?: AbilityRefusal;
}

/** Why an ability cannot be used: its limit cannot pay (ADR 0049 step 5). */
export type AbilityRefusal = "exhausted";

/** What a heal effect did to the HP ledger (itemsvc Applied): the points taken (capped at max HP) and the HP around them. */
export interface EffectApplied {
  hpDelta: number;
  hpBefore: number;
  hpAfter: number;
}

/** The row a buff effect put on the buff list (itemsvc AppliedBuff): rounds 0 is until dismissed. */
export interface AppliedBuff {
  id: string;
  name: string;
  rounds: number;
  effects: BuffEffects;
}

/**
 * One effect of an ability's use (itemsvc EffectResult): a dice effect's rolls, applied to nothing;
 * a heal's rolls (none for a full heal) with `applied`, what the HP ledger took; a buff's `buff`,
 * the row put on the list; a spell's `cast`, its breakdown.
 */
export interface AbilityEffectResult {
  kind: "dice" | "heal" | "buff" | "spell";
  buff?: AppliedBuff;
  /** A spell effect's spell and the breakdown of its cast, as a cast from a slot logs it (action "item"). */
  spellId?: string;
  cast?: CastBreakdown;
  /** Absent for a full heal. */
  dice?: string;
  full?: boolean;
  applied?: EffectApplied;
  rolls: number[];
  /** The flat "+K" of the dice ("1d8+2"); absent for none. */
  modifier?: number;
  total: number;
  save?: string;
  label?: string;
}

/**
 * POST /api/items/{id}/abilities/{abilityId}/use (itemhdl AbilityUseResultDTO), ADR 0049 step 7: the
 * limit as it stands after the use and each effect's result. The item_use log payload carries the
 * same fields plus the first dice effect in the flat shape of `UseResult`.
 */
export interface AbilityUseResult {
  itemId: string;
  abilityId: string;
  /** The item's name; `abilityName` is the ability's own, absent when it has none. */
  name: string;
  abilityName?: LocalizedText;
  type: string;
  limit: AbilityLimitKind;
  /** The counter after the use, for per_day and charges only. */
  left?: number;
  max?: number;
  /** The counter or the quantity left to read; 0 for unlimited. */
  remaining: number;
  /** The use spent the last potion or scroll: the item is gone until the use is popped. */
  removed: boolean;
  effects: AbilityEffectResult[];
  notes?: string;
  logEntryId: number;
}

/** One of an item's powers: its trigger (minStr; absent: always on) and its effects. */
export interface ItemPowerDef {
  id: string;
  minStr?: number;
  effects: BuffEffects;
}

/**
 * POST /api/items/{id}/use (itemhdl UseResultDTO), spend one (ADR 0049 step 2): nothing is rolled, the
 * dice fields are the journal's shape of a use with none. The item_use log payload has the same shape.
 */
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
  /**
   * Set by the use of an ability (ADR 0049 step 7; the entry carries them too): the ability, its
   * limit kind, the counter after the use and every effect's roll. Absent on a spend-one and on
   * entries logged before abilities.
   */
  abilityId?: string;
  abilityName?: LocalizedText;
  limit?: AbilityLimitKind;
  left?: number;
  max?: number;
  effects?: AbilityEffectResult[];
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
  /** A bow's Strength rating: an object sets it, null clears it, absent keeps it. */
  strengthRating?: StrengthRating | null;
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
   * The whole ability list (ADR 0049 step 4): a PATCH's replaces the stored one (an empty list too),
   * absent keeps it; on a POST absent leaves the piece not converted. Refused on field `abilities`.
   */
  abilities?: ItemAbility[];
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

/**
 * GET /api/equipment/rules (wire.EquipmentRulesDTO), ADR 0049 step 2: which categories have a quantity
 * the player sets. `magicKinds` is filled for `magic` only, whose answer is per kind.
 */
export interface EquipmentRules {
  /** The most a quantity may be (itemsvc.MaxQuantity): the form's field and its check. */
  maxQuantity: number;
  categories: {
    category: Category;
    hasQuantity: boolean;
    magicKinds: { kind: MagicKind; hasQuantity: boolean }[];
  }[];
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
