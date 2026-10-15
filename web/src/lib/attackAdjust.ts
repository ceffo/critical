// Input helpers for the target AC correction and the GM verdict on the attack card. No game rules:
// the server recomputes hit, crit and damage (POST /api/roll/attack/{id}/adjust, /verdict) and
// validates the AC.

/** The −/+ buttons and the field stop at the server's limit (combatsvc.MaxTargetAC). */
export const MAX_TARGET_AC = 20;

/**
 * ac + step, kept within -MAX_TARGET_AC and max: the highest real AC, served by the attack form
 * (maxRealAc), MAX_TARGET_AC until it has loaded.
 */
export function stepTargetAc(ac: number, step: number, max: number = MAX_TARGET_AC): number {
  return Math.max(-MAX_TARGET_AC, Math.min(max, ac + step));
}

/** The AC typed in the field, or null when it is not a whole number between -MAX_TARGET_AC and max. */
export function parseTargetAc(text: string, max: number = MAX_TARGET_AC): number | null {
  const s = text.trim().replace("−", "-");
  if (!/^-?\d+$/.test(s)) return null;
  const n = Number(s);
  return n >= -MAX_TARGET_AC && n <= max ? n : null;
}

/**
 * The key of the AC correction form: it remounts, so its text starts from the AC shown, on a new
 * attack, a popped correction and "AC unknown again" (targetAc null).
 */
export function adjustFormKey(r: { logEntryId?: number; targetAc: number | null }): string {
  return `${r.logEntryId}:${r.targetAc}`;
}

/** Where the − / + of an empty (unknown) AC field start from: a UI default, not a rule. */
export const UNKNOWN_AC_STEP_BASE = 10;

/**
 * Whether the attack's hit, crit and damage are final: its AC is known (attacks logged before
 * unknown ACs have no acKnown and were all known) or the GM gave a verdict. Read from the server's
 * fields, no rule here.
 */
export function attackSettled(r: { acKnown?: boolean; verdict?: string }): boolean {
  return r.acKnown !== false || r.verdict !== undefined;
}

/**
 * Whether the tell-the-GM line mentions the crit: only when the server says some real AC can crit
 * (critPossible, docs/adr/0012 step 7); an entry without the field keeps the crit part.
 */
export function tellGmWithCrit(r: { critPossible?: boolean }): boolean {
  return r.critPossible !== false;
}

/** The lowest AC the attack hits, as the server sent it (older entries only have targetACNeeded). */
export function hitsAc(r: { hitsAc?: number; targetACNeeded: number }): number {
  return r.hitsAc ?? r.targetACNeeded;
}
