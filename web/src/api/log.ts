import type { LogEntry, LogResponse } from "@/types/log";
import { post, request } from "./http";

/** Action log entries, newest first (GET /api/log). */
export async function fetchLog(limit?: number): Promise<LogEntry[]> {
  const q = limit ? `?limit=${limit}` : "";
  const res = await request<LogResponse>(`/api/log${q}`);
  return res.entries ?? [];
}

/** Amend an hp_change entry: new HP delta, and optionally a new source. */
export function amendLogEntry(
  id: number,
  delta: number,
  source?: string,
): Promise<LogEntry> {
  return post<LogEntry>(`/api/log/${id}/amend`, { delta, source: source ?? "" });
}

/** Void an entry (restores any state it changed). */
export function voidLogEntry(id: number): Promise<LogEntry> {
  return post<LogEntry>(`/api/log/${id}/void`);
}
