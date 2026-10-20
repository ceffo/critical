import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Minus, Plus, X } from "@phosphor-icons/react";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Note } from "@/components/creation/kit";
import { Button } from "@/components/ui/button";
import { skillName } from "@/journal/words";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";
import type {
  CreationOptions,
  NonweaponPool,
  PickFigures,
  ProficiencySlots,
} from "@/types/character";
import {
  addPick,
  removePick,
  setSlots,
  splitCatalogue,
  type CatalogueRow,
  type PickInput,
} from "./catalogue";

/** What the picker reads from the options: the catalogue, the class's groups and the server's
 * cost of each proficiency by class. */
export type PickerOptions = Pick<
  CreationOptions,
  "proficiencies" | "proficiencyGroups" | "proficiencyCosts"
>;

/**
 * The non-weapon proficiency picker (ADR 0037 steps 3 to 5 and 12), shared by the Skills tab's
 * editor and the creation's step: the slots with their terms, the picks with their cost, extra
 * slots and check target, and the catalogue (the class's groups first, a search). Every number is
 * the server's: the pool and each pick's figures come from its preview or checks (`figures`), a
 * catalogue row's cost from `proficiencyCosts`; the web only words them. A pick added spends its
 * cost; the stepper changes the slots spent and the server says whether it takes them.
 */
export function ProficiencyPicker({
  options,
  cls,
  picks,
  onChange,
  figures,
  stale = false,
  errorFor,
}: {
  options: PickerOptions;
  cls: string;
  picks: PickInput[];
  onChange: (picks: PickInput[]) => void;
  /** The server's figures for the picks shown; null before its first answer. */
  figures: ProficiencySlots | null;
  /** True while the figures shown are not those of the picks (a refusal, a failed preview). */
  stale?: boolean;
  /** A refusal's words for a field (`nonweapon.<id>`, or `nonweapon` for the budget), or null. */
  errorFor: (field: string) => string | null;
}) {
  const { t, i18n } = useTranslation();
  const [query, setQuery] = useState("");
  const costs = options.proficiencyCosts?.[cls] ?? {};
  const groups = options.proficiencyGroups?.[cls] ?? [];
  const { own, other } = splitCatalogue(
    options,
    cls,
    (id) => skillName(t, id),
    query,
    i18n.language,
  );
  const picked = new Set(picks.map((p) => p.id));
  const figureOf = (id: string) => figures?.picks.find((f) => f.id === id);
  const budgetError = errorFor("nonweapon");

  return (
    <div
      className="flex min-w-0 flex-col gap-4"
      data-testid="proficiency-picker"
    >
      {figures && (
        <div
          className={cn("flex flex-col gap-2", stale && "opacity-60")}
          data-stale={stale ? "true" : undefined}
        >
          <PoolSummary pool={figures.nonWeapon} />
          {stale && (
            <p className="text-label break-words text-ink-muted">
              {t("proficiencies.previewStale")}
            </p>
          )}
        </div>
      )}
      {budgetError && <Note kind="refusal">{budgetError}</Note>}
      <div className="grid grid-cols-[repeat(auto-fit,minmax(18rem,1fr))] items-start gap-5">
        <section
          aria-labelledby="proficiency-picks-title"
          className="flex min-w-0 flex-col gap-2"
        >
          <h4
            id="proficiency-picks-title"
            className="group-heading section-label px-2.5"
          >
            {t("proficiencies.picked", { count: picks.length })}
          </h4>
          {picks.length === 0 ? (
            <p className="px-2.5 text-body break-words text-ink-muted">
              {t("proficiencies.none")}
            </p>
          ) : (
            <ul className="flex flex-col gap-1" data-testid="proficiency-picks">
              {picks.map((p) => (
                <PickRow
                  key={p.id}
                  pick={p}
                  figure={figureOf(p.id)}
                  cost={costs[p.id]}
                  error={errorFor(`nonweapon.${p.id}`)}
                  onSlots={(n) => onChange(setSlots(picks, p.id, n))}
                  onRemove={() => onChange(removePick(picks, p.id))}
                />
              ))}
            </ul>
          )}
        </section>
        <section
          aria-labelledby="proficiency-catalogue-title"
          className="flex min-w-0 flex-col gap-2"
          data-testid="proficiency-catalogue"
        >
          <h4
            id="proficiency-catalogue-title"
            className="group-heading section-label px-2.5"
          >
            {t("proficiencies.catalogue")}
          </h4>
          <div className="flex flex-col gap-1 px-2.5">
            <label htmlFor="proficiency-search" className={labelClass}>
              {t("proficiencies.search")}
            </label>
            <input
              id="proficiency-search"
              type="search"
              className={fieldClass}
              autoComplete="off"
              placeholder={t("proficiencies.searchPlaceholder")}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              data-testid="proficiency-search"
            />
          </div>
          <CatalogueGroup
            title={t("proficiencies.ownGroups", {
              groups: groups
                .map((g) => t(`proficiencies.groups.${g}`, { defaultValue: g }))
                .join(", "),
            })}
            rows={own}
            picked={picked}
            testId="proficiency-own"
            onAdd={(r) => onChange(addPick(picks, r.option.id, r.cost))}
          />
          <CatalogueGroup
            title={t("proficiencies.otherGroups")}
            rows={other}
            picked={picked}
            testId="proficiency-other"
            onAdd={(r) => onChange(addPick(picks, r.option.id, r.cost))}
          />
          {own.length === 0 && other.length === 0 && (
            <p className="px-2.5 text-body break-words text-ink-muted">
              {t("proficiencies.noMatch")}
            </p>
          )}
        </section>
      </div>
    </div>
  );
}

/**
 * The non-weapon pool as the server derives it (ADR 0037 step 3): its terms, what is spent and
 * left, the reminder of slots to spend (step 11) or the overspend's warning (step 10: an existing
 * overspend may only shrink).
 */
export function PoolSummary({ pool }: { pool: NonweaponPool }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-1" data-testid="proficiency-pool">
      <p
        className="text-body break-words tabular-nums"
        data-testid="proficiency-pool-terms"
      >
        {t("proficiencies.pool", {
          initial: pool.initial,
          levels: pool.levelGained,
          perLevels: pool.perLevels,
          intelligence: pool.intelligence,
          available: pool.available,
        })}
      </p>
      <p
        className="text-body break-words text-ink-muted tabular-nums"
        data-testid="proficiency-pool-spent"
      >
        {t("proficiencies.spentLeft", { spent: pool.spent, left: pool.left })}
      </p>
      {pool.left > 0 && (
        <Note kind="info" testId="proficiency-to-spend">
          {t("proficiencies.toSpend", { count: pool.left })}
        </Note>
      )}
      {pool.left < 0 && (
        <Note kind="warning" testId="proficiency-over">
          {t("proficiencies.over", {
            spent: pool.spent,
            available: pool.available,
          })}
        </Note>
      )}
    </div>
  );
}

/** One pick: its name, the server's figures (cost, extra slots, check target) and the stepper. */
function PickRow({
  pick,
  figure,
  cost,
  error,
  onSlots,
  onRemove,
}: {
  pick: PickInput;
  figure: PickFigures | undefined;
  cost: number | undefined;
  error: string | null;
  onSlots: (slots: number) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const name = skillName(t, pick.id);
  // The cost is the server's: the preview's figure for the pick, else the catalogue's for the class.
  const shownCost = figure?.cost ?? cost;
  return (
    <li
      className="flex flex-col gap-1 rounded-md bg-raised px-2.5 py-1"
      data-testid={`proficiency-pick-${pick.id}`}
    >
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
        <span className="flex min-w-0 flex-col">
          <span className="text-body break-words">{name}</span>
          <span
            className="text-label break-words text-ink-muted"
            data-testid="proficiency-pick-terms"
          >
            <PickTerms figure={figure} cost={shownCost} />
          </span>
        </span>
        <span className="flex items-center gap-1">
          <Button
            type="button"
            size="icon-xs"
            aria-label={t("proficiencies.fewer", { name })}
            title={t("proficiencies.fewer", { name })}
            disabled={shownCost !== undefined && pick.slots <= shownCost}
            onClick={() => onSlots(pick.slots - 1)}
            data-testid="proficiency-fewer"
          >
            <Minus aria-hidden />
          </Button>
          <span
            className="min-w-6 text-center text-body font-semibold tabular-nums"
            aria-label={t("proficiencies.slotsOf", {
              name,
              count: pick.slots,
            })}
            data-testid="proficiency-slots"
          >
            {pick.slots}
          </span>
          <Button
            type="button"
            size="icon-xs"
            aria-label={t("proficiencies.more", { name })}
            title={t("proficiencies.more", { name })}
            onClick={() => onSlots(pick.slots + 1)}
            data-testid="proficiency-more"
          >
            <Plus aria-hidden />
          </Button>
          <Button
            type="button"
            size="icon-xs"
            aria-label={t("proficiencies.remove", { name })}
            title={t("proficiencies.remove", { name })}
            onClick={onRemove}
            data-testid="proficiency-remove"
          >
            <X aria-hidden />
          </Button>
        </span>
      </div>
      {error && (
        <p role="alert" className="text-label break-words text-danger">
          {error}
        </p>
      )}
    </li>
  );
}

/** "Coût 1 · +2 au jet · cible DEX 18": the server's figures; before its answer, the cost alone. */
function PickTerms({
  figure,
  cost,
}: {
  figure: PickFigures | undefined;
  cost: number | undefined;
}) {
  const { t } = useTranslation();
  if (figure && !figure.known) return <>{t("proficiencies.unknown")}</>;
  const parts: string[] = [];
  if (cost !== undefined) parts.push(t("proficiencies.cost", { count: cost }));
  if (figure && figure.extraSlots > 0)
    parts.push(t("proficiencies.extra", { count: figure.extraSlots }));
  if (figure)
    parts.push(
      figure.target === null
        ? t("proficiencies.noCheck")
        : t("proficiencies.target", {
            ability: t(`attr.${figure.ability}`, {
              defaultValue: figure.ability,
            }),
            target: figure.target,
          }),
    );
  return <>{parts.join(" · ")}</>;
}

/** One part of the catalogue: a heading and its rows, each a button that adds the proficiency. */
function CatalogueGroup({
  title,
  rows,
  picked,
  testId,
  onAdd,
}: {
  title: string;
  rows: CatalogueRow[];
  picked: Set<string>;
  testId: string;
  onAdd: (r: CatalogueRow) => void;
}) {
  const { t } = useTranslation();
  if (rows.length === 0) return null;
  return (
    <div className="flex flex-col gap-1" data-testid={testId}>
      <h5 className="px-2.5 text-label font-medium break-words text-ink-muted">
        {title}
      </h5>
      <ul className="flex flex-col gap-1" aria-label={title}>
        {rows.map((r) => {
          const id = r.option.id;
          const has = picked.has(id);
          const label = has
            ? t("proficiencies.alreadyPicked", { name: r.name })
            : t("proficiencies.add", { name: r.name, count: r.cost ?? 0 });
          return (
            <li key={id}>
              <button
                type="button"
                className="grid min-h-7 w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md bg-raised px-2.5 py-0.5 text-left text-body transition-colors outline-none hover:bg-raised-hover focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-ink-muted"
                disabled={has || r.cost === undefined}
                aria-label={label}
                title={label}
                onClick={() => onAdd(r)}
                data-testid={`proficiency-add-${id}`}
              >
                <span className="flex min-w-0 items-center gap-1.5">
                  {has ? (
                    <Check aria-hidden className="size-3.5 shrink-0" />
                  ) : (
                    <Plus aria-hidden className="size-3.5 shrink-0" />
                  )}
                  <span className="min-w-0 break-words">{r.name}</span>
                </span>
                <span className="flex items-center gap-1.5 whitespace-nowrap tabular-nums">
                  {/* A proficiency with no check (Table 37's "NA") shows a dash. */}
                  <span className="text-ink-muted">
                    {r.option.ability
                      ? `${t(`attr.${r.option.ability}`, { defaultValue: r.option.ability })}${r.option.modifier !== 0 ? ` ${formatSigned(r.option.modifier)}` : ""}`
                      : "—"}
                  </span>
                  {r.cost !== undefined && (
                    <span
                      className="chip px-1.5 text-center font-semibold"
                      data-testid="proficiency-cost"
                    >
                      {t("proficiencies.slotsShort", { count: r.cost })}
                    </span>
                  )}
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
