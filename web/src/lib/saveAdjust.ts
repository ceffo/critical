// Input helpers for the save adjustment control on the result card. No game rules: the server
// recomputes the verdict and margin (POST /api/roll/save/{id}/adjust) and validates the value.
import type { RollResult } from "@/types/combat";

/** The −/+ buttons stop at the server's limit (combatsvc.MaxSaveAdjustment). */
export const MAX_ADJUSTMENT = 20;

/** value + step, kept within ±MAX_ADJUSTMENT. */
export function stepAdjustment(value: number, step: number): number {
  return Math.max(-MAX_ADJUSTMENT, Math.min(MAX_ADJUSTMENT, value + step));
}

/** Whether applying the draft would change the save (the reason only matters with an adjustment). */
export function adjustmentChanged(r: RollResult, value: number, reason: string): boolean {
  if (value !== (r.adjustment ?? 0)) return true;
  return value !== 0 && reason.trim() !== (r.adjustmentReason ?? "");
}
