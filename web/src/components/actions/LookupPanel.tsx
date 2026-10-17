import { useEffect, useRef, type KeyboardEvent, type UIEvent } from "react";
import { useTranslation } from "react-i18next";
import { CircleNotch, MagnifyingGlass } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { SpellDetail, SpellName } from "@/components/dashboard/SpellDetail";
import { isQuerySet, useLookupStore } from "@/stores/lookup";
import type { SpellClass } from "@/api/spells_db";
import { localized } from "@/i18n/localized";
import { classLabel, sphereLabel, schoolLabel } from "@/lib/lookupLabels";

const CLASSES: { value: SpellClass; key: string }[] = [
  { value: "mine", key: "classMine" },
  { value: "wizard", key: "classWizard" },
  { value: "priest", key: "classPriest" },
  { value: "all", key: "classAll" },
];
const LEVELS = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
/** Load the next page when the list is this close (px) to its bottom. */
const LOAD_MORE_MARGIN = 400;

export function LookupPanel() {
  const { t, i18n } = useTranslation();
  const {
    query,
    items,
    total,
    loading,
    error,
    shownKey,
    selectedId,
    options,
    setQuery,
    reset,
    search,
    loadMore,
    select,
    move,
    loadOptions,
  } = useLookupStore();
  const listRef = useRef<HTMLDivElement>(null);
  const priest = query.cls === "priest";
  const selected = items.find((s) => s.id === selectedId) ?? null;

  useEffect(() => {
    void loadOptions();
  }, [loadOptions]);

  // Search as the query changes (debounced while typing); the store skips a query
  // whose rows are already shown, so coming back to the tab keeps the selection.
  useEffect(() => {
    const timer = setTimeout(() => void search(), 150);
    return () => clearTimeout(timer);
  }, [query, search]);

  // Keep the selected row visible (keyboard moves, coming back to the tab).
  useEffect(() => {
    if (!selectedId) return;
    document
      .getElementById(`lookup-row-${selectedId}`)
      ?.scrollIntoView({ block: "nearest" });
  }, [selectedId]);

  // A new query starts at the top of the list. Not on mount: coming back to the tab
  // keeps the selected row in view (the effect above).
  const mountedKey = useRef(shownKey);
  useEffect(() => {
    if (shownKey === mountedKey.current) return;
    mountedKey.current = shownKey;
    listRef.current?.scrollTo({ top: 0 });
  }, [shownKey]);

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      move(e.key === "ArrowDown" ? 1 : -1);
    }
  };

  const onScroll = (e: UIEvent<HTMLDivElement>) => {
    const el = e.currentTarget;
    if (el.scrollTop + el.clientHeight >= el.scrollHeight - LOAD_MORE_MARGIN) {
      void loadMore();
    }
  };

  const toggleLevel = (n: number) =>
    setQuery({
      levels: query.levels.includes(n)
        ? query.levels.filter((l) => l !== n)
        : [...query.levels, n].sort((a, b) => a - b),
    });

  const levelChips = (
    <div
      role="group"
      aria-label={t("lookup.levelsLabel")}
      className="flex flex-wrap gap-1"
    >
      {LEVELS.map((n) => {
        const on = query.levels.includes(n);
        return (
          <button
            key={n}
            type="button"
            aria-pressed={on}
            aria-label={t("lookup.levelLabel", { level: n })}
            className={cn(
              "h-7 w-7 text-body font-medium tabular-nums transition-colors",
              on
                ? "chip-chosen"
                : "chip text-muted-foreground hover:text-foreground",
            )}
            onClick={() => toggleLevel(n)}
          >
            {n}
          </button>
        );
      })}
    </div>
  );

  // ADR 0022 step 2.5 (A2's pane): filter and list in one card on the left, the detail on the
  // right; the cards follow step 4 (outlined, 14 / 16 px padding, no rules).
  return (
    <div className="grid min-h-0 flex-1 grid-cols-[minmax(18rem,22rem)_minmax(0,1fr)] gap-3">
      <section className="zone-card flex min-h-0 min-w-0 flex-col gap-3 py-3.5">
        <div
          role="group"
          aria-label={t("lookup.filters")}
          className="flex shrink-0 flex-col gap-2 px-4"
        >
          <div className="relative min-w-0">
            <MagnifyingGlass className="pointer-events-none absolute top-1.5 left-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="search"
              className={cn(fieldClass, "pl-8")}
              placeholder={t("lookup.search")}
              aria-label={t("lookup.search")}
              role="combobox"
              aria-expanded
              aria-controls="lookup-list"
              aria-activedescendant={
                selectedId ? `lookup-row-${selectedId}` : undefined
              }
              autoComplete="off"
              value={query.q}
              onChange={(e) => setQuery({ q: e.target.value })}
              onKeyDown={onKeyDown}
            />
          </div>
          <select
            className={fieldClass}
            aria-label={priest ? t("lookup.sphere") : t("lookup.school")}
            value={priest ? query.sphere : query.school}
            onChange={(e) =>
              setQuery(
                priest
                  ? { sphere: e.target.value }
                  : { school: e.target.value },
              )
            }
          >
            <option value="">
              {priest ? t("lookup.anySphere") : t("lookup.anySchool")}
            </option>
            {priest
              ? options?.spheres.map((s) => (
                  <option key={s} value={s}>
                    {sphereLabel(t, s)}
                  </option>
                ))
              : options?.schools.map((s) => (
                  <option key={s} value={s}>
                    {t(`lookup.schools.${s}`, { defaultValue: s })}
                  </option>
                ))}
          </select>
          <div
            role="group"
            aria-label={t("lookup.classLabel")}
            className="flex flex-wrap self-start rounded-lg bg-raised p-0.5"
          >
            {CLASSES.map(({ value, key }) => (
              <button
                key={value}
                type="button"
                aria-pressed={query.cls === value}
                className={cn(
                  "rounded-md px-2.5 py-1 text-body font-medium transition-colors",
                  query.cls === value
                    ? "chip-chosen"
                    : "text-muted-foreground hover:text-foreground",
                )}
                onClick={() => setQuery({ cls: value })}
              >
                {t(`lookup.${key}`)}
              </button>
            ))}
          </div>
          {levelChips}
          <div className="flex flex-wrap items-center justify-end gap-x-2 text-body text-muted-foreground">
            {loading && (
              <CircleNotch className="h-3 w-3 animate-spin" aria-hidden />
            )}
            {error && (
              <span role="alert" className="text-destructive">
                {error}
              </span>
            )}
            {shownKey && (
              <span className="tabular-nums" data-testid="lookup-count">
                {t("lookup.count", { count: total })}
              </span>
            )}
            {isQuerySet(query) && (
              <button
                type="button"
                className="text-tint hover:underline"
                onClick={reset}
              >
                {t("lookup.reset")}
              </button>
            )}
          </div>
        </div>

        <div
          ref={listRef}
          id="lookup-list"
          role="listbox"
          aria-label={t("lookup.results")}
          className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-4"
          onScroll={onScroll}
        >
          {items.length === 0 && !loading && shownKey ? (
            <p className="p-2 text-body text-muted-foreground">
              {t("lookup.noMatch")}
            </p>
          ) : (
            items.map((s) => (
              <div
                key={s.id}
                id={`lookup-row-${s.id}`}
                role="option"
                aria-selected={s.id === selectedId}
                title={schoolLabel(t, localized(i18n.language, s.school))}
                data-testid="spell-row"
                data-id={s.id}
                data-class={s.class}
                data-level={s.level}
                className={cn(
                  "grid shrink-0 cursor-pointer grid-cols-[1.25rem_minmax(0,1fr)] items-baseline gap-x-2 rounded-md px-2 py-1.5 text-body",
                  s.id === selectedId ? "selected" : "hover:bg-raised",
                )}
                onClick={() => select(s.id)}
              >
                <span className="text-body font-medium text-magic tabular-nums">
                  {s.level}
                </span>
                <SpellName spell={s} />
                {query.cls === "all" && (
                  <span
                    className="col-start-2 text-label text-muted-foreground"
                    data-testid="spell-class"
                  >
                    {classLabel(t, s.class)}
                  </span>
                )}
              </div>
            ))
          )}
          {items.length > 0 && items.length < total && (
            <p className="p-2 text-center text-label text-muted-foreground">
              {t("lookup.loadingMore")}
            </p>
          )}
        </div>
      </section>

      <section
        aria-label={t("lookup.detail")}
        data-testid="spell-detail"
        className="zone-card flex min-h-0 min-w-0 flex-col overflow-y-auto px-4 py-3.5"
      >
        {selected ? (
          <SpellDetail key={selected.id} spell={selected} />
        ) : (
          <p className="text-body text-muted-foreground">
            {t("lookup.noSelection")}
          </p>
        )}
      </section>
    </div>
  );
}
