import type { BuffEdit, BuffEffect, SavedBuff, Strength } from "../types/buffs";
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

export function fetchSavedBuffs(): Promise<SavedBuff[]> {
  return request<SavedBuff[]>("/api/buffs/saved");
}

/** Stores a template (the same name replaces it); returns the list. */
export function saveBuffTemplate(
  saved: Omit<SavedBuff, "id">,
): Promise<SavedBuff[]> {
  return post<SavedBuff[]>("/api/buffs/saved", saved);
}

/** Deletes a template; active buffs are untouched. */
export function deleteSavedBuff(id: number): Promise<SavedBuff[]> {
  return request<SavedBuff[]>(`/api/buffs/saved/${id}`, { method: "DELETE" });
}
