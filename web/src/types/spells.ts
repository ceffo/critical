export interface SpellSlot {
  level: number;
  /** Slug of the English name; empty on slots saved before preparation existed. */
  prepared_spell_id: string;
  prepared_spell_name: string;
  used: boolean;
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
}

/** Mirrors api/internal/rest/handler/spellprephdl (InfoDTO). */
export interface SpellbookEntry {
  id: string;
  name: string;
  level: number;
}

export interface LevelLimit {
  level: number;
  max: number;
}

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
