import { useTranslation } from "react-i18next";
import { useHpStore } from "@/stores/hp";
import { useLogStore } from "@/stores/log";
import { formatSigned } from "@/lib/format";
import { hpBarModel } from "@/lib/hpBar";
import { lastDamage, lastHpChange } from "@/lib/moiLog";
import { cn } from "@/lib/utils";
import { useSpellName } from "@/i18n/spellName";
import type { HpCondition } from "@/types/hp";

/** Bar fill per threshold band (display only, `lib/hpBar.ts`). */
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
 * HP block (ADR 0022 step 2.3): the section label with the last change still in force on the right
 * (from the log, wrapping when long, never cut), current HP large over max, and the bar
 * (`lib/hpBar.ts`): a hatched dying zone from the server's floor (-10) to 0, filled from 0 down to
 * current HP below 0, with "0" at its boundary; the fill from 0 coloured by threshold; the last
 * damage taken as a faint ghost after the fill. The condition (ADR 0011) is the server's; it is only
 * labelled here. Every number is the server's; the bar only draws them.
 */
export function HpBar() {
  const { t } = useTranslation();
  const spellName = useSpellName();
  const { currentHp, maxHp, floor, condition, loaded } = useHpStore();
  const entries = useLogStore((s) => s.entries);
  const last = lastHpChange(entries);
  const damage = lastDamage(entries);
  const bar = hpBarModel(currentHp, maxHp, floor, damage);

  // A change logged without a source is worded like the journal does: Damage N / Healing N.
  const lastText = last
    ? last.exposure
      ? t("hp.lastExposure", {
          delta: formatSigned(last.delta),
          spell: spellName(last.exposure.spellId, last.exposure.spell),
        })
      : last.source
        ? t("hp.lastChange", {
            delta: formatSigned(last.delta),
            source: last.source,
          })
        : t(last.delta < 0 ? "log.hpDamage" : "log.hpHeal", {
            amount: Math.abs(last.delta),
          })
    : "";

  return (
    <div
      className="flex min-w-0 flex-col gap-1"
      role="group"
      aria-label={t("hp.title")}
    >
      {/* The last change sits beside the label; a long one drops under it and wraps (never cut). */}
      <div className="flex min-w-0 flex-wrap items-baseline justify-between gap-x-2">
        <h2 className="shrink-0 section-label">{t("hp.title")}</h2>
        {last && (
          <span
            className={cn(
              "ml-auto max-w-full text-body break-words tabular-nums",
              last.delta < 0 ? "text-danger" : "text-success",
            )}
            data-testid="hp-last-change"
          >
            {lastText}
          </span>
        )}
      </div>
      <div className="flex min-w-0 items-baseline justify-between gap-2">
        <div
          className="flex items-baseline gap-1 tabular-nums"
          data-testid="hp-value"
        >
          {loaded ? (
            <>
              <span className="text-headline">{currentHp}</span>
              <span className="text-body text-muted-foreground">
                {" "}
                / {maxHp}
              </span>
            </>
          ) : (
            "–"
          )}
        </div>
        {loaded && condition !== "ok" && (
          <span
            className={cn(
              "min-w-0 text-right text-body break-words",
              CONDITION[condition],
            )}
            data-testid="hp-condition"
            data-condition={condition}
          >
            {t(`hp.condition.${condition}`)}
          </span>
        )}
      </div>
      <div className="flex flex-col gap-1">
        <div
          className="flex h-3 w-full overflow-hidden rounded bg-neu-900"
          role="meter"
          aria-label={t("hp.title")}
          aria-valuemin={floor}
          aria-valuemax={maxHp}
          aria-valuenow={currentHp}
        >
          {bar.dyingPct > 0 && (
            <div
              className="relative h-full shrink-0 hatch-danger"
              style={{ width: `${bar.dyingPct}%` }}
              title={t("hp.dyingZone", { floor })}
              data-testid="hp-dying-zone"
            >
              {bar.deepPct > 0 && (
                <div
                  className="absolute inset-y-0 right-0 bg-danger transition-all duration-300"
                  style={{ width: `${bar.deepPct}%` }}
                  data-testid="hp-dying-fill"
                />
              )}
            </div>
          )}
          <div className="flex h-full min-w-0 flex-1">
            <div
              className={cn(
                "h-full transition-all duration-300",
                TONE[bar.tone],
              )}
              style={{ width: `${bar.fillPct}%` }}
              data-testid="hp-fill"
            />
            {bar.ghostPct > 0 && (
              <div
                className="h-full bg-danger/35 transition-all duration-300"
                style={{ width: `${bar.ghostPct}%` }}
                title={t("hp.lastDamage", { amount: damage })}
                data-testid="hp-ghost"
              />
            )}
          </div>
        </div>
        {bar.dyingPct > 0 && (
          <div
            className="relative h-4 text-label text-muted-foreground tabular-nums"
            aria-hidden
            data-testid="hp-scale"
          >
            <span
              className="absolute -translate-x-1/2"
              style={{ left: `${bar.dyingPct}%` }}
            >
              0
            </span>
            <span className="absolute right-0">{maxHp}</span>
          </div>
        )}
      </div>
    </div>
  );
}
