export interface Modifier {
  source: string;
  value: number;
}

export interface RollResult {
  d20Roll: number;
  d100CritRoll: number;
  isCrit: boolean;
  critMultiplier: number;
  baseThac0: number;
  modifiers: Modifier[];
  effectiveThac0: number;
  targetACNeeded: number;
  hit: boolean;
  hitMargin: number;
  baseDamage: number;
  spellsBuff: number;
  weaponBuff: number;
  multipliers: number;
  strengthBonus: number;
  totalDamage: number;
  damageBreakdown: string;
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
