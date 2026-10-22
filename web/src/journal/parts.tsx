// The result card's shared pieces (ADR 0027 step 8): the hexagon die, the stat tiles, the verdict
// pill, the one-line correction fold, the big total, the die chips and the check's layout with its
// adjustment after the roll. Each kind's card body composes them.
import { useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { CaretRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { fieldClass } from "@/components/dashboard/fieldStyles";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { formatSigned } from "@/lib/format";
import { adjustmentChanged, adjustmentInEffect } from "@/lib/saveAdjust";
import { cn } from "@/lib/utils";
import type { HpBreakdown } from "@/types/log";
import { dieTone, type DieChipView } from "./dice";

export const labelClass = "text-label text-muted-foreground";

/** The natural die of a roll in a hexagon, the handoff's d20 shape. */
export function HexDie({ value, label }: { value: number; label: string }) {
  return (
    <span
      role="img"
      aria-label={label}
      className="flex h-12 w-11 shrink-0 items-center justify-center bg-tint text-key text-on-accent tabular-nums [clip-path:polygon(50%_0,100%_25%,100%_75%,50%_100%,0_75%,0_25%)]"
      data-testid="hex-die"
    >
      {value}
    </span>
  );
}

/**
 * A stat tile (a label over its value). A row of them is the tile grid: as many columns as fit at
 * 5rem each, so the save's four facts sit on one row on a wide card and wrap, never cut, on a
 * narrow one.
 */
export function Fact({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <div className="rounded-lg bg-raised px-3 py-2">
      <dt className={labelClass}>{label}</dt>
      <dd className="text-body font-semibold tabular-nums">{children}</dd>
    </div>
  );
}

/**
 * The verdict as a filled pill (ADR 0026 step 5.5): success or danger fill, on-accent text. It sits
 * right after the result it judges (ADR 0027 step 8.1), never at the card's far edge.
 */
export function Verdict({
  ok,
  yes,
  no,
}: {
  ok: boolean;
  yes: string;
  no: string;
}) {
  return (
    <span
      data-testid="verdict"
      data-ok={ok}
      className={cn(
        "inline-flex shrink-0 items-center rounded-full px-2.5 py-0.5 text-body font-semibold text-on-accent",
        ok ? "bg-success" : "bg-danger",
      )}
    >
      {ok ? yes : no}
    </span>
  );
}

/** The key line of a check: its result (the save's total, the check's roll, the attack's AC) and the verdict beside it. */
export function ResultLine({
  children,
  verdict,
}: {
  children: ReactNode;
  verdict: ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
      {children}
      {verdict}
    </div>
  );
}

/**
 * A correction after the roll folded to one line (ADR 0027 step 8.2): closed by default, open when a
 * correction is in effect. The line is a real disclosure (keyboard and screen reader), the form
 * opens under it.
 */
export function AdjustFold({
  title,
  open: initiallyOpen,
  testId,
  children,
}: {
  title: string;
  open: boolean;
  testId: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(initiallyOpen);
  return (
    <details
      open={open}
      onToggle={(ev) => setOpen(ev.currentTarget.open)}
      className="group/fold rounded-lg bg-raised px-3 py-1.5"
      data-testid={testId}
    >
      <summary className="flex cursor-pointer list-none items-center gap-1.5 text-body font-medium [&::-webkit-details-marker]:hidden">
        <CaretRight
          aria-hidden
          className="size-3.5 shrink-0 transition-transform group-open/fold:rotate-90"
        />
        {title}
      </summary>
      <div className="pt-1.5 pb-1">{children}</div>
    </details>
  );
}

/** A big total on the right of a row, in ink (gold only when the roll says crit or max). */
export function Total({
  label,
  value,
  testId,
  dim,
  gold,
}: {
  label: string;
  value: ReactNode;
  testId: string;
  dim?: boolean;
  /** Gold only for a crit or a maximum (ADR 0026 steps 5.3 and 5.8); a total is in ink. */
  gold?: boolean;
}) {
  return (
    <div className="shrink-0 text-right">
      <div className={labelClass}>{label}</div>
      <div
        className={cn(
          "text-headline tabular-nums",
          dim
            ? "text-muted-foreground"
            : gold
              ? "text-highlight"
              : "text-foreground",
        )}
        data-testid={testId}
      >
        {value}
      </div>
    </div>
  );
}

/** One die as a chip: its maximum in the accent, a 1 dimmed. */
export function DieChip({
  value,
  faces,
  tone: given,
}: {
  value: number;
  faces?: number | null;
  tone?: DieChipView["tone"];
}) {
  const tone = given ?? dieTone(value, faces ?? null);
  return (
    <li
      className={cn(
        "min-w-7 rounded-md px-2 py-0.5 text-center text-body font-semibold tabular-nums",
        tone === "max" ? "chip-max" : "chip",
        tone === "min" && "text-muted-foreground",
      )}
      data-tone={tone}
    >
      {value}
    </li>
  );
}

/**
 * An entry without a layout of its own: its journal text and, for an HP or PSP change, its value (red
 * for damage or PSPs spent, green for healing or PSPs recovered) and what was asked for when the server
 * cut it (an HP change's `capped`, any other kind's worded `note`).
 */
export function InfoBody({
  text,
  value,
  capped,
  note,
}: {
  text: string;
  value?: { amount: number; tone: "damage" | "heal" } | null;
  capped?: HpBreakdown | null;
  note?: string | null;
}) {
  const { t } = useTranslation();
  return (
    <div
      className="flex items-start justify-between gap-3"
      data-testid="info-result"
    >
      <div className="flex min-w-0 flex-col gap-1">
        <p className="text-body break-words">{text}</p>
        {capped && (
          <p className="text-label text-muted-foreground">
            {t(capped.hpRequested < 0 ? "log.floored" : "log.capped", {
              requested: formatSigned(capped.hpRequested),
            })}
          </p>
        )}
        {note && <p className="text-label text-muted-foreground">{note}</p>}
      </div>
      {value && (
        <span
          className={cn(
            "shrink-0 text-headline tabular-nums",
            value.tone === "damage" ? "text-danger" : "text-success",
          )}
          data-testid="info-value"
        >
          {formatSigned(value.amount)}
        </span>
      )}
    </div>
  );
}

export type AdjustCheckProps = {
  current: { adjustment: number; adjustmentReason?: string };
  label: string;
  entryId: number;
  valueTestId: string;
  min?: number;
  max?: number;
  step: (value: number, by: 1 | -1) => number;
  onApply: (value: number, reason: string) => Promise<unknown>;
};

/**
 * "Ajuster après le jet" on the latest check, folded to one line (ADR 0027 steps 8.2 and 8.3):
 * closed by default, open while an adjustment is in effect. Open, one row: − / + around a signed
 * value starting at the current adjustment, an optional reason, Appliquer (disabled only while
 * nothing changed). The value's steps and bounds come from the caller; the server recomputes.
 * Hidden once the entry is voided.
 */
export function AdjustCheck({
  current,
  label,
  entryId,
  valueTestId,
  min,
  max,
  step,
  onApply,
}: AdjustCheckProps) {
  const { t } = useTranslation();
  const voided = useLogStore((st) =>
    st.entries.some((e) => e.id === entryId && e.status === "voided"),
  );
  if (voided) return null;
  return (
    <AdjustFold
      title={t("result.adjust.title")}
      open={adjustmentInEffect(current)}
      testId="adjust-check"
    >
      {/* Keyed by the adjustment so an applied or popped one restarts the form from what is shown. */}
      <AdjustCheckForm
        key={`${current.adjustment}:${current.adjustmentReason ?? ""}`}
        current={current}
        label={label}
        valueTestId={valueTestId}
        min={min}
        max={max}
        step={step}
        onApply={onApply}
      />
    </AdjustFold>
  );
}

/** The open fold's one row: − value +, the reason, Appliquer. */
function AdjustCheckForm({
  current,
  label,
  valueTestId,
  min,
  max,
  step,
  onApply,
}: Omit<AdjustCheckProps, "entryId">) {
  const { t } = useTranslation();
  const rolling = useRollStore((s) => s.rolling);
  const [value, setValue] = useState(current.adjustment);
  const [reason, setReason] = useState(current.adjustmentReason ?? "");
  const changed = adjustmentChanged(current, value, reason);

  return (
    <form
      className="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2"
      aria-label={label}
      onSubmit={(ev) => {
        ev.preventDefault();
        if (changed) void onApply(value, value === 0 ? "" : reason.trim());
      }}
    >
      <div className="flex items-center gap-1">
        <Button
          type="button"
          size="sm"
          aria-label={t("result.adjust.decrease")}
          disabled={min !== undefined && value <= min}
          onClick={() => setValue((v) => step(v, -1))}
        >
          −
        </Button>
        <output
          className="w-12 text-center text-key tabular-nums"
          aria-label={t("result.adjust.value")}
          data-testid={valueTestId}
        >
          {formatSigned(value)}
        </output>
        <Button
          type="button"
          size="sm"
          aria-label={t("result.adjust.increase")}
          disabled={max !== undefined && value >= max}
          onClick={() => setValue((v) => step(v, 1))}
        >
          +
        </Button>
      </div>
      <input
        className={fieldClass}
        type="text"
        maxLength={80}
        value={reason}
        aria-label={t("result.adjust.reason")}
        placeholder={t("result.adjust.reason")}
        onChange={(ev) => setReason(ev.target.value)}
      />
      <Button type="submit" size="sm" disabled={rolling || !changed}>
        {t("result.adjust.apply")}
      </Button>
    </form>
  );
}

/**
 * A d20/d100-under check (skill or ability): the die, "roll vs adjusted target", the verdict, what
 * the target is made of, the DM's adjustment (with its reason) and, on the latest check only, the
 * control to change it. Every number is the server's.
 */
export function CheckView({
  die,
  roll,
  target,
  adjustment,
  adjustmentReason,
  adjustedTarget,
  success,
  heading,
  percent,
  adjust,
  testId,
}: {
  testId: string;
  die: number;
  roll: number;
  target: number;
  adjustment: number;
  adjustmentReason?: string;
  adjustedTarget: number;
  success: boolean;
  heading: string;
  /** A bard skill's targets are percentages. */
  percent?: boolean;
  /** The adjustment control, on the latest check only. */
  adjust: ReactNode;
}) {
  const { t } = useTranslation();
  const show = (n: number) => (percent ? t("skills.percent", { value: n }) : n);
  return (
    <div className="flex flex-col gap-3" data-testid={testId}>
      <div className="flex items-center gap-3">
        <HexDie
          value={roll}
          label={t("journal.die", { faces: die, value: roll })}
        />
        <div className="min-w-0 flex-1">
          <ResultLine
            verdict={
              <Verdict
                ok={success}
                yes={t("result.skill.success")}
                no={t("result.skill.failure")}
              />
            }
          >
            <span className="text-key tabular-nums" data-testid="skill-vs">
              {t(percent ? "journal.skillVsPercent" : "journal.skillVs", {
                roll,
                target: adjustedTarget,
              })}
            </span>
          </ResultLine>
          <div className="text-body text-muted-foreground tabular-nums">
            {heading}
          </div>
        </div>
      </div>
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(5rem,1fr))] gap-2">
        <Fact label={t("result.skill.die", { die })}>{roll}</Fact>
        <Fact label={t("result.target")}>{show(target)}</Fact>
        {adjustment !== 0 && (
          <Fact label={t("result.skill.adjustedTarget")}>
            <span data-testid="skill-adjusted-target">
              {show(adjustedTarget)}
            </span>
          </Fact>
        )}
      </dl>
      {adjustment !== 0 && (
        <div
          className="flex justify-between gap-2 text-body"
          data-testid="skill-adjustment-line"
        >
          <span className="min-w-0 break-words">
            {adjustmentReason ?? t("result.skill.adjustment")}
          </span>
          <span className="tabular-nums">{formatSigned(adjustment)}</span>
        </div>
      )}
      {adjust}
    </div>
  );
}
