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

export function rollSavingThrow(category: SaveCategory): Promise<RollResult> {
  return post<RollResult>("/api/roll/save", { category });
}
