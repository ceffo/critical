import type { LogEntry } from "@/types/log";

export interface RoundGroup {
  round: number;
  entries: LogEntry[];
}

/**
 * Display grouping of the action log: entries (newest first) bucketed by the round the
 * server stamped on them, newest round first. The current round always gets a group,
 * even when nothing has been recorded in it yet.
 */
export function groupByRound(entries: LogEntry[], currentRound: number): RoundGroup[] {
  const groups = new Map<number, LogEntry[]>();
  groups.set(currentRound, []);
  for (const e of entries) groups.set(e.round, [...(groups.get(e.round) ?? []), e]);
  return [...groups.entries()]
    .sort(([a], [b]) => b - a)
    .map(([round, list]) => ({ round, entries: list }));
}
