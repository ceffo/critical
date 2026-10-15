import type { LogEntry } from "@/types/log";

export interface RoundGroup {
  /** Unique within one grouping: the same round number can appear as several groups. */
  key: string;
  round: number;
  entries: LogEntry[];
}

/**
 * Display grouping of the action log: entries arrive newest first (server order) and are cut
 * into consecutive runs of the same round stamp. Rounds restart (End combat, Set round, reset),
 * so a round number can head several groups; order is never re-sorted by number. When the
 * newest entry is not in the current round, an empty group for it is put on top.
 */
export function groupByRound(
  entries: LogEntry[],
  currentRound: number,
): RoundGroup[] {
  const groups: RoundGroup[] = [];
  for (const e of entries) {
    const last = groups[groups.length - 1];
    if (last && last.round === e.round) last.entries.push(e);
    else groups.push({ key: `e${e.id}`, round: e.round, entries: [e] });
  }
  if (groups.length === 0 || groups[0].round !== currentRound) {
    groups.unshift({ key: "current", round: currentRound, entries: [] });
  }
  return groups;
}
