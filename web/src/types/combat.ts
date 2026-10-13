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
  /** English label from the server. */
  source: string;
  /** Score, weapon, ammo, buff name or situational label. */
  detail?: string;
  value: number;
}

/** Place of a contribution in ((dice × spells) + weapon) × multiplier + strength + bonus. */
export type DamageTermKind = "spells" | "weapon" | "multiplier" | "strength" | "bonus";

export interface DamageTerm {
  term: DamageTermKind;
  kind: ModifierKind;
  source: string;
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
  d100CritRoll: number;
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
  strengthBonus: number;
  bonusDamage: number;
  damageTerms: DamageTerm[];
  totalDamage: number;
  damageBreakdown: string;
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
