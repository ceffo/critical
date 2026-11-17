import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Check, Minus, Plus, X } from "@phosphor-icons/react";
import { fieldClass, labelClass } from "@/components/dashboard/fieldStyles";
import { Note } from "@/components/creation/kit";
import { Button } from "@/components/ui/button";
import { ProficiencyNameButton } from "./ProficiencyDialog";
import { skillName } from "@/journal/words";
import { cn } from "@/lib/utils";
import type {
  NonweaponPool,
  PickFigures,
  ProficiencySlots,
} from "@/types/character";
import {
  addPick,
  catalogueRows,
  groupBands,
  removePick,
  setSlots,
  type CatalogueRow,
  type PickInput,
  type ServedCatalogue,
} from "./catalogue";
import { AbilityTarget, Band, PoolBar } from "./pickParts";
import { useAbilityHighlight } from "./pickHooks";

/**
 * The non-weapon proficiency picker (ADR 0037 steps 3 to 5 and 12), shared by the Skills activity's
 * editor and the creation's step: the pool bar that stays at the top of the scroll, the picks on the
 * left (cost, extra slots, the check's ability and target) and the catalogue on the right (a search,
 * then the book's five groups, the character's first, each saying its cost rule). Every number is
 * the server's: the pool and each pick's figures come from its preview or checks (`figures`), a
 * catalogue row's cost from the served catalogue; the web only words them. A pick added spends its
 * cost; the stepper changes the slots spent, never below the cost (removing is its own button). A
 * proficiency the class grants free (step 22) is marked free; one the server offers (`offered`) is
 * listed first, added at 0 slots in one click.
 */
export function ProficiencyPicker({
  catalogue,
  picks,
  onChange,
  figures,
  stale = false,
  errorFor,
  offered = [],
  scores = {},
  header,
  aside,
  barClassName,
}: {
  catalogue: ServedCatalogue;
  picks: PickInput[];
  onChange: (picks: PickInput[]) => void;
  /** The server's figures for the picks shown; null before its first answer. */
  figures: ProficiencySlots | null;
  /** True while the figures shown are not those of the picks (a refusal, a failed preview). */
  stale?: boolean;
  /** A refusal's words for a field (`nonweapon.<id>`, or `nonweapon` for the budget), or null. */
  errorFor: (field: string) => string | null;
  /** The ids the server offers free and the picks do not hold (ADR 0037 step 22.4), in its order. */
  offered?: string[];
  /** The scores in effect by ability code, as the character card shows them. */
  scores?: Record<string, number>;
  /** The editor's title and Save / Cancel, kept with the pool bar at the top. */
  header?: ReactNode;
  /** What goes under the picks, in their column (the languages). */
  aside?: ReactNode;
  barClassName?: string;
}) {
  const { t, i18n } = useTranslation();
  const [query, setQuery] = useState("");
  const rows = catalogueRows(
    catalogue,
    (id) => skillName(t, id),
    query,
    i18n.language,
  );
  const groupName = (g: string) =>
    t(`proficiencies.groups.${g}`, { defaultValue: g });
  const picked = new Set(picks.map((p) => p.id));
  // ADR 0037 step 22.4: the offered ids first, out of the groups below; one already picked (the
  // preview not yet back) is no offer.
  const offer = new Set(offered.filter((id) => !picked.has(id)));
  const offeredRows = offered
    .map((id) => rows.find((r) => r.option.id === id))
    .filter(
      (r): r is CatalogueRow => r !== undefined && offer.has(r.option.id),
    );
  const bands = groupBands(
    rows.filter((r) => !offer.has(r.option.id)),
    catalogue.groups,
    groupName,
    i18n.language,
  );
  const figureOf = (id: string) => figures?.picks.find((f) => f.id === id);
  const budgetError = errorFor("nonweapon");
  const left = figures?.nonWeapon.left;
  const searching = query.trim() !== "";
  const add = (r: CatalogueRow, cost: number) =>
    onChange(addPick(picks, r.option.id, cost));

  return (
    <div
      className="flex min-w-0 flex-col gap-3"
      data-testid="proficiency-picker"
    >
      {figures ? (
        <PoolSummary
          pool={figures.nonWeapon}
          header={header}
          stale={stale}
          className={barClassName}
        />
      ) : (
        header && <div className="sticky top-0 z-10">{header}</div>
      )}
      {budgetError && <Note kind="refusal">{budgetError}</Note>}
      <div className="grid grid-cols-1 items-start gap-5 min-[1100px]:grid-cols-[minmax(0,5fr)_minmax(0,6fr)]">
        <div className="flex min-w-0 flex-col gap-4">
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
              <ul
                className="flex flex-col gap-1"
                data-testid="proficiency-picks"
              >
                {picks.map((p) => (
                  <PickRow
                    key={p.id}
                    pick={p}
                    figure={figureOf(p.id)}
                    cost={catalogue.costs[p.id]}
                    ability={
                      figureOf(p.id)?.ability ??
                      catalogue.proficiencies.find((o) => o.id === p.id)
                        ?.ability
                    }
                    modifier={
                      figureOf(p.id)?.modifier ??
                      catalogue.proficiencies.find((o) => o.id === p.id)
                        ?.modifier ??
                      0
                    }
                    scores={scores}
                    error={errorFor(`nonweapon.${p.id}`)}
                    onSlots={(n) => onChange(setSlots(picks, p.id, n))}
                    onRemove={() => onChange(removePick(picks, p.id))}
                  />
                ))}
              </ul>
            )}
          </section>
          {aside}
        </div>
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
          <CatalogueRows
            title={t("proficiencies.offered")}
            rows={offeredRows}
            picked={picked}
            scores={scores}
            left={undefined}
            testId="proficiency-offered"
            onAdd={(r) => add(r, 0)}
          />
          {bands.map((b, i) => (
            <Band
              key={b.group}
              title={groupName(b.group)}
              note={t(
                b.own ? "proficiencies.groupOwn" : "proficiencies.groupOther",
              )}
              count={b.rows.length}
              defaultOpen={i === 0}
              forceOpen={searching}
              testId={`proficiency-group-${b.group}`}
            >
              <CatalogueRows
                title={groupName(b.group)}
                rows={b.rows}
                picked={picked}
                scores={scores}
                left={left}
                testId={`proficiency-${b.own ? "own" : "other"}-${b.group}`}
                onAdd={(r) => add(r, r.cost)}
              />
            </Band>
          ))}
          {bands.length === 0 && offeredRows.length === 0 && (
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
 * The non-weapon pool as the server derives it (ADR 0037 step 3), in the bar that stays at the top:
 * what is left of what is available, the terms, the languages' part (step 6) and the overspend's
 * warning (step 10: an existing overspend may only shrink).
 */
export function PoolSummary({
  pool,
  header,
  stale = false,
  className,
}: {
  pool: NonweaponPool;
  header?: ReactNode;
  stale?: boolean;
  className?: string;
}) {
  const { t } = useTranslation();
  return (
    <PoolBar
      testId="proficiency-pool"
      header={header}
      left={pool.left}
      available={pool.available}
      stale={stale}
      className={className}
      terms={t("proficiencies.pool", {
        initial: pool.initial,
        levels: pool.levelGained,
        perLevels: pool.perLevels,
        intelligence: pool.intelligence,
        available: pool.available,
      })}
      detail={
        pool.languages > 0 && (
          <span
            className="text-label text-ink-muted tabular-nums"
            data-testid="proficiency-pool-languages"
          >
            {t("proficiencies.languagesLine", { count: pool.languages })}
          </span>
        )
      }
    >
      {stale && (
        <p className="px-2.5 text-label break-words text-ink-muted">
          {t("proficiencies.previewStale")}
        </p>
      )}
      {pool.left < 0 && (
        <div className="px-2.5">
          <Note kind="warning" testId="proficiency-over">
            {t("proficiencies.over", {
              spent: pool.spent,
              available: pool.available,
            })}
          </Note>
        </div>
      )}
    </PoolBar>
  );
}

/** One pick: its name, the ability's score and the target, the server's figures (cost, extra slots)
 * and the stepper (never below the cost) with its own Remove. */
function PickRow({
  pick,
  figure,
  cost,
  ability,
  modifier,
  scores,
  error,
  onSlots,
  onRemove,
}: {
  pick: PickInput;
  figure: PickFigures | undefined;
  cost: number | undefined;
  ability: string | undefined;
  modifier: number;
  scores: Record<string, number>;
  error: string | null;
  onSlots: (slots: number) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const name = skillName(t, pick.id);
  // The cost is the server's: the preview's figure for the pick, else the catalogue's for the class.
  const shownCost = figure?.cost ?? cost;
  const highlight = useAbilityHighlight(ability);
  return (
    <li
      className="flex flex-col gap-1 rounded-md bg-sub-band px-2.5 py-1"
      data-testid={`proficiency-pick-${pick.id}`}
      {...highlight}
    >
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
        <span className="flex min-w-0 flex-col">
          <ProficiencyNameButton
            facts={{
              id: pick.id,
              ability: figure?.ability,
              modifier: figure?.modifier,
              slots: figure?.slots ?? pick.slots,
              extraSlots: figure?.extraSlots,
              cost: shownCost,
            }}
            className="w-fit max-w-full rounded-sm text-left text-body hover:underline"
          />
          <span
            className="flex flex-wrap items-baseline gap-x-2 text-label break-words text-ink-muted"
            data-testid="proficiency-pick-terms"
          >
            {figure?.known === false ? (
              t("proficiencies.unknown")
            ) : (
              <>
                {figure && (
                  <AbilityTarget
                    ability={figure.target === null ? undefined : ability}
                    score={ability ? scores[ability] : undefined}
                    modifier={modifier}
                    target={figure.target}
                    testId="proficiency-pick-target"
                  />
                )}
                <PickTerms figure={figure} cost={shownCost} />
              </>
            )}
          </span>
        </span>
        <span className="flex items-center gap-1">
          <Button
            type="button"
            size="icon-xs"
            aria-label={t("proficiencies.fewer", { name })}
            title={t("proficiencies.fewer", { name })}
            // The stepper stops at the proficiency's cost: fewer slots than that is no pick, and
            // taking it away is the Remove button's. The server still judges the count.
            disabled={pick.slots <= (shownCost ?? 0)}
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

/** "Coût 1 · +2 au jet": the server's figures; before its answer, the cost alone. */
function PickTerms({
  figure,
  cost,
}: {
  figure: PickFigures | undefined;
  cost: number | undefined;
}) {
  const { t } = useTranslation();
  const parts: string[] = [];
  // ADR 0037 step 22.2: a free pick's cost is 0, every slot on it an extra slot.
  if (figure?.free) parts.push(t("proficiencies.free"));
  else if (cost !== undefined)
    parts.push(t("proficiencies.cost", { count: cost }));
  if (figure && figure.extraSlots > 0)
    parts.push(t("proficiencies.extra", { count: figure.extraSlots }));
  return <span>{parts.join(" · ")}</span>;
}

/** The rows of one catalogue part, each a name that opens its description and a button that adds
 * the proficiency, at the cost the server gives. `left` (the pool's) disables the add of what the
 * pool cannot take; the server still judges. */
function CatalogueRows({
  title,
  rows,
  picked,
  scores,
  left,
  testId,
  onAdd,
}: {
  title: string;
  rows: CatalogueRow[];
  picked: Set<string>;
  scores: Record<string, number>;
  left: number | undefined;
  testId: string;
  onAdd: (r: CatalogueRow) => void;
}) {
  if (rows.length === 0) return null;
  return (
    <div className="flex flex-col gap-1" data-testid={testId}>
      {testId === "proficiency-offered" && (
        <h5 className="px-2.5 text-label font-medium break-words text-ink-muted">
          {title}
        </h5>
      )}
      <ul className="flex flex-col gap-1" aria-label={title}>
        {rows.map((r) => (
          <CatalogueItem
            key={r.option.id}
            row={r}
            has={picked.has(r.option.id)}
            full={left !== undefined && r.cost > 0 && left < r.cost}
            scores={scores}
            onAdd={() => onAdd(r)}
          />
        ))}
      </ul>
    </div>
  );
}

function CatalogueItem({
  row: r,
  has,
  full,
  scores,
  onAdd,
}: {
  row: CatalogueRow;
  has: boolean;
  full: boolean;
  scores: Record<string, number>;
  onAdd: () => void;
}) {
  const { t } = useTranslation();
  const id = r.option.id;
  const highlight = useAbilityHighlight(r.option.ability);
  const label = has
    ? t("proficiencies.alreadyPicked", { name: r.name })
    : full
      ? t("proficiencies.noRoom", { name: r.name, count: r.cost })
      : r.cost === 0
        ? t("proficiencies.addFree", { name: r.name })
        : t("proficiencies.add", { name: r.name, count: r.cost });
  return (
    <li
      className={cn(
        "flex items-stretch gap-1 rounded-md text-body",
        has ? "text-ink-muted" : "bg-raised",
      )}
      {...highlight}
    >
      <ProficiencyNameButton
        facts={{
          id,
          ability: r.option.ability ?? "",
          modifier: r.option.modifier,
          cost: r.cost,
        }}
        className="flex min-h-7 min-w-0 flex-1 items-center rounded-md px-2.5 py-0.5 text-left transition-colors hover:bg-raised-hover"
      >
        <span className="min-w-0 break-words">{r.name}</span>
      </ProficiencyNameButton>
      <span className="flex items-center gap-2 whitespace-nowrap px-1 tabular-nums">
        <AbilityTarget
          ability={r.option.ability}
          score={r.option.ability ? scores[r.option.ability] : undefined}
          modifier={r.option.modifier}
        />
        <span className="text-label" data-testid="proficiency-cost">
          {/* The server's cost 0: a proficiency the class grants free (step 22.2). */}
          {r.cost === 0
            ? t("proficiencies.freeShort")
            : t("proficiencies.slotsShort", { count: r.cost })}
        </span>
      </span>
      <button
        type="button"
        className="flex items-center rounded-md px-2 py-0.5 transition-colors hover:bg-raised-hover disabled:text-ink-muted disabled:hover:bg-transparent"
        disabled={has || full}
        aria-label={label}
        title={label}
        onClick={onAdd}
        data-testid={`proficiency-add-${id}`}
      >
        {has ? (
          <Check aria-hidden className="size-3.5 shrink-0" />
        ) : (
          <Plus aria-hidden className="size-3.5 shrink-0" />
        )}
      </button>
    </li>
  );
}
