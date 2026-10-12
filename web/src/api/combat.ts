import type { RollResult } from "../types/combat";
import { post } from "./http";

export function resolveAttack(
  weaponId: string,
  targetAc: number,
): Promise<RollResult> {
  return post<RollResult>("/api/roll/attack", {
    weapon_id: weaponId,
    target_ac: targetAc,
  });
}
