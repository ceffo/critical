import type { LogEntry, LogResponse } from "@/types/log";
import { post, request } from "./http";

/** Action log entries, newest first, and the top of the stack (GET /api/log). */
export async function fetchLog(limit?: number): Promise<LogResponse> {
  const q = limit ? `?limit=${limit}` : "";
  const res = await request<LogResponse>(`/api/log${q}`);
  return {
    entries: res.entries ?? [],
    topId: res.topId ?? null,
    redo: res.redo ?? null,
  };
}

/** Pop the newest non-voided entry, any kind, if it is still `expectedTopId` (409 otherwise). */
export function undoLogEntry(expectedTopId: number): Promise<LogEntry> {
  return post<LogEntry>("/api/log/undo", { expectedTopId });
}

/** Replay the newest popped transaction if it is still `expectedRedoId` (409 otherwise). */
export function redoLogEntry(expectedRedoId: number): Promise<LogEntry> {
  return post<LogEntry>("/api/log/redo", { expectedRedoId });
}
