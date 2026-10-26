import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Minus, Moon, Plus, Warning } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import { useBuffsStore } from "@/stores/buffs";
import { useCharacterStore } from "@/stores/character";
import { useTableStore } from "@/stores/table";
import {
  DEFAULT_REST_HOURS,
  REST_PRESETS,
  defaultRow,
  freeRows,
  payingRows,
  previewRest,
  visibleRecovery,
} from "@/lib/pspRest";
import { cn } from "@/lib/utils";
import type { BuffEffect } from "@/types/buffs";
import { usePowerNames } from "./usePowerNames";

/**
 * The rest popover (ADR 0047 items 4, 5 and 11): Table 6's rows as the character reads them (a
 * gated row only with its proficiency), the hours (1, 4 and 8 or any number), the result before
 * the press, and the one button: with a power paying PSPs kept up it stops those and recovers in
 * one undoable action (`Stop <power> and recover N h`), otherwise it recovers. Rows that cost
 * nothing stay up. The long rest is one line away. Every figure comes from the server; the preview
 * only shows what the recovery will then log.
 */
export function RestPopover() {
  const { t } = useTranslation();
  const pool = useCharacterStore((s) => s.character?.derived?.psp ?? null);
  const picks = useCharacterStore(
    (s) => s.character?.derived?.proficiencySlots?.picks,
  );
  const buffs = useBuffsStore((s) => s.buffs);
  const { recoverPsp, stopAndRecoverPsp, longRest, busy } = useTableStore();
  const [open, setOpen] = useState(false);
  const [chosen, setChosen] = useState<string | null>(null);
  const [hours, setHours] = useState(String(DEFAULT_REST_HOURS));

  const held = useMemo(() => new Set((picks ?? []).map((p) => p.id)), [picks]);
  const rows = visibleRecovery(pool?.recovery, held);
  const paying = payingRows(buffs);
  const free = freeRows(buffs);
  const name = usePowerNames(
    [...paying, ...free].flatMap((b) => (b.powerId ? [b.powerId] : [])),
  );
  if (!pool) return null;

  const row =
    rows.find((r) => r.activity === chosen) ?? defaultRow(rows) ?? undefined;
  const h = Number.parseFloat(hours);
  const valid = h > 0 && !!row;
  const after = row ? previewRest(pool, row.perHour, h) : null;
  const label = (b: BuffEffect) =>
    [b.powerId ? (name(b.powerId) ?? b.name) : "", b.target]
      .filter(Boolean)
      .join(" · ") || b.name;
  const hoursText = Number.isFinite(h) ? String(h) : "";

  async function rest() {
    if (!valid || busy || !row) return;
    const ok = paying.length
      ? await stopAndRecoverPsp(h, row.activity)
      : await recoverPsp(h, row.activity);
    if (ok) setOpen(false);
  }

  async function sleep() {
    if (busy) return;
    if (await longRest()) setOpen(false);
  }

  const step = (by: number) => setHours(String(Math.max(1, (h || 0) + by)));

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={<Button className="min-w-0 gap-1 px-2" />}
        data-testid="psp-rest"
        title={t("psp.rest.hint")}
      >
        <Moon aria-hidden />
        {t("psp.rest.open")}
      </PopoverTrigger>
      <PopoverContent
        side="right"
        align="start"
        className="flex w-[28rem] max-w-[calc(100vw-2rem)] flex-col gap-3 p-4"
        aria-label={t("psp.rest.title")}
        data-testid="psp-rest-popover"
      >
        <PopoverTitle className="section-label">
          {t("psp.rest.title")}
        </PopoverTitle>
        <div
          className="flex flex-wrap gap-1.5"
          role="radiogroup"
          aria-label={t("psp.activityLabel")}
        >
          {rows.map((r) => {
            const on = r.activity === row?.activity;
            return (
              <button
                key={r.activity}
                type="button"
                role="radio"
                aria-checked={on}
                data-testid="psp-rest-row"
                onClick={() => setChosen(r.activity)}
                className={cn(
                  "flex min-w-[5.5rem] flex-1 flex-col rounded-lg border px-2.5 py-1.5 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                  on
                    ? "border-transparent bg-selected shadow-[inset_0_0_0_1.5px_var(--tint)]"
                    : "border-input hover:bg-raised-hover",
                )}
              >
                <span className="text-body">
                  {t(`psp.activity.${r.activity}`, {
                    defaultValue: r.activity,
                  })}
                </span>
                <span className="text-label text-muted-foreground tabular-nums">
                  {t("psp.rest.perHour", { count: r.perHour })}
                </span>
              </button>
            );
          })}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label={t("psp.rest.less")}
            disabled={!(h > 1)}
            onClick={() => step(-1)}
          >
            <Minus aria-hidden />
          </Button>
          <input
            className="h-7 w-14 rounded-lg border border-input bg-surface px-2 text-center text-body tabular-nums outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
            type="number"
            min={0}
            step="any"
            inputMode="decimal"
            aria-label={t("psp.hours")}
            value={hours}
            onChange={(e) => setHours(e.target.value)}
            onKeyDown={(e) => {
              if (e.key !== "Enter" || e.nativeEvent.isComposing) return;
              e.preventDefault();
              void rest();
            }}
          />
          <Button
            size="icon-sm"
            variant="ghost"
            aria-label={t("psp.rest.more")}
            onClick={() => step(1)}
          >
            <Plus aria-hidden />
          </Button>
          {REST_PRESETS.map((p) => (
            <Button
              key={p}
              size="sm"
              aria-pressed={h === p}
              onClick={() => setHours(String(p))}
            >
              {t("psp.rest.preset", { hours: p })}
            </Button>
          ))}
          {after && valid && (
            <span
              className="ml-auto text-body tabular-nums"
              data-testid="psp-rest-preview"
              aria-label={t("psp.rest.previewLabel")}
            >
              <span className="text-success">+{after.gain}</span>
              <span className="text-muted-foreground">
                {" "}
                · {pool.current} →{" "}
              </span>
              <span className="font-semibold">{after.after}</span>
            </span>
          )}
        </div>
        {paying.length > 0 && (
          <div
            className="flex flex-col gap-1 rounded-lg border border-warning/60 px-3 py-2 text-body"
            data-testid="psp-rest-stops"
          >
            <p className="flex items-start gap-1.5 text-warning">
              <Warning aria-hidden className="mt-0.5 shrink-0" />
              {t("psp.rest.stopsTitle")}
            </p>
            <ul className="flex flex-col">
              {paying.map((b) => (
                <li key={b.id}>
                  {label(b)}{" "}
                  <span className="text-muted-foreground">
                    ({t("psp.rest.cost", { count: b.maintenanceCost })})
                  </span>
                </li>
              ))}
            </ul>
            {free.length > 0 && (
              <p className="text-label text-muted-foreground">
                {t("psp.rest.staysUp", {
                  powers: free.map(label).join(", "),
                })}
              </p>
            )}
          </div>
        )}
        <Button
          variant="main"
          disabled={!valid || busy}
          data-testid="psp-rest-go"
          onClick={() => void rest()}
        >
          <Moon aria-hidden />
          {paying.length
            ? t("psp.rest.stopAndRecover", {
                powers: paying.map(label).join(", "),
                hours: hoursText,
              })
            : t("psp.rest.recover", { hours: hoursText })}
        </Button>
        <div className="flex items-center justify-between gap-3 border-t border-border pt-3">
          <p className="min-w-0 text-label text-muted-foreground">
            {t("psp.rest.longLine")}
          </p>
          <Button
            size="sm"
            className="shrink-0"
            disabled={busy}
            data-testid="psp-long-rest"
            onClick={() => void sleep()}
          >
            {t("psp.rest.long")}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
