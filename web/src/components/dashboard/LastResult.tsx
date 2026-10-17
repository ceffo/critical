import { useTranslation } from "react-i18next";
import { useSpellName } from "@/i18n/spellName";
import { useInventoryName } from "@/i18n/inventoryName";
import { lastResultChip, lastResultEntry, type RowValue } from "@/lib/filCard";
import { formatTime } from "@/lib/format";
import { entryText } from "@/lib/logSummary";
import { cn } from "@/lib/utils";
import { useLogStore } from "@/stores/log";

const TONES: Record<RowValue["tone"], string> = {
  damage: "text-danger",
  heal: "text-success",
  pass: "text-success",
  fail: "text-danger",
  ink: "text-foreground",
  crit: "text-highlight",
  muted: "text-foreground",
};

/**
 * The last result, under the brand in the header's right column (ADR 0022 step 2.2, ADR 0027 steps
 * 4.5 and 4.6): its label with the turn and time, then one line, the die in `text-key` and the
 * journal's wording (verdict, damage) in `text-body`. Visible in every activity, so a roll made
 * anywhere shows here. It takes the width its line needs; only when the header has no room left
 * (a very long wording) does the text wrap, never cut.
 */
export function LastResult() {
  const { t, i18n } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
  const entries = useLogStore((s) => s.entries);
  const entry = lastResultEntry(entries);
  const chip = entry ? lastResultChip(entry) : null;
  const when = entry
    ? t(entry.round > 0 ? "fil.when" : "fil.whenOutOfCombat", {
        round: entry.round,
        time: formatTime(entry.createdAt, i18n.language),
      })
    : null;

  return (
    <div
      role="group"
      aria-label={t("shell.lastResult")}
      // A banded cell of the header card (ADR 0026 step 6.3), like the turn block.
      className="group-band flex min-w-0 flex-col items-end gap-0.5 rounded-lg px-3 py-1"
      data-testid="last-result"
    >
      <span className="section-label">
        {t("shell.lastResult")}
        {when && (
          <span className="tracking-normal normal-case tabular-nums">
            {" · "}
            {when}
          </span>
        )}
      </span>
      {entry ? (
        <p
          className="flex min-w-0 items-baseline gap-2 text-body"
          data-entry-id={entry.id}
        >
          {chip && (
            <span
              className={cn(
                "chip shrink-0 px-1.5 text-key tabular-nums",
                TONES[chip.tone],
              )}
              data-testid="last-result-chip"
            >
              {chip.text}
            </span>
          )}
          <span className="min-w-0 break-words" data-testid="last-result-text">
            {entryText(entry, t, spellName, names)}
          </span>
        </p>
      ) : (
        <p className="text-body text-muted-foreground">{t("shell.noResult")}</p>
      )}
    </div>
  );
}
