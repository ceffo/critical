import type { LogEntry } from "@/types/log";

export interface RoundGroup {
  /** Unique within one grouping: the same round number can appear as several groups. */
  key: string;
  round: number;
  entries: LogEntry[];
  /** `entries` cut into transactions (consecutive runs of one transactionId), newest first. */
  transactions: LogEntry[][];
}

/** True when the entry is popped: a whole transaction is undone when all its entries are. */
export function isUndone(tx: LogEntry[]): boolean {
  return tx.length > 0 && tx.every((e) => e.status === "voided");
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
  // A transaction is one unit (ADR 0018) and never splits across groups. Its entries can carry
  // different round stamps (Next Round logs the advance in the round it ends and the initiative in
  // the round it opens), so the transaction goes in the round of its newest entry.
  const transactions: LogEntry[][] = [];
  for (const e of entries) {
    const last = transactions[transactions.length - 1];
    if (last && last[0].transactionId === e.transactionId) last.push(e);
    else transactions.push([e]);
  }
  const groups: RoundGroup[] = [];
  for (const tx of transactions) {
    const last = groups[groups.length - 1];
    if (last && last.round === tx[0].round) {
      last.entries.push(...tx);
      last.transactions.push(tx);
    } else {
      groups.push({
        key: `e${tx[0].id}`,
        round: tx[0].round,
        entries: [...tx],
        transactions: [tx],
      });
    }
  }
  if (groups.length === 0 || groups[0].round !== currentRound) {
    groups.unshift({
      key: "current",
      round: currentRound,
      entries: [],
      transactions: [],
    });
  }
  return groups;
}
