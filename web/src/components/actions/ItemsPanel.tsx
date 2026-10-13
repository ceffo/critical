import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { captionClass } from "@/components/dashboard/fieldStyles";
import type { Item } from "@/types/items";

/**
 * Items tab: ammunition counts per arrow type, and the charged items and consumables with a Use
 * button. The API rolls the effect and logs the use; the result card shows the breakdown.
 */
export function ItemsPanel() {
  const { t } = useTranslation();
  const items = useItemsStore((s) => s.items);
  const { spendItem, rolling } = useRollStore();

  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("items.loading")}</p>;
  }
  const ammo = items.filter((i) => i.kind === "ammo");
  const usable = items.filter((i) => i.kind !== "ammo");

  return (
    <div className="flex max-w-xl flex-col gap-4">
      <section aria-label={t("items.ammo")}>
        <h3 className={captionClass}>{t("items.ammo")}</h3>
        <ul className="mt-1 flex flex-col gap-1">
          {ammo.map((i) => (
            <li
              key={i.id}
              className="flex items-center justify-between gap-2 rounded-lg border px-2.5 py-1.5 text-sm"
              data-testid={`item-${i.id}`}
            >
              <span className="min-w-0 truncate">{i.name}</span>
              <span className="font-mono font-semibold tabular-nums" data-testid="item-count">
                {t("items.left", { count: i.count })}
              </span>
            </li>
          ))}
        </ul>
      </section>

      <section aria-label={t("items.charged")}>
        <h3 className={captionClass}>{t("items.charged")}</h3>
        <ul className="mt-1 flex flex-col gap-1">
          {usable.map((i) => (
            <UsableRow key={i.id} item={i} busy={rolling} onUse={() => void spendItem(i.id)} />
          ))}
        </ul>
      </section>
    </div>
  );
}

function UsableRow({ item: i, busy, onUse }: { item: Item; busy: boolean; onUse: () => void }) {
  const { t } = useTranslation();
  const spent = i.count === 0;
  return (
    <li
      className="flex items-center gap-2 rounded-lg border px-2.5 py-1.5 text-sm"
      data-testid={`item-${i.id}`}
    >
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="flex items-center gap-1.5">
          <span className="min-w-0 truncate font-medium">{i.name}</span>
          {i.effect_dice && <Badge variant="outline">{i.effect_dice}</Badge>}
          {i.per_day && <Badge variant="secondary">{t("items.perDay")}</Badge>}
        </span>
      </div>
      <span
        className="font-mono font-semibold tabular-nums"
        data-testid="item-count"
        aria-label={t("items.left", { count: i.count })}
      >
        {t("items.count", { count: i.count, max: i.max_count })}
      </span>
      <Button
        size="sm"
        disabled={busy || spent}
        onClick={onUse}
        aria-label={`${spent ? t("items.empty") : t("items.use")} ${i.name}`}
      >
        {spent ? t("items.empty") : t("items.use")}
      </Button>
    </li>
  );
}
