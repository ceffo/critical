import type { RollResult, SaveBreakdown, SaveCategory } from "@/types/combat";
import { post } from "./http";

/**
 * TRANSITION ONLY (rolling deploy of CRI-61): a still-old machine answers a save with the previous
 * release's names, saveCategory/baseThac0/hit/hitMargin, where this release reads
 * category/target/passed/margin. Accept both here, at the boundary, so the rest of the web sees one
 * shape. Remove with the server's compat keys (saveAnswer in combathdl) in the follow-up release.
 */
export function normalizeSaveAnswer(raw: SaveBreakdown): SaveBreakdown {
  const old = raw as Partial<SaveBreakdown> & {
    saveCategory?: string;
    baseThac0?: number;
    hit?: boolean;
    hitMargin?: number;
  };
  return {
    ...raw,
    category: old.category ?? old.saveCategory ?? "",
    target: old.target ?? old.baseThac0 ?? 0,
    passed: old.passed ?? old.hit ?? false,
    margin: old.margin ?? old.hitMargin ?? 0,
  };
}

/** Sets the adjustment of the latest save (log entry id); the server recomputes the verdict. */
export function adjustSave(
  entryId: number,
  adjustment: number,
  reason: string,
): Promise<SaveBreakdown> {
  return post<SaveBreakdown>(`/api/roll/save/${entryId}/adjust`, {
    adjustment,
    reason,
  }).then(normalizeSaveAnswer);
}

/** Corrects the target AC of the latest attack (log entry id); the server recomputes the attack. */
export function adjustAttack(
  entryId: number,
  targetAc: number,
): Promise<RollResult> {
  return post<RollResult>(`/api/roll/attack/${entryId}/adjust`, { targetAc });
}

/** Makes the target AC of the latest attack unknown again (docs/adr/0012 step 8). */
export function clearAttackAc(entryId: number): Promise<RollResult> {
  return post<RollResult>(`/api/roll/attack/${entryId}/unknown-ac`, {});
}

/** The GM's hit or miss on the latest attack while its AC is unknown; the server settles it (no crit). */
export function attackVerdict(
  entryId: number,
  hit: boolean,
): Promise<RollResult> {
  return post<RollResult>(`/api/roll/attack/${entryId}/verdict`, { hit });
}

export function rollSavingThrow(
  category: SaveCategory,
): Promise<SaveBreakdown> {
  return post<SaveBreakdown>("/api/roll/save", { category }).then(
    normalizeSaveAnswer,
  );
}
