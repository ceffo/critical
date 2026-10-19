import type { LocalizedText } from "@/types/localized";
import type { Modifier } from "@/types/combat";
import type { MarkedTexts } from "@/lib/valueMarks";

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
  or_more?: boolean;
  max?: number;
  max_unit?: string;
  /** The way an area's extent runs ("2 ft. wide"). */
  axis?: "wide" | "long" | "high" | "thick" | "deep";
  /** Where on its figure the extent is measured ("wide at base"). */
  site?: "base" | "end";
}

/** Mirrors spellstats.AreaOfEffect: one reading of an area of effect, or a choice between readings. */
export interface SpellArea {
  raw: string;
  kind:
    | "area"
    | "targets"
    | "self"
    | "touch"
    | "alternatives"
    | "together"
    | "special";
  shape?: string;
  /** What a radius or diameter measures ("sphere" in "10-ft. radius sphere"). */
  figure?: string;
  dims?: SpellQuantity[];
  /** The number of such areas when more than one ("Two 10-ft. cubes/level"). */
  count?: number;
  targets?: SpellQuantity;
  noun?: string;
  /** The size of the group `noun` names ("unit of up to 200 individuals"), counted in `members_noun`. */
  members?: SpellQuantity;
  members_noun?: string;
  /** The extents bound where the targets are ("1 animal within 30 ft."). */
  within?: boolean;
  /** The extents are the size of each target ("1 object up to 10 ft. long"), not an area. */
  size?: boolean;
  /** The area is centred on the creature or object touched. */
  around_touched?: boolean;
  /** The text says more than this typed form holds: a reader shows the raw text. */
  partial?: boolean;
  /** The readings of an "alternatives" kind ("1 creature or object"), each an area or targets. */
  alternatives?: SpellArea[];
  /** The readings of a "together" kind ("Caster + 1 person/8 levels"), summed. */
  parts?: SpellArea[];
  /** The book adds "special": "+ special", "or special" or "(special)". */
  special?: "plus" | "or" | "note";
  /** Who it usually is ("One individual (usually caster)"). */
  usually?: string;
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
  area: SpellArea;
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

/** Mirrors spelleffectsvc.DiceSumWire: dice groups (in the data's order) plus a constant; no groups is a plain number. */
export interface DiceSum {
  groups: { count: number; sides: number }[];
  modifier: number;
}

/**
 * Mirrors spelleffectsvc.DamageAtWire: the damage a cast rolls at caster_level, evaluated by the
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

/** Mirrors spelleffectsvc.DamageStepAtWire: one dice step at caster_level (no groups and 0: no damage). */
export interface SpellDamageStep {
  step: number;
  dice: DiceSum;
}

/** What a successful save does to a damage part. */
export type DamagePartSave = "none" | "negates";

/** Mirrors spelleffectsvc.DamagePartAtWire: one part of a damage instance at caster_level. */
export interface SpellDamagePart {
  dice: DiceSum;
  save: DamagePartSave;
}

/**
 * Mirrors spelleffectsvc.ModeAtWire: a cast-time mode the caster picks (ADR 0008 step 14), worded by
 * its key, and the damage a cast in it rolls (absent: none).
 */
export interface SpellMode {
  key: string;
  damage?: SpellDamage;
}

/**
 * Mirrors spelleffectsvc.ExposureAtWire: the damage of one round of exposure to the area a spell leaves
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
 * Mirrors wire.SpellDTO (api/internal/rest/wire). Every human-language text is a language map (ADR 0023): pick the UI
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
  /**
   * The gameplay values of the description and the six stat texts, by field then language, as
   * spans that concatenate to the text (ADR 0021 step 15). Only a text with a value is listed.
   */
  marked?: MarkedTexts;
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
  /** POST /api/spells/effect takes the spell: it adds a buff (ADR 0004 step 19); absent: it adds none. */
  effect?: boolean;
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
  /** The choice of target a buff's cast offers, as the server decides; absent when none. */
  cast_target?: CastTarget;
}

/** Who a cast lands on. */
export type CastOn = "self" | "other";

/**
 * Mirrors spellprephdl.CastTargetDTO: where a cast naming no target lands (`default`) and the
 * one other target the caster may name (`alternate`, sent as `onSelf`).
 */
export interface CastTarget {
  default: CastOn;
  alternate: CastOn;
}

/** Mirrors spellprephdl.SpendableItemDTO. */
export interface SpendableItem {
  id: string;
  /** A language map (ADR 0023 step 9): pick with `localized`. */
  name: LocalizedText;
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

/** Mirrors castsvc.FormulaRoll: one evaluated formula of a cast, every die it threw. */
export interface FormulaRoll {
  formula: string;
  dice: { sides: number; value: number }[];
  total: number;
  /** A damage instance's part rolls, in the order of CastDamage.parts; absent when it has none. */
  parts?: FormulaRoll[];
  /** The round or instance (1-based) of a damage made of dice steps; absent without steps. */
  step?: number;
}

/**
 * Mirrors castsvc.CastBuff. `applied` is false when the buff went on somebody else. The fields
 * from `id` on are absent on casts logged before the payload was the cast's answer.
 */
export interface CastBuff {
  name: string;
  rounds?: number;
  charges?: number;
  strGain?: number;
  applied: boolean;
  id?: string;
  durationRoll?: FormulaRoll;
  chargesRoll?: FormulaRoll;
  chargeUnit?: string;
  strGainRoll?: FormulaRoll;
  /** Engine-tracked effects, keyed like the buff API (to_hit, ac, str_bonus, spells_buff...). */
  effects?: Record<string, number>;
}

/** Mirrors castsvc.CastDamage. `dice` to `rolls` are absent on casts logged before they were recorded. */
export interface CastDamage {
  total: number;
  type: string;
  dice?: string;
  instances?: number;
  rounds?: number;
  rolls?: FormulaRoll[];
  /** The parts of one instance and what a save does to each; absent when it has none. */
  parts?: CastDamagePart[];
  /** The total without the parts a save negates; absent when the damage has no parts. */
  totalOnSave?: number;
  /** Set for a damage made of dice steps: each roll is one step (its `step`), `dice` is empty. */
  stepsPer?: StepPer;
}

/** Mirrors castsvc.CastDamagePart. */
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

/**
 * Mirrors castsvc.CastBreakdown: what a cast, a received spell or an added effect did. It is both
 * the payload of the cast's journal entry and (with CastResult's keys) the cast endpoints' answer.
 * Fields marked "older casts" are absent on entries logged before they were recorded.
 */
export interface CastBreakdown {
  action: "cast" | "received" | "effect";
  spellId: string;
  /** The spell's name as recorded; the UI names the spell from its id. */
  spell: string;
  casterLevel: number;
  /** The key of the cast-time mode the spell was cast in; absent for a spell without modes. */
  mode?: string;
  buff?: CastBuff;
  damage?: CastDamage;
  /** Levels of the extra slots this cast added. */
  extraSlots?: number[];
  /** The inventory item the cast spent. */
  item?: CastItem;
  /** The spell's to-hit roll. */
  attack?: CastAttack;
  /** The target's saving throw (the DM's side); absent on older casts. */
  save?: { type: string; effect: string };
  /** The low-Wisdom spell failure d100 (ADR 0030 step 14); absent: no die. `failed`: the slot is
   * spent and nothing else applies. */
  failure?: SpellFailure;
  /** The exposure damage at casterLevel, to roll each round of exposure from; absent: none. */
  exposure?: SpellExposure;
  /** The spell's level (older casts: absent). */
  level?: number;
  /** The slot spent (a cast only). */
  slotIndex?: number;
  kind?: "buff" | "damage" | "utility";
  confidence?: "high" | "low";
  target?: string;
  textOnly?: CastTextOnly;
  /** Effects to apply by hand (older casts: absent). */
  untracked?: CastUntracked[];
}

/**
 * Mirrors casthdl castAnswer: the answer of POST /api/spells/{slot}/cast, /receive and /effect,
 * the cast's logged payload plus the entry id, the catalogue spell and the slots after a cast.
 */
export interface CastResult extends CastBreakdown {
  logEntryId: number;
  /** The spell from the catalogue, values at casterLevel; absent when not in the catalogue. */
  catalogueSpell?: Spell;
  /** The slots after a cast. */
  slots?: SpellSlot[];
}

/** A cast's spell failure roll (ADR 0030 step 14): failed when the d100 is at or under the chance. */
export interface SpellFailure {
  d100: number;
  chance: number;
  failed: boolean;
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

/**
 * Mirrors castsvc.ExposureBreakdown: one round of exposure, the payload of its journal entry and
 * (with ExposureResult's keys) the exposure endpoint's answer.
 */
export interface ExposureBreakdown {
  spellId: string;
  /** The spell's name as recorded. */
  spell: string;
  casterLevel: number;
  /** The exposure mode rolled; absent when the exposure has none. */
  mode?: string;
  round: number;
  /** The Bard was exposed: the entry's HP delta is the damage. */
  onSelf: boolean;
  damage: CastDamage;
  /** The exposure damage at casterLevel, for the next round of exposure. */
  exposure?: SpellExposure;
}

/** Mirrors casthdl exposureAnswer: one round of exposure rolled and logged. */
export interface ExposureResult extends ExposureBreakdown {
  logEntryId: number;
  /** The HP change the ledger applied on the Bard (the floor may cut it); 0 when not on him. */
  hpDelta: number;
}

/** Mirrors castsvc.CastItem: the item a cast spent and how many are left. */
export interface CastItem {
  itemId: string;
  name: string;
  remaining: number;
}

/**
 * Mirrors castsvc.CastAttack: a spell's to-hit roll (one d20, no crit). The fields from
 * `baseThac0` on are absent on casts logged before the payload was the cast's answer.
 */
export interface CastAttack {
  d20: number;
  targetAc: number;
  hit: boolean;
  baseThac0?: number;
  /** As the attack's (combatsvc.BreakdownModifier, the one modifier shape). */
  modifiers?: Modifier[];
  effectiveThac0?: number;
  targetACNeeded?: number;
  hitMargin?: number;
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
  /** The spell levels the catalogue holds, ascending: the lookup's level chips. */
  levels: number[];
}
