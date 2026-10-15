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
    kind: "instantaneous" | "permanent" | "concentration" | "until" | "timed" | "special";
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
}

export interface Spell {
  /** Unique key of the spell ("fortify_l4", "hold_person_priest"); cast and receive take it. */
  id: string;
  name: string;
  class: string;
  level: number;
  school: string;
  sphere: string;
  casting_time: string;
  range: string;
  duration: string;
  area_of_effect: string;
  components: string;
  saving_throw: string;
  description: string;
  /** The six stat fields above, typed. Raw text stays in the string fields. */
  stats: SpellStats;
  /** Level `values` are computed for: ?caster_level=, default the character's. */
  caster_level?: number;
  /** Absent when no caster level is known (no character and no ?caster_level=). */
  values?: SpellValues;
  /** The damage a cast rolls at caster_level; absent when it rolls none or no level is known. */
  damage?: SpellDamage;
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
}

/** An effect the engine does not track (seed modifier key, or "<ability>_gain"/"_set"). */
export interface CastUntracked {
  key: string;
  value: number;
  condition?: string;
}

export type CastTextOnly = "no_effect" | "low_confidence" | "utility" | "not_on_self" | "by_hand";

/** Mirrors casthdl CastResultDTO: what POST /api/spells/{slot}/cast and /api/spells/receive did. */
export interface CastResult {
  entryId: number;
  source: "cast" | "received";
  spellId: string;
  name: string;
  level: number;
  casterLevel: number;
  slotIndex?: number;
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
