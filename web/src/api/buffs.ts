import type {
  BuffEdit,
  BuffEffect,
  BuffList,
  EndCause,
  SavedBuff,
  Strength,
} from "../types/buffs";
import { post, request } from "./http";

/** The effects list: the active buffs and the item powers that are on (ADR 0016 step 13). */
export function fetchActiveBuffs(): Promise<BuffList> {
  return request<BuffList>("/api/buffs");
}

export function activateBuff(
  buff: Omit<Partial<BuffEffect>, "startRound">,
): Promise<BuffList> {
  return post<BuffList>("/api/buffs", buff);
}

export function editBuff(id: string, edit: BuffEdit): Promise<BuffList> {
  return request<BuffList>(`/api/buffs/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(edit),
  });
}

/** Stops a row; `cause` says how it ended when its spell has end effects (ADR 0004 step 21). */
export function dismissBuff(id: string, cause?: EndCause): Promise<BuffList> {
  const query = cause ? `?cause=${cause}` : "";
  return request<BuffList>(`/api/buffs/${encodeURIComponent(id)}${query}`, {
    method: "DELETE",
  });
}

/** One charge absorbed a hit (Stoneskin layer, Mirror Image). */
export function triggerBuff(id: string): Promise<BuffList> {
  return post<BuffList>(`/api/buffs/${encodeURIComponent(id)}/trigger`);
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
