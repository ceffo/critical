// Input helpers for the DM adjustment of skill checks (before the roll: the section steppers of
// the Skills tab; after it: the result card). No game rules: the server validates the range,
// moves the target and judges the roll (ADR 0014 steps 5 to 8). + is easier, - is harder.
import type { SkillCheck, SkillKind } from "@/types/skills";

/** The −/+ buttons stop at the server's limit: ±20 for a proficiency, ±100 points for a bard skill. */
export function maxSkillAdjustment(kind: SkillKind): number {
  return kind === "proficiency" ? 20 : 100;
}

/** One click: 1 on a proficiency, 5 percentage points on a bard skill. */
export function skillAdjustmentStep(kind: SkillKind): number {
  return kind === "proficiency" ? 1 : 5;
}

/** value moved one step up or down, kept within the kind's limit. */
export function stepSkillAdjustment(
  kind: SkillKind,
  value: number,
  direction: 1 | -1,
): number {
  const max = maxSkillAdjustment(kind);
  return Math.max(
    -max,
    Math.min(max, value + direction * skillAdjustmentStep(kind)),
  );
}

/** A row's target with the pending adjustment, as a preview: the roll's verdict is the server's. */
export function previewTarget(target: number, adjustment: number): number {
  return target + adjustment;
}

/** The target a check was judged against; a payload logged before adjustments has none. */
export function checkAdjustedTarget(
  c: Pick<SkillCheck, "target" | "adjustment" | "adjustedTarget">,
): number {
  return c.adjustedTarget ?? c.target + (c.adjustment ?? 0);
}
