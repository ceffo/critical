import type { ReactNode } from "react";
import { Plus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

/**
 * A row's edit / delete buttons (ADR 0027 step 14.1): shown on the row's hover and keyboard focus
 * (step 1.4), always on a device without hover. `opacity`, not `visibility`, so Tab still reaches
 * them and the focus shows them.
 */
export const ROW_ACTIONS =
  "flex justify-end opacity-0 transition-opacity group-hover/row:opacity-100 group-focus-within/row:opacity-100 [@media(hover:none)]:opacity-100";

/**
 * A card of the Inventaire stage: its section-label title, the quiet one-word Add (ADR 0027 steps
 * 3.4 and 14.2, the detail in its tooltip and `aria-label`), then its body, which scrolls inside the
 * card when the stage is shorter than it (step 14.3: the page never scrolls). The title and Add sit
 * in the card's header band (ADR 0026 step 6.2); `focused`: the activity's focused card (step 6.1),
 * one tone up with the 2 px outline.
 */
export function InventoryCard({
  titleId,
  title,
  label,
  addLabel,
  showAdd,
  busy,
  onAdd,
  focused = false,
  toolbar,
  children,
}: {
  titleId?: string;
  title: string;
  /** The region's name when it differs from the title. */
  label?: string;
  /** "Add item" / "Add weapon": the tooltip and accessible name of the Add button. */
  addLabel: string;
  showAdd: boolean;
  busy: boolean;
  onAdd: () => void;
  focused?: boolean;
  /** A line under the title and Add, in the band (the category chips, ADR 0035 item 2). */
  toolbar?: ReactNode;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <section
      aria-labelledby={label ? undefined : titleId}
      aria-label={label}
      className={cn(
        focused ? "stage-card" : "zone-card",
        "flex min-h-0 min-w-0 flex-col pb-3",
      )}
    >
      <div className="card-band mb-1.5 flex shrink-0 flex-col gap-1.5 px-3 py-1.5">
        <div className="flex min-h-7 items-center justify-between gap-3">
          <h2 id={titleId} className="section-label">
            {title}
          </h2>
          {showAdd && (
            <Button
              size="sm"
              disabled={busy}
              title={addLabel}
              aria-label={addLabel}
              onClick={onAdd}
            >
              <Plus /> {t("items.form.add")}
            </Button>
          )}
        </div>
        {toolbar}
      </div>
      <div className="flex min-h-0 flex-col gap-1 overflow-y-auto px-3">
        {children}
      </div>
    </section>
  );
}
