// Mirrors api/internal/rest/handler/achdl (ACDTO, RowDTO, OverrideDTO).

import type { ClassWarningCode } from "./items";

/** A term of the AC (ADR 0033 step 10), in the order the server sums them. */
export type AcTerm =
  | "base"
  | "armour_magic"
  | "shield"
  | "shield_magic"
  | "dexterity"
  | "protection"
  | "buffs";

/** Why a row is shown but does not add (ADR 0033 steps 11 and 12). */
export type AcOff = "magic_armour" | "duplicate" | "armour_worn" | "not_best";

/**
 * One row of the breakdown. Its source is a piece (`itemId`, named from the possessions), a buff
 * (`buff` and `spellId`, named like the buff list), or none. `value` is the base AC on a base row,
 * a change of the AC number on every other (negative betters it).
 */
export interface AcRow {
  term: AcTerm;
  itemId?: string;
  buff?: string;
  spellId?: string;
  /** A buff a successful proficiency check put on (ADR 0037 step 20), named from the proficiency. */
  proficiencyId?: string;
  value: number;
  /** The shield row against missiles, when it differs (a body shield). */
  missileValue?: number;
  /** The sheet's score, on the dexterity row. */
  dexterity?: number;
  off?: AcOff;
}

/** The AC the player set for what the derived AC cannot hold (ADR 0033 step 13). */
export interface AcOverride {
  value: number;
  /** What the player typed ("" for none). */
  label: string;
}

/** A class warning of the worn armour or shield (ADR 0033 step 20), with the piece's id. */
export interface AcWarning {
  itemId: string;
  code: ClassWarningCode;
}

/** GET /api/ac: derived on every read, never stored; the override shown while it is set. */
export interface ArmourClass {
  ac: number;
  /** The AC against missiles when a body shield makes it differ; absent otherwise. */
  missile?: number;
  /** The -10 floor raised the sum (DMG p.181). */
  floored: boolean;
  rows: AcRow[];
  override: AcOverride | null;
  /** The class warnings of the worn armour and shield (ADR 0033 step 20). */
  warnings: AcWarning[];
}
