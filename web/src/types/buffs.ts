// Mirrors api/internal/rest/handler/buffhdl (BuffEffectDTO, StrengthDTO, RoundAdvanceDTO,
// PassTimePreviewDTO).

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

export type BuffEffects = Partial<Record<BuffEffectKey, number>>;

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
   * On list responses: false while min_str is not met by the effective STR, or (a Strength bonus)
   * while an Improved Strength replaces it.
   */
  applies?: boolean;
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
