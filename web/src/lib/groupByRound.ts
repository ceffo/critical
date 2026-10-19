import type { LogEntry } from "@/types/log";

export interface RoundGroup {
  /** Unique within one grouping: the same round number can appear as several groups. */
  key: string;
  round: number;
  entries: LogEntry[];
  /** `entries` cut into transactions (consecutive runs of one transactionId), newest first. */
  transactions: LogEntry[][];
  /** The one group of the current round: at most one per grouping. */
  current: boolean;
}

/** True when the entry is popped: a whole transaction is undone when all its entries are. */
export function isUndone(tx: LogEntry[]): boolean {
  return tx.length > 0 && tx.every((e) => e.status === "voided");
}

/**
 * True when the transaction opens a round: Next Round's advance and the new round's initiative (ADR
 * 0013, one transaction per ADR 0018). The journal shows those rows as the round's own first rows,
 * not framed as a multi-entry transaction: the round block and its marker already say what they are
 * (ADR 0027 step 9.6).
 */
export function opensRound(tx: LogEntry[]): boolean {
  return tx.some((e) => e.kind === "round_advance");
}

/**
 * Display grouping of the action log: entries arrive newest first (server order) and are cut
 * into consecutive runs of the same round stamp. Rounds restart (End combat, Set round, reset),
 * so a round number can head several groups; order is never re-sorted by number. When the
 * newest live entry is not in the current round, an empty group for it is put on top.
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
        current: false,
      });
    }
  }
  // The current round's group is the newest one with a live entry: entries popped by an undo
  // (a later round's, say) stay listed above it but do not make it stale. When there is none,
  // an empty group goes on top, so the round never appears twice.
  const live = groups.find((g) => g.transactions.some((tx) => !isUndone(tx)));
  if (live && live.round === currentRound) {
    live.current = true;
  } else {
    groups.unshift({
      key: "current",
      round: currentRound,
      entries: [],
      transactions: [],
      current: true,
    });
  }
  return groups;
}
