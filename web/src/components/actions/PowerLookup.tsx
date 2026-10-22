import {
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type UIEvent,
} from "react";
import { useTranslation } from "react-i18next";
import { CircleNotch, MagnifyingGlass } from "@phosphor-icons/react";
import { cn } from "@/lib/utils";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { SpellDescription } from "@/components/dashboard/SpellDescription";
import { getPower } from "@/api/powers";
import { localized } from "@/i18n/localized";
import { isPowerQuerySet, usePowerStore } from "@/stores/powers";
import type { Power, PowerCost, PowerFilterOptions } from "@/types/powers";

/** Load the next page when the list is this close (px) to its bottom. */
const LOAD_MORE_MARGIN = 400;

/** The lookup's powers view (ADR 0043 step 8): the catalogue's psionic powers beside the spells. */
export function PowerLookup({ modeSwitch }: { modeSwitch: React.ReactNode }) {
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
  } = usePowerStore();
  const listRef = useRef<HTMLDivElement>(null);
  const selected = items.find((p) => p.id === selectedId) ?? null;

  useEffect(() => {
    void loadOptions();
  }, [loadOptions]);

  useEffect(() => {
    const timer = setTimeout(() => void search(), 150);
    return () => clearTimeout(timer);
  }, [query, search]);

  useEffect(() => {
    if (!selectedId) return;
    document
      .getElementById(`power-row-${selectedId}`)
      ?.scrollIntoView({ block: "nearest" });
  }, [selectedId]);

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

  return (
    <div className="grid min-h-0 flex-1 grid-cols-[minmax(18rem,22rem)_minmax(0,1fr)] gap-3">
      <section className="stage-card flex min-h-0 min-w-0 flex-col gap-3 pb-3.5">
        <div
          role="group"
          aria-label={t("lookup.filters")}
          className="card-band flex shrink-0 flex-col gap-2 px-4 py-2.5"
        >
          <div className="relative min-w-0">
            <MagnifyingGlass className="pointer-events-none absolute top-1.5 left-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="search"
              className={cn(fieldClass, "pl-8")}
              placeholder={t("lookup.powers.search")}
              aria-label={t("lookup.powers.search")}
              role="combobox"
              aria-expanded
              aria-controls="power-list"
              aria-activedescendant={
                selectedId ? `power-row-${selectedId}` : undefined
              }
              autoComplete="off"
              value={query.q}
              onChange={(e) => setQuery({ q: e.target.value })}
              onKeyDown={onKeyDown}
            />
          </div>
          {modeSwitch}
          <ChipGroup
            label={t("lookup.powers.disciplineLabel")}
            value={query.discipline}
            onChange={(v) => setQuery({ discipline: v })}
            choices={disciplineChoices(options)}
            wording={(id) => disciplineName(options, id, i18n.language)}
          />
          <ChipGroup
            label={t("lookup.powers.kindLabel")}
            value={query.kind}
            onChange={(v) => setQuery({ kind: v })}
            choices={(options?.kinds ?? []).map((k) => k as string)}
            wording={(k) => t(`lookup.powers.kinds.${k}`)}
          />
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1.5 text-body text-muted-foreground">
            <span className="flex-1" />
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
                {t("lookup.powers.count", { count: total })}
              </span>
            )}
            {isPowerQuerySet(query) && (
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
          id="power-list"
          role="listbox"
          aria-label={t("lookup.powers.results")}
          className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-4"
          onScroll={onScroll}
        >
          {items.length === 0 && !loading && shownKey ? (
            <p className="p-2 text-body text-muted-foreground">
              {t("lookup.powers.noMatch")}
            </p>
          ) : (
            items.map((p) => (
              <PowerRow
                key={p.id}
                power={p}
                options={options}
                selected={p.id === selectedId}
                onSelect={() => select(p.id)}
              />
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
        aria-label={t("lookup.powers.detail")}
        data-testid="power-detail"
        className="zone-card flex min-h-0 min-w-0 flex-col overflow-y-auto px-4 pb-3.5"
      >
        {selected ? (
          <PowerDetail key={selected.id} power={selected} options={options} />
        ) : (
          <p className="pt-3.5 text-body text-muted-foreground">
            {t("lookup.powers.noSelection")}
          </p>
        )}
      </section>
    </div>
  );
}

function disciplineChoices(options: PowerFilterOptions | null): string[] {
  return (options?.disciplines ?? []).map((d) => d.id);
}

function disciplineName(
  options: PowerFilterOptions | null,
  id: string,
  lang: string,
): string {
  const d = options?.disciplines.find((x) => x.id === id);
  return d ? localized(lang, d.name) : id;
}

/** A single-choice set of chips (pressed = the filter is on; pressing it again clears it). */
function ChipGroup({
  label,
  value,
  onChange,
  choices,
  wording,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  choices: string[];
  wording: (id: string) => string;
}) {
  const { i18n } = useTranslation();
  return (
    <div role="group" aria-label={label} className="flex flex-wrap gap-1">
      {choices.map((id) => {
        const on = value === id;
        return (
          <button
            key={id}
            type="button"
            aria-pressed={on}
            lang={i18n.language}
            className={cn(
              "px-2 py-0.5 text-body font-medium transition-colors",
              on
                ? "chip-chosen"
                : "chip text-muted-foreground hover:text-foreground",
            )}
            onClick={() => onChange(on ? "" : id)}
          >
            {wording(id)}
          </button>
        );
      })}
    </div>
  );
}

function PowerRow({
  power,
  options,
  selected,
  onSelect,
}: {
  power: Power;
  options: PowerFilterOptions | null;
  selected: boolean;
  onSelect: () => void;
}) {
  const { t, i18n } = useTranslation();
  return (
    <div
      id={`power-row-${power.id}`}
      role="option"
      aria-selected={selected}
      data-testid="power-row"
      data-id={power.id}
      data-discipline={power.discipline}
      data-kind={power.kind}
      className={cn(
        "flex shrink-0 cursor-pointer flex-col rounded-md px-2 py-1.5 text-body",
        selected ? "selected" : "hover:bg-raised",
      )}
      onClick={onSelect}
    >
      <span
        lang={i18n.language}
        className="min-w-0 break-words"
        data-testid="power-name"
      >
        {localized(i18n.language, power.name)}
      </span>
      <span className="text-label text-muted-foreground">
        {disciplineName(options, power.discipline, i18n.language)} ·{" "}
        {t(`lookup.powers.kinds.${power.kind}`)}
      </span>
    </div>
  );
}

function PowerDetail({
  power,
  options,
}: {
  power: Power;
  options: PowerFilterOptions | null;
}) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const names = usePrerequisiteNames(power.prerequisites);
  const cost = (c: PowerCost) =>
    typeof c === "number"
      ? t("lookup.powers.psp", { count: c })
      : t(`lookup.powers.costWords.${c}`);
  const sign = (n: number) => (n < 0 ? `−${-n}` : `+${n}`);
  const tiles: { key: string; value: string }[] = [
    { key: "initialCost", value: cost(power.initial_cost) },
    {
      key: "maintenance",
      value: power.maintenance
        ? t("lookup.powers.maintenanceValue", {
            cost: cost(power.maintenance.cost),
            count: power.maintenance.period,
          })
        : t("lookup.powers.noMaintenance"),
    },
    {
      key: "score",
      value: power.score
        ? `${t(`attr.${power.score.ability}`)} ${sign(power.score.modifier)}`
        : t("lookup.powers.scoreInText"),
    },
    { key: "range", value: localized(lang, power.range) },
    { key: "preparationTime", value: localized(lang, power.preparation_time) },
    { key: "area", value: localized(lang, power.area) },
  ];
  const results: { key: string; text: string }[] = [
    {
      key: "powerScoreResult",
      text: localized(lang, power.power_score_result),
    },
    { key: "natural20Result", text: localized(lang, power.natural_20_result) },
  ].filter((r) => r.text);
  const description = localized(lang, power.description);
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      <div className="card-band bleed-x-4 flex shrink-0 flex-col gap-1 py-2.5">
        <h3 className="text-headline" lang={lang} data-testid="power-title">
          {localized(lang, power.name)}
        </h3>
        <p className="text-body text-muted-foreground" data-testid="power-meta">
          {[
            disciplineName(options, power.discipline, lang),
            t(`lookup.powers.kinds.${power.kind}`),
            power.min_level
              ? t("lookup.powers.minLevel", { level: power.min_level })
              : "",
          ]
            .filter(Boolean)
            .join(" · ")}
        </p>
      </div>
      <dl className="grid grid-cols-[repeat(auto-fill,minmax(7.5rem,1fr))] gap-2">
        {tiles.map((x) => (
          <div
            key={x.key}
            className="flex flex-col gap-0.5 rounded-lg bg-raised px-3 py-2"
            data-testid={`power-stat-${x.key}`}
          >
            <dt className="text-label text-muted-foreground">
              {t(`lookup.powers.${x.key}`)}
            </dt>
            <dd className="min-w-0 break-words text-body">{x.value}</dd>
          </div>
        ))}
      </dl>
      {power.prerequisites.length > 0 && (
        <p className="text-body" data-testid="power-prerequisites">
          <span className="text-muted-foreground">
            {t("lookup.powers.prerequisites")}{" "}
          </span>
          {power.prerequisites.map((id) => names[id] ?? id).join(", ")}
        </p>
      )}
      {description && (
        <section
          className="flex min-h-40 flex-1 flex-col gap-1.5"
          data-testid="power-description"
        >
          <p className="section-label shrink-0">{t("lookup.description")}</p>
          <div
            className="min-h-0 flex-1 overflow-y-auto pr-2 text-body leading-relaxed text-foreground/85"
            data-testid="power-description-text"
            tabIndex={0}
          >
            <SpellDescription text={description} />
            {results.map((r) => (
              <div key={r.key} className="mt-4" data-testid={`power-${r.key}`}>
                <p className="section-label">{t(`lookup.powers.${r.key}`)}</p>
                <SpellDescription text={r.text} />
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}

/** The names of the powers `ids`, from the list when loaded, else fetched once by id. */
function usePrerequisiteNames(ids: string[]): Record<string, string> {
  const { i18n } = useTranslation();
  const items = usePowerStore((s) => s.items);
  const [fetched, setFetched] = useState<Record<string, string>>({});
  const fromList = (id: string) => items.find((p) => p.id === id);
  const missing = ids.filter((id) => !fromList(id) && !(id in fetched));
  const key = missing.join(",");
  useEffect(() => {
    let live = true;
    for (const id of key ? key.split(",") : []) {
      getPower(id).then(
        (p) =>
          live &&
          setFetched((f) => ({ ...f, [id]: localized(i18n.language, p.name) })),
        () => undefined,
      );
    }
    return () => {
      live = false;
    };
  }, [key, i18n.language]);
  const out: Record<string, string> = { ...fetched };
  for (const id of ids) {
    const p = fromList(id);
    if (p) out[id] = localized(i18n.language, p.name);
  }
  return out;
}
