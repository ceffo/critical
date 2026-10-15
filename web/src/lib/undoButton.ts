import type { TFunction } from "i18next";
import type { LogEntry } from "@/types/log";
import { entryText } from "@/lib/logSummary";

/** What the global Undo button shows (ADR 0010 step 5): disabled with "Nothing to undo" when the
 * server has nothing to pop, else a tooltip naming the transaction on top, worded from the payload
 * of its first entry (ADR 0018 step 2: the Next Round, not the initiative it rolled). */
export function undoButtonState(
  topId: number | null,
  entries: LogEntry[],
  busy: boolean,
  t: TFunction,
): { disabled: boolean; label: string } {
  if (topId === null) return { disabled: true, label: t("log.undo.nothing") };
  const top = entries.find((e) => e.id === topId);
  const first = top && entries.find((e) => e.id === top.transactionId);
  const named = first ?? top;
  return {
    disabled: busy,
    label: t("log.undo.tooltip", {
      action: named ? entryText(named, t) : `#${topId}`,
    }),
  };
}
