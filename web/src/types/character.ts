import type { LocalizedText } from "@/types/localized";
// Mirrors api/internal/rest/handler/characterhdl (CharacterDTO, DerivedDTO).

/** Five numbers, one per save category (targets, or a bonus per category). */
export interface SaveNumbers {
  paraPoison: number;
  rodStaffWand: number;
  petriPoly: number;
  breath: number;
  spell: number;
}

/**
 * One level whose hit points are owed (ADR 0030 step 7): `die` the hit die's sides to roll up to
 * the class group's name level (0 above it), `fixedGain` the gain above it (0 up to it).
 */
export interface PendingLevel {
  level: number;
  die: number;
  fixedGain: number;
  /** ADR 0037 step 15: what the level grants beyond hit points. */
  grants: LevelGrants;
}

/** ADR 0037 step 15: weapon and non-weapon slots and a thief's or a bard's skill points, granted
 * by a level or left to spend (0: none). */
export interface LevelGrants {
  weaponSlots: number;
  nonWeaponSlots: number;
  skillPoints: number;
}

/** ADR 0030 step 7: the levels from the realized level up to the target, still to take. */
export interface LevelUp {
  /** Lowest first: the prompt offers the first one. */
  pending: PendingLevel[];
  /** Step 7.4: an XP correction brought the target level under the realized one (the card warns). */
  realizedAboveTarget: boolean;
  /** ADR 0037 step 15: what is left to spend at the realized level, each 0 when none or over
   * budget; the card reminds while any is above 0. */
  toSpend: LevelGrants;
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
  /** ADR 0037 step 15: what the level reached grants beyond hit points. The card shows the pending level's `grants` (the same value: a roll only takes the first pending level), not this one. */
  grants: LevelGrants;
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
  levelUp: LevelUp;
  /** Step 12: the class may specialize in a weapon (the weapon editor offers the flag). */
  specializes: boolean;
  /** Step 20.1: what a mage or a bard may write in the spellbook; null for the other classes. */
  spellbook: SpellbookRules | null;
  /** Step 14: the percent chance a cast fails (low Wisdom); 0: no die. */
  spellFailure: number;
  /** Step 16: PHB Table 61 at the realized level, in the book's order; null for a class that does
   * not turn undead. */
  turning: TurningTarget[] | null;
  /** Step 17: each thieving skill with its terms; null for a class without them. */
  thiefSkills: ThiefSkillTerms[] | null;
  /** Step 18: each bard skill with its terms; null for a class without them. */
  bardSkills: ThiefSkillTerms[] | null;
  /** ADR 0033 step 14: the skill armour the thief's or bard's skills read, from the worn body
   * armour (none, leather, padded_studded_leather, elven_chain, metal); "" for another class. */
  skillArmor: string;
  /** Steps 17 and 18: the entered points against the class's budget at the realized level
   * (warnings; a bard's has no per-skill limit). */
  skillBudget: SkillBudgetWarning[];
  /** Step 19: PHB Table 30's damage multiplier at the realized level; 0 without the backstab. */
  backstab: number;
  /** ADR 0037 steps 3 to 5: the non-weapon pool and each pick's figures; null without the
   * catalogue's Table 34. */
  proficiencySlots: ProficiencySlots | null;
}

/** ADR 0037 step 3: available = initial (Table 34) + levelGained (one slot every `perLevels`
 * realized levels) + intelligence (Table 4); left = available - spent, negative when over. */
export interface NonweaponPool {
  initial: number;
  perLevels: number;
  levelGained: number;
  intelligence: number;
  available: number;
  spent: number;
  left: number;
  /** The part of `spent` the languages beyond the native one and Common spend, one slot each (ADR
   * 0037 step 6). */
  languages: number;
}

/** One non-weapon pick (ADR 0037 steps 4 and 5): `cost` for the class, `extraSlots` beyond it (+1
 * each to the check), `target` null for a pick with no check; `known` false for an id the
 * catalogue no longer holds. */
export interface PickFigures {
  id: string;
  slots: number;
  cost: number;
  extraSlots: number;
  ability: string;
  modifier: number;
  known: boolean;
  target: number | null;
}

/** ADR 0037 step 3's weapon pool: available = initial (Table 34) + levelGained (one slot every
 * `perLevels` realized levels); left = available - spent, negative when over. */
export interface WeaponPool {
  initial: number;
  perLevels: number;
  levelGained: number;
  available: number;
  spent: number;
  left: number;
}

/** One weapon pick (ADR 0037 steps 7 and 9): the weapon type and the slots spent (1, or the
 * specialization's cost); `allowed` false for a kept type the class may not pick. */
export interface WeaponPickFigures {
  type: string;
  slots: number;
  allowed: boolean;
}

/** One weapon type a class may pick and its specialization's slots (0: the class does not
 * specialize), from the creation options. */
export interface WeaponTypeOption {
  type: string;
  specializationSlots: number;
}

/** The slot pools and the picks' figures (`derived.proficiencySlots`). */
export interface ProficiencySlots {
  nonWeapon: NonweaponPool;
  picks: PickFigures[];
  /** Absent from a release before the weapon pool. */
  weapon?: WeaponPool;
  weaponPicks?: WeaponPickFigures[];
}

/** One weapon pick as sent: the weapon type and its slots. */
export interface WeaponPickInput {
  type: string;
  slots: number;
}

/** The body of `PUT /api/proficiencies` and its preview, and the creation's `proficiencies`: a
 * pool absent is left as stored (none at creation), so each editor sends its own. */
export interface PicksInput {
  nonweapon?: { id: string; slots: number }[];
  weapon?: WeaponPickInput[];
}

/** The body of `PUT /api/proficiencies` and its preview: the picks by pool and the languages (ADR
 * 0037 step 18: the codes, the native one kept by the server); a pool absent is left as stored. */
export interface ProficienciesInput extends PicksInput {
  languages?: string[];
}

/** The answer of `PUT /api/proficiencies` and `POST /api/proficiencies/preview`. */
export interface ProficienciesAnswer {
  proficiencySlots: ProficiencySlots | null;
  warnings: Character["warnings"];
}

/** ADR 0030 steps 17 and 18: percent = base + points + race + dexterity + armor, capped at 95,
 * floored at 0. */
export interface ThiefSkillTerms {
  skill: string;
  base: number;
  points: number;
  race: number;
  dexterity: number;
  armor: number;
  /** The sum before the cap and the floor. */
  rawPercent: number;
  percent: number;
}

/** ADR 0030 step 17: a breach of the skill point budget. `skill_points_total` has no `skill`. */
export interface SkillBudgetWarning {
  code: "skill_points_total" | "skill_points_per_skill";
  skill?: string;
  points: number;
  limit: number;
}

/** ADR 0030 step 16: one undead type and its Table 61 cell at the cleric's level. `hdMin` 0: by
 * type only; `hdMax` 0: "and more". `roll` is the d20 target, or `outcome` when no die is rolled. */
export interface TurningTarget {
  undead: string;
  hdMin: number;
  hdMax: number;
  roll?: number;
  outcome?: TurningOutcome;
}

export type TurningOutcome = "turn" | "destroy" | "destroy_more" | "cannot";

/** The breakdown of a turning roll (`POST /api/character/turn-undead`, ADR 0030 step 16), also
 * (without `entryId` and the Hit Dice) the payload of a turn_undead journal entry. */
export interface TurnResult {
  entryId?: number;
  undead: string;
  hdMin?: number;
  hdMax?: number;
  clericLevel: number;
  /** The d20 target; absent when the cell needs no die. */
  target?: number;
  outcome?: TurningOutcome;
  /** The turning d20; absent when the cell needs no die. */
  d20?: number;
  success: boolean;
  effect?: "turn" | "destroy";
  /** The 2d6 undead affected (turned, or destroyed for D and D*), then the 2d4 more of a D* cell
   * (turned). */
  affected?: number[];
  more?: number[];
  /** How many undead are destroyed and turned, worked out by the server (D*: both). */
  destroyed: number;
  turned: number;
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
  /** The realized level from which a failed spell may be tried again, given by the server
   * (the next one, ADR 0030 step 20.3); 0 when learned. */
  retryLevel: number;
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

  // Details the player edits; age is null when not set. `alignment` and `sex` are codes (ADR 0032
  // steps 2 and 3), "" when the character lacks them (see `missing`).
  alignment: string;
  age: number | null;
  sex: string;
  /** Centimetres, null when not set (ADR 0032 step 5); shown per language by `lib/measures`. */
  height_cm: number | null;
  /** Kilograms, null when not set. */
  weight_kg: number | null;
  hair: string;
  eyes: string;
  /** The language catalogue codes the character knows, its race's native language included (ADR
   * 0032 step 4); named by the creation options' `languages`. */
  languages: string[];

  /** The required details the character lacks ("alignment", "sex"): the card asks to pick them
   * (ADR 0032 step 8.2). */
  missing: ("alignment" | "sex")[];
  /** Rules the stored character breaks without being refused (steps 5.2 and 8.4, ADR 0037 step
   * 11), worded by the web. */
  warnings: {
    code: string;
    field: string;
    /** `height_outside_range`, `weight_outside_range` and `age_outside_range` (step 5.2): the
     * stored value and the race's range, in centimetres, kilograms or years. */
    value?: number;
    min?: number;
    max?: number;
    /** ADR 0037 step 11: `slots_over_budget` carries `pool`, `spent` and `available`,
     * `slots_to_spend` `pool` and `left`; `proficiency_unknown` names its pick in `field`
     * (`nonweapon.<id>`). */
    pool?: "nonweapon" | "weapon";
    spent?: number;
    available?: number;
    left?: number;
  }[];
}

/** The editable details, as the server stores them. */
export type CharacterDetails = Pick<
  Character,
  | "alignment"
  | "age"
  | "sex"
  | "height_cm"
  | "weight_kg"
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
  /** The race code, worded by `details.races.<code>`; "" when none. */
  race?: string;
  /** The alignment and sex codes (`details.alignments.*`, `details.sexes.*`); "" while still to choose. */
  alignment?: string;
  sex?: string;
  /** The identity fields still to choose, in the card's order (`alignment`, `sex`). */
  missing?: ("alignment" | "sex")[];
  /** The HP the ledger gives and the max HP. */
  currentHp?: number;
  maxHp?: number;
  // race to maxHp are optional: the previous API release (rolling deploy) sends none of them.
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

/** A catalogue language: its code and its name by language code. */
export interface LanguageOption {
  code: string;
  name: LocalizedText;
}

/** One proficiency of the non-weapon catalogue; `slots` is Table 37's cost before the class's crossover. */
export interface ProficiencyOption {
  id: string;
  ability?: string;
  modifier: number;
  slots: number;
  groups: string[];
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
    /** The skills the class enters points in (ADR 0030 steps 17 and 18); empty for none. */
    skills: string[];
    /** The alignment codes the class admits (ADR 0032 step 2). */
    alignments: string[];
    /**
     * The sides of the class's hit die and, for levels 1 to 20, the lowest XP of each level (ADR
     * 0034 step 4.1): a level picked is sent as that XP. Absent from a release that predates them.
     */
    hitDie?: number;
    levelXp?: LevelXp[];
    /** The class's ability minimums (PHB Table 13), shown on its card and not enforced; absent from an older release. */
    minimums?: { ability: string; min: number }[];
    /** ADR 0037 steps 7 and 9: the weapon types the class may pick, in display order, each with
     * the slots its specialization spends in all (0: the class does not specialize). Absent from
     * an older release. */
    weaponTypes?: WeaponTypeOption[];
  }[];
  /** ADR 0032 step 6's score methods, in the book's order; absent from an older release. */
  scoreMethods?: ScoreMethod[];
  /** PHB Table 8 by race (each ability's adjustment); absent from an older release. */
  raceAdjustments?: { race: string; adjustments: Record<string, number> }[];
  /** PHB Tables 10 and 11 by race (ADR 0032 step 5): the dice each figure's roll draws and the range
   * outside which a value warns; absent from an older release. */
  raceIdentity?: RaceIdentity[];
  /** The sex codes (ADR 0032 step 3). */
  sexes: string[];
  /**
   * The language catalogue (ADR 0032 step 4.3), the names by language (ADR 0023), each race's own
   * language and PHB list, and PHB Table 4's number of additional languages by Intelligence
   * score. Absent from a release that predates them (a rolling deploy, ADR 0024) and empty
   * before catalogue format 8: no language is offered.
   */
  languages?: LanguageOption[];
  raceLanguages?: { race: string; native: string; languages: string[] }[];
  /**
   * The non-weapon proficiency catalogue (PHB Table 37) and the groups each class learns at the
   * table's cost (Table 38) (ADR 0037 steps 1 and 2). A name is the i18n key `skills.names.<id>`.
   * `ability` is absent for a proficiency with no check. Absent from a release that predates them.
   */
  proficiencies?: ProficiencyOption[];
  proficiencyGroups?: Record<string, string[]>;
  /** Each proficiency's cost by class and id, the crossover's +1 included (ADR 0037 step 4): the
   * editor shows and sends it, never computes it. Absent from a release that predates it. */
  proficiencyCosts?: Record<string, Record<string, number>>;
  ability: ValueRange;
  /** The 18/xx percentile's range (1 = 18/01, 100 = 18/00). */
  strExceptional: ValueRange;
  /** The range of a thief's or a bard's points in one skill. */
  skillPoints: ValueRange;
  hpMax: ValueRange;
  xp: ValueRange;
  maxNameLength: number;
}

/** One identity figure of a race: `count` dice of `sides` and the range (`min`, `max`) in the stored
 * unit (centimetres, kilograms or years); `sex` names a height or weight row. */
export interface IdentityFigureRange {
  sex?: string;
  count: number;
  sides: number;
  min: number;
  max: number;
}

/** A race's height and weight by sex, its age range, and the starting ages a roll gives. */
export interface RaceIdentity {
  race: string;
  height: IdentityFigureRange[];
  weight: IdentityFigureRange[];
  age: IdentityFigureRange;
  startAge: ValueRange;
}

/** A warning the server reads on a character (ADR 0032 steps 4.2, 5.2 and 8.4), worded by the web. */
export type CharacterWarning = Character["warnings"][number];

/**
 * `POST /api/characters/preview`: what the creation's body would be read as once created (the
 * derived block, the warnings, the languages with the native one), and the required fields still
 * blank (`name`, `alignment`, `sex`). Writes nothing.
 */
export interface CharacterPreview {
  derived: Derived;
  warnings: CharacterWarning[];
  languages: string[];
  missing: string[];
}

/** A level and the lowest XP that reaches it, for one class. */
export interface LevelXp {
  level: number;
  xp: number;
}

/**
 * A score method (ADR 0032 step 6): how many totals of how many d6, the lowest die dropped, and how
 * the totals become scores (`assign`); method VI's `base` and `max` (0 for the others).
 */
export interface ScoreMethod {
  code: string;
  rolls: number;
  dice: number;
  dropLowest: boolean;
  assign: "in_order" | "keep_per_ability" | "arrange" | "add_to_base";
  keep: number;
  base: number;
  max: number;
}

/** One total of a score roll: its dice in the order drawn, the dropped die's index, the ability (I and II). */
export interface ScoreRoll {
  dice: number[];
  dropped: number | null;
  total: number;
  ability: string;
}

/**
 * `POST /api/characters/rolls/scores`: the method, the totals, PHB Table 8's adjustments for the
 * race given (null without one) and, for method I alone, the finished scores (adjusted).
 */
export interface ScoresRoll {
  method: ScoreMethod;
  rolls: ScoreRoll[];
  adjustments: Record<string, number> | null;
  scores: Record<string, number> | null;
}

/**
 * `POST /api/characters/rolls/scores/arrangement`: the scores a placement of a roll's totals gives
 * (ADR 0032 step 6), by ability code. `rolled` is the method's base plus the totals placed, `scores`
 * that plus the race's Table 8 adjustment (the score stored); an ability with nothing placed and no
 * base is absent from both. `adjustments` is null without a race.
 */
export interface ScoresArrangement {
  method: string;
  adjustments: Record<string, number> | null;
  rolled: Record<string, number>;
  scores: Record<string, number>;
}

/** One level of a max HP roll (ADR 0032 step 7): a rolled die, or a fixed gain above the name level. */
export interface HpRollLevel {
  level: number;
  hitDie: number;
  rolled: number | null;
  counted: number | null;
  conAdjustment: number;
  gain: number;
  fixed: boolean;
}

/** `POST /api/characters/rolls/hp`: the class, the starting level, one row per level and the total. */
export interface HpRoll {
  class: string;
  level: number;
  levels: HpRollLevel[];
  total: number;
}

/** One figure of an identity roll (ADR 0032 step 5.3): `sides`-sided dice in the order drawn, the
 * table's `base` (inches, pounds or years in `unit`), their `total`, and the `value` to store
 * (centimetres, kilograms or years). */
export interface IdentityRollFigure {
  figure: IdentityFigure;
  dice: number[];
  sides: number;
  base: number;
  total: number;
  unit: "in" | "lb" | "year";
  value: number;
}

export type IdentityFigure = "height" | "weight" | "age";

/** `POST /api/characters/rolls/identity`. */
export interface IdentityRoll {
  raceCode: string;
  sex: string;
  rolls: IdentityRollFigure[];
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
  /** A thief's or a bard's points by skill (ADR 0030 steps 1, 17 and 18); absent for the other
   * classes. The skills' armour is the one worn (ADR 0033 step 14), never entered. */
  skillPoints?: Record<string, number>;
  /** The non-weapon picks (ADR 0037 step 14), checked as the editor's save checks them. */
  proficiencies?: PicksInput;
  /** The details, as the card shows them (age null for none); a refusal's field is `details.<field>`. */
  details?: Partial<CharacterDetails>;
} & Record<AbilityCode, number | null>;

/** The draft as far as it is entered, asked at `POST /api/characters/checks`. */
export interface DraftChecksRequest {
  class: string;
  /** The abilities entered so far, by code. */
  scores: Record<string, number>;
  xp: number | null;
  skillPoints: Record<string, number>;
  /** The non-weapon picks so far (ADR 0037 step 14). */
  proficiencies?: PicksInput;
  /** The race and the languages so far: the languages beyond the native one and Common spend the
   * non-weapon pool (ADR 0037 step 6), counted by the server. */
  race?: string;
  languages?: string[];
}

/**
 * The server's verdicts on a raw draft (`POST /api/characters/checks`): for every creatable class the
 * abilities whose entered score is under its minimum, and the draft class's skill point budget at the
 * level its XP gives (null for a class without skill points).
 */
export interface DraftChecks {
  unmetMinimums: { class: string; abilities: string[] }[];
  skillBudget: {
    level: number;
    /** The terms of the formula: total = first + perLevel x (level - 1). */
    first: number;
    perLevel: number;
    /** The points the level allows in all, those entered, and what is left (negative when over). */
    available: number;
    spent: number;
    left: number;
    /** The most one skill holds at the level, perSkillFirst + perSkillPerLevel x (level - 1); all
     * 0 for no such limit. */
    perSkillFirst: number;
    perSkillPerLevel: number;
    perSkillCap: number;
  } | null;
  /** ADR 0037 step 14: the draft's non-weapon pool and picks; null while the class, the XP or the
   * Intelligence is missing. */
  proficiencySlots: ProficiencySlots | null;
}
