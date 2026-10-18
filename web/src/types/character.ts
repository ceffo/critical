// Mirrors api/internal/rest/handler/characterhdl (CharacterDTO, DerivedDTO).

/** Five numbers, one per save category (targets, or a bonus per category). */
export interface SaveNumbers {
  paraPoison: number;
  rodStaffWand: number;
  petriPoly: number;
  breath: number;
  spell: number;
}

/** A value the imported sheet states differently from the rules (ADR 0030 step 24). */
export interface SheetDifference {
  value:
    | "level"
    | "thac0"
    | "save_para_poison"
    | "save_rod_staff_wand"
    | "save_petri_poly"
    | "save_breath"
    | "save_spell";
  sheet: number;
  computed: number;
}

/** What the server's rules derive from class, race and XP (ADR 0030 steps 3 to 8). */
export interface Derived {
  /** The starting XP plus the ledger. */
  xpTotal: number;
  level: number;
  /** The XP that reaches the next level; null at the last level. */
  nextLevelXp: number | null;
  thac0: number;
  /** The save targets. */
  saves: SaveNumbers;
  /** The race's bonus on the save roll, per category (0: none). */
  raceSaveBonus: SaveNumbers;
  /** Slots per day by ascending spell level (levels without a slot left out). */
  spellSlots: { level: number; count: number }[];
  sheetDifferences: SheetDifference[];
}

export interface Character {
  id: number;
  name: string;
  str: number;
  dex: number;
  con: number;
  int: number;
  wis: number;
  cha: number;
  hpMax: number;
  /** The sheet's starting XP (the ledger adds to it: derived.xpTotal). */
  xpTotal: number;

  /** Class key (e.g. "bard"), read-only. */
  class: string;
  /** The PHB race the rules read ("half_elf"…), "" when none is set. */
  raceCode: string;
  derived: Derived;

  // Details the player edits; age is null when not set. `race` is free text.
  race: string;
  alignment: string;
  age: number | null;
  sex: string;
  height: string;
  weight: string;
  hair: string;
  eyes: string;
  languages: string[];
}

/** The editable details, as the server stores them. */
export type CharacterDetails = Pick<
  Character,
  | "race"
  | "alignment"
  | "age"
  | "sex"
  | "height"
  | "weight"
  | "hair"
  | "eyes"
  | "languages"
>;
