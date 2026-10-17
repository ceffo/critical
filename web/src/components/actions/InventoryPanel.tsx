import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import { useState } from "react";
import {
  DiceFive,
  Minus,
  PencilSimple,
  Plus,
  Trash,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { InventoryCard, ROW_ACTIONS } from "./InventoryCard";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { useTableStore } from "@/stores/table";
import { useDiceStore } from "@/stores/dice";
import {
  deleteItem,
  patchItem,
  postCreateItem,
  postRestockItem,
} from "@/api/items";
import { draftFromItem, emptyDraft, typeSuggestions } from "@/lib/itemForm";
import { cn } from "@/lib/utils";
import { ItemForm } from "./ItemForm";
import { WeaponsSection } from "./WeaponsSection";
import { formatSigned } from "@/lib/format";
import type { Item } from "@/types/items";

/**
 * Objets grid (ADR 0027 step 14.1): name (behaviour) | dice chip right after the name | quantity
 * stepper | spare width | edit / delete on hover. The name column is as wide as the longest name
 * (wrapping past it, never cut); the header and every row share the columns through `subgrid`.
 */
const ITEM_COLS =
  "grid grid-cols-[minmax(8rem,max-content)_auto_auto_minmax(0,1fr)_auto] items-center gap-x-3";
const ITEM_ROW = "col-span-full grid grid-cols-subgrid items-center";

/**
 * Inventaire workspace: the Objets card and the Armes card, side by side when the stage is at least
 * 1200 px wide, stacked below that (ADR 0027 step 14.3); each card is as tall as its rows and
 * scrolls inside when the stage is shorter, so the page never scrolls.
 * − uses one (the API rolls the effect, logs it, and removes a charges item at 0); + adds one,
 * and exists only on stock items. The dice chip puts the item's effect dice in the dice composer.
 * ✎ edits any field in place, 🗑 removes the item, "Add" opens a form. Every change is a log
 * entry the journal can pop.
 */
export function InventoryPanel() {
  const { t, i18n } = useTranslation();
  const items = useItemsStore((s) => s.items);
  const { spendItem, rolling } = useRollStore();
  const run = useTableStore((s) => s.run);
  const submit = useTableStore((s) => s.submit);
  const busy = useTableStore((s) => s.busy) || rolling;
  const toComposer = useDiceStore((s) => s.setFormula);
  // "add", an item id being edited, or nothing.
  const [form, setForm] = useState<"add" | string | null>(null);
  const types = typeSuggestions(items);
  const close = () => setForm(null);

  return (
    <div className="@container flex min-h-0 flex-1 flex-col">
      <div
        className="flex min-h-0 flex-1 flex-col gap-3 @min-[75rem]:flex-row @min-[75rem]:items-start"
        data-testid="inventory-cards"
      >
        <InventoryCard
          titleId="inventory-items-title"
          title={t("items.heading")}
          addLabel={t("items.form.openAdd")}
          showAdd={form !== "add"}
          busy={busy}
          onAdd={() => setForm("add")}
          focused
        >
          {form === "add" && (
            <div className="mb-2">
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
            </div>
          )}
          {items.length === 0 ? (
            <p className="text-body text-muted-foreground">{t("items.none")}</p>
          ) : (
            <div className={ITEM_COLS}>
              <div
                className={cn(
                  ITEM_ROW,
                  "px-2 pb-1 text-label text-muted-foreground",
                )}
                aria-hidden
              >
                <span>{t("items.columns.name")}</span>
                <span>{t("items.columns.effect")}</span>
                <span className="text-center">
                  {t("items.columns.quantity")}
                </span>
              </div>
              <ul
                className={cn(ITEM_ROW, "gap-y-1")}
                aria-label={t("items.title")}
              >
                {items.map((i) =>
                  form === i.id ? (
                    <li key={i.id} className="col-span-full">
                      <ItemForm
                        initial={draftFromItem(i, i18n.language)}
                        editing={i}
                        types={types}
                        busy={busy}
                        onCancel={close}
                        onSave={async (body) => {
                          const rejected = await submit(() =>
                            patchItem(i.id, body),
                          );
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
                      onDice={() => i.effectDice && toComposer(i.effectDice)}
                      onUse={() => void spendItem(i.id)}
                      onRestock={() => void run(() => postRestockItem(i.id, 1))}
                      onEdit={() => setForm(i.id)}
                      onRemove={() => void run(() => deleteItem(i.id))}
                    />
                  ),
                )}
              </ul>
            </div>
          )}
        </InventoryCard>
        <WeaponsSection />
      </div>
    </div>
  );
}

function ItemRow({
  item: i,
  busy,
  onDice,
  onUse,
  onRestock,
  onEdit,
  onRemove,
}: {
  item: Item;
  busy: boolean;
  onDice: () => void;
  onUse: () => void;
  onRestock: () => void;
  onEdit: () => void;
  onRemove: () => void;
}) {
  const { t, i18n } = useTranslation();
  return (
    <li
      className={cn(
        ITEM_ROW,
        "group/row min-h-7 rounded-lg bg-raised px-2 py-1.5 text-body",
      )}
      data-testid={`item-${i.id}`}
    >
      <div className="flex min-w-0 flex-col">
        <span className="flex min-w-0 items-center gap-2">
          <span className="min-w-0 break-words font-medium">
            {localized(i18n.language, i.name)}
          </span>
          {i.magicalBonus !== 0 && (
            <span
              className="chip shrink-0 px-1.5 text-label font-medium tabular-nums"
              title={t("items.magicalBonus")}
              aria-label={`${t("items.magicalBonus")} ${formatSigned(i.magicalBonus)}`}
              data-testid="item-bonus"
            >
              {formatSigned(i.magicalBonus)}
            </span>
          )}
        </span>
        {/* The behaviour only: the free-text type stays in the edit form (ADR 0027 step 14.1). */}
        <span className="min-w-0 break-words text-label text-muted-foreground">
          <span
            title={t(`items.behaviourHint.${i.behaviour}`)}
            data-testid="item-behaviour"
          >
            {t(`items.behaviour.${i.behaviour}`)}
          </span>
        </span>
      </div>
      <div className="min-w-0">
        {i.effectDice ? (
          <button
            type="button"
            className="chip inline-flex max-w-full items-center gap-1 px-2 py-0.5 text-body font-medium tabular-nums outline-none hover:text-tint focus-visible:ring-3 focus-visible:ring-ring/50"
            title={t("dice.send", { formula: i.effectDice })}
            aria-label={t("dice.send", { formula: i.effectDice })}
            data-testid="item-dice"
            onClick={onDice}
          >
            <DiceFive aria-hidden className="size-4 shrink-0" />
            <span className="break-words">{i.effectDice}</span>
          </button>
        ) : (
          <span className="text-muted-foreground" aria-hidden>
            —
          </span>
        )}
      </div>
      <div className="flex items-center justify-center gap-1">
        <Button
          size="icon-sm"
          disabled={busy || i.count === 0}
          onClick={onUse}
          aria-label={t("items.useOne", {
            name: localized(i18n.language, i.name),
          })}
        >
          <Minus />
        </Button>
        <span
          className="min-w-11 text-center font-semibold tabular-nums"
          data-testid="item-count"
        >
          {i.behaviour === "daily"
            ? t("items.count", { count: i.count, max: i.maxCount })
            : i.count}
        </span>
        {i.behaviour === "stock" ? (
          <Button
            size="icon-sm"
            disabled={busy}
            onClick={onRestock}
            aria-label={t("items.addOne", {
              name: localized(i18n.language, i.name),
            })}
          >
            <Plus />
          </Button>
        ) : (
          // Keeps the counts aligned: charges and daily items have no +.
          <span className="size-7" aria-hidden />
        )}
      </div>
      <span aria-hidden />
      <div className={ROW_ACTIONS}>
        <Button
          size="icon-sm"
          variant="ghost"
          className="text-muted-foreground"
          disabled={busy}
          onClick={onEdit}
          aria-label={t("items.form.edit", {
            name: localized(i18n.language, i.name),
          })}
        >
          <PencilSimple />
        </Button>
        <Button
          size="icon-sm"
          variant="ghost"
          className="text-muted-foreground"
          disabled={busy}
          onClick={onRemove}
          aria-label={t("items.form.remove", {
            name: localized(i18n.language, i.name),
          })}
        >
          <Trash />
        </Button>
      </div>
    </li>
  );
}
