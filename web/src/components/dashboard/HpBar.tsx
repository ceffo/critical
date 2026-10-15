import { useTranslation } from "react-i18next";
import { useHpStore } from "@/stores/hp";
import { useLogStore } from "@/stores/log";
import { formatSigned, hpPercent, hpTone } from "@/lib/format";
import { lastDamage, lastHpChange } from "@/lib/moiLog";
import { cn } from "@/lib/utils";

/** Bar fill per threshold (display only, docs/frontend.md "Display Conventions"). */
const TONE = {
  good: "bg-tint",
  warn: "bg-warning",
  bad: "bg-danger",
} as const;

/**
 * HP block: the last change still in force on the right (from the log), current HP large over max,
 * and a bar coloured by threshold with the last damage taken as a faint ghost segment after the
 * fill. Every number is the server's; the bar only draws them.
 */
export function HpBar() {
  const { t } = useTranslation();
  const { currentHp, maxHp, loaded } = useHpStore();
  const entries = useLogStore((s) => s.entries);
  const last = lastHpChange(entries);
  const pct = hpPercent(currentHp, maxHp);
  const ghost = Math.max(0, hpPercent(currentHp + lastDamage(entries), maxHp) - pct);

  return (
    <div className="flex min-w-0 flex-col gap-1.5" role="group" aria-label={t("hp.title")}>
      <div className="flex min-w-0 items-baseline justify-between gap-2">
        <span className="shrink-0 text-sm text-muted-foreground">{t("hp.title")}</span>
        {last && (
          <span
            className={cn(
              "min-w-0 truncate text-sm tabular-nums",
              last.delta < 0 ? "text-danger" : "text-heal",
            )}
            title={last.source}
            data-testid="hp-last-change"
          >
            {t("hp.lastChange", { delta: formatSigned(last.delta), source: last.source })}
          </span>
        )}
      </div>
      <div className="flex items-baseline gap-1.5 tabular-nums" data-testid="hp-value">
        {loaded ? (
          <>
            <span className="text-4xl leading-none font-semibold tracking-tight">{currentHp}</span>
            <span className="text-base text-muted-foreground"> / {maxHp}</span>
          </>
        ) : (
          "–"
        )}
      </div>
      <div
        className="flex h-3 w-full overflow-hidden rounded-full bg-neu-900"
        role="meter"
        aria-label={t("hp.title")}
        aria-valuemin={0}
        aria-valuemax={maxHp}
        aria-valuenow={currentHp}
      >
        <div
          className={cn("h-full transition-all duration-300", TONE[hpTone(pct)])}
          style={{ width: `${pct}%` }}
          data-testid="hp-fill"
        />
        {ghost > 0 && (
          <div
            className="h-full bg-danger/35 transition-all duration-300"
            style={{ width: `${ghost}%` }}
            title={t("hp.lastDamage", { amount: lastDamage(entries) })}
            data-testid="hp-ghost"
          />
        )}
      </div>
    </div>
  );
}
