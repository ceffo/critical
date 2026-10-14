import { useTranslation } from "react-i18next";
import { useState } from "react";
import { Minus, Pencil, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { useTableStore } from "@/stores/table";
import {
  deleteItem,
  patchItem,
  postCreateItem,
  postRestockItem,
} from "@/api/items";
import { draftFromItem, emptyDraft, typeSuggestions } from "@/lib/itemForm";
import { ItemForm } from "./ItemForm";
import { WeaponsSection } from "./WeaponsSection";
import { formatSigned } from "@/lib/format";
import type { Item } from "@/types/items";

/**
 * Inventory tab (critical-qki.13, .31): every item in one list with its type, behaviour and count.
 * − uses one (the API rolls the effect, logs it, and removes a charges item at 0); + adds one,
 * and exists only on stock items. ✎ edits any field in place, 🗑 removes the item, "+ Add item"
 * opens a form. Every change is a log entry the Combat Log can pop.
 */
export function InventoryPanel() {
  const { t } = useTranslation();
  const items = useItemsStore((s) => s.items);
  const { spendItem, rolling } = useRollStore();
  const run = useTableStore((s) => s.run);
  const submit = useTableStore((s) => s.submit);
  const busy = useTableStore((s) => s.busy) || rolling;
  // "add", an item id being edited, or nothing.
  const [form, setForm] = useState<"add" | string | null>(null);
  const types = typeSuggestions(items);
  const close = () => setForm(null);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex max-w-2xl flex-col gap-2">
        {form === "add" ? (
          <ItemForm
            initial={emptyDraft}
            types={types}
            busy={busy}
            onCancel={close}
            onSave={async (body) => {
              const rejected = await submit(() => postCreateItem(body));
              if (!rejected) close();
              return rejected;
            }}
          />
        ) : (
          <div>
            <Button
              size="sm"
              variant="outline"
              disabled={busy}
              onClick={() => setForm("add")}
            >
              <Plus /> {t("items.form.openAdd")}
            </Button>
          </div>
        )}
        {items.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("items.none")}</p>
        )}
        <ul className="flex flex-col gap-1" aria-label={t("items.title")}>
          {items.map((i) =>
            form === i.id ? (
              <li key={i.id}>
                <ItemForm
                  initial={draftFromItem(i)}
                  editing={i}
                  types={types}
                  busy={busy}
                  onCancel={close}
                  onSave={async (body) => {
                    const rejected = await submit(() => patchItem(i.id, body));
                    if (!rejected) close();
                    return rejected;
                  }}
                />
              </li>
            ) : (
              <ItemRow
                key={i.id}
                item={i}
                busy={busy}
                onUse={() => void spendItem(i.id)}
                onRestock={() => void run(() => postRestockItem(i.id, 1))}
                onEdit={() => setForm(i.id)}
                onRemove={() => void run(() => deleteItem(i.id))}
              />
            ),
          )}
        </ul>
      </div>
      <WeaponsSection />
    </div>
  );
}

function ItemRow({
  item: i,
  busy,
  onUse,
  onRestock,
  onEdit,
  onRemove,
}: {
  item: Item;
  busy: boolean;
  onUse: () => void;
  onRestock: () => void;
  onEdit: () => void;
  onRemove: () => void;
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
          <span
            className="text-xs text-muted-foreground"
            data-testid="item-type"
          >
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
      <Button
        size="icon-sm"
        variant="ghost"
        disabled={busy}
        onClick={onEdit}
        aria-label={t("items.form.edit", { name: i.name })}
      >
        <Pencil />
      </Button>
      <Button
        size="icon-sm"
        variant="ghost"
        disabled={busy}
        onClick={onRemove}
        aria-label={t("items.form.remove", { name: i.name })}
      >
        <Trash2 />
      </Button>
    </li>
  );
}
