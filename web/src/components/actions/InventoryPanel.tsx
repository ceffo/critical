import { useTranslation } from "react-i18next";
import { Minus, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { useTableStore } from "@/stores/table";
import { postRestockItem } from "@/api/items";
import { formatSigned } from "@/lib/format";
import type { Item } from "@/types/items";

/**
 * Inventory tab (critical-qki.13): every item in one list with its type, behaviour and count.
 * − uses one (the API rolls the effect, logs it, and removes a charges item at 0); + adds one,
 * and exists only on stock items. Every change is a log entry the Combat Log can pop.
 */
export function InventoryPanel() {
  const { t } = useTranslation();
  const items = useItemsStore((s) => s.items);
  const { spendItem, rolling } = useRollStore();
  const run = useTableStore((s) => s.run);
  const busy = useTableStore((s) => s.busy) || rolling;

  if (items.length === 0) {
    return <p className="text-sm text-muted-foreground">{t("items.none")}</p>;
  }
  return (
    <ul className="flex max-w-2xl flex-col gap-1" aria-label={t("items.title")}>
      {items.map((i) => (
        <ItemRow
          key={i.id}
          item={i}
          busy={busy}
          onUse={() => void spendItem(i.id)}
          onRestock={() => void run(() => postRestockItem(i.id, 1))}
        />
      ))}
    </ul>
  );
}

function ItemRow({
  item: i,
  busy,
  onUse,
  onRestock,
}: {
  item: Item;
  busy: boolean;
  onUse: () => void;
  onRestock: () => void;
}) {
  const { t } = useTranslation();
  return (
    <li
      className="flex items-center gap-2 rounded-lg border px-2.5 py-1.5 text-sm"
      data-testid={`item-${i.id}`}
    >
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="min-w-0 truncate font-medium">{i.name}</span>
          {i.magicalBonus !== 0 && (
            <Badge
              variant="outline"
              title={t("items.magicalBonus")}
              aria-label={`${t("items.magicalBonus")} ${formatSigned(i.magicalBonus)}`}
              data-testid="item-bonus"
            >
              {formatSigned(i.magicalBonus)}
            </Badge>
          )}
          {i.effectDice && <Badge variant="outline">{i.effectDice}</Badge>}
        </span>
        {i.type && (
          <span className="text-xs text-muted-foreground" data-testid="item-type">
            {i.type}
          </span>
        )}
      </div>
      <Badge
        variant="secondary"
        title={t(`items.behaviourHint.${i.behaviour}`)}
        data-testid="item-behaviour"
      >
        {t(`items.behaviour.${i.behaviour}`)}
      </Badge>
      <Button
        size="icon-sm"
        variant="outline"
        disabled={busy || i.count === 0}
        onClick={onUse}
        aria-label={t("items.useOne", { name: i.name })}
      >
        <Minus />
      </Button>
      <span
        className="min-w-14 text-center font-mono font-semibold tabular-nums"
        data-testid="item-count"
      >
        {i.behaviour === "daily"
          ? t("items.count", { count: i.count, max: i.maxCount })
          : i.count}
      </span>
      {i.behaviour === "stock" ? (
        <Button
          size="icon-sm"
          variant="outline"
          disabled={busy}
          onClick={onRestock}
          aria-label={t("items.addOne", { name: i.name })}
        >
          <Plus />
        </Button>
      ) : (
        // Keeps the counts aligned: charges and daily items have no +.
        <span className="size-7" aria-hidden />
      )}
    </li>
  );
}
