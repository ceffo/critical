import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { MagnifyingGlass, Plus, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { ApiError, codeMessage } from "@/api/http";
import { getPower, getPowerFilterOptions, searchPowers } from "@/api/powers";
import { localized } from "@/i18n/localized";
import { cn } from "@/lib/utils";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";
import type { PowerPick } from "@/types/character";
import type { Power, PowerCost, PowerFilterOptions } from "@/types/powers";

const COUNT_KEYS = [
  "disciplines",
  "sciences",
  "devotions",
  "defenceModes",
] as const;

/** The kinds of power in the order the list shows them. */
const KINDS: Power["kind"][] = [
  "science",
  "devotion",
  "defence_mode",
  "attack_mode",
];

/** Powers read so far, by id: the known list shows each one's name, cost and score. */
function usePowerCache(
  ids: string[],
  seen: Power[],
): Record<string, Power | undefined> {
  const [byId, setById] = useState<Record<string, Power>>({});
  const missing = ids.filter(
    (id) => !byId[id] && !seen.some((p) => p.id === id),
  );
  const key = [...new Set(missing)].sort().join(",");
  useEffect(() => {
    if (!key) return;
    let live = true;
    for (const id of key.split(",")) {
      getPower(id).then(
        (p) => live && setById((m) => ({ ...m, [id]: p })),
        () => undefined,
      );
    }
    return () => {
      live = false;
    };
  }, [key]);
  const out: Record<string, Power | undefined> = { ...byId };
  for (const p of seen) out[p.id] = p;
  return out;
}

/**
 * A psionicist's known powers (ADR 0043 step 9): what Table 4 allows at the level beside what the
 * picks take, the picks with their costs and scores, and a picker over the powers catalogue. One
 * list, saved whole by every change (an add, a remove, a relearn): the server checks the rules and
 * a refusal names the power, the discipline and the slot, the list staying as it was. The journal's
 * Undo restores the previous list. Nothing for a character with no known powers block.
 */
export function KnownPowersPanel() {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const known = useCharacterStore((s) => s.character?.derived?.knownPowers);
  const save = useCharacterStore((s) => s.saveKnownPicks);
  const busy = useTableStore((s) => s.busy);
  const [query, setQuery] = useState("");
  const [found, setFound] = useState<Power[]>([]);
  const [options, setOptions] = useState<PowerFilterOptions | null>(null);
  const [refusal, setRefusal] = useState<ApiError | null>(null);

  useEffect(() => {
    if (!known) return;
    let live = true;
    getPowerFilterOptions().then(
      (o) => live && setOptions(o),
      () => undefined,
    );
    return () => {
      live = false;
    };
  }, [known]);

  const picking = known !== undefined && known !== null;
  useEffect(() => {
    if (!picking) return;
    let live = true;
    const timer = setTimeout(() => {
      searchPowers({ q: query, discipline: "", kind: "" }).then(
        (page) => live && setFound(page.items),
        () => undefined,
      );
    }, 150);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [picking, query]);

  const picks = known?.picks ?? [];
  const powers = usePowerCache(
    picks.filter((p) => p.kind !== "discipline").map((p) => p.id),
    found,
  );
  if (!known) return null;

  const disciplineName = (id: string) => {
    const d = options?.disciplines.find((x) => x.id === id);
    return d ? localized(lang, d.name) : id;
  };
  const powerName = (id: string) => {
    const p = powers[id];
    return p ? localized(lang, p.name) : id;
  };

  async function change(next: PowerPick[]) {
    if (busy) return;
    setRefusal(null);
    setRefusal(await save(next));
  }
  const isPicked = (kind: PowerPick["kind"], id: string) =>
    picks.some((p) => p.kind === kind && p.id === id);
  const add = (pick: PowerPick) => void change([...picks, pick]);
  const remove = (pick: PowerPick) =>
    void change(
      picks.filter((p) => !(p.kind === pick.kind && p.id === pick.id)),
    );

  const cost = (c: PowerCost) =>
    typeof c === "number"
      ? t("lookup.powers.psp", { count: c })
      : t(`lookup.powers.costWords.${c}`);
  const costLine = (p: Power) =>
    p.maintenance
      ? `${cost(p.initial_cost)} · ${t("lookup.powers.maintenanceValue", {
          cost: cost(p.maintenance.cost),
          count: p.maintenance.period,
        })}`
      : cost(p.initial_cost);
  const scoreLine = (p: Power) =>
    p.score
      ? `${t(`attr.${p.score.ability}`)} ${
          p.score.modifier < 0
            ? `−${-p.score.modifier}`
            : `+${p.score.modifier}`
        }`
      : t("lookup.powers.scoreInText");

  const disciplinePicks = picks.filter((p) => p.kind === "discipline");
  const powerPicks = (kind: Power["kind"]) =>
    picks.filter((p) => p.kind !== "discipline" && powers[p.id]?.kind === kind);

  return (
    <div
      className="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_minmax(18rem,26rem)] gap-3"
      data-testid="known-powers"
    >
      <section
        className="zone-card flex min-h-0 min-w-0 flex-col gap-3 overflow-y-auto px-4 pb-3.5"
        aria-label={t("powers.known")}
      >
        <div className="card-band bleed-x-4 flex shrink-0 flex-col gap-2 py-2.5">
          <h2 className="text-headline">{t("powers.known")}</h2>
          <dl className="grid grid-cols-[repeat(auto-fill,minmax(7.5rem,1fr))] gap-2">
            {COUNT_KEYS.map((k) => (
              <div
                key={k}
                className="flex flex-col gap-0.5 rounded-lg bg-raised px-3 py-2"
                data-testid={`powers-count-${k}`}
              >
                <dt className="text-label text-muted-foreground">
                  {t(`powers.counts.${k}`)}
                </dt>
                <dd
                  className={cn(
                    "text-body tabular-nums",
                    known.used[k] > known.allowance[k] && "text-danger",
                  )}
                >
                  {t("powers.usedOf", {
                    used: known.used[k],
                    allowed: known.allowance[k],
                  })}
                </dd>
              </div>
            ))}
          </dl>
          {refusal && (
            <Refusal
              error={refusal}
              power={refusal.body?.power as string | undefined}
              discipline={refusal.body?.discipline as string | undefined}
              slot={refusal.body?.slot as string | undefined}
              powerName={powerName}
              disciplineName={disciplineName}
            />
          )}
        </div>

        <KnownGroup title={t("powers.counts.disciplines")}>
          {disciplinePicks.length === 0 && <Empty />}
          {disciplinePicks.map((p) => (
            <KnownRow
              key={`d-${p.id}`}
              testId="known-discipline"
              name={disciplineName(p.id)}
              tag={p.id === known.primary ? t("powers.primary") : undefined}
              removeLabel={t("powers.remove", { name: disciplineName(p.id) })}
              disabled={busy}
              onRemove={() => remove(p)}
            />
          ))}
        </KnownGroup>
        {KINDS.map((kind) => {
          const rows = powerPicks(kind);
          if (rows.length === 0) return null;
          return (
            <KnownGroup key={kind} title={t(`lookup.powers.kinds.${kind}`)}>
              {rows.map((p) => {
                const pw = powers[p.id];
                const relearn = p.kind === "relearn";
                return (
                  <KnownRow
                    key={`${p.kind}-${p.id}`}
                    testId="known-power"
                    name={
                      relearn
                        ? t("powers.relearned", { name: powerName(p.id) })
                        : powerName(p.id)
                    }
                    sub={
                      pw
                        ? `${disciplineName(pw.discipline)} · ${costLine(pw)} · ${scoreLine(pw)}`
                        : undefined
                    }
                    removeLabel={t("powers.remove", { name: powerName(p.id) })}
                    disabled={busy}
                    onRemove={() => remove(p)}
                    action={
                      !relearn &&
                      (kind === "science" || kind === "devotion") ? (
                        <Button
                          className="px-2"
                          disabled={busy}
                          title={t("powers.relearnHint")}
                          onClick={() => add({ kind: "relearn", id: p.id })}
                        >
                          {t("powers.relearn")}
                        </Button>
                      ) : undefined
                    }
                  />
                );
              })}
            </KnownGroup>
          );
        })}
      </section>

      <section
        className="stage-card flex min-h-0 min-w-0 flex-col gap-3 pb-3.5"
        aria-label={t("powers.picker")}
      >
        <div className="card-band flex shrink-0 flex-col gap-2 px-4 py-2.5">
          <div className="relative min-w-0">
            <MagnifyingGlass className="pointer-events-none absolute top-1.5 left-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="search"
              className={cn(fieldClass, "pl-8")}
              placeholder={t("lookup.powers.search")}
              aria-label={t("lookup.powers.search")}
              autoComplete="off"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          <div
            role="group"
            aria-label={t("powers.addDiscipline")}
            className="flex flex-wrap gap-1"
          >
            {(options?.disciplines ?? []).map((d) => (
              <button
                key={d.id}
                type="button"
                lang={lang}
                disabled={busy || isPicked("discipline", d.id)}
                className="chip px-2 py-0.5 text-body font-medium text-muted-foreground hover:text-foreground disabled:text-ink-muted"
                aria-label={t("powers.add", { name: localized(lang, d.name) })}
                onClick={() => add({ kind: "discipline", id: d.id })}
              >
                {localized(lang, d.name)}
              </button>
            ))}
          </div>
        </div>
        <ul
          className="flex min-h-0 flex-1 list-none flex-col gap-0.5 overflow-y-auto px-4"
          aria-label={t("lookup.powers.results")}
        >
          {found.length === 0 && (
            <li className="p-2 text-body text-muted-foreground">
              {t("lookup.powers.noMatch")}
            </li>
          )}
          {found.map((p) => (
            <li
              key={p.id}
              data-testid="pick-row"
              data-id={p.id}
              className="flex shrink-0 items-center gap-2 rounded-md px-2 py-1.5 text-body hover:bg-raised"
            >
              <span className="flex min-w-0 flex-1 flex-col">
                <span lang={lang} className="min-w-0 break-words">
                  {localized(lang, p.name)}
                </span>
                <span className="text-label break-words text-muted-foreground">
                  {disciplineName(p.discipline)} ·{" "}
                  {t(`lookup.powers.kinds.${p.kind}`)} · {costLine(p)} ·{" "}
                  {scoreLine(p)}
                </span>
              </span>
              <Button
                className="shrink-0 gap-1 px-2"
                disabled={busy || isPicked("power", p.id)}
                aria-label={t("powers.add", { name: localized(lang, p.name) })}
                onClick={() => add({ kind: "power", id: p.id })}
              >
                <Plus aria-hidden />
                {t("powers.addShort")}
              </Button>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

function Empty() {
  const { t } = useTranslation();
  return <p className="text-body text-muted-foreground">{t("powers.none")}</p>;
}

function KnownGroup({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="flex shrink-0 flex-col gap-1">
      <h3 className="section-label">{title}</h3>
      <div className="flex flex-col gap-0.5">{children}</div>
    </section>
  );
}

function KnownRow({
  testId,
  name,
  sub,
  tag,
  action,
  removeLabel,
  disabled,
  onRemove,
}: {
  testId: string;
  name: string;
  sub?: string;
  tag?: string;
  action?: React.ReactNode;
  removeLabel: string;
  disabled: boolean;
  onRemove: () => void;
}) {
  const { i18n } = useTranslation();
  return (
    <div
      data-testid={testId}
      className="flex items-center gap-2 rounded-md bg-raised px-3 py-1.5 text-body"
    >
      <span className="flex min-w-0 flex-1 flex-col">
        <span lang={i18n.language} className="min-w-0 break-words">
          {name}
          {tag && (
            <span className="ml-2 text-label text-muted-foreground">{tag}</span>
          )}
        </span>
        {sub && (
          <span className="text-label break-words text-muted-foreground">
            {sub}
          </span>
        )}
      </span>
      {action}
      <Button
        className="shrink-0 px-2"
        disabled={disabled}
        aria-label={removeLabel}
        title={removeLabel}
        onClick={onRemove}
      >
        <X aria-hidden />
      </Button>
    </div>
  );
}

/** A refused save: the rule's wording, then what it names (the power, its discipline, the slot). */
function Refusal({
  error,
  power,
  discipline,
  slot,
  powerName,
  disciplineName,
}: {
  error: ApiError;
  power?: string;
  discipline?: string;
  slot?: string;
  powerName: (id: string) => string;
  disciplineName: (id: string) => string;
}) {
  const { t } = useTranslation();
  // Worded now, not when it was raised: the player may have changed the language since.
  const figures = Object.fromEntries(
    Object.entries(error.body ?? {}).filter(
      (e): e is [string, number] => typeof e[1] === "number",
    ),
  );
  const wording = codeMessage(error.code, figures) ?? error.message;
  const named = [
    power ? t("powers.refusedPower", { name: powerName(power) }) : "",
    discipline
      ? t("powers.refusedDiscipline", { name: disciplineName(discipline) })
      : "",
    slot ? t("powers.refusedSlot", { name: t(`powers.slots.${slot}`) }) : "",
  ].filter(Boolean);
  return (
    <p
      role="alert"
      className="text-body text-danger"
      data-testid="powers-refusal"
    >
      {wording}
      {named.length > 0 && ` (${named.join(" · ")})`}
    </p>
  );
}
