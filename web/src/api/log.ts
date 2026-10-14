import type { LogEntry, LogResponse } from "@/types/log";
import { post, request } from "./http";

/** Action log entries, newest first (GET /api/log). */
export async function fetchLog(limit?: number): Promise<LogEntry[]> {
  const q = limit ? `?limit=${limit}` : "";
  const res = await request<LogResponse>(`/api/log${q}`);
  return res.entries ?? [];
}

/** Set the note of an entry, any kind ("" clears it). The note is the only
 * thing an amend may change (critical-qki.38). */
export function amendLogEntry(id: number, note: string): Promise<LogEntry> {
  return post<LogEntry>(`/api/log/${id}/amend`, { note });
}

/** Pop an entry: only the newest non-voided one (restores any state it changed). */
export function voidLogEntry(id: number): Promise<LogEntry> {
  return post<LogEntry>(`/api/log/${id}/void`);
}
