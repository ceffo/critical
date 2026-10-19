import { ArrowCounterClockwise } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { clearAcOverride } from "@/api/ac";
import { useTableStore } from "@/stores/table";
import { formatAc } from "@/lib/format";
import type { ArmourClass } from "@/types/ac";

/**
 * An override that is set (ADR 0033 step 13, ADR 0035 item 8): the AC shown, marked as overridden
 * with the player's label, the derived AC beside it, and Clear (one logged move).
 */
export function AcOverrideLine({ ac }: { ac: ArmourClass }) {
  const { t } = useTranslation();
  const run = useTableStore((s) => s.run);
  const busy = useTableStore((s) => s.busy);
  if (!ac.override) return null;
  const o = ac.override;
  return (
    <div
      className="flex flex-wrap items-end justify-between gap-x-3 gap-y-2"
      data-testid="ac-override"
    >
      <div className="flex min-w-0 flex-col">
        <span className="text-label break-words text-warning">
          {o.label
            ? t("ac.override.shownWith", { label: o.label })
            : t("ac.override.shown")}
        </span>
        <span
          className="text-hero leading-none tabular-nums"
          data-testid="ac-override-value-shown"
        >
          {formatAc(o.value)}
        </span>
      </div>
      <div className="flex flex-col items-end">
        <span className="text-label text-muted-foreground">
          {t("ac.override.derived")}
        </span>
        <span className="text-key text-muted-foreground tabular-nums">
          {formatAc(ac.ac)}
        </span>
      </div>
      <Button
        size="sm"
        disabled={busy}
        title={t("ac.override.clearHint")}
        aria-label={t("ac.override.clearHint")}
        data-testid="ac-override-clear"
        onClick={() => void run(() => clearAcOverride())}
      >
        <ArrowCounterClockwise aria-hidden />
        {t("ac.override.clear")}
      </Button>
    </div>
  );
}
