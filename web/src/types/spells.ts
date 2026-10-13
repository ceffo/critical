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
