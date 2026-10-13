import type { AttackRequest, CritRow, CritTable, RollResult, Weapon } from "../types/combat";
import { post, request } from "./http";

export function resolveAttack(req: AttackRequest): Promise<RollResult> {
  return post<RollResult>("/api/roll/attack", req);
}

export function fetchWeapons(): Promise<Weapon[]> {
  return request<Weapon[]>("/api/weapons");
}

export function fetchCritTable(): Promise<CritTable> {
  return request<CritTable>("/api/crit-table");
}

/** Replace the whole crit table; rejects with the server's message naming the bad row. */
export function saveCritTable(rows: CritRow[]): Promise<CritTable> {
  return request<CritTable>("/api/crit-table", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ rows }),
  });
}
