import { useTranslation } from "react-i18next";
import { useHpStore } from "@/stores/hp";
import { useLogStore } from "@/stores/log";
import { formatSigned, hpPercent, hpTone } from "@/lib/format";
import { lastDamage, lastHpChange } from "@/lib/moiLog";
import { cn } from "@/lib/utils";
import type { HpCondition } from "@/types/hp";

/** Bar fill per threshold (display only, docs/frontend.md "Display Conventions"). */
const TONE = {
  good: "bg-tint",
  warn: "bg-warning",
  bad: "bg-danger",
} as const;

/** Condition label style; "ok" shows no label. */
const CONDITION: Record<Exclude<HpCondition, "ok">, string> = {
  badlyWounded: "text-warning",
  unconscious: "text-danger",
  dead: "text-danger font-semibold",
};

/**
 * HP block: the last change still in force on the right (from the log), current HP large over max,
 * and a bar coloured by threshold with the last damage taken as a faint ghost segment after the
 * fill. The scale runs from the server's floor (-10) to max: below 0 a flat-tinted dying zone
 * (ADR 0009: no gradient), filled from 0 down to current HP when the character is below 0, with
 * "0" at its boundary. The condition (ADR 0011) is the server's; it is only labelled here.
 * Every number is the server's; the bar only draws them.
 */
export function HpBar() {
  const { t } = useTranslation();
  const { currentHp, maxHp, floor, condition, loaded } = useHpStore();
  const entries = useLogStore((s) => s.entries);
  const last = lastHpChange(entries);
  const pct = hpPercent(currentHp, maxHp);
  const ghost = Math.max(0, hpPercent(currentHp + lastDamage(entries), maxHp) - pct);
  // Width of the dying zone on the floor..max scale, and how deep below 0 HP is within it.
  const span = maxHp - floor;
  const dyingPct = span > 0 && floor < 0 ? (-floor / span) * 100 : 0;
  const deepPct = currentHp < 0 && floor < 0 ? Math.min(100, (currentHp / floor) * 100) : 0;

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
      <div className="flex min-w-0 items-baseline justify-between gap-2">
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
        {loaded && condition !== "ok" && (
          <span
            className={cn("min-w-0 truncate text-sm", CONDITION[condition])}
            data-testid="hp-condition"
            data-condition={condition}
          >
            {t(`hp.condition.${condition}`)}
          </span>
        )}
      </div>
      <div className="flex flex-col gap-1">
        <div
          className="flex h-3 w-full overflow-hidden rounded-full bg-neu-900"
          role="meter"
          aria-label={t("hp.title")}
          aria-valuemin={floor}
          aria-valuemax={maxHp}
          aria-valuenow={currentHp}
        >
          {dyingPct > 0 && (
            <div
              className="relative h-full shrink-0 bg-danger/15"
              style={{ width: `${dyingPct}%` }}
              title={t("hp.dyingZone", { floor })}
              data-testid="hp-dying-zone"
            >
              {deepPct > 0 && (
                <div
                  className="absolute inset-y-0 right-0 bg-danger transition-all duration-300"
                  style={{ width: `${deepPct}%` }}
                  data-testid="hp-dying-fill"
                />
              )}
            </div>
          )}
          <div className="flex h-full min-w-0 flex-1">
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
        {dyingPct > 0 && (
          <div
            className="relative h-4 text-xs leading-4 text-muted-foreground tabular-nums"
            aria-hidden
            data-testid="hp-scale"
          >
            <span className="absolute -translate-x-1/2" style={{ left: `${dyingPct}%` }}>
              0
            </span>
            <span className="absolute right-0">{maxHp}</span>
          </div>
        )}
      </div>
    </div>
  );
}
