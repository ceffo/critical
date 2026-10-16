// Mirrors api/internal/rest/handler/buffhdl (BuffEffectDTO, BuffListDTO, ItemPowerDTO, StrengthDTO,
// RoundAdvanceDTO, PassTimePreviewDTO).

/** Effect keys a buff can carry (snake_case, as on the wire). */
export const BUFF_EFFECT_KEYS = [
  "weapon_buff",
  "spells_buff",
  "multiplier",
  "save_bonus",
  "str_bonus",
  "str_max",
  "min_str",
  "ac",
  "to_hit",
  "damage",
] as const;

export type BuffEffectKey = (typeof BUFF_EFFECT_KEYS)[number];

/** The kind of weapon a roll is made with (docs/adr/0016-inventory.md step 8). */
export type WeaponKind = "melee" | "missile";

/**
 * A buff's (or an item power's) effects. `weapon_kind` is a condition, not an effect (ADR 0004
 * step 18): with one, the other effects count only for an attack with a weapon of that kind.
 */
export type BuffEffects = Partial<Record<BuffEffectKey, number>> & {
  weapon_kind?: WeaponKind;
};

export interface BuffEffect {
  id: string;
  name: string;
  /** Left to absorb (Stoneskin layers, Mirror Images); 0 = none. The buff ends at 0. */
  charges: number;
  /** Rounds left, counted down by Next Round; 0 = not timed. The buff expires at 0. */
  duration: number;
  effects: BuffEffects;
  /** Free text shown on the effect's row ("" = none). */
  note: string;
  /**
   * Round the buff was added in (0: out of combat, or stored before the server kept it). Set by the
   * server; never sent. The timeline draws the elapsed turns from it.
   */
  startRound: number;
  /**
   * Charges the buff started with, kept by the server (0: unknown, a buff stored before it was kept,
   * or no charges). Never sent. The timeline shows "left / total" and the used pips from it.
   */
  chargesTotal: number;
  /**
   * Catalogue id of the spell the buff comes from (cast or received), absent for a buff entered by
   * hand or stored before the server kept it. Set by the server; never sent. The UI names the buff
   * from the catalogue in the player's language (buffName), `name` otherwise.
   */
  spellId?: string;
  /**
   * On list responses: false while min_str is not met by the effective STR, or (a Strength bonus)
   * while an Improved Strength replaces it.
   */
  applies?: boolean;
}

/**
 * A carried item's power that is on (ADR 0016 step 13): served next to the buffs, never mixed into
 * them. No name (the UI names it from the item by id), no counters, nothing to edit or dismiss.
 */
export interface ItemPower {
  itemId: string;
  /** The power's id within its item ("thunder_hammer"). */
  powerId: string;
  effects: BuffEffects;
}

/** GET /api/buffs, also the answer of every buff change: the buffs and the item powers that are on. */
export interface BuffList {
  buffs: BuffEffect[];
  powers: ItemPower[];
}

/** PATCH /api/buffs/{id}: omitted fields are unchanged; 0 on a counter the buff uses ends it. */
export interface BuffEdit {
  duration?: number;
  charges?: number;
  /** "" clears the note. */
  note?: string;
}

/** A saved buff template (GET/POST /api/buffs/saved): the starting values of a buff. */
export interface SavedBuff {
  id: number;
  name: string;
  duration: number;
  charges: number;
  effects: BuffEffects;
  note: string;
}

/** GET /api/stats/str */
export interface Strength {
  base: number;
  effective: number;
}

/**
 * The round's initiative (ADR 0013): the plain d10, lowest acts first, and the log entry that
 * rolled it. Null out of combat.
 */
export interface Initiative {
  value: number;
  logEntryId: number;
}

/** GET /api/round */
export interface RoundResponse {
  round: number;
  initiative: Initiative | null;
}

/** GET /api/round/pass-time/preview?hours=: what Time passes would do, nothing written. */
export interface PassTimePreview {
  rounds: number;
  /** The buffs that would expire. */
  expiring: BuffEffect[];
}

/**
 * POST /api/round/advance, POST /api/round/reset, PUT /api/round (the last two never expire buffs),
 * POST /api/round/pass-time.
 */
export interface RoundAdvanceResponse {
  roundNumber: number;
  activeBuffs: BuffEffect[];
  expiredBuffs: BuffEffect[];
  /** pass-time only: the rounds the hours gave. */
  roundsPassed?: number;
  /** advance only: the new round's initiative. */
  initiative?: Initiative;
}
