import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { MagnifyingGlass, Minus, Plus, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { localized } from "@/i18n/localized";
import { cn } from "@/lib/utils";
import { formatSigned } from "@/lib/format";
import {
  COLUMNS,
  canEquip,
  catalogueRows,
  minMagic,
  pickBody,
  type CatalogueRow,
  type CopyDraft,
} from "@/lib/catalogue";
import { emptyDraft } from "@/lib/itemForm";
import { postCreateItem } from "@/api/items";
import type { ApiError } from "@/api/http";
import { useEquipmentCatalogueStore } from "@/stores/equipmentCatalogue";
import { useTableStore } from "@/stores/table";
import {
  PICKED_CATEGORIES,
  type Category,
  type ItemBody,
  type PickedCategory,
} from "@/types/items";
import { ItemForm } from "./ItemForm";

type Tab = "catalogue" | "typed";

/** The magic a copy may be given here, as the editor accepts it (itemsvc MaxBonus). */
const MAX_MAGIC = 10;

/** The list's grid per number of cells after the name (static class names for Tailwind). */
const GRID: Record<number, string> = {
  2: "grid-cols-[minmax(10rem,1fr)_repeat(2,auto)]",
  3: "grid-cols-[minmax(10rem,1fr)_repeat(3,auto)]",
};

function isPicked(c: Category | "all"): c is PickedCategory {
  return (PICKED_CATEGORIES as readonly string[]).includes(c);
}

/**
 * Ajouter (ADR 0033 step 24, ADR 0035 item 10): opens in the stage in place of the possessions,
 * with the tabs Catalogue and Saisir. Catalogue: the category, a search over the names in the
 * player's language, the PHB rows, and "Votre exemplaire" (name, "+N", count for ammunition, "Equip
 * it now"); the server copies the row (one logged add, equipped in the same undo). Saisir: the
 * homebrew form, every field typed. `filter` is the inventory's chip: it chooses the category, and
 * a magic or gear chip (typed only) opens Saisir.
 */
export function AddPiece({
  filter,
  specializes,
  busy,
  onClose,
}: {
  filter: Category | "all";
  specializes: boolean;
  busy: boolean;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const titleId = useId();
  const submit = useTableStore((s) => s.submit);
  const [tab, setTab] = useState<Tab>(
    filter === "all" || isPicked(filter) ? "catalogue" : "typed",
  );
  const save = async (body: ItemBody) => {
    const rejected = await submit(() => postCreateItem(body));
    if (!rejected) onClose();
    return rejected;
  };

  return (
    <section
      aria-labelledby={titleId}
      className="stage-card flex min-h-0 min-w-0 flex-1 flex-col pb-3"
      data-testid="add-piece"
    >
      <div className="card-band mb-1.5 flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2 px-3 py-1.5">
        <h2 id={titleId} className="section-label">
          {t("equipment.add.title")}
        </h2>
        <div
          role="tablist"
          aria-label={t("equipment.add.title")}
          className="flex gap-1 rounded-lg bg-raised p-1"
        >
          {(["catalogue", "typed"] as const).map((id) => (
            <button
              key={id}
              type="button"
              role="tab"
              aria-selected={tab === id}
              title={t(`equipment.add.tabHint.${id}`)}
              className={cn(
                "rounded-md px-2 py-0.5 text-body outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                tab === id
                  ? "chip-chosen font-medium"
                  : "text-muted-foreground hover:text-foreground",
              )}
              onClick={() => setTab(id)}
            >
              {t(`equipment.add.tab.${id}`)}
            </button>
          ))}
        </div>
        <span className="flex-1" />
        <Button
          size="sm"
          title={t("equipment.add.closeHint")}
          aria-label={t("equipment.add.closeHint")}
          onClick={onClose}
        >
          <X /> {t("equipment.add.close")}
        </Button>
      </div>
      {tab === "catalogue" ? (
        <CataloguePick
          initial={isPicked(filter) ? filter : "weapon"}
          busy={busy}
          onSave={save}
        />
      ) : (
        <div className="min-h-0 overflow-y-auto px-3">
          <ItemForm
            initial={emptyDraft(filter === "all" ? "gear" : filter)}
            specializes={specializes}
            busy={busy}
            onCancel={onClose}
            onSave={save}
          />
        </div>
      )}
    </section>
  );
}

/** Ajouter > Catalogue: the rows of a category, searchable, and the player's copy of the one chosen. */
function CataloguePick({
  initial,
  busy,
  onSave,
}: {
  initial: PickedCategory;
  busy: boolean;
  onSave: (body: ItemBody) => Promise<ApiError | null>;
}) {
  const { t, i18n } = useTranslation();
  const catalogue = useEquipmentCatalogueStore((s) => s.catalogue);
  const load = useEquipmentCatalogueStore((s) => s.load);
  const fail = useTableStore((s) => s.fail);
  const [category, setCategory] = useState<PickedCategory>(initial);
  const [query, setQuery] = useState("");
  const [chosen, setChosen] = useState<CatalogueRow | null>(null);
  useEffect(() => {
    load().catch(fail);
  }, [load, fail]);

  const rows = catalogue
    ? catalogueRows(catalogue, category, query, i18n.language, t)
    : [];
  const columns = COLUMNS[category];
  const empty =
    catalogue !== null &&
    catalogue.weapons.length +
      catalogue.ammunition.length +
      catalogue.armour.length +
      catalogue.shields.length ===
      0;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 px-3 min-[1800px]:flex-row">
      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <div
            role="group"
            aria-label={t("equipment.add.category")}
            className="flex flex-wrap gap-1"
            data-testid="add-categories"
          >
            {PICKED_CATEGORIES.map((c) => (
              <button
                key={c}
                type="button"
                className={cn(
                  "h-7 px-2 text-body font-medium outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50",
                  category === c
                    ? "chip-chosen"
                    : "chip text-muted-foreground hover:text-foreground",
                )}
                aria-pressed={category === c}
                onClick={() => {
                  setCategory(c);
                  setChosen(null);
                }}
              >
                {t(`equipment.categories.${c}`)}
              </button>
            ))}
          </div>
          <label className="relative flex w-56 items-center">
            <span className="sr-only">{t("equipment.add.searchLabel")}</span>
            <MagnifyingGlass
              aria-hidden
              className="pointer-events-none absolute left-2 size-4 text-muted-foreground"
            />
            <input
              type="search"
              className={cn(fieldClass, "w-full pl-7")}
              placeholder={t("equipment.add.search")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              data-testid="add-search"
            />
          </label>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {empty ? (
            <p className="text-body text-muted-foreground">
              {t("equipment.add.noCatalogue")}
            </p>
          ) : catalogue && rows.length === 0 ? (
            <p className="text-body text-muted-foreground">
              {t("equipment.add.noMatch", { query: query.trim() })}
            </p>
          ) : (
            <div
              className={cn("grid gap-x-4 gap-y-1", GRID[columns.length])}
              role="group"
              aria-label={t(`equipment.categories.${category}`)}
              data-testid="add-rows"
            >
              <div className="col-span-full grid grid-cols-subgrid px-3 text-label font-medium text-muted-foreground">
                <span>{t("equipment.add.columns.name")}</span>
                {columns.map((c) => (
                  <span key={c}>{t(`equipment.add.columns.${c}`)}</span>
                ))}
              </div>
              {rows.map((r) => (
                <button
                  key={r.code}
                  type="button"
                  aria-pressed={chosen?.code === r.code}
                  className={cn(
                    "col-span-full grid grid-cols-subgrid items-center rounded-md px-3 py-1 text-left text-body outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                    chosen?.code === r.code
                      ? "chip-chosen"
                      : "bg-raised hover:text-tint",
                  )}
                  data-testid={`add-row-${r.code}`}
                  onClick={() => setChosen(r)}
                >
                  <span className="min-w-0 break-words font-medium">
                    {localized(i18n.language, r.name)}
                  </span>
                  {r.cells.map((cell, n) => (
                    <span key={n} className="tabular-nums">
                      {cell}
                    </span>
                  ))}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
      <YourCopy
        key={chosen ? `${category}:${chosen.code}` : category}
        row={chosen}
        category={category}
        busy={busy}
        onSave={onSave}
      />
    </div>
  );
}

/** "Votre exemplaire": the player's copy of the row chosen, then Add. */
function YourCopy({
  row,
  category,
  busy,
  onSave,
}: {
  row: CatalogueRow | null;
  category: PickedCategory;
  busy: boolean;
  onSave: (body: ItemBody) => Promise<ApiError | null>;
}) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const [copy, setCopy] = useState<CopyDraft>(() => ({
    name: row ? localized(i18n.language, row.name) : "",
    magic: 0,
    count: "1",
    equip: true,
  }));
  const [error, setError] = useState<string | null>(null);
  const set = (patch: Partial<CopyDraft>) => {
    setCopy((c) => ({ ...c, ...patch }));
    setError(null);
  };
  const columns = COLUMNS[category];

  async function add() {
    if (!row) return;
    const body = pickBody(row, category, copy, i18n.language);
    if (!body) {
      setError(t("equipment.add.countInvalid"));
      return;
    }
    const rejected = await onSave(body);
    if (rejected?.field)
      setError(
        t(`equipment.form.errors.${rejected.message}`, {
          defaultValue: rejected.message,
        }),
      );
  }

  return (
    <div
      className="flex shrink-0 flex-col gap-3 rounded-lg bg-raised p-4 min-[1800px]:w-[26rem]"
      data-testid="add-copy"
    >
      <span className="section-label">{t("equipment.add.yourCopy")}</span>
      {!row ? (
        <p className="text-body text-muted-foreground">
          {t("equipment.add.pickFirst")}
        </p>
      ) : (
        <>
          <div className="flex flex-wrap items-end gap-3">
            <label className="flex min-w-[14rem] flex-1 flex-col gap-1">
              <span className="text-label text-muted-foreground">
                {t("equipment.add.name")}
              </span>
              <input
                className={fieldClass}
                value={copy.name}
                onChange={(e) => set({ name: e.target.value })}
                data-testid="add-name"
              />
            </label>
            <div
              role="group"
              aria-labelledby={`${id}-magic`}
              className="flex flex-col gap-1"
            >
              <span
                id={`${id}-magic`}
                className="text-label text-muted-foreground"
              >
                {t("equipment.add.magic")}
              </span>
              <span className="flex items-center gap-1">
                <Button
                  size="icon-sm"
                  disabled={copy.magic <= minMagic(category)}
                  aria-label={t("equipment.add.magicLess")}
                  onClick={() => set({ magic: copy.magic - 1 })}
                >
                  <Minus />
                </Button>
                <span
                  className="min-w-10 text-center font-semibold tabular-nums"
                  data-testid="add-magic"
                >
                  {formatSigned(copy.magic)}
                </span>
                <Button
                  size="icon-sm"
                  disabled={copy.magic >= MAX_MAGIC}
                  aria-label={t("equipment.add.magicMore")}
                  onClick={() => set({ magic: copy.magic + 1 })}
                >
                  <Plus />
                </Button>
              </span>
            </div>

            {category === "ammunition" && (
              <label className="flex w-20 flex-col gap-1">
                <span className="text-label text-muted-foreground">
                  {t("equipment.add.count")}
                </span>
                <input
                  className={fieldClass}
                  inputMode="numeric"
                  value={copy.count}
                  onChange={(e) => set({ count: e.target.value })}
                  data-testid="add-count"
                />
              </label>
            )}
          </div>
          <p
            className="break-words text-body text-muted-foreground"
            data-testid="add-summary"
          >
            {row.cells
              .map((cell, n) =>
                t("equipment.add.cell", {
                  label: t(`equipment.add.columns.${columns[n]}`),
                  value: cell,
                }),
              )
              .join(" · ")}
            . {t("equipment.add.editLater")}
          </p>
          {error && (
            <p className="text-body text-destructive" role="alert">
              {error}
            </p>
          )}
          <div className="flex flex-wrap items-center justify-end gap-3">
            {canEquip(category) && (
              <label className="flex items-center gap-2 text-body">
                <input
                  type="checkbox"
                  className="size-4"
                  checked={copy.equip}
                  onChange={(e) => set({ equip: e.target.checked })}
                  data-testid="add-equip"
                />
                {t("equipment.add.equipNow")}
              </label>
            )}
            <Button
              variant="main"
              disabled={busy}
              title={t("equipment.add.addHint", { name: copy.name })}
              aria-label={t("equipment.add.addHint", { name: copy.name })}
              onClick={() => void add()}
              data-testid="add-submit"
            >
              <Plus /> {t("equipment.add.add")}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
