import type { BuffEffect } from "../types/buffs";
import { post, request } from "./http";

export function fetchActiveBuffs(): Promise<BuffEffect[]> {
  return request<BuffEffect[]>("/api/buffs");
}

export function activateBuff(buff: Partial<BuffEffect>): Promise<BuffEffect[]> {
  return post<BuffEffect[]>("/api/buffs", buff);
}

export function dismissBuff(id: string): Promise<BuffEffect[]> {
  return request<BuffEffect[]>(`/api/buffs/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
}

export function triggerBuff(id: string): Promise<BuffEffect[]> {
  return post<BuffEffect[]>(`/api/buffs/${encodeURIComponent(id)}/trigger`);
}
