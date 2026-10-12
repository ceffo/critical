import { useTranslation } from "react-i18next";
import { useHpStore } from "@/stores/hp";
import { hpPercent, hpTone } from "@/lib/format";
import { cn } from "@/lib/utils";
import { captionClass } from "./fieldStyles";

const TONE = {
  good: "bg-green-500",
  warn: "bg-yellow-400",
  bad: "bg-red-500",
} as const;

/** Current / max HP with a coloured bar (docs/frontend.md display conventions). */
export function HpBar() {
  const { t } = useTranslation();
  const { currentHp, maxHp, loaded } = useHpStore();
  const pct = hpPercent(currentHp, maxHp);
  return (
    <div
      className="flex min-w-0 flex-col gap-1"
      role="group"
      aria-label={t("hp.title")}
    >
      <div className="flex items-baseline justify-between gap-2">
        <span className={captionClass}>{t("hp.title")}</span>
        <span
          className="font-mono text-2xl font-black tabular-nums leading-none"
          data-testid="hp-value"
        >
          {loaded ? `${currentHp} / ${maxHp}` : "–"}
        </span>
      </div>
      <div
        className="h-3 w-full overflow-hidden rounded-full bg-muted"
        role="meter"
        aria-label={t("hp.title")}
        aria-valuemin={0}
        aria-valuemax={maxHp}
        aria-valuenow={currentHp}
      >
        <div
          className={cn("h-full transition-all duration-300", TONE[hpTone(pct)])}
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
