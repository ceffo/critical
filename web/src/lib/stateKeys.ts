/**
 * What a write changed, as the server names it (ADR 0018 step 11), and what the web reads again for
 * it. Every state-changing answer carries the state keys of the transactions it committed in
 * `X-State-Keys` ("buffs,hp,items:arrows,log"; empty: nothing logged). The web refetches the slices
 * mapped to each key's family and nothing else. A refusal of the input (400) names nothing and reads
 * nothing. A success without the header (a server of the release before), a stale refusal (404,
 * 409), a server error or a lost answer is unknown: every slice is read again (noteAnswer).
 */

/** The answer header naming the state keys a write changed. */
export const STATE_KEYS_HEADER = "X-State-Keys";

/**
 * Every state-key family the server names: the log's own two ("log": the journal changed, "hp": an
 * entry moved current HP) and the families the services snapshot. The composition test
 * `TestStateKeys_TheWebMapsEveryFamily` keeps this list equal to the server's.
 */
export const STATE_FAMILIES = [
  "log",
  "hp",
  "ac_override",
  "buffs",
  "class_entries",
  "initiative",
  "items",
  "proficiencies",
  "realized_level",
  "round",
  "saved_buffs",
  "skill_armor",
  "skill_points",
  "spell_slots",
  "spellbook",
  "weapons",
  "xp",
] as const;
export type StateFamily = (typeof STATE_FAMILIES)[number];

/** The slices of server state the web holds, each read by one fetch of the action module. */
export const SLICES = [
  "log",
  "hp",
  "buffs",
  "savedBuffs",
  "round",
  "strength",
  "items",
  "weapons",
  "ac",
  "slots",
  "spellInfo",
  "skills",
  "character",
  "xp",
] as const;
export type Slice = (typeof SLICES)[number];

/** What an item changes beside its count: the weapons are pieces, the AC follows the worn pieces
 * (ADR 0033 step 10), the effects list shows the carried items' powers (ADR 0016 step 13) and
 * STR in effect with them, a thief's skills follow the armour, and the spellbook lists the items a
 * spell can spend. */
const ITEM_SLICES = [
  "items",
  "weapons",
  "ac",
  "buffs",
  "strength",
  "skills",
  "spellInfo",
] as const satisfies readonly Slice[];

/** What the level changes: the sheet (THAC0, saves, budget), the spells a level allows and their
 * slots, the skills (ADR 0030 step 3). */
const LEVEL_SLICES = [
  "character",
  "slots",
  "spellInfo",
  "skills",
] as const satisfies readonly Slice[];

/**
 * The one invalidation map: the slices a change of each family can change. Exhaustive by type: a
 * family without its entry (or an entry for no family) fails the type check.
 */
export const INVALIDATES = {
  log: ["log"],
  hp: ["hp"],
  ac_override: ["ac"],
  // STR, the AC and the abilities a skill uses follow the effects in play.
  buffs: ["buffs", "strength", "ac", "skills"],
  // Read with the round.
  initiative: ["round"],
  items: ITEM_SLICES,
  // The picks' checks are skills; the sheet carries the slots and warns (ADR 0037 steps 11 and 12).
  // A weapon is specialized by its type's pick, so the pieces and the attack form follow (step 9).
  proficiencies: ["skills", "character", "items", "weapons"],
  // The level and its max HP (ADR 0030 step 7.3).
  // The class entries: a class taken or left again reshapes the whole sheet (ADR 0042 step 3).
  // The XP ledger's total and progress belong to the first active entry, so they change too.
  class_entries: [...LEVEL_SLICES, "hp", "xp"],
  realized_level: [...LEVEL_SLICES, "hp"],
  round: ["round"],
  saved_buffs: ["savedBuffs"],
  skill_armor: ["skills", "character"],
  // The sheet warns about the budget.
  skill_points: ["skills", "character"],
  spell_slots: ["slots"],
  // The sheet's "to spend" counts the free spells the book does not hold yet (ADR 0030 step 20.8).
  spellbook: ["spellInfo", "character"],
  // The weapons of before the one equipment model are pieces now (ADR 0033 step 26.2).
  weapons: ITEM_SLICES,
  // The level follows the XP (ADR 0030 step 3).
  xp: ["xp", ...LEVEL_SLICES],
} as const satisfies Record<StateFamily, readonly Slice[]>;

/** What one or more writes changed: the keys they named, or unknown (read everything again). */
export type Changes =
  { known: true; keys: readonly string[] } | { known: false };

const isFamily = (f: string): f is StateFamily =>
  (STATE_FAMILIES as readonly string[]).includes(f);

/** The family of a state key: the part before ":" ("items" for "items:arrows"). */
export function family(key: string): string {
  const at = key.indexOf(":");
  return at < 0 ? key : key.slice(0, at);
}

/** The slices to read again for changes: every slice when they are unknown or name a family this
 * web does not know (a newer server). */
export function slicesFor(changes: Changes): Set<Slice> {
  if (!changes.known) return new Set(SLICES);
  const out = new Set<Slice>();
  for (const key of changes.keys) {
    const f = family(key);
    if (!isFamily(f)) return new Set(SLICES);
    for (const s of INVALIDATES[f]) out.add(s);
  }
  return out;
}

/** The changes an answer's header names: unknown when it has none. */
export function parseStateKeys(header: string | null): Changes {
  if (header === null) return { known: false };
  return {
    known: true,
    keys: header
      .split(",")
      .map((k) => k.trim())
      .filter((k) => k !== ""),
  };
}

let pending: Changes = { known: true, keys: [] };

/** Notes the answer to a write (its header, null when it had none or never came). */
export function noteWrite(header: string | null) {
  const next = parseStateKeys(header);
  pending =
    pending.known && next.known
      ? { known: true, keys: [...pending.keys, ...next.keys] }
      : { known: false };
}

/**
 * Notes a write's answer by its status. A success names what it changed in its header. A refusal
 * of the input (400, 422) rolled back and changed nothing. A stale refusal says the page's state is
 * stale: a conflict (409: a stale undo or redo top, a level-up no longer pending, a slot already
 * used) or something the page shows that is gone (404: a weapon removed on another device). It and
 * a server error (5xx) read every slice, so the player sees the state that refused the write.
 */
export function noteAnswer(status: number, header: string | null) {
  if (status >= 200 && status < 300) noteWrite(header);
  else if (status === 404 || status === 409 || status >= 500) noteWrite(null);
  else noteWrite("");
}

/** The changes of every write answered since the last call, and forget them. */
export function takeChanges(): Changes {
  const out = pending;
  pending = { known: true, keys: [] };
  return out;
}
