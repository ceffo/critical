import type { RollResult, SaveCategory } from "@/types/combat";
import { post } from "./http";

/** Sets the adjustment of the latest save (log entry id); the server recomputes the verdict. */
export function adjustSave(entryId: number, adjustment: number, reason: string): Promise<RollResult> {
  return post<RollResult>(`/api/roll/save/${entryId}/adjust`, { adjustment, reason });
}

/** Corrects the target AC of the latest attack (log entry id); the server recomputes the attack. */
export function adjustAttack(entryId: number, targetAc: number): Promise<RollResult> {
  return post<RollResult>(`/api/roll/attack/${entryId}/adjust`, { targetAc });
}

/** Makes the target AC of the latest attack unknown again (docs/adr/0012 step 8). */
export function clearAttackAc(entryId: number): Promise<RollResult> {
  return post<RollResult>(`/api/roll/attack/${entryId}/unknown-ac`, {});
}

/** The GM's hit or miss on the latest attack while its AC is unknown; the server settles it (no crit). */
export function attackVerdict(entryId: number, hit: boolean): Promise<RollResult> {
  return post<RollResult>(`/api/roll/attack/${entryId}/verdict`, { hit });
}

export function rollSavingThrow(category: SaveCategory): Promise<RollResult> {
  return post<RollResult>("/api/roll/save", { category });
}
