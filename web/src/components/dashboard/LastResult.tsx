import { useTranslation } from "react-i18next";
import { useSpellName } from "@/i18n/spellName";
import { useInventoryName } from "@/i18n/inventoryName";
import { lastResultEntry } from "@/journal/card";
import { entryText, shortResult } from "@/journal/entry";
import type { RowValue, ShortVerdict } from "@/journal/kindView";
import { entryWhen } from "@/journal/rounds";
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

const VERDICT_TONES: Record<ShortVerdict["tone"], string> = {
  pass: "text-success",
  fail: "text-danger",
  crit: "text-highlight",
};

/**
 * The last result, under the brand in the header's right column (ADR 0027 step 4.5, amending ADR
 * 0022 step 2.2): its label, then one short line, "value · what · verdict" (the value in
 * `text-key`, the verdict coloured, in sentence case). The column is at most a sixth of the
 * viewport and the line never wraps: a longer one ends with an ellipsis, and the journal's full
 * wording (with its round and time) is in the tooltip (the exception recorded in ADR 0022 step
 * 1.2). Visible in every activity, so a roll made anywhere shows here.
 */
export function LastResult() {
  const { t, i18n } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
  const entries = useLogStore((s) => s.entries);
  const entry = lastResultEntry(entries);
  const short = entry ? shortResult(entry, t, spellName, names) : null;
  const when = entry ? entryWhen(t, entry, i18n.language) : null;
  const full = entry
    ? `${entryText(entry, t, spellName, names)} (${when})`
    : undefined;

  return (
    <div
      role="group"
      aria-label={t("shell.lastResult")}
      // A banded cell of the header card (ADR 0026 step 6.3), like the round block.
      className="group-band flex max-w-[16.67vw] min-w-0 flex-col items-end gap-0.5 rounded-lg px-3 py-1"
      data-testid="last-result"
    >
      <span className="section-label max-w-full truncate">
        {t("shell.lastResult")}
      </span>
      {entry && short ? (
        <p
          className="flex max-w-full min-w-0 items-baseline gap-1.5 text-body whitespace-nowrap"
          data-entry-id={entry.id}
          title={full}
        >
          {short.value && (
            <span
              className={cn(
                "shrink-0 text-key tabular-nums",
                TONES[short.value.tone],
              )}
              data-testid="last-result-chip"
            >
              {short.value.text}
            </span>
          )}
          <span className="min-w-0 truncate" data-testid="last-result-text">
            {short.what}
            {short.verdict && (
              <>
                {" · "}
                <span
                  className={VERDICT_TONES[short.verdict.tone]}
                  data-testid="last-result-verdict"
                >
                  {short.verdict.text}
                </span>
              </>
            )}
          </span>
        </p>
      ) : (
        <p className="text-body text-muted-foreground">{t("shell.noResult")}</p>
      )}
    </div>
  );
}
