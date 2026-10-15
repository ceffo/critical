import { useTranslation } from "react-i18next";
import { Shield } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";

/**
 * One card per active buff that absorbs hits (charges left, e.g. Stoneskin layers), above the
 * damage entry: its name, the charges left and Absorb, which spends one through the buff trigger
 * endpoint (logged, voidable). Nothing shows while no such buff is active.
 */
export function ChargeCards() {
  const { t } = useTranslation();
  const buffs = useBuffsStore((s) => s.buffs);
  const triggerBuff = useBuffsStore((s) => s.triggerBuff);
  const { run, busy } = useTableStore();
  const absorbing = buffs.filter((b) => b.charges > 0 && b.applies !== false);
  if (absorbing.length === 0) return null;

  return (
    <div className="flex flex-col gap-1.5">
      {absorbing.map((b) => (
        <div
          key={b.id}
          className="flex min-w-0 items-center gap-3 rounded-lg bg-amb-900 px-3 py-2"
          role="group"
          aria-label={b.name}
          data-testid="charge-card"
          data-buff={b.name}
        >
          <Shield aria-hidden className="size-5 shrink-0 text-amb-300" weight="duotone" />
          <div className="flex min-w-0 flex-1 flex-col leading-tight">
            <span className="truncate text-sm font-medium" title={b.name}>
              {b.name}
            </span>
            <span
              className="truncate text-xs text-muted-foreground tabular-nums"
              title={t("moi.chargesHint")}
              data-testid="charge-count"
            >
              {b.chargesTotal >= b.charges
                ? t("effects.chargesOf", { count: b.charges, total: b.chargesTotal })
                : t("moi.chargesLeft", { count: b.charges })}
            </span>
          </div>
          <Button
            size="sm"
            variant="secondary"
            disabled={busy}
            aria-label={t("moi.absorbLabel", { name: b.name })}
            onClick={() => void run(() => triggerBuff(b.id))}
          >
            {t("moi.absorb")}
          </Button>
        </div>
      ))}
    </div>
  );
}
