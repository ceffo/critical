// Mirrors combatsvc.AttackBreakdown (RollResult) and combatsvc.SaveBreakdown, each both the roll
// endpoints' answer and their entries' log payload, and api/internal/rest/handler/combathdl
// (AttackRequestDTO, WeaponDTO, the attack form).

import type { LocalizedText } from "./localized";

/** Where a modifier comes from; the UI labels it with `result.mods.<kind>`. */
export type ModifierKind =
  | "strength"
  | "dexterity"
  | "weapon_magic"
  | "weapon_buff"
  | "ammo_magic"
  | "buff"
  | "situational"
  | "race"
  | "crit"
  | "specialization"
  | "point_blank"
  | "backstab"
  /** The flat part of the damage as printed (1d4+1), a weapon damage term; detail is the damage (ADR 0003 step 2). */
  | "dice_modifier";

export interface Modifier {
  kind: ModifierKind;
  /**
   * Score, weapon, ammo, buff name or situational label (empty: an unnamed situational modifier);
   * an item power's: the item's name as stored, shown when no item has `itemId` any more.
   */
  detail?: string;
  /** Set on an item power's contribution: the UI names it from the item (ADR 0016 step 14). */
  itemId?: string;
  value: number;
  /** A Strength modifier's 18/xx percentile (1 to 100, ADR 0030 step 9); the UI writes "18/50". */
  strExceptional?: number;
}

/**
 * Place of a contribution in (((dice × dice_multiplier + flat part) × spells × standard_multiplier)
 * + weapon) × multiplier + strength + bonus (docs/adr/0003 step 2). standard_multiplier is the
 * backstab; attacks logged before it list the backstab as a dice_multiplier.
 */
export type DamageTermKind =
  | "spells"
  | "dice_multiplier"
  | "standard_multiplier"
  | "weapon"
  | "multiplier"
  | "strength"
  | "bonus";

export interface DamageTerm {
  term: DamageTermKind;
  kind: ModifierKind;
  /** As on Modifier; the crit term has none (its range is the roll's critFrom–critTo). */
  detail?: string;
  /** As on Modifier. */
  itemId?: string;
  value: number;
  /** As on Modifier. */
  strExceptional?: number;
}

export interface AmmoUse {
  itemId: string;
  name: string;
  magic: number;
  remaining: number;
}

export type TargetSize = "sm" | "l";

/** The GM's word on an attack against an unknown AC (POST /api/roll/attack/{id}/verdict). */
export type AttackVerdict = "hit" | "miss";

export interface RollResult {
  d20Roll: number;
  /** The crit-check d100 (crit when d20 − it ≥ the roll needed). */
  d100CritRoll: number;
  /**
   * The second d100, whose crit-table row gives the multiplier of a crit (docs/adr/0001). Absent on
   * attacks logged before it existed.
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
  /** The lowest AC the d20 hits (effective THAC0 − d20), same as hitsAc; kept for older entries. */
  targetACNeeded: number;
  /** The lowest target AC the attack hits (docs/adr/0012 step 2); absent on older entries. */
  hitsAc?: number;
  /** The lowest target AC the attack crits (docs/adr/0012 step 3); absent on older entries. */
  critsFromAc?: number;
  /** Some real AC can crit (critsFromAc within the highest AC the rules allow, docs/adr/0012 step 7). */
  critPossible?: boolean;
  /**
   * Final only when the AC is known or the GM gave a verdict (attackSettled); until then false, with
   * the damage of a hit at ×1 ("if it hits").
   */
  hit: boolean;
  hitMargin: number;
  baseDamage: number;
  damageDice: string;
  /** The flat part of damageDice ("1d4+1": 1), absent when none (ADR 0003 step 2). */
  diceModifier?: number;
  spellsBuff: number;
  weaponBuff: number;
  multipliers: number;
  /** Buff multiplier on the raw dice only (Power Gem ×2); absent on old log entries. */
  diceMultiplier?: number;
  /** The backstab's factor on the standard damage, dice and flat part (docs/adr/0030 step 19); absent without one. */
  backstabMultiplier?: number;
  strengthBonus: number;
  bonusDamage: number;
  damageTerms: DamageTerm[];
  /** The damage dice as thrown; absent on attacks logged before they were recorded and on a miss. */
  damageRolls?: { sides: number; value: number }[];
  totalDamage: number;
  /** The damage formula's result before the final floor; absent on attacks logged before it was recorded. */
  unroundedDamage?: number;
  // Attack context.
  weaponId?: string;
  weaponName?: string;
  /** Null on an attack whose target AC is unknown (docs/adr/0012). */
  targetAc: number | null;
  /** False while the target AC is unknown; absent on attacks logged before it existed (known). */
  acKnown?: boolean;
  /** The attack was rolled without the AC (still true once the GM gave it). */
  rolledAcUnknown?: boolean;
  /** The GM's hit or miss while the AC is unknown (no crit). */
  verdict?: AttackVerdict;
  targetSize?: TargetSize;
  ammo?: AmmoUse;
  /** Target AC and outcome as rolled; present while a correction differs from the rolled AC. */
  rolledTargetAc?: number;
  rolledHit?: boolean;
  rolledCrit?: boolean;
  /** The action-log entry recording the attack. */
  logEntryId?: number;
}

/**
 * A saving throw: the payload of its save entry (an adjusted save's is the latest amend's) and the
 * answer of POST /api/roll/save and its adjust route (combatsvc.SaveBreakdown, one struct).
 */
export interface SaveBreakdown {
  category: string;
  d20Roll: number;
  /** The modifiers applied when rolling (race, buffs), not the adjustment. */
  modifiers?: Modifier[];
  /** The sheet's target, never adjusted. */
  target: number;
  /** The verdict as rolled, before any adjustment. */
  rolledPassed?: boolean;
  /** Moves the target, + easier, − harder (ADR 0006 step 5); 0: none. */
  adjustment: number;
  adjustmentReason?: string;
  /** d20 + modifiers, the adjustment left out (ADR 0006 step 5). */
  rollTotal: number;
  /** target − adjustment: the target the save was judged against. */
  adjustedTarget: number;
  /** rollTotal + adjustment (kept by the server; not displayed). */
  total?: number;
  passed: boolean;
  /** rollTotal − adjustedTarget, as computed by the server. */
  margin: number;
  /** Verdict and adjustment before the latest adjustment; set once the save was adjusted. */
  previousPassed?: boolean;
  previousAdjustment?: number;
  /** The entry recording the save: on the endpoints' answer only (a journal entry has its own id). */
  logEntryId?: number;
}

/** POST /api/roll/attack */
export interface AttackRequest {
  weapon_id: string;
  /** Required for weapons with an ammo_kind (bows). */
  ammo_item_id?: string;
  /** Absent: the target AC is unknown. */
  target_ac?: number;
  target_size: TargetSize;
  situational?: { value: number; label: string };
  /** A specialized launcher's point-blank option (ADR 0030 step 12); refused for another weapon. */
  point_blank?: boolean;
  /** A thief's backstab (ADR 0030 step 19); refused for another class. */
  backstab?: boolean;
}

/** The attack form's last choices, kept on the server (combathdl AttackFormDTO). */
export interface AttackForm {
  weaponId: string;
  /** The weapon held before the current one (one-click switch back); empty when none. */
  previousWeaponId: string;
  /** Empty for a weapon without ammunition, or when none is left to offer. */
  ammoItemId: string;
  /** Null while the target AC is unknown (the default). */
  targetAc: number | null;
  targetSize: TargetSize;
  /** Kept with its label at 0. */
  situational: { value: number; label: string };
}

/** One ammunition the weapon in hand can fire (combathdl AmmoChoiceDTO). */
export interface AmmoChoice {
  itemId: string;
  /** A language map (ADR 0023 step 9): pick with `localized`. */
  name: LocalizedText;
  magic: number;
  count: number;
}

/** The effective THAC0 of the current choices, before rolling (ADR 0002 step 12). */
export interface AttackPreview {
  baseThac0: number;
  modifiers: Modifier[];
  effectiveThac0: number;
  /** Sum of the modifiers (baseThac0 − effectiveThac0). */
  toHit: number;
  /** The d20 that hits the target AC; null while the AC is unknown. */
  rollNeeded: number | null;
  /** Flat damage terms: weapon or buff "+N", ammunition, Strength, specialization, buff damage. */
  damageBonus: number;
  /**
   * The dice the attack rolls against each size, as printed (ADR 0033 step 4: a launcher's come from
   * its ammunition); both empty when there are none.
   */
  damageSm: string;
  damageL: string;
  /** The same preview with the point-blank option (ADR 0030 step 12); absent when not offered. */
  pointBlank?: AttackPreview;
  /** The same preview with the backstab option (ADR 0030 step 19); absent for another class. */
  backstab?: AttackPreview;
}

/** The weapon in hand's attacks per round (ADR 0030 step 11): `attacks` every `rounds` rounds. */
export interface AttackRate {
  attacks: number;
  rounds: number;
  /** The attacks in the current round (3/2: 1 in odd rounds, 2 in even ones). */
  thisRound: number;
}

/** GET / PATCH /api/attack/form (combathdl AttackFormViewDTO). */
export interface AttackFormView {
  form: AttackForm;
  /** No ammunition was chosen: ammoItemId is the first one left; an attack makes it the choice. */
  ammoFromDefault: boolean;
  ammoChoices: AmmoChoice[];
  /** Null when there is no weapon at all. */
  preview: AttackPreview | null;
  /** Absent when the weapon shows no rate (no type, or not a warrior). */
  attackRate?: AttackRate;
  /** The highest AC the rules allow: the target AC stepper stops there (docs/adr/0012 step 7). */
  maxRealAc: number;
}

/** PATCH /api/attack/form: only the fields present change; targetAc null makes the AC unknown. */
export interface AttackFormPatch {
  weaponId?: string;
  ammoItemId?: string;
  targetAc?: number | null;
  targetSize?: TargetSize;
  situational?: { value: number; label: string };
}

/**
 * Weapon type codes, language-neutral (weaponsvc.Types, ADR 0030 step 23); "" is a weapon with no
 * type. Display names come from i18n, never from here.
 */
export const WEAPON_TYPES = [
  "long_bow",
  "short_bow",
  "light_crossbow",
  "heavy_crossbow",
  "hand_crossbow",
  "dagger",
  "dart",
  "sling",
  "short_sword",
  "long_sword",
  "other_melee",
  "other_thrown",
  "other_missile",
] as const;
export type WeaponType = (typeof WEAPON_TYPES)[number] | "";

/** GET /api/weapons: the weapon pieces as the attack reads them (combathdl WeaponDTO). */
export interface Weapon {
  id: string;
  /** A language map (ADR 0023 step 9): pick with `localized`. */
  name: LocalizedText;
  damage_sm: string;
  damage_l: string;
  magic_to_hit: number;
  magic_damage: number;
  kind: "melee" | "missile";
  two_handed: boolean;
  /** Ammo item kind the weapon fires ("arrow"); empty for melee and hurled weapons. */
  ammo_kind: string;
  weapon_type: WeaponType;
  /** The fighter's weapon specialization (ADR 0030 step 12). */
  specialized: boolean;
  /** The speed factor (ADR 0033 step 3). */
  speed: number;
  /** Wielded (ADR 0033 step 6): only a wielded weapon is in hand; picking another wields it. */
  wielded: boolean;
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
/** One edited field in a weapon_edit log payload (values as text). */
export interface WeaponFieldChange {
  field: string;
  before: string;
  after: string;
}

/**
 * Payload of weapon_add, weapon_edit and weapon_remove log entries, written before the one
 * equipment model (ADR 0033 step 26.2): they still show in the journal and pop.
 */
export interface WeaponChangeRecord extends Omit<Weapon, "name"> {
  weapon_id: string;
  /** The name recorded at write time (ADR 0023 step 12): one string, not a language map. */
  name: string;
  /** Only in weapon_edit. */
  changes?: WeaponFieldChange[];
}
