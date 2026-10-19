import { useTranslation } from "react-i18next";
import { localized } from "@/i18n/localized";
import { useEffect, useState } from "react";
import {
  Backpack,
  Crosshair,
  Diamond,
  DiceFive,
  MagicWand,
  Minus,
  PencilSimple,
  Plus,
  Shield,
  Sword,
  TShirt,
  Trash,
  Warning,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { InventoryCard, ROW_ACTIONS } from "./InventoryCard";
import { useItemsStore } from "@/stores/items";
import { useRollStore } from "@/stores/roll";
import { useTableStore } from "@/stores/table";
import { useDiceStore } from "@/stores/dice";
import { useCharacterStore } from "@/stores/character";
import {
  deleteItem,
  dismissReview,
  patchItem,
  postCreateItem,
  postItemState,
  postRestockItem,
} from "@/api/items";
import { draftFromItem, emptyDraft } from "@/lib/itemForm";
import { cn } from "@/lib/utils";
import { ItemForm } from "./ItemForm";
import { formatSigned } from "@/lib/format";
import { CATEGORIES, type Category, type Item } from "@/types/items";
import { pieceStats, piecesIn, equipped } from "@/lib/equipment";

/**
 * Possessions grid (ADR 0035 item 2): icon | name, tags and stats | dice chip | count | Equip /
 * Unequip | edit / delete on hover. The name wraps, never cut (ADR 0022 step 1); the rows share the
 * columns through `subgrid`.
 */
const ITEM_COLS =
  "grid grid-cols-[auto_minmax(10rem,1fr)_auto_auto_auto_auto] items-center gap-x-3";
const ITEM_ROW = "col-span-full grid grid-cols-subgrid items-center";

/** The icon of a piece's category (a wand has its own). */
function CategoryIcon({ item }: { item: Item }) {
  const cls = "size-5 shrink-0 text-muted-foreground";
  if (item.category === "magic" && item.magicKind === "wand")
    return <MagicWand aria-hidden className={cls} />;
  const Icon = {
    weapon: Sword,
    ammunition: Crosshair,
    armour: TShirt,
    shield: Shield,
    magic: Diamond,
    gear: Backpack,
  }[item.category];
  return <Icon aria-hidden className={cls} />;
}

/** The categories whose pieces always count at the table (stock, charges, daily). */
const COUNTED: readonly Category[] = ["ammunition", "magic", "gear"];

/**
 * Whether the row shows its count and use controls: always for a counted category; for a weapon,
 * armour or shield only when the piece counts (a charges or daily behaviour, more than one, or
 * effect dice), so a plain one-off weapon stays free of them.
 */
function isCounted(i: Item): boolean {
  return (
    COUNTED.includes(i.category) ||
    i.behaviour !== "stock" ||
    i.count > 1 ||
    i.effectDice !== ""
  );
}

/**
 * Inventaire workspace (ADR 0033 step 1, ADR 0035 items 2, 3 and 7): one flat possessions list in
 * the player's order, the category chips the only filter; each piece with its "Equipped" tag and
 * Equip / Unequip (one logged move, ADR 0033 step 7). The pieces the conversion could not settle
 * show in an amber band; Check opens them above the list, each with Edit and Dismiss (ADR 0033
 * step 27). − uses one (the API rolls the effect, logs it, and removes a charges item at 0); + adds
 * one, and exists only on stock items. The dice chip puts the piece's effect dice in the dice
 * composer. ✎ edits any field in place, 🗑 removes the piece, "Add" opens a form. Every change is a
 * log entry the journal can pop. The card is as tall as its rows and scrolls inside when the stage
 * is shorter (ADR 0027 step 14.3).
 */
export function InventoryPanel() {
  const { t, i18n } = useTranslation();
  const items = useItemsStore((s) => s.items);
  const { spendItem, rolling } = useRollStore();
  const run = useTableStore((s) => s.run);
  const submit = useTableStore((s) => s.submit);
  const busy = useTableStore((s) => s.busy) || rolling;
  const toComposer = useDiceStore((s) => s.setFormula);
  const specializes = useCharacterStore(
    (s) => s.character?.derived.specializes ?? false,
  );
  // "add", a piece id being edited, or nothing.
  const [form, setForm] = useState<"add" | string | null>(null);
  const [filter, setFilter] = useState<Category | "all">("all");
  const [reviewOpen, setReviewOpen] = useState(false);
  const close = () => setForm(null);
  // Edit from the "To check" panel opens the form in the list below it, often out of view: bring it up.
  useEffect(() => {
    if (form === null) return;
    document
      .querySelector("[data-testid='inventory-form']")
      ?.scrollIntoView({ block: "nearest" });
  }, [form]);
  const toReview = items.filter((i) => i.review.length > 0);
  const shown = piecesIn(items, filter);
  const nameOf = (i: Item) => localized(i18n.language, i.name);

  const editor = (i: Item) => (
    <ItemForm
      initial={draftFromItem(i, i18n.language)}
      editing={i}
      specializes={specializes}
      busy={busy}
      onCancel={close}
      onSave={async (body) => {
        const rejected = await submit(() => patchItem(i.id, body));
        if (!rejected) close();
        return rejected;
      }}
    />
  );

  return (
    <div className="@container flex min-h-0 flex-1 flex-col gap-3">
      {reviewOpen && toReview.length > 0 && (
        <section
          aria-labelledby="inventory-review-title"
          className="zone-card flex max-h-[45%] min-h-0 shrink-0 flex-col pb-3"
          data-testid="inventory-review"
        >
          <div className="card-band mb-1.5 flex min-h-10 flex-wrap items-center justify-between gap-x-3 px-3 py-1">
            <h2
              id="inventory-review-title"
              className="section-label flex items-center gap-2 text-warning"
            >
              <Warning aria-hidden className="size-4" />
              {t("equipment.review.title")}
              <span className="tabular-nums">· {toReview.length}</span>
            </h2>
            <span className="text-label text-muted-foreground">
              {t("equipment.review.intro")}
            </span>
          </div>
          <ul
            className="flex min-h-0 flex-col gap-1 overflow-y-auto px-3"
            data-testid="inventory-review-list"
          >
            {toReview.map((i) => (
              <li
                key={i.id}
                className="flex items-start gap-3 rounded-lg bg-raised px-2 py-1.5 text-body"
                data-testid={`review-${i.id}`}
              >
                <CategoryIcon item={i} />
                <div className="flex min-w-0 flex-1 flex-col">
                  <span className="break-words font-medium">{nameOf(i)}</span>
                  {i.review.map((r) => (
                    <span
                      key={r}
                      className="break-words text-label text-muted-foreground"
                    >
                      {t(`equipment.review.reasons.${r}`, { type: i.type })}
                    </span>
                  ))}
                </div>
                <Button
                  size="sm"
                  disabled={busy}
                  title={t("equipment.review.editHint", { name: nameOf(i) })}
                  aria-label={t("equipment.review.editHint", {
                    name: nameOf(i),
                  })}
                  onClick={() => {
                    setFilter("all");
                    setForm(i.id);
                  }}
                >
                  <PencilSimple /> {t("equipment.review.edit")}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={busy}
                  title={t("equipment.review.dismissHint", { name: nameOf(i) })}
                  aria-label={t("equipment.review.dismissHint", {
                    name: nameOf(i),
                  })}
                  onClick={() => void run(() => dismissReview(i.id))}
                >
                  {t("equipment.review.dismiss")}
                </Button>
              </li>
            ))}
          </ul>
        </section>
      )}
      <InventoryCard
        titleId="inventory-items-title"
        title={t("equipment.title")}
        addLabel={t("items.form.openAdd")}
        showAdd={form !== "add"}
        busy={busy}
        onAdd={() => setForm("add")}
        focused
        toolbar={
          <div
            role="group"
            aria-label={t("equipment.filter")}
            className="flex flex-wrap gap-1"
            data-testid="category-chips"
          >
            {(["all", ...CATEGORIES] as const).map((c) => (
              <button
                key={c}
                type="button"
                className={cn(
                  "h-7 px-2 text-body font-medium outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50",
                  filter === c
                    ? "chip-chosen"
                    : "chip text-muted-foreground hover:text-foreground",
                )}
                aria-pressed={filter === c}
                onClick={() => setFilter(c)}
              >
                {c === "all"
                  ? t("equipment.all")
                  : t(`equipment.categories.${c}`)}
              </button>
            ))}
          </div>
        }
      >
        {toReview.length > 0 && (
          <div
            className="mb-2 flex flex-wrap items-center justify-between gap-2 rounded-lg border border-warning/40 bg-warning/10 px-3 py-1.5 text-body text-warning"
            data-testid="review-band"
          >
            <span className="flex min-w-0 items-center gap-2 break-words">
              <Warning aria-hidden className="size-4 shrink-0" />
              {t("equipment.review.band", { count: toReview.length })}
            </span>
            <Button
              size="sm"
              title={t("equipment.review.openHint")}
              aria-label={t("equipment.review.openHint")}
              aria-expanded={reviewOpen}
              onClick={() => setReviewOpen((o) => !o)}
            >
              {reviewOpen
                ? t("equipment.review.close")
                : t("equipment.review.open")}
            </Button>
          </div>
        )}
        {form === "add" && (
          <div className="mb-2" data-testid="inventory-form">
            <ItemForm
              initial={emptyDraft(filter === "all" ? "gear" : filter)}
              specializes={specializes}
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
          <p className="text-body text-muted-foreground">
            {t("equipment.none")}
          </p>
        ) : shown.length === 0 ? (
          <p className="text-body text-muted-foreground">
            {t("equipment.noneInCategory")}
          </p>
        ) : (
          <ul
            className={cn(ITEM_COLS, "gap-y-1")}
            aria-label={t("items.title")}
          >
            {shown.map((i) =>
              form === i.id ? (
                <li
                  key={i.id}
                  className="col-span-full"
                  data-testid="inventory-form"
                >
                  {editor(i)}
                </li>
              ) : (
                <ItemRow
                  key={i.id}
                  item={i}
                  busy={busy}
                  onDice={() => i.effectDice && toComposer(i.effectDice)}
                  onUse={() => void spendItem(i.id)}
                  onRestock={() => void run(() => postRestockItem(i.id, 1))}
                  onEquip={() =>
                    void run(() =>
                      postItemState(
                        i.id,
                        equipped(i)
                          ? { state: "carried" }
                          : { state: i.equipState || "carried", swap: true },
                      ),
                    )
                  }
                  onEdit={() => setForm(i.id)}
                  onRemove={() => void run(() => deleteItem(i.id))}
                />
              ),
            )}
          </ul>
        )}
      </InventoryCard>
    </div>
  );
}

function ItemRow({
  item: i,
  busy,
  onDice,
  onUse,
  onRestock,
  onEquip,
  onEdit,
  onRemove,
}: {
  item: Item;
  busy: boolean;
  onDice: () => void;
  onUse: () => void;
  onRestock: () => void;
  onEquip: () => void;
  onEdit: () => void;
  onRemove: () => void;
}) {
  const { t, i18n } = useTranslation();
  const name = localized(i18n.language, i.name);
  const counted = isCounted(i);
  return (
    <li
      className={cn(
        ITEM_ROW,
        "group/row min-h-7 rounded-lg bg-raised px-2 py-1.5 text-body",
      )}
      data-testid={`item-${i.id}`}
      data-category={i.category}
    >
      <CategoryIcon item={i} />
      <div className="flex min-w-0 flex-col">
        <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5">
          <span className="min-w-0 break-words font-medium">{name}</span>
          {i.category === "ammunition" && i.magicalBonus !== 0 && (
            <span
              className="chip shrink-0 px-1.5 text-label font-medium tabular-nums"
              title={t("items.magicalBonus")}
              aria-label={`${t("items.magicalBonus")} ${formatSigned(i.magicalBonus)}`}
              data-testid="item-bonus"
            >
              {formatSigned(i.magicalBonus)}
            </span>
          )}
          {equipped(i) && (
            <span
              className="chip-chosen shrink-0 px-1.5 text-label font-medium"
              data-testid="item-equipped"
            >
              {t("equipment.equipped")}
            </span>
          )}
          {i.review.length > 0 && (
            <span
              className="flex shrink-0 items-center gap-1 text-label text-warning"
              data-testid="item-review"
            >
              <Warning aria-hidden className="size-3.5" />
              {t("equipment.review.tag")}
            </span>
          )}
        </span>
        <span
          className="min-w-0 break-words text-label text-muted-foreground"
          data-testid="item-stats"
        >
          {pieceStats(i, t).map((part, n) => (
            <span key={n}>
              {n > 0 && " · "}
              {part.behaviour ? (
                <span
                  title={t(`items.behaviourHint.${i.behaviour}`)}
                  data-testid="item-behaviour"
                >
                  {part.text}
                </span>
              ) : (
                part.text
              )}
            </span>
          ))}
        </span>
      </div>
      <div className="min-w-0">
        {i.effectDice && (
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
        )}
      </div>
      <div className="flex items-center justify-center gap-1">
        {counted && (
          <>
            <Button
              size="icon-sm"
              disabled={busy || i.count === 0}
              onClick={onUse}
              aria-label={t("items.useOne", { name })}
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
                aria-label={t("items.addOne", { name })}
              >
                <Plus />
              </Button>
            ) : (
              // Keeps the counts aligned: charges and daily items have no +.
              <span className="size-7" aria-hidden />
            )}
          </>
        )}
      </div>
      <div>
        {i.equipState && (
          <Button
            size="sm"
            disabled={busy}
            title={t(
              equipped(i) ? "equipment.unequipHint" : "equipment.equipHint",
              { name },
            )}
            aria-label={t(
              equipped(i) ? "equipment.unequipHint" : "equipment.equipHint",
              { name },
            )}
            onClick={onEquip}
            data-testid="item-equip"
          >
            {equipped(i) ? t("equipment.unequip") : t("equipment.equip")}
          </Button>
        )}
      </div>
      <div className={ROW_ACTIONS}>
        <Button
          size="icon-sm"
          variant="ghost"
          className="text-muted-foreground"
          disabled={busy}
          onClick={onEdit}
          aria-label={t("items.form.edit", { name })}
        >
          <PencilSimple />
        </Button>
        <Button
          size="icon-sm"
          variant="ghost"
          className="text-muted-foreground"
          disabled={busy}
          onClick={onRemove}
          aria-label={t("items.form.remove", { name })}
        >
          <Trash />
        </Button>
      </div>
    </li>
  );
}
