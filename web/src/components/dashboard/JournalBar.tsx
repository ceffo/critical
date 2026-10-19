import { useTranslation } from "react-i18next";
import { useSpellName } from "@/i18n/spellName";
import { useInventoryName } from "@/i18n/inventoryName";
import { entryText, rowValue } from "@/journal/entry";
import { EntryIcon } from "@/journal/EntryIcon";
import { roundLabel } from "@/journal/rounds";
import { formatTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useLogStore } from "@/stores/log";
import { VALUE_TONES } from "./journalStyle";
import { RedoButton } from "./RedoButton";
import { UndoButton } from "./UndoButton";

/**
 * The collapsed journal (ADR 0022 step 5.4): one line in the journal's column order (round, value,
 * icon, the newest entry, time), then the entry count and Undo / Redo. The entry text wraps rather
 * than being cut.
 */
export function JournalBar() {
  const { t, i18n } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
  const entries = useLogStore((s) => s.entries);
  const last = entries[0];
  const value = last ? rowValue(last) : null;
  const voided = last?.status === "voided";

  return (
    <section
      className="zone-card flex shrink-0 items-center gap-4 px-4 py-2"
      aria-label={t("log.title")}
      data-testid="journal-bar"
    >
      <h2 className="shrink-0 section-label">{t("log.title")}</h2>
      {last ? (
        <p
          className="flex min-w-0 flex-1 items-center gap-3 text-body"
          data-entry-id={last.id}
        >
          <span className="shrink-0 chip px-1.5 text-label font-medium">
            {roundLabel(t, last.round)}
          </span>
          {value && (
            <span
              className={cn(
                "shrink-0 font-semibold tabular-nums",
                voided ? "text-muted-foreground" : VALUE_TONES[value.tone],
              )}
            >
              {value.text}
            </span>
          )}
          <EntryIcon
            entry={last}
            className="size-[1.125rem] shrink-0 text-muted-foreground"
          />
          <span
            className={cn(
              "min-w-0 break-words",
              voided && "text-muted-foreground line-through",
            )}
          >
            {entryText(last, t, spellName, names)}
          </span>
          <span className="shrink-0 text-label text-muted-foreground tabular-nums">
            {formatTime(last.createdAt, i18n.language)}
          </span>
        </p>
      ) : (
        <p className="min-w-0 flex-1 text-body text-muted-foreground">
          {t("log.empty")}
        </p>
      )}
      <span className="shrink-0 text-body text-muted-foreground tabular-nums">
        {t("shell.journal.count", { count: entries.length })}
      </span>
      <span className="flex shrink-0 gap-1">
        <UndoButton />
        <RedoButton />
      </span>
    </section>
  );
}
