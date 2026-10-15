import type { TFunction } from "i18next";
import type { LogEntry } from "@/types/log";
import { entryText } from "@/lib/logSummary";

/** What the global Undo button shows (ADR 0010 step 5): disabled with "Nothing to undo" when the
 * server has nothing to pop, else a tooltip naming the action on top, worded from its payload. */
export function undoButtonState(
  topId: number | null,
  entries: LogEntry[],
  busy: boolean,
  t: TFunction,
): { disabled: boolean; label: string } {
  if (topId === null) return { disabled: true, label: t("log.undo.nothing") };
  const top = entries.find((e) => e.id === topId);
  return {
    disabled: busy,
    label: t("log.undo.tooltip", {
      action: top ? entryText(top, t) : `#${topId}`,
    }),
  };
}
