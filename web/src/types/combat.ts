// Mirrors api/internal/rest/handler/combathdl (RollResultDTO, ModifierDTO, DamageTermDTO, AmmoDTO,
// AttackRequestDTO, WeaponDTO). The attack fields also match the attack entries' log payload.

/** Where a modifier comes from; the UI labels it with `result.mods.<kind>`. */
export type ModifierKind =
  | "strength"
  | "dexterity"
  | "weapon_magic"
  | "weapon_buff"
  | "ammo_magic"
  | "buff"
  | "situational"
  | "crit"
  /** A save adjustment set after the roll; detail = its reason. */
  | "adjustment";

export interface Modifier {
  kind: ModifierKind;
  /** Score, weapon, ammo, buff name or situational label (empty: an unnamed situational modifier). */
  detail?: string;
  value: number;
}

/** Place of a contribution in ((dice × spells) + weapon) × multiplier + strength + bonus. */
export type DamageTermKind =
  "spells" | "dice_multiplier" | "weapon" | "multiplier" | "strength" | "bonus";

export interface DamageTerm {
  term: DamageTermKind;
  kind: ModifierKind;
  /** As on Modifier; the crit term has none (its range is the roll's critFrom–critTo). */
  detail?: string;
  value: number;
}

export interface AmmoUse {
  itemId: string;
  name: string;
  magic: number;
  remaining: number;
}

export type TargetSize = "sm" | "l";

export interface RollResult {
  d20Roll: number;
  /** The crit-check d100 (crit when d20 − it ≥ the roll needed). */
  d100CritRoll: number;
  /**
   * The second d100, whose crit-table row gives the multiplier of a crit (docs/adr/0001). Absent on
   * saves and on attacks logged before it existed.
   */
  d100MultiplierRoll?: number;
  isCrit: boolean;
  critMultiplier: number;
  /** The multiplier came from the default crit table, which nobody has edited yet. */
  critPlaceholder: boolean;
  /** The crit-table range the crit d100 fell in (absent on entries logged before the table existed). */
  critFrom?: number;
  critTo?: number;
  baseThac0: number;
  modifiers: Modifier[];
  effectiveThac0: number;
  targetACNeeded: number;
  hit: boolean;
  hitMargin: number;
  baseDamage: number;
  damageDice: string;
  spellsBuff: number;
  weaponBuff: number;
  multipliers: number;
  /** Buff multiplier on the raw dice only (Power Gem ×2); absent on old log entries. */
  diceMultiplier?: number;
  strengthBonus: number;
  bonusDamage: number;
  damageTerms: DamageTerm[];
  totalDamage: number;
  /** The damage formula's result before the final floor; absent on attacks logged before it was recorded. */
  unroundedDamage?: number;
  // Attack context (absent on saving throws).
  weaponId?: string;
  weaponName?: string;
  targetAc: number;
  targetSize?: TargetSize;
  ammo?: AmmoUse;
  /** Target AC and outcome as rolled; present while a correction differs from the rolled AC. */
  rolledTargetAc?: number;
  rolledHit?: boolean;
  rolledCrit?: boolean;
  // Saving-throw context (absent on attacks).
  saveCategory?: SaveCategory;
  /** Adjustment set after the roll (0: none); always present on saves. */
  adjustment?: number;
  adjustmentReason?: string;
  /** A save's modifiers summed (not the adjustment), and d20 + modifiers + adjustment. */
  saveBonus?: number;
  saveTotal?: number;
  /** The action-log entry recording the attack or save. */
  logEntryId?: number;
}

/** POST /api/roll/attack */
export interface AttackRequest {
  weapon_id: string;
  /** Required for weapons with an ammo_kind (bows). */
  ammo_item_id?: string;
  target_ac: number;
  target_size: TargetSize;
  situational?: { value: number; label: string };
}

/** GET /api/weapons */
export interface Weapon {
  id: string;
  name: string;
  damage_sm: string;
  damage_l: string;
  magic_to_hit: number;
  magic_damage: number;
  kind: "melee" | "missile";
  two_handed: boolean;
  /** Ammo item kind the weapon fires ("arrow"); empty for melee and hurled weapons. */
  ammo_kind: string;
}

/** Saving throw categories accepted by POST /api/roll/save. */
export const SAVE_CATEGORIES = [
  "para_poison",
  "rod_staff_wand",
  "petri_poly",
  "breath",
  "spell",
] as const;
export type SaveCategory = (typeof SAVE_CATEGORIES)[number];

/** One inclusive D100 range of the house crit table and its multiplier. */
export interface CritRow {
  from: number;
  to: number;
  /** May be a decimal (1.5); the server floors the final damage once. */
  multiplier: number;
}

export interface CritTable {
  rows: CritRow[];
  /** Still the unedited default (1-100 → ×2). */
  placeholder: boolean;
}

/** Body of POST /api/weapons and PATCH /api/weapons/{id} (weaponhdl WeaponBody); a PATCH changes only the fields present. */
export interface WeaponBody {
  name?: string;
  damage_sm?: string;
  damage_l?: string;
  magic_to_hit?: number;
  magic_damage?: number;
  kind?: "melee" | "missile";
  two_handed?: boolean;
  ammo_kind?: string;
}

/** POST/PATCH/DELETE /api/weapons (weaponhdl ChangeDTO): the weapon as it is now (as it was, for a delete). */
export interface WeaponChange {
  weapon: Weapon;
  logEntryId: number;
}

/** One edited field in a weapon_edit log payload (values as text). */
export interface WeaponFieldChange {
  field: string;
  before: string;
  after: string;
}

/** Payload of weapon_add, weapon_edit and weapon_remove log entries (weaponsvc changeRecord). */
export interface WeaponChangeRecord extends Weapon {
  weapon_id: string;
  /** Only in weapon_edit. */
  changes?: WeaponFieldChange[];
}
