export interface SpellSlot {
  level: number;
  /** Slug of the English name; empty on slots saved before preparation existed. */
  prepared_spell_id: string;
  prepared_spell_name: string;
  used: boolean;
}

export interface Spell {
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
