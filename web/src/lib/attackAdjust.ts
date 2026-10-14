// Input helpers for the target AC correction on the attack card. No game rules: the server
// recomputes hit, crit and damage (POST /api/roll/attack/{id}/adjust) and validates the AC.

/** The −/+ buttons and the field stop at the server's limit (combatsvc.MaxTargetAC). */
export const MAX_TARGET_AC = 20;

/** ac + step, kept within ±MAX_TARGET_AC. */
export function stepTargetAc(ac: number, step: number): number {
  return Math.max(-MAX_TARGET_AC, Math.min(MAX_TARGET_AC, ac + step));
}

/** The AC typed in the field, or null when it is not a whole number within ±MAX_TARGET_AC. */
export function parseTargetAc(text: string): number | null {
  const s = text.trim().replace("−", "-");
  if (!/^-?\d+$/.test(s)) return null;
  const n = Number(s);
  return Math.abs(n) <= MAX_TARGET_AC ? n : null;
}
