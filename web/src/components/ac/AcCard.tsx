import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { useAcStore } from "@/stores/ac";
import { formatAc } from "@/lib/format";
import { AcBreakdown } from "./AcBreakdown";
import { AcWarnings } from "./AcWarnings";
import { AcOverrideForm } from "./AcOverrideForm";
import { AcOverrideLine } from "./AcOverrideLine";

/**
 * The Inventaire's AC card (ADR 0035 item 7): on the right of the possessions,
 * `clamp(300px, 22vw, 420px)` wide (ADR 0027 step 5), the AC as the card's number, Override in its
 * band, the override with its derived AC and Clear when one is set, then the breakdown rows. It
 * scrolls inside when the stage is shorter (ADR 0027 step 14.3).
 */
export function AcCard() {
  const { t } = useTranslation();
  const ac = useAcStore((s) => s.ac);
  const [overriding, setOverriding] = useState(false);
  return (
    <section
      aria-labelledby="ac-card-title"
      className="zone-card flex w-[clamp(300px,22vw,420px)] min-h-0 shrink-0 flex-col overflow-y-auto"
      data-testid="ac-card"
    >
      <div className="card-band flex min-h-10 shrink-0 items-center justify-between gap-3 px-4 py-1.5">
        <h2 id="ac-card-title" className="section-label">
          {t("ac.title")}
        </h2>
        {ac && !ac.override && !overriding && (
          <Button
            size="sm"
            title={t("ac.override.openHint")}
            aria-label={t("ac.override.openHint")}
            data-testid="ac-card-override"
            onClick={() => setOverriding(true)}
          >
            {t("ac.override.open")}
          </Button>
        )}
      </div>
      {ac && (
        <div className="flex flex-col gap-3 p-4">
          {ac.override ? (
            <AcOverrideLine ac={ac} />
          ) : (
            <span
              className="text-hero leading-none tabular-nums"
              data-testid="ac-card-value"
            >
              {formatAc(ac.ac)}
            </span>
          )}
          {overriding && (
            <AcOverrideForm
              initial={ac.ac}
              onDone={() => setOverriding(false)}
            />
          )}
          <AcWarnings ac={ac} />
          <AcBreakdown ac={ac} />
        </div>
      )}
    </section>
  );
}
