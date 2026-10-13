import { useEffect, useRef, type KeyboardEvent, type UIEvent } from "react";
import { useTranslation } from "react-i18next";
import { Search, Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { fieldClass, captionClass } from "@/components/dashboard/fieldStyles";
import { isQuerySet, useLookupStore } from "@/stores/lookup";
import type { SpellClass } from "@/api/spells_db";
import { otherSpellName, spellName } from "@/i18n/spellName";
import { statRaw, statValue, type StatKey } from "@/lib/spellValues";
import type { Spell } from "@/types/spells";

const CLASSES: { value: SpellClass; key: string }[] = [
  { value: "mine", key: "classMine" },
  { value: "wizard", key: "classWizard" },
  { value: "priest", key: "classPriest" },
  { value: "all", key: "classAll" },
];
const LEVELS = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
const STATS: StatKey[] = ["range", "duration", "area", "castingTime", "save"];

/** "1 rd" and "1 rd." say the same: the raw text is not repeated after its value. */
function same(a: string, b: string): boolean {
  const norm = (s: string) => s.toLowerCase().replace(/[^a-z0-9½/-]/g, "");
  return norm(a) === norm(b);
}

/** Load the next page when the list is this close (px) to its bottom. */
const LOAD_MORE_MARGIN = 400;

/** "Mains brûlantes" + grey "· Burning Hands" (the other language), when it exists. */
function SpellNames({ spell, className }: { spell: Spell; className?: string }) {
  const { t, i18n } = useTranslation();
  const other = otherSpellName(t, i18n.language, spell.id, spell.name);
  return (
    <span className={cn("min-w-0 truncate", className)}>
      <span data-testid="spell-name">{spellName(t, spell.id, spell.name)}</span>
      {other && (
        <span className="text-muted-foreground font-normal" data-testid="spell-other-name">
          {" · "}
          {other}
        </span>
      )}
    </span>
  );
}

function SpellDetail({ spell }: { spell: Spell }) {
  const { t } = useTranslation();
  const unit = (w: string) => t(`lookup.units.${w}`, { defaultValue: w });
  const value = (key: StatKey): string | null =>
    key === "save"
      ? spell.stats.save.kinds.length
        ? spell.stats.save.kinds.map((k) => t(`lookup.saveKinds.${k}`)).join(" / ")
        : null
      : statValue(spell, key, unit);
  const meta = [
    spell.class,
    t("lookup.levelLabel", { level: spell.level }),
    spell.school,
    spell.sphere,
    spell.components,
  ].filter(Boolean);

  return (
    <div className="space-y-3">
      <div>
        <h3 className="text-base font-semibold leading-tight">
          <SpellNames spell={spell} className="whitespace-normal" />
        </h3>
        <p className="text-xs text-muted-foreground" data-testid="spell-meta">
          {meta.join(" · ")}
        </p>
      </div>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
        {STATS.map((key) => {
          const v = value(key);
          const raw = statRaw(spell, key);
          return (
            <div key={key} className="min-w-0" data-testid={`stat-${key}`}>
              <dt className={captionClass}>{t(`lookup.${key}`)}</dt>
              <dd className="min-w-0">
                {v ? (
                  <>
                    <span className="font-medium" data-testid="stat-value">
                      {v}
                    </span>
                    {raw && !same(v, raw) && (
                      <span className="text-muted-foreground" data-testid="stat-raw">
                        {" · "}
                        {raw}
                      </span>
                    )}
                  </>
                ) : (
                  <span data-testid="stat-raw">{raw || "—"}</span>
                )}
              </dd>
            </div>
          );
        })}
      </dl>
      {spell.caster_level ? (
        <p className="text-[11px] text-muted-foreground">
          {t("lookup.atLevel", { level: spell.caster_level })}
        </p>
      ) : null}
      {spell.description && (
        <details className="text-sm" data-testid="spell-description">
          <summary className={cn(captionClass, "cursor-pointer select-none")}>
            {t("lookup.description")}
          </summary>
          <p className="mt-1 whitespace-pre-line leading-relaxed">
            {spell.description}
          </p>
        </details>
      )}
    </div>
  );
}

export function LookupPanel() {
  const { t } = useTranslation();
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

  return (
    <div className="flex h-full min-h-0 flex-col gap-2">
      <div className="relative shrink-0">
        <Search className="pointer-events-none absolute left-2.5 top-2 h-4 w-4 text-muted-foreground" />
        <input
          type="search"
          className={cn(fieldClass, "pl-8")}
          placeholder={t("lookup.search")}
          aria-label={t("lookup.search")}
          role="combobox"
          aria-expanded
          aria-controls="lookup-list"
          aria-activedescendant={selectedId ? `lookup-row-${selectedId}` : undefined}
          autoComplete="off"
          value={query.q}
          onChange={(e) => setQuery({ q: e.target.value })}
          onKeyDown={onKeyDown}
        />
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <div
          role="group"
          aria-label={t("lookup.classLabel")}
          className="inline-flex rounded-lg border p-0.5"
        >
          {CLASSES.map(({ value, key }) => (
            <button
              key={value}
              type="button"
              aria-pressed={query.cls === value}
              className={cn(
                "rounded-md px-3 py-1 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground",
                query.cls === value && "bg-primary text-primary-foreground hover:text-primary-foreground",
              )}
              onClick={() => setQuery({ cls: value })}
            >
              {t(`lookup.${key}`)}
            </button>
          ))}
        </div>
        {isQuerySet(query) && (
          <button
            type="button"
            className="ml-auto text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
            onClick={reset}
          >
            {t("lookup.reset")}
          </button>
        )}
      </div>

      <div className="flex shrink-0 items-center gap-2">
        <div role="group" aria-label={t("lookup.levelsLabel")} className="flex gap-1">
          {LEVELS.map((n) => {
            const on = query.levels.includes(n);
            return (
              <button
                key={n}
                type="button"
                aria-pressed={on}
                aria-label={t("lookup.levelLabel", { level: n })}
                className={cn(
                  "h-7 w-8 rounded-md border text-xs font-medium text-muted-foreground transition-colors hover:text-foreground",
                  on && "border-primary bg-primary text-primary-foreground hover:text-primary-foreground",
                )}
                onClick={() => toggleLevel(n)}
              >
                {t("lookup.levelTag", { level: n })}
              </button>
            );
          })}
        </div>
        <select
          className={cn(fieldClass, "ml-auto h-7 w-44 text-xs")}
          aria-label={priest ? t("lookup.sphere") : t("lookup.school")}
          value={priest ? query.sphere : query.school}
          onChange={(e) =>
            setQuery(priest ? { sphere: e.target.value } : { school: e.target.value })
          }
        >
          <option value="">{priest ? t("lookup.anySphere") : t("lookup.anySchool")}</option>
          {priest
            ? options?.spheres.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))
            : options?.schools.map((s) => (
                <option key={s} value={s}>
                  {t(`lookup.schools.${s}`, { defaultValue: s })}
                </option>
              ))}
        </select>
      </div>

      <div className="flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
        {shownKey && (
          <span data-testid="lookup-count">{t("lookup.count", { count: total })}</span>
        )}
        {loading && <Loader2 className="h-3 w-3 animate-spin" aria-hidden />}
        {error && (
          <span role="alert" className="text-destructive">
            {error}
          </span>
        )}
      </div>

      <div
        ref={listRef}
        id="lookup-list"
        role="listbox"
        aria-label={t("lookup.results")}
        className="h-[45%] shrink-0 overflow-y-auto rounded-lg border"
        onScroll={onScroll}
      >
        {items.length === 0 && !loading && shownKey ? (
          <p className="p-3 text-sm italic text-muted-foreground">{t("lookup.noMatch")}</p>
        ) : (
          items.map((s) => (
            <div
              key={s.id}
              id={`lookup-row-${s.id}`}
              role="option"
              aria-selected={s.id === selectedId}
              data-testid="spell-row"
              data-id={s.id}
              data-class={s.class}
              data-level={s.level}
              className={cn(
                "flex cursor-pointer items-center gap-2 border-b px-2 py-1 text-sm last:border-b-0 hover:bg-muted/60",
                s.id === selectedId && "bg-muted",
              )}
              onClick={() => select(s.id)}
            >
              <span className="w-6 shrink-0 text-xs font-semibold text-muted-foreground tabular-nums">
                {t("lookup.levelTag", { level: s.level })}
              </span>
              <SpellNames spell={s} className="flex-1" />
              {query.cls === "all" && (
                <Badge variant="secondary" className="h-4 text-[10px]" data-testid="spell-class">
                  {s.class}
                </Badge>
              )}
              <span className="ml-auto max-w-[35%] shrink-0 truncate text-[11px] text-muted-foreground">
                {s.school}
              </span>
            </div>
          ))
        )}
        {items.length > 0 && items.length < total && (
          <p className="p-2 text-center text-xs text-muted-foreground">{t("lookup.loadingMore")}</p>
        )}
      </div>

      <section
        aria-label={t("lookup.detail")}
        data-testid="spell-detail"
        className="min-h-0 flex-1 overflow-y-auto rounded-lg border p-3"
      >
        {selected ? (
          <SpellDetail key={selected.id} spell={selected} />
        ) : (
          <p className="text-sm italic text-muted-foreground">{t("lookup.noSelection")}</p>
        )}
      </section>
    </div>
  );
}
