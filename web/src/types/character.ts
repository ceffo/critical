import type { LocalizedText } from "@/types/localized";
import type { SkillBudget } from "@/types/skills";
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
  /** ADR 0042 step 7: the class entry the level is taken in. */
  class: string;
  /** ADR 0042 step 10: the number of the character's classes, what the die, the fixed gain and the
   * Constitution adjustment are divided by (1: one class). */
  classes: number;
  level: number;
  die: number;
  fixedGain: number;
  /** ADR 0037 step 15: what the level grants beyond hit points. */
  grants: LevelGrants;
}

/** ADR 0037 step 15: weapon and non-weapon slots and a thief's or a bard's skill points, granted
 * by a level or left to spend (0: none). ADR 0030 step 20.8: a mage's free spells, the level's or
 * the ones owed (empty: none). */
export interface LevelGrants {
  weaponSlots: number;
  nonWeaponSlots: number;
  skillPoints: number;
  freeSpells: FreeSpell[];
}

/** ADR 0030 step 20.8: one free spell, of `spellLevel`, granted by the realized level `level` (the
 * add's `grantLevel`). */
export interface FreeSpell {
  level: number;
  spellLevel: number;
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
  /** ADR 0042 step 10.4: the class entry, the divisor and the die's (or fixed gain's) and the
   * Constitution adjustment's shares; absent from a record written before multi-class. */
  class?: string;
  classes?: number;
  dieShare?: number;
  conShare?: number;
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
  levelUp: LevelUp;
  /** Step 12: the class may specialize in a weapon (the weapon editor offers the flag). */
  specializes: boolean;
  /** Steps 9 and 10: the class enters an 18/xx and reads the warrior's scale above 18 (its
   * `exceptional_strength` feature). */
  exceptionalStrength: boolean;
  /** Step 20.1: what a mage or a bard may write in the spellbook; null for the other classes. */
  spellbook: SpellbookRules | null;
  /** Step 14: the percent chance a cast fails (low Wisdom); 0: no die. */
  spellFailure: number;
  /** Step 16: PHB Table 61 at the realized level, in the book's order; null for a class that does
   * not turn undead. */
  turning: TurningTarget[] | null;
  /** Steps 17 and 18 (ADR 0040 step 3.5): one set per skill set the class grants at the realized
   * level, each with its own budget; empty for a class without. */
  skillSets: DerivedSkillSet[];
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

/** One skill set of the sheet (ADR 0040 step 3.5): its `set` code, the skill armour it reads (ADR
 * 0033 step 14), each skill with its terms, the entered points against its budget at the realized
 * level (`warnings`; a bard's has no per-skill limit) and the budget with the terms of its formula
 * (GET /api/skills' set `budget`; null before the level its points start). */
export interface DerivedSkillSet {
  set: string;
  skillArmor: string;
  skills: ThiefSkillTerms[];
  warnings: SkillBudgetWarning[];
  budget: SkillBudget | null;
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

/** How a spellbook entry came in (ADR 0030 step 20.5). */
export type SpellbookOrigin = "learned" | "given" | "starting" | "level_grant";

/** The entry a roll-free add wrote (`POST /api/character/spellbook/add`, ADR 0030 step 20.6;
 * learnhdl.AddedDTO), also the payload of a spell_add journal entry. */
export interface SpellAdded {
  spellId: string;
  spellLevel: number;
  origin: SpellbookOrigin;
  /** ADR 0030 step 20.8: the realized level whose free spell this is; absent otherwise. */
  grantLevel?: number;
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

  /** The class entries, in the order the classes were taken (ADR 0042 step 1). */
  classes: ClassEntry[];
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
    /** ADR 0042 step 7: `level_limit` (field `level`) names the class entry above the race's
     * limit, its level in `value` and the limit in `max`. */
    class?: string;
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

/** One class of a character (ADR 0042 step 1): active earns XP and levels, frozen is a dual-class's old class. */
export interface ClassEntry {
  /** The class code, worded by `details.classes.<code>`. */
  class: string;
  /** The level taken in the class (ADR 0030 step 7). */
  level: number;
  /** The XP entered for the class. */
  startingXp: number;
  state: "active" | "frozen";
  /** The level a frozen entry was left at; 0 for an active one. */
  leftAtLevel: number;
}

/** A character in its owner's list (ADR 0029 step 7.2). */
export interface CharacterSummary {
  id: number;
  name: string;
  /** The class entries, each with its class code (worded by `details.classes.<code>`) and its level (ADR 0030 step 3). */
  classes: ClassEntry[];
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
  /** The 18/xx is not here: the draft answers it (`CreationDraft.exceptionalStrength`). */
  classes: {
    class: string;
    races: string[];
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
    /** ADR 0030 step 20.7: the class's starting spellbook, `startingSpells` the spells it is
     * offered pre-ticked (removable); null for a class without a spellbook, absent from an older
     * release. */
    spellbook?: { startingSpells: string[] } | null;
  }[];
  /** The catalogue's wizard spells by spell level then id (ADR 0030 step 20.7): the starting
   * spellbook's choices, named by `GET /api/spells/names`. Absent from an older release. */
  wizardSpells?: SpellOption[];
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

/** A catalogue wizard spell the starting spellbook offers: its id and spell level. */
export interface SpellOption {
  id: string;
  level: number;
}

/**
 * A draft's starting spellbook (ADR 0030 step 20.7): the limits of step 20.1 at the starting level
 * (`classMaxSpellLevel`, the class's highest spell level there, 0 for none; the Intelligence's;
 * Table 4's maximum per spell level, meaningless when `noMaximum`) and, for each spell level the
 * character may enter (`levels`, empty for none), the spells entered of it and not refused.
 */
export interface StartingSpellbook {
  classMaxSpellLevel: number;
  intMaxSpellLevel: number;
  maxSpellsPerLevel: number;
  noMaximum: boolean;
  levels: { level: number; count: number }[];
}

/** One refusal of a creation draft, as `POST /api/characters` would answer it: the field and the code
 * (an invalid input's code, or `over_budget` with the budget's `spent` and `limit`). */
export interface DraftRefusal {
  field: string;
  code: string;
  spent?: number;
  limit?: number;
}

/**
 * `POST /api/characters/draft` (ADR 0034, amendment of 2026-10-20): the sheet the draft would read
 * as once created, in `GET /api/character`'s shapes (`derived` null and the lists empty while the
 * class, race, scores, skill points or XP are missing or refused; the languages with the native one), the
 * creation's refusals, the required fields still blank, and for every creatable class the abilities
 * whose entered score is under its minimum. Writes nothing.
 */
export interface CreationDraft {
  derived: Derived | null;
  /** A thief's or a bard's skill point budget at the starting level, as soon as the class and XP
   * read (a score out of range does not hide it); null otherwise. The derived block's when there
   * is one. */
  skillPoints: SkillBudget | null;
  /** A mage's or a bard's starting spellbook at the starting level (ADR 0030 step 20.7); null for
   * another class or while `derived` is. A refused spell is a refusal on `spellbook.<id>`. Absent
   * from an older release. */
  spellbook?: StartingSpellbook | null;
  warnings: CharacterWarning[];
  languages: string[];
  refusals: DraftRefusal[];
  missing: string[];
  unmetMinimums: { class: string; abilities: string[] }[];
  /** ADR 0030 step 9 through the class's features: the class lists `exceptional_strength`, the
   * Strength entered is 18 and the race rolls it. The form offers the 18/xx field on it (no rule
   * of its own). */
  exceptionalStrength: boolean;
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
  /** A mage's or a bard's starting spellbook (ADR 0030 step 20.7): the catalogue ids, in order,
   * written without a roll; absent for none. A refused spell's field is `spellbook.<id>`. */
  spellbook?: string[];
} & Record<AbilityCode, number | null>;
