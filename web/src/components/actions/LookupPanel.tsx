import { useEffect, useRef, type KeyboardEvent, type UIEvent } from "react";
import { useTranslation } from "react-i18next";
import {
  ArrowRight,
  CircleNotch,
  DiceFive,
  MagnifyingGlass,
} from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { useDiceStore } from "@/stores/dice";
import { isQuerySet, useLookupStore } from "@/stores/lookup";
import type { SpellClass } from "@/api/spells_db";
import { spellName, spellTexts } from "@/i18n/spellName";
import { SpellDescription } from "@/components/dashboard/SpellDescription";
import { statDice, statValue, type StatKey } from "@/lib/spellValues";
import { statLocale, statText, type StatField } from "@/lib/statText";
import { damageFormula, damageText } from "@/lib/spellDice";
import { damageRows, exposureRows } from "@/lib/spellMode";
import { exposureDamageText } from "@/lib/spellDice";
import {
  classLabel,
  sphereLabel,
  schoolLabel,
  spheresLabel,
} from "@/lib/lookupLabels";
import type { Spell } from "@/types/spells";

const CLASSES: { value: SpellClass; key: string }[] = [
  { value: "mine", key: "classMine" },
  { value: "wizard", key: "classWizard" },
  { value: "priest", key: "classPriest" },
  { value: "all", key: "classAll" },
];
const LEVELS = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];
const STATS: StatKey[] = ["range", "duration", "area", "castingTime", "save"];

/** The catalogue field of a stat of the detail grid. */
const FIELD: Record<StatKey, StatField> = {
  range: "range",
  duration: "duration",
  area: "area_of_effect",
  castingTime: "casting_time",
  save: "saving_throw",
};

/** "1 rd" and "1 rd." say the same: the raw text is not repeated after its value. */
function same(a: string, b: string): boolean {
  const norm = (s: string) => s.toLowerCase().replace(/[^a-z0-9½/-]/g, "");
  return norm(a) === norm(b);
}

/** Load the next page when the list is this close (px) to its bottom. */
const LOAD_MORE_MARGIN = 400;

/** The spell's name in the UI language; it wraps onto more lines, never cut. */
function SpellName({ spell, className }: { spell: Spell; className?: string }) {
  const { i18n } = useTranslation();
  return (
    <span
      lang={i18n.language}
      className={cn("min-w-0 break-words hyphens-auto", className)}
      data-testid="spell-name"
    >
      {spellName(i18n.language, spell)}
    </span>
  );
}

function SpellDetail({ spell }: { spell: Spell }) {
  const { t, i18n } = useTranslation();
  const { description } = spellTexts(i18n.language, spell);
  const toComposer = useDiceStore((s) => s.setFormula);
  const roll = useDiceStore((s) => s.roll);
  const rolling = useDiceStore((s) => s.rolling);
  const loc = statLocale(t, i18n.language);
  const value = (key: StatKey): string | null =>
    key === "save"
      ? spell.stats.save.kinds.length
        ? spell.stats.save.kinds
            .map((k) => t(`lookup.saveKinds.${k}`))
            .join(" / ")
        : null
      : statValue(spell, key, loc);
  const meta = [
    classLabel(t, spell.class),
    t("lookup.levelLabel", { level: spell.level }),
    schoolLabel(t, spell.school),
    spheresLabel(t, spell.sphere),
    statText(loc, spell, "components").text,
  ].filter(Boolean);
  // The dice row: the damage a cast rolls (one instance to the composer), one row per cast-time
  // mode with damage for a spell with modes, then the stats with dice.
  const damages = damageRows(t, spell);
  const dice: { key: string; label: string; text: string; formula: string }[] =
    [
      ...damages.map((d) => ({
        key: d.key,
        label: d.label,
        text: damageText(t, d.damage),
        formula: damageFormula(d.damage),
      })),
      // One round of exposure to the spell's area (ADR 0008 step 16), per mode when it has modes.
      ...exposureRows(t, spell).map((d) => ({
        key: d.key,
        label: d.label,
        text: exposureDamageText(t, d.damage),
        formula: damageFormula(d.damage),
      })),
      ...statDice(spell, loc).map((d) => ({
        key: d.key,
        label: t(`lookup.${d.key}`),
        text: d.formula,
        formula: d.formula,
      })),
    ];

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="flex flex-col gap-1">
        <h3 className="text-title">
          <SpellName spell={spell} />
        </h3>
        <p
          className="text-label text-muted-foreground"
          data-testid="spell-meta"
        >
          {meta.join(" · ")}
        </p>
      </div>
      <dl className="flex flex-col text-body">
        {STATS.map((key) => {
          const v = value(key);
          const raw = statText(loc, spell, FIELD[key]).text;
          return (
            <div
              key={key}
              className="grid min-w-0 grid-cols-[7rem_minmax(0,1fr)] gap-x-3 rounded-md px-2 py-1.5 even:bg-neu-900/40"
              data-testid={`stat-${key}`}
            >
              <dt className="text-muted-foreground">{t(`lookup.${key}`)}</dt>
              <dd className="flex min-w-0 flex-col">
                {v ? (
                  <>
                    <span className="font-medium" data-testid="stat-value">
                      {v}
                    </span>
                    {raw && !same(v, raw) && (
                      <span
                        className="text-label text-muted-foreground"
                        data-testid="stat-raw"
                      >
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
        dice.length > 0 ? (
          <ul className="flex flex-col gap-2" aria-label={t("lookup.diceRow")}>
            {dice.map((d) => (
              <li
                key={d.key}
                className="flex flex-wrap items-center gap-x-2 gap-y-2 rounded-lg bg-neu-900/60 px-3 py-2 text-body"
                data-testid={`spell-dice-${d.key}`}
              >
                <span className="min-w-36 flex-1">
                  {t("lookup.atLevel", { level: spell.caster_level })}
                  {" · "}
                  {d.label}{" "}
                  <span
                    className="font-medium text-magic tabular-nums"
                    data-testid="spell-dice-formula"
                  >
                    {d.text}
                  </span>
                </span>
                <span className="flex shrink-0 gap-2">
                  <Button
                    size="sm"
                    variant="secondary"
                    aria-label={t("dice.send", { formula: d.formula })}
                    onClick={() => toComposer(d.formula)}
                  >
                    <ArrowRight aria-hidden /> {t("lookup.toDice")}
                  </Button>
                  <Button
                    size="sm"
                    disabled={rolling}
                    aria-label={t("dice.rollFormula", { formula: d.formula })}
                    onClick={() => void roll(d.formula)}
                  >
                    <DiceFive aria-hidden /> {t("lookup.rollDice")}
                  </Button>
                </span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-label text-muted-foreground">
            {t("lookup.atLevel", { level: spell.caster_level })}
          </p>
        )
      ) : null}
      {/* The description takes the panel's width and the height left below the stats, and
          scrolls inside it; a block container, so the text can hold any flow content. When the
          panel is too short for its floor, the whole detail scrolls instead. */}
      {description && (
        <section
          className="flex min-h-40 flex-1 flex-col text-body"
          data-testid="spell-description"
        >
          <p className="shrink-0 py-1 font-medium">{t("lookup.description")}</p>
          <div
            className="mt-1 min-h-0 flex-1 overflow-y-auto pr-2 leading-relaxed text-foreground/85"
            data-testid="spell-description-text"
            tabIndex={0}
          >
            <SpellDescription text={description} />
          </div>
        </section>
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

  const levelChips = (
    <div
      role="group"
      aria-label={t("lookup.levelsLabel")}
      className="flex gap-1"
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
              "h-7 w-7 rounded-md bg-neu-900/60 text-label font-medium text-muted-foreground tabular-nums transition-colors hover:text-foreground",
              on && "bg-amb-800 text-amb-100 hover:text-amb-100",
            )}
            onClick={() => toggleLevel(n)}
          >
            {n}
          </button>
        );
      })}
    </div>
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <section
        aria-label={t("lookup.filters")}
        className="flex shrink-0 flex-col gap-2 rounded-xl bg-card px-3 py-3"
      >
        <div className="flex min-w-0 gap-2">
          <div className="relative min-w-0 flex-1">
            <MagnifyingGlass className="pointer-events-none absolute top-2 left-2.5 h-4 w-4 text-muted-foreground" />
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
            className={cn(fieldClass, "w-[clamp(9rem,30%,14rem)] shrink-0")}
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
        </div>

        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <div
            role="group"
            aria-label={t("lookup.classLabel")}
            className="inline-flex rounded-lg bg-neu-900/60 p-0.5"
          >
            {CLASSES.map(({ value, key }) => (
              <button
                key={value}
                type="button"
                aria-pressed={query.cls === value}
                className={cn(
                  "rounded-md px-2.5 py-1 text-label font-medium text-muted-foreground transition-colors hover:text-foreground",
                  query.cls === value &&
                    "bg-card text-foreground ring-1 ring-amb-600 ring-inset",
                )}
                onClick={() => setQuery({ cls: value })}
              >
                {t(`lookup.${key}`)}
              </button>
            ))}
          </div>
          {levelChips}
          <div className="ml-auto flex items-center gap-2 text-label text-muted-foreground">
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
                className="underline underline-offset-2 hover:text-foreground"
                onClick={reset}
              >
                {t("lookup.reset")}
              </button>
            )}
          </div>
        </div>
      </section>

      <div className="grid min-h-0 flex-1 grid-cols-[clamp(13.75rem,30%,26rem)_minmax(0,1fr)] gap-3">
        <div
          ref={listRef}
          id="lookup-list"
          role="listbox"
          aria-label={t("lookup.results")}
          className="flex min-h-0 flex-col gap-px overflow-y-auto rounded-xl bg-card p-1.5"
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
                title={schoolLabel(t, s.school)}
                data-testid="spell-row"
                data-id={s.id}
                data-class={s.class}
                data-level={s.level}
                className={cn(
                  "grid shrink-0 cursor-pointer grid-cols-[1.25rem_minmax(0,1fr)] items-baseline gap-2 rounded-md px-2 py-1.5 text-body hover:bg-neu-900/50",
                  s.id === selectedId &&
                    "bg-amb-900 ring-1 ring-amb-700 ring-inset hover:bg-amb-900",
                )}
                onClick={() => select(s.id)}
              >
                <span className="text-label text-magic tabular-nums">
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

        <section
          aria-label={t("lookup.detail")}
          data-testid="spell-detail"
          className="flex min-h-0 flex-col overflow-y-auto rounded-xl bg-card px-3 py-3"
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
    </div>
  );
}
