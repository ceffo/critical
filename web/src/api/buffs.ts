import type { BuffEdit, BuffEffect, Strength } from "../types/buffs";
import { post, request } from "./http";

export function fetchActiveBuffs(): Promise<BuffEffect[]> {
  return request<BuffEffect[]>("/api/buffs");
}

export function activateBuff(buff: Partial<BuffEffect>): Promise<BuffEffect[]> {
  return post<BuffEffect[]>("/api/buffs", buff);
}

export function editBuff(id: string, edit: BuffEdit): Promise<BuffEffect[]> {
  return request<BuffEffect[]>(`/api/buffs/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(edit),
  });
}

export function dismissBuff(id: string): Promise<BuffEffect[]> {
  return request<BuffEffect[]>(`/api/buffs/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

/** One charge absorbed a hit (Stoneskin layer, Mirror Image). */
export function triggerBuff(id: string): Promise<BuffEffect[]> {
  return post<BuffEffect[]>(`/api/buffs/${encodeURIComponent(id)}/trigger`);
}

export function fetchStrength(): Promise<Strength> {
  return request<Strength>("/api/stats/str");
}
