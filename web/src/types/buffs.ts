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
  "base_ac",
  "to_hit",
  "damage",
  "initiative",
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
  /** A condition on the AC effects (ADR 0033 step 12): they count only without magic armour. */
  without_magic_armour?: boolean;
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
   * server; never sent. The timeline draws the elapsed rounds from it.
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
   * The proficiency whose successful check put the buff on (ADR 0037 step 20: Tumbling), absent for
   * any other buff. Set by the server; never sent. The UI names the buff from the proficiency
   * (buffName), as it does from `spellId`.
   */
  proficiencyId?: string;
  /**
   * The psionic power the row keeps up (ADR 0043 step 13), with what Next Round charges every
   * `maintenancePeriod` rounds. Absent for any other buff. Set by the server; never sent.
   */
  powerId?: string;
  maintenanceCost?: number;
  maintenancePeriod?: number;
  /**
   * The opponent mind a tangent or contact row holds (ADR 0044 step 3), by the player's label, with
   * a tangent row's count (1 or 2; absent on a contact row). Absent for any other buff. Set by the
   * server; never sent. Such a row has no `powerId`.
   */
  target?: string;
  tangents?: number;
  /** A defence mode's row (ADR 0044 steps 4 and 5): raised for the round, or standing until dismissed. */
  defence?: "raised" | "standing";
  /**
   * A row for a spell cast on someone else or on an area (ADR 0004 step 20.5): it only counts down
   * and never applies. Absent for a buff on the character. Set by the server; never sent.
   */
  onOther?: boolean;
  /**
   * The row's spell has end effects (ADR 0004 step 21): stopping it asks how it ended (`EndCause`).
   * Absent otherwise. Set by the server; never sent.
   */
  asksEndCause?: boolean;
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
  /** DELETE /api/buffs/{id} only: what the stopped row's end effects did (absent: none applied). */
  ended?: Ended;
}

/** How the player says a row with end effects ended (ADR 0004 step 21); running out is the server's. */
export type EndCause = "voluntary" | "dispelled";

/** One end effect applied (ADR 0004 step 21): the dice, every die, the total. */
export interface EndEffectRoll {
  spellId: string;
  /** The row's stored name; the UI names it from spellId. */
  buff: string;
  cause: EndCause | "expired";
  kind: "heal";
  dice: string;
  rolls: { sides: number; value: number }[];
  constant: number;
  total: number;
}

/** What the end of one or more rows did: the effects, and current HP before and after. */
export interface Ended {
  effects: EndEffectRoll[];
  hpBefore: number;
  hpAfter: number;
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
  /** A warrior's 18/xx percentiles (ADR 0030 steps 9 and 10), 0 for none. */
  baseExceptional: number;
  effectiveExceptional: number;
}

/**
 * The round's initiative (ADR 0013): the plain d10, lowest acts first, and the log entry that
 * rolled it. Null out of combat.
 */
/** What an initiative modifier comes from (ADR 0035 item 6). */
export type InitiativeSource = "weapon" | "effect" | "spell" | "item";

/**
 * How a casting time counted: absent, its segments; a round or more; special (adds 0); an item whose
 * term is the casting time of the spell it holds (a scroll), not counted (adds 0).
 */
export type CastTiming = "end_of_round" | "special" | "spell_casting_time";

/**
 * One term of the initiative, named by its source: the weapon in hand's speed, an initiative effect
 * (a buff, `spellId` its spell, or an item power, `itemId` its piece), or the spell cast or item
 * used that corrected the round. `name` is the name recorded with it (the fallback label).
 */
export interface InitiativeModifier {
  source: InitiativeSource;
  value: number;
  itemId?: string;
  name?: string;
  spellId?: string;
  magicKind?: string;
  timing?: CastTiming;
}

/**
 * The round's initiative (ADR 0035 item 6): `value` the total, the lowest acts first; the d10 and
 * the modifiers are its breakdown; `endOfRound` when a spell cast that round takes a round or more.
 */
export interface Initiative {
  value: number;
  d10: number;
  modifiers: InitiativeModifier[];
  endOfRound: boolean;
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
  /** advance and pass-time: what the expired rows' end effects did (absent: none applied). */
  ended?: Ended;
}

/** One maintained row's share of the next Next Round (ADR 0043 step 14). */
export interface RowCharge {
  buffId: string;
  /** PSPs it pays: 0 when its period is not due, or a contact is free while a power uses it. */
  charge: number;
  /** The PSPs left cannot pay it: it would end. */
  ends?: boolean;
}

/** GET /api/round/advance/preview: what Next Round would charge, from the server, nothing written. */
export interface NextRoundPreview {
  pspSpent: number;
  pspBefore: number;
  pspAfter: number;
  rows: RowCharge[];
}
