/**
 * How far a skill's points can go, for the slider's scale and its "/ N" (ADR 0030 steps 17 and 18):
 * a thief's cap per skill when the budget has one, and never past the point where the percentage
 * would reach its maximum. A scale only: the server checks every total (the range, the budget and
 * the cap), and its refusal is what the player reads.
 */
export function pointsLimit(
  row: { points: number; rawPercent: number },
  perSkillCap: number,
  maxPercent: number,
): number {
  const others = row.rawPercent - row.points;
  const to95 = maxPercent - others;
  const limit = Math.min(perSkillCap > 0 ? perSkillCap : to95, to95);
  return Math.max(0, limit);
}
