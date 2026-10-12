import type { RollResult, SaveCategory } from "@/types/combat";
import { post } from "./http";

export function rollSavingThrow(category: SaveCategory): Promise<RollResult> {
  return post<RollResult>("/api/roll/save", { category });
}
