import type { LocalizedText } from "@/types/localized";

export interface SpellSlot {
  level: number;
  /** Slug of the English name; empty on slots saved before preparation existed. */
  prepared_spell_id: string;
  prepared_spell_name: string;
  used: boolean;
  /** A slot granted by a spell (grant_slots): blank until cast, gone at the long rest. */
  extra?: boolean;
}

/** Mirrors api/internal/domain/spellstats. Every typed field keeps `raw`, the PHB text. */
export interface SpellTerm {
  count: number;
  upper?: number;
  dice?: string;
  unit?: string;
  per_level?: boolean;
  step?: number;
  per?: string;
}

export interface SpellQuantity {
  terms: SpellTerm[];
  up_to?: boolean;
  max?: number;
  max_unit?: string;
}

/** A quantity computed at a caster level; `dice` is still to be rolled on top of `amount`. */
export interface SpellValue {
  amount: number;
  upper?: number;
  dice?: string;
  unit?: string;
}

export interface SpellStats {
  casting_time: { raw: string; kind: "timed" | "special"; time: SpellQuantity };
  range: {
    raw: string;
    kind: "self" | "touch" | "distance" | "unlimited" | "special";
    distance: SpellQuantity;
    radius?: boolean;
  };
  duration: {
    raw: string;
    kind:
      | "instantaneous"
      | "permanent"
      | "concentration"
      | "until"
      | "timed"
      | "special";
    time: SpellQuantity;
    condition?: string;
  };
  area: {
    raw: string;
    kind: "area" | "targets" | "self" | "touch" | "special";
    shape?: string;
    dims?: SpellQuantity[];
    targets?: SpellQuantity;
    noun?: string;
    /** The text says more than this typed form holds: a reader shows the raw text. */
    partial?: boolean;
  };
  components: {
    raw: string;
    v: boolean;
    s: boolean;
    m: boolean;
    m_optional?: boolean;
    special?: boolean;
  };
  save: { raw: string; kinds: ("none" | "negates" | "half" | "special")[] };
}

export interface SpellValues {
  caster_level: number;
  casting_time: SpellValue;
  range: SpellValue;
  duration: SpellValue;
  area: { dims?: SpellValue[]; targets?: SpellValue };
}

/** Mirrors spellhdl.DiceSumDTO: dice groups (in the data's order) plus a constant; no groups is a plain number. */
export interface DiceSum {
  groups: { count: number; sides: number }[];
  modifier: number;
}

/**
 * Mirrors spellhdl.SpellDamageDTO: the damage a cast rolls at caster_level, evaluated by the
 * server. `dice` is one instance; `instances` of them every round, for `rounds` rounds.
 */
export interface SpellDamage {
  caster_level: number;
  dice: DiceSum;
  instances: DiceSum;
  rounds: DiceSum;
  /** Damage type key ("fire"), from the spell effects data. */
  type?: string;
  /** The parts of one instance, rolled separately (`dice` adds them up); absent when it has none. */
  parts?: SpellDamagePart[];
  /**
   * What one dice step is, for a damage whose dice change from one round or instance to the next
   * (ADR 0008 step 15); `steps` then holds each step's dice, `dice` is empty and `rounds` or
   * `instances` counts the steps. Both absent without steps.
   */
  steps_per?: StepPer;
  steps?: SpellDamageStep[];
}

/** What one dice step of a damage is: a round, or one separate roll of the casting. */
export type StepPer = "round" | "instance";

/** Mirrors spellhdl.SpellDamageStepDTO: one dice step at caster_level (no groups and 0: no damage). */
export interface SpellDamageStep {
  step: number;
  dice: DiceSum;
}

/** What a successful save does to a damage part. */
export type DamagePartSave = "none" | "negates";

/** Mirrors spellhdl.SpellDamagePartDTO: one part of a damage instance at caster_level. */
export interface SpellDamagePart {
  dice: DiceSum;
  save: DamagePartSave;
}

/**
 * Mirrors spellhdl.SpellModeDTO: a cast-time mode the caster picks (ADR 0008 step 14), worded by
 * its key, and the damage a cast in it rolls (absent: none).
 */
export interface SpellMode {
  key: string;
  damage?: SpellDamage;
}

/**
 * Mirrors spellhdl.SpellExposureDTO: the damage of one round of exposure to the area a spell leaves
 * (ADR 0008 step 16), at caster_level: one `damage` (its `steps`, if any, are the rounds of
 * exposure; a later round rolls the last step), or the `modes` picked at each exposure.
 */
export interface SpellExposure {
  damage?: SpellDamage;
  modes?: SpellMode[];
}

/** Mirrors spellhdl.SpellNameDTO: a spell's id and its name in every language the catalogue has. */
export interface SpellName {
  id: string;
  name: LocalizedText;
}

/** The catalogue's spell names by id. */
export type SpellNames = Readonly<Record<string, SpellName>>;

/**
 * Mirrors spellhdl.SpellDTO. Every human-language text is a language map (ADR 0023): pick the UI
 * language with `localized`, which falls back to English per field. The six stat texts, the school
 * and the sphere are the English books' only. Descriptions are in the catalogue's Markdown subset
 * (paragraphs, `- ` bullet lists, pipe tables, blocks separated by a blank line).
 */
export interface Spell {
  /** Catalogue key of the spell ("fortify_l4", "hold_person_priest"); cast and receive take it. */
  id: string;
  name: LocalizedText;
  class: string;
  level: number;
  school: LocalizedText;
  sphere: LocalizedText;
  casting_time: LocalizedText;
  range: LocalizedText;
  duration: LocalizedText;
  area_of_effect: LocalizedText;
  components: LocalizedText;
  saving_throw: LocalizedText;
  description: LocalizedText;
  /** Book code of the English text (PHB, TOM, ...; WEB when no book was matched). */
  source: string;
  /** The six stat fields above, typed. Raw text stays in the text fields. */
  stats: SpellStats;
  /** Level `values` are computed for: ?caster_level=, default the character's. */
  caster_level?: number;
  /** Absent when no caster level is known (no character and no ?caster_level=). */
  values?: SpellValues;
  /** The damage a cast rolls at caster_level; absent when it rolls none or no level is known. */
  damage?: SpellDamage;
  /** Cast-time modes, each with its damage at caster_level (the first is the default; `damage` is its). */
  modes?: SpellMode[];
  /** The exposure damage at caster_level; absent when the spell has none. */
  exposure?: SpellExposure;
}

/** Mirrors spellprephdl.SpellbookEntryDTO. The cast fields come from the spell effects data. */
export interface SpellbookEntry {
  id: string;
  name: string;
  level: number;
  /** The spell grants extra slots: the ways to split them (levels, ascending); the first is the default. */
  slot_splits?: number[][];
  /** The spell spends one inventory item of this type. */
  consume_item_type?: string;
  /** The spell rolls a to-hit: a target AC is required. */
  attack_roll?: boolean;
  /** The damage a cast rolls at the Bard's level; absent when it rolls none. */
  damage?: SpellDamage;
  /** Cast-time modes to pick from, each with its damage (the first is the default); absent: none. */
  modes?: SpellMode[];
  /** The exposure damage at the Bard's level; absent when the spell has none. */
  exposure?: SpellExposure;
  /** Inventory items the spell can spend now, as the server decides; absent when none. */
  items?: SpendableItem[];
}

/** Mirrors spellprephdl.SpendableItemDTO. */
export interface SpendableItem {
  id: string;
  name: string;
  count: number;
}

export interface LevelLimit {
  level: number;
  max: number;
}

/** Mirrors api/internal/rest/handler/spellprephdl (InfoDTO). */
export interface PreparationInfo {
  spellbook: SpellbookEntry[];
  limits: LevelLimit[];
}

/** Mirrors api/internal/rest/handler/casthdl (RollDTO). */
export interface DiceRoll {
  formula: string;
  dice: { sides: number; value: number }[];
  total: number;
  /** A damage instance's part rolls, in the order of CastDamage.parts; absent when it has none. */
  parts?: DiceRoll[];
  /** The round or instance (1-based) of a damage made of dice steps; absent without steps. */
  step?: number;
}

/** Mirrors casthdl AppliedBuffDTO. `applied` is false when the buff went on somebody else. */
export interface CastBuff {
  id: string;
  name: string;
  rounds: number;
  charges: number;
  applied: boolean;
  durationRoll?: DiceRoll;
  chargesRoll?: DiceRoll;
  chargeUnit?: string;
  strGainRoll?: DiceRoll;
  /** Engine-tracked effects, keyed like the buff API (to_hit, ac, str_bonus, spells_buff...). */
  effects: Record<string, number>;
}

export interface CastDamage {
  dice: string;
  type: string;
  instances: number;
  rounds: number;
  rolls: DiceRoll[];
  total: number;
  /** The parts of one instance and what a save does to each; absent when it has none. */
  parts?: CastDamagePart[];
  /** The total without the parts a save negates; absent when the damage has no parts. */
  totalOnSave?: number;
  /** Set for a damage made of dice steps: each roll is one step (its `step`), `dice` is empty. */
  stepsPer?: StepPer;
}

/** Mirrors casthdl DamagePartDTO. */
export interface CastDamagePart {
  formula: string;
  save: DamagePartSave;
}

/** An effect the engine does not track (seed modifier key, or "<ability>_gain"/"_set"). */
export interface CastUntracked {
  key: string;
  value: number;
  condition?: string;
}

export type CastTextOnly =
  "no_effect" | "low_confidence" | "utility" | "not_on_self" | "by_hand";

/** Mirrors casthdl CastResultDTO: what POST /api/spells/{slot}/cast and /api/spells/receive did. */
export interface CastResult {
  entryId: number;
  source: "cast" | "received";
  spellId: string;
  name: string;
  level: number;
  casterLevel: number;
  slotIndex?: number;
  /** The key of the cast-time mode the spell was cast in; absent for a spell without modes. */
  mode?: string;
  kind?: "buff" | "damage" | "utility";
  confidence?: "high" | "low";
  target?: string;
  note?: string;
  spell?: Spell;
  save?: { type: string; effect: string };
  textOnly?: CastTextOnly;
  buff?: CastBuff;
  damage?: CastDamage;
  untracked: CastUntracked[];
  slots?: SpellSlot[];
  /** Levels of the extra slots this cast added. */
  extraSlots?: number[];
  /** The inventory item the cast spent. */
  consumedItem?: CastConsumedItem;
  /** The spell's to-hit roll. */
  attack?: CastAttack;
  /** The exposure damage at casterLevel, to roll each round of exposure from; absent: none. */
  exposure?: SpellExposure;
}

/** The body of POST /api/spells/exposure (casthdl ExposureRequestDTO). */
export interface ExposureRequest {
  spellId: string;
  casterLevel: number;
  /** The exposure mode's key (omitted: the first). */
  mode?: string;
  /** The round of exposure, from 1 (omitted: 1). */
  round?: number;
  /** The Bard is the one exposed: the server takes the damage off his HP. */
  onSelf?: boolean;
}

/** Mirrors casthdl ExposureResultDTO: one round of exposure rolled and logged. */
export interface ExposureResult {
  entryId: number;
  spellId: string;
  name: string;
  casterLevel: number;
  /** The exposure mode rolled; absent when the exposure has none. */
  mode?: string;
  round: number;
  onSelf: boolean;
  /** The HP change the ledger applied on the Bard (the floor may cut it); 0 when not on him. */
  hpDelta: number;
  damage: CastDamage;
  /** The exposure damage at casterLevel, for the next round of exposure. */
  exposure?: SpellExposure;
}

/** Mirrors casthdl ConsumedItemDTO. */
export interface CastConsumedItem {
  itemId: string;
  name: string;
  remaining: number;
}

/** Mirrors casthdl SpellAttackDTO: a spell's to-hit roll (one d20, no crit). */
export interface CastAttack {
  d20Roll: number;
  baseThac0: number;
  modifiers: { kind: string; detail?: string; value: number }[];
  effectiveThac0: number;
  targetAc: number;
  targetACNeeded: number;
  hit: boolean;
  hitMargin: number;
}

/** Mirrors spellhdl.SpellPageDTO: one page of GET /api/spells. */
export interface SpellPage {
  items: Spell[];
  /** Matches after all filters, across every page. */
  total: number;
  page: number;
  page_size: number;
}

/** Mirrors spellhdl.FilterOptionsDTO (GET /api/spells/filters): the lookup's dropdown choices. */
export interface SpellFilterOptions {
  /** School group keys: the eight PHB schools, then "other". */
  schools: string[];
  /** Priest sphere names. */
  spheres: string[];
}
