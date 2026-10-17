import type { ReactNode } from "react";
import { Plus } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";

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
 * card when the stage is shorter than it (step 14.3: the page never scrolls). Side by side (the
 * Inventaire stage's container from 1200 px), the two cards share the width.
 */
export function InventoryCard({
  titleId,
  title,
  label,
  addLabel,
  showAdd,
  busy,
  onAdd,
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
  children: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <section
      aria-labelledby={label ? undefined : titleId}
      aria-label={label}
      className="flex min-h-0 min-w-0 flex-col rounded-xl bg-card py-3 @min-[75rem]:max-h-full @min-[75rem]:flex-1"
    >
      <div className="mb-1.5 flex shrink-0 items-center justify-between gap-3 px-3">
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
      <div className="flex min-h-0 flex-col gap-1 overflow-y-auto px-3">
        {children}
      </div>
    </section>
  );
}
