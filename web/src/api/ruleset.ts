import type { CritRow, CritTable } from "../types/combat";
import type {
  RacialLift,
  Ruleset,
  RulesetChange,
  SuccessEffect,
} from "../types/ruleset";
import { request } from "./http";

export function fetchRuleset(): Promise<Ruleset> {
  return request<Ruleset>("/api/ruleset");
}

export async function fetchRulesetChanges(): Promise<RulesetChange[]> {
  const res = await request<{ changes: RulesetChange[] }>(
    "/api/ruleset/changes",
  );
  return res.changes;
}

function put<T>(path: string, body: unknown): Promise<T> {
  return request<T>(path, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

/** Replace the whole crit table (admin); rejects with the server's message naming the bad row. */
export function saveCritTable(rows: CritRow[]): Promise<CritTable> {
  return put<CritTable>("/api/ruleset/crit-table", { rows });
}

export function saveRacialLift(lift: RacialLift): Promise<RacialLift> {
  return put<RacialLift>("/api/ruleset/racial-lift", lift);
}

export async function saveSuccessEffects(
  effects: SuccessEffect[],
): Promise<SuccessEffect[]> {
  const res = await put<{ effects: SuccessEffect[] }>(
    "/api/ruleset/success-effects",
    { effects },
  );
  return res.effects;
}
