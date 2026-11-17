import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { CaretDown, CaretRight } from "@phosphor-icons/react";
import { formatSigned } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * The bar the pickers keep at the top of their scroll (ADR 0027 step 1: what is left is read every
 * time a choice is made): the editor's own header (title, Save and Cancel) when it has one, then the
 * pool with its slots left, what it is out of and the server's terms. It sticks to the top of the
 * scroller it sits in, so the count stays in sight however long the catalogue is. Every figure is
 * the server's, worded here.
 */
export function PoolBar({
  header,
  left,
  available,
  terms,
  detail,
  stale = false,
  leftOf,
  testId,
  className,
  children,
}: {
  header?: ReactNode;
  left: number;
  available: number;
  /** The formula of the pool as the server derives it, in small type. */
  terms: string;
  /** What else the pool says (the languages' part, the spent count), under the formula. */
  detail?: ReactNode;
  stale?: boolean;
  /** The words after the count ("slots left of 7"); the slots' by default. */
  leftOf?: string;
  testId: string;
  /** The tone the bar covers the list with while it scrolls under it. */
  className?: string;
  /** Notes under the figures: an overspend's warning, the stale-figures line. */
  children?: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <div
      className={cn(
        "sticky top-0 z-10 flex flex-col gap-1.5 pb-2",
        className ?? "bg-raised",
      )}
      data-testid="pool-bar"
    >
      {header}
      <div
        className={cn(
          "flex flex-wrap items-baseline gap-x-3 gap-y-0.5 px-2.5",
          stale && "opacity-60",
        )}
        data-testid={testId}
        data-stale={stale ? "true" : undefined}
      >
        <span
          className={cn(
            "text-headline tabular-nums",
            left !== 0 ? "text-danger" : "text-ink",
          )}
          data-testid={`${testId}-left`}
        >
          {left}
        </span>
        <span
          className="text-body tabular-nums"
          data-testid={`${testId}-spent`}
        >
          {leftOf ?? t("proficiencies.leftOf", { count: left, available })}
        </span>
        <span
          className="min-w-0 text-label break-words text-ink-muted tabular-nums"
          data-testid={`${testId}-terms`}
        >
          {terms}
        </span>
        {detail}
      </div>
      {children}
    </div>
  );
}

/**
 * A collapsible band of a catalogue (a group of proficiencies, a weapon family): its title, a short
 * note (the cost rule, "outside your class"), the count of rows, and the rows when open. A `quiet`
 * band is the one with nothing to take: a closed band without the colour. With a search, the caller
 * opens every band that has a match (`forceOpen`).
 */
export function Band({
  title,
  note,
  count,
  quiet = false,
  forceOpen = false,
  defaultOpen = false,
  testId,
  children,
}: {
  title: string;
  note?: string;
  count: number;
  quiet?: boolean;
  forceOpen?: boolean;
  defaultOpen?: boolean;
  testId?: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const shown = open || forceOpen;
  return (
    <section className="flex min-w-0 flex-col gap-1" data-testid={testId}>
      <button
        type="button"
        aria-expanded={shown}
        onClick={() => setOpen(!shown)}
        className={cn(
          "flex min-h-7 w-full items-center gap-2 rounded-md px-2.5 py-1 text-left text-body transition-colors",
          quiet
            ? "bg-sub-band text-ink-muted hover:bg-raised-hover"
            : "bg-selected hover:bg-raised-hover",
        )}
      >
        {shown ? (
          <CaretDown aria-hidden className="size-3.5 shrink-0" />
        ) : (
          <CaretRight aria-hidden className="size-3.5 shrink-0" />
        )}
        <span className="min-w-0 flex-1 break-words">
          <span className={cn(!quiet && "font-semibold")}>{title}</span>
          {note && (
            <span className="ml-2 text-label text-ink-muted">{note}</span>
          )}
        </span>
        <span className="text-label tabular-nums text-ink-muted">{count}</span>
      </button>
      {shown && children}
    </section>
  );
}

/**
 * "SAG 9 −2 → 7": the ability, the score in effect, the proficiency's modifier and, when the
 * server serves it, the target the check rolls under. Pointing at it or focusing the row
 * highlights that ability's score on the character card. A proficiency with no check shows a dash.
 */
export function AbilityTarget({
  ability,
  score,
  modifier,
  target,
  testId,
}: {
  ability: string | undefined;
  score: number | undefined;
  modifier: number;
  target?: number | null;
  testId?: string;
}) {
  const { t } = useTranslation();
  if (!ability)
    return (
      <span className="text-label text-ink-muted" data-testid={testId}>
        {t("proficiencies.noCheck")}
      </span>
    );
  return (
    <span className="whitespace-nowrap tabular-nums" data-testid={testId}>
      <span className="text-ink-muted">
        {t(`attr.${ability}`, { defaultValue: ability })}
        {score !== undefined && ` ${score}`}
        {modifier !== 0 && ` ${formatSigned(modifier)}`}
        {target != null && " → "}
      </span>
      {target != null && <b className="font-semibold">{target}</b>}
    </span>
  );
}
