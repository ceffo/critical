import type { XpAmount, XpEntryBody, XpLedger } from "../types/xp";
import { del, patch, post, request } from "./http";

export function fetchXp(): Promise<XpLedger> {
  return request<XpLedger>("/api/xp");
}

export function fetchXpPreview(
  base: number,
  bonus: boolean,
): Promise<XpAmount> {
  return request<XpAmount>(`/api/xp/preview?base=${base}&bonus=${bonus}`);
}

export function postXpEntry(body: XpEntryBody): Promise<XpLedger> {
  return post<XpLedger>("/api/xp", body);
}

/** Only the fields present in `body` change. */
export function patchXpEntry(id: number, body: XpEntryBody): Promise<XpLedger> {
  return patch<XpLedger>(`/api/xp/${id}`, body);
}

export function deleteXpEntry(id: number): Promise<XpLedger> {
  return del<XpLedger>(`/api/xp/${id}`);
}
