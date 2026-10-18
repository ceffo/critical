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

/**
 * One level whose hit points are owed (ADR 0030 step 7): `die` the hit die's sides to roll up to
 * the class group's name level (0 above it), `fixedGain` the gain above it (0 up to it).
 */
export interface PendingLevel {
  level: number;
  die: number;
  fixedGain: number;
}

/** ADR 0030 step 7: the levels from the realized level up to the target, still to take. */
export interface LevelUp {
  /** Lowest first: the prompt offers the first one. */
  pending: PendingLevel[];
  /** Step 7.4: an XP correction brought the target level under the realized one (the card warns). */
  realizedAboveTarget: boolean;
}

/**
 * The breakdown of a level-up roll (`POST /api/character/levelup/roll`), and, without `pending`,
 * the payload of its `level_up` log entry. `rolled` is the die as rolled or entered, `counted` its
 * value after the Constitution 20+ floor (both null for a fixed gain).
 */
export interface LevelUpRecord {
  level: number;
  hitDie: number;
  rolled: number | null;
  counted: number | null;
  conAdjustment: number;
  gain: number;
  fixed: boolean;
  manual: boolean;
  hpMaxBefore: number;
  hpMaxAfter: number;
}

export interface LevelUpResult extends LevelUpRecord {
  /** The levels still owed after this one. */
  pending: PendingLevel[];
}

/** What the server's rules derive from class, race and XP (ADR 0030 steps 3 to 8). */
export interface Derived {
  /** The starting XP plus the ledger. */
  xpTotal: number;
  /** Step 3: the level the XP gives; never stored, it only drives the level-up prompt (step 7). */
  targetLevel: number;
  /** Step 7: the level taken, which THAC0, saves and slots (below) follow. */
  realizedLevel: number;
  /** The XP that reaches the target's next level; null at the last level. */
  nextLevelXp: number | null;
  thac0: number;
  /** The save targets. */
  saves: SaveNumbers;
  /** The race's bonus on the save roll, per category (0: none). */
  raceSaveBonus: SaveNumbers;
  /** Slots per day by ascending spell level (levels without a slot left out). */
  spellSlots: { level: number; count: number }[];
  sheetDifferences: SheetDifference[];
  levelUp: LevelUp;
  /** Step 12: the class may specialize in a weapon (the weapon editor offers the flag). */
  specializes: boolean;
  /** Step 20.1: what a mage or a bard may write in the spellbook; null for the other classes. */
  spellbook: SpellbookRules | null;
}

/** ADR 0030 step 20.1: the class's and PHB Table 4's highest spell level, Table 4's chance to
 * learn (percent) and its maximum spells per level (`noMaximum`: the book's "All"). */
export interface SpellbookRules {
  classMaxSpellLevel: number;
  intMaxSpellLevel: number;
  learnChance: number;
  maxSpellsPerLevel: number;
  noMaximum: boolean;
}

/** The breakdown of a chance-to-learn roll (`POST /api/character/spellbook/learn`, ADR 0030 step
 * 20.2), also the payload of a spell_learn journal entry. */
export interface LearnResult {
  spellId: string;
  spellLevel: number;
  roll: number;
  chance: number;
  learned: boolean;
  /** The realized level the roll was made at: a failure is refused until a higher one. */
  characterLevel: number;
}

export interface Character {
  id: number;
  name: string;
  str: number;
  /** A warrior's exceptional strength at 18 (ADR 0030 step 9): 1 to 100 (100 = 18/00), 0 none. */
  strExceptional: number;
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

// Mirrors api/internal/rest/handler/characterhdl (SummaryDTO, CharactersDTO, OptionsDTO, CreateBody).

/** A character in its owner's list (ADR 0029 step 7.2). */
export interface CharacterSummary {
  id: number;
  name: string;
  /** The class code, worded by `details.classes.<code>`. */
  class: string;
  /** The level the rules derive (ADR 0030 step 3). */
  level: number;
}

/** The user's characters, not retired, in creation order, and the current one (null: none). */
export interface CharacterList {
  characters: CharacterSummary[];
  currentId: number | null;
}

/** An inclusive range the server checks. */
export interface ValueRange {
  min: number;
  max: number;
}

/** What the creation form offers (`GET /api/characters/options`): the creatable classes and their races. */
export interface CreationOptions {
  /**
   * `exceptionalStrengthRaces`: the races that enter an 18/xx at Strength 18 with the class (ADR
   * 0030 step 9; a halfling fighter does not).
   */
  classes: {
    class: string;
    races: string[];
    exceptionalStrengthRaces: string[];
  }[];
  ability: ValueRange;
  /** The 18/xx percentile's range (1 = 18/01, 100 = 18/00). */
  strExceptional: ValueRange;
  hpMax: ValueRange;
  xp: ValueRange;
  maxNameLength: number;
}

/** The six ability codes, in the sheet's order. */
export const ABILITIES = ["str", "dex", "con", "int", "wis", "cha"] as const;
export type AbilityCode = (typeof ABILITIES)[number];

/**
 * A new character (`POST /api/characters`): only what is entered (ADR 0030 step 1), never a level
 * or a THAC0. A number left blank is null, which the server refuses with its field.
 */
export type NewCharacter = {
  name: string;
  class: string;
  raceCode: string;
  hpMax: number | null;
  xp: number | null;
  /** The 18/xx percentile (ADR 0030 step 9), 0 for none. */
  strExceptional: number;
} & Record<AbilityCode, number | null>;
