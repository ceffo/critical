import type {
  AttackFormPatch,
  AttackFormView,
  AttackRequest,
  RollResult,
  Weapon,
} from "../types/combat";
import { patch, post, request } from "./http";

export function resolveAttack(req: AttackRequest): Promise<RollResult> {
  return post<RollResult>("/api/roll/attack", req);
}

export function fetchAttackForm(): Promise<AttackFormView> {
  return request<AttackFormView>("/api/attack/form");
}

/** Only the fields present in `body` change; returns the whole form with its preview. */
export function patchAttackForm(
  body: AttackFormPatch,
): Promise<AttackFormView> {
  return patch<AttackFormView>("/api/attack/form", body);
}

export function fetchWeapons(): Promise<Weapon[]> {
  return request<Weapon[]>("/api/weapons");
}
