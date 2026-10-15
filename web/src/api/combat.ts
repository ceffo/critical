import type {
  AttackFormPatch,
  AttackFormView,
  AttackRequest,
  CritRow,
  CritTable,
  RollResult,
  Weapon,
  WeaponBody,
  WeaponChange,
} from "../types/combat";
import { del, patch, post, request } from "./http";

export function resolveAttack(req: AttackRequest): Promise<RollResult> {
  return post<RollResult>("/api/roll/attack", req);
}

export function fetchAttackForm(): Promise<AttackFormView> {
  return request<AttackFormView>("/api/attack/form");
}

/** Only the fields present in `body` change; returns the whole form with its preview. */
export function patchAttackForm(body: AttackFormPatch): Promise<AttackFormView> {
  return patch<AttackFormView>("/api/attack/form", body);
}

export function fetchWeapons(): Promise<Weapon[]> {
  return request<Weapon[]>("/api/weapons");
}

export function postCreateWeapon(body: WeaponBody): Promise<WeaponChange> {
  return post<WeaponChange>("/api/weapons", body);
}

/** Only the fields present in `body` change. */
export function patchWeapon(
  id: string,
  body: WeaponBody,
): Promise<WeaponChange> {
  return patch<WeaponChange>(`/api/weapons/${encodeURIComponent(id)}`, body);
}

export function deleteWeapon(id: string): Promise<WeaponChange> {
  return del<WeaponChange>(`/api/weapons/${encodeURIComponent(id)}`);
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
