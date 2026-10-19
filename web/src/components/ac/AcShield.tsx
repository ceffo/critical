import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Warning } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useAcStore } from "@/stores/ac";
import { shownAc } from "@/lib/acView";
import { formatAc } from "@/lib/format";
import { cn } from "@/lib/utils";
import { AcBreakdown } from "./AcBreakdown";
import { AcWarnings } from "./AcWarnings";
import { AcOverrideForm } from "./AcOverrideForm";
import { AcOverrideLine } from "./AcOverrideLine";

/**
 * The character's AC beside the HP (ADR 0035 item 4, user: "a number in a filled shield beside HP,
 * no label, smaller than HP"): the number alone inside a filled shield in the selected tone with a
 * tinted outline, amber while an override is set. Its tooltip and accessible name say "Armour
 * class"; clicking it opens the breakdown (item 8) with Inventory and Override.
 */
export function AcShield({ onOpenInventory }: { onOpenInventory: () => void }) {
  const { t } = useTranslation();
  const ac = useAcStore((s) => s.ac);
  const [open, setOpen] = useState(false);
  const [overriding, setOverriding] = useState(false);
  if (!ac) return null;
  const shown = formatAc(shownAc(ac));
  const base = ac.override
    ? t("ac.openOverridden", { value: shown, derived: formatAc(ac.ac) })
    : t("ac.open", { value: shown });
  const name = ac.warnings.length > 0 ? `${base} (${t("ac.warned")})` : base;
  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setOverriding(false);
      }}
    >
      <PopoverTrigger
        className={cn(
          "relative flex size-16 shrink-0 items-center justify-center self-center rounded-lg outline-none transition-colors hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50",
          open && "bg-raised-hover",
        )}
        title={t("ac.title")}
        aria-label={name}
        data-testid="ac-shield"
        data-overridden={ac.override ? true : undefined}
        data-warned={ac.warnings.length > 0 ? true : undefined}
      >
        <svg
          viewBox="0 0 48 56"
          className="absolute inset-1 size-14"
          aria-hidden
        >
          <path
            d="M24 2 L44 9 V27 C44 40 35 49 24 54 C13 49 4 40 4 27 V9 Z"
            fill="var(--amb-800)"
            stroke={ac.override ? "var(--warning)" : "var(--tint)"}
            strokeWidth="2"
          />
        </svg>
        <span
          className="relative -mt-1 text-key font-bold tabular-nums"
          data-testid="ac-value"
        >
          {shown}
        </span>
        {ac.warnings.length > 0 && (
          // ADR 0035 item 4: a class warning adds a warning badge on the shield.
          <Warning
            weight="fill"
            aria-hidden
            className="absolute -right-0.5 -top-0.5 size-5 text-warning"
            data-testid="ac-shield-warning"
          />
        )}
      </PopoverTrigger>
      <PopoverContent
        side="right"
        align="start"
        className="flex max-h-(--available-height) w-[23rem] max-w-[calc(100vw-2rem)] flex-col gap-3 overflow-y-auto p-4"
        data-testid="ac-popover"
      >
        <PopoverTitle className="section-label">
          {t("ac.titleWith", { value: shown })}
        </PopoverTitle>
        <AcOverrideLine ac={ac} />
        {overriding ? (
          <AcOverrideForm
            initial={shownAc(ac)}
            onDone={() => setOverriding(false)}
          />
        ) : null}
        <AcWarnings ac={ac} />
        <AcBreakdown ac={ac} />
        <div className="flex flex-wrap justify-between gap-2">
          <Button
            size="sm"
            title={t("ac.inventoryHint")}
            aria-label={t("ac.inventoryHint")}
            onClick={() => {
              setOpen(false);
              onOpenInventory();
            }}
          >
            {t("ac.inventory")}
          </Button>
          {!ac.override && !overriding && (
            <Button
              size="sm"
              title={t("ac.override.openHint")}
              aria-label={t("ac.override.openHint")}
              data-testid="ac-override-open"
              onClick={() => setOverriding(true)}
            >
              {t("ac.override.open")}
            </Button>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
