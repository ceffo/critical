import type { TFunction } from "i18next";
import type { LogEntry } from "@/types/log";
import { entryText } from "@/journal/entry";
import { englishSpellName, type SpellNamer } from "@/i18n/spellName";
import { type InventoryNamer, recordedNames } from "@/i18n/inventoryName";

/** What the global Undo button shows (ADR 0010 step 5): disabled with "Nothing to undo" when the
 * server has nothing to pop, else a tooltip naming the transaction on top, worded from the payload
 * of its first entry (ADR 0018 step 2: the Next Round, not the initiative it rolled). */
export function undoButtonState(
  topId: number | null,
  entries: LogEntry[],
  busy: boolean,
  t: TFunction,
  spellName: SpellNamer = englishSpellName,
  names: InventoryNamer = recordedNames,
): { disabled: boolean; label: string } {
  if (topId === null) return { disabled: true, label: t("log.undo.nothing") };
  const top = entries.find((e) => e.id === topId);
  const first = top && entries.find((e) => e.id === top.transactionId);
  const named = first ?? top;
  return {
    disabled: busy,
    label: t("log.undo.tooltip", {
      action: named ? entryText(named, t, spellName, names) : `#${topId}`,
    }),
  };
}

/** What the Redo button shows (ADR 0019 step 2): disabled with "Nothing to redo" when the server
 * has no popped transaction to replay, else a tooltip naming it, worded from the payload of its
 * first entry (the server sends it, so it is named even when it is not among the listed entries). */
export function redoButtonState(
  redo: LogEntry | null,
  busy: boolean,
  t: TFunction,
  spellName: SpellNamer = englishSpellName,
  names: InventoryNamer = recordedNames,
): { disabled: boolean; label: string } {
  if (redo === null) return { disabled: true, label: t("log.redo.nothing") };
  return {
    disabled: busy,
    label: t("log.redo.tooltip", {
      action: entryText(redo, t, spellName, names),
    }),
  };
}
