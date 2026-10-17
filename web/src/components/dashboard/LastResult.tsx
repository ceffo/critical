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
  accent: "text-highlight",
  muted: "text-foreground",
};

/**
 * The last result, docked at the right of the header card (ADR 0022 step 2.2): its label with the
 * turn and time, then one line (the die, then the journal's wording: verdict, damage). Visible in every
 * activity, so a roll made anywhere shows here. The text wraps; it is never cut.
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
      className="flex w-[clamp(13rem,18vw,22rem)] shrink-0 flex-col justify-center gap-1"
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
          className="flex items-baseline gap-2 text-body"
          data-entry-id={entry.id}
        >
          {chip && (
            <span
              className={cn(
                "chip shrink-0 px-1.5 font-semibold tabular-nums",
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
