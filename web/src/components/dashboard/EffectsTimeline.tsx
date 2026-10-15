import { useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import {
  CaretDown,
  CaretUp,
  LockSimple,
  PencilSimple,
  Shield,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { formatTurnsLeft } from "@/lib/passTime";
import {
  effectSpan,
  trackSize,
  windowStart,
  type ColumnRun,
  type TrackSize,
} from "@/lib/timeline";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import type { BuffEffect } from "@/types/buffs";
import { AddEffect } from "./AddEffect";
import { EditBuffForm } from "./BuffForms";

/** The label column, then one column per turn of the window. */
const LABEL_COLUMN = "clamp(10rem, 13vw, 15rem)";

/** Effect rows shown at rest; more than this and an expander shows the rest (ADR 0017 step 6). */
const REST_ROWS = 3;

/**
 * The effects track: one row per active buff over a window of turns that follows the current one.
 * A timed buff is a thin line for the turns it has run and a pill for what is left (from the
 * server's `duration`), in the warning colour on its last turn; a charge buff shows its charges as
 * pips; a buff that does not apply is dashed with a lock and the reason. Edit and remove per row,
 * Add next to the label. The number of turn columns follows the width available (8, 12 or 16). At
 * rest the track shows its turn row and up to 3 effect rows, no scrollbar; with more effects an
 * expander (aria-expanded) grows the track in place to show them all, capped at 40% of the viewport
 * height where it scrolls (ADR 0017 step 6).
 */
export function EffectsTimeline() {
  const { t } = useTranslation();
  const buffs = useBuffsStore((s) => s.buffs);
  const turn = useBuffsStore((s) => s.round);
  const [editing, setEditing] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);
  const edited = buffs.find((b) => b.id === editing);
  const trackRef = useRef<HTMLDivElement>(null);
  const size = useTrackSize(trackRef);
  const hidden = Math.max(buffs.length - REST_ROWS, 0);
  const shown = expanded ? buffs : buffs.slice(0, REST_ROWS);
  const start = windowStart(turn);
  const current = turn - start;
  const columns = Array.from({ length: size }, (_, i) => start + i);
  const grid: CSSProperties = {
    gridTemplateColumns: `${LABEL_COLUMN} repeat(${size}, minmax(0, 1fr))`,
    gridTemplateRows: `1.5rem repeat(${Math.max(shown.length, 1)}, 1.5rem)`,
  };

  return (
    <div
      className="flex min-w-0 flex-1 flex-col gap-2"
      role="group"
      aria-label={t("effects.title")}
    >
      <div
        ref={trackRef}
        className={cn("min-w-0", expanded && "max-h-[40vh] overflow-y-auto")}
      >
        <div
          className="relative grid items-center"
          style={grid}
          data-testid="effects-track"
          data-columns={size}
        >
          {/* The current turn, highlighted across every row. */}
          <div
            aria-hidden
            className="pointer-events-none self-stretch rounded-md bg-tint/10 ring-1 ring-tint/30 ring-inset"
            style={{ gridColumn: current + 2, gridRow: "1 / -1" }}
          />
          <div
            className="sticky top-0 z-10 flex h-full items-center gap-2 bg-card"
            style={{ gridColumn: 1, gridRow: 1 }}
          >
            <span className="text-title">{t("effects.title")}</span>
            <AddEffect />
            {hidden > 0 && (
              <Button
                size="xs"
                variant="ghost"
                aria-expanded={expanded}
                aria-label={
                  expanded
                    ? t("effects.showLess")
                    : t("effects.showMore", { count: hidden })
                }
                title={
                  expanded
                    ? t("effects.showLess")
                    : t("effects.showMore", { count: hidden })
                }
                data-testid="effects-expander"
                onClick={() => setExpanded((v) => !v)}
              >
                {expanded ? <CaretUp /> : <CaretDown />}
                {!expanded && <span className="tabular-nums">+{hidden}</span>}
              </Button>
            )}
          </div>
          {columns.map((n, i) => (
            <span
              key={n}
              className={cn(
                "sticky top-0 z-10 flex h-full items-center justify-center text-label tabular-nums",
                // Opaque, so effect rows scrolled under the sticky turn row stay hidden; the current
                // column keeps its highlight tint.
                i === current
                  ? "bg-[color-mix(in_oklab,var(--tint)_10%,var(--card))]"
                  : "bg-card",
                i === current
                  ? "font-semibold text-amb-300"
                  : "text-muted-foreground",
                i < current && "text-muted-foreground/40",
              )}
              style={{ gridColumn: i + 2, gridRow: 1 }}
              aria-current={i === current ? "step" : undefined}
            >
              {t("effects.column", { n })}
            </span>
          ))}
          {shown.map((b, row) => (
            <EffectRow
              key={b.id}
              buff={b}
              row={row + 2}
              turn={turn}
              start={start}
              size={size}
              onEdit={() => {
                setEditing(b.id);
              }}
            />
          ))}
          {buffs.length === 0 && (
            <span
              className="text-body text-muted-foreground italic"
              style={{ gridColumn: "1 / -1", gridRow: 2 }}
            >
              {t("effects.none")}
            </span>
          )}
        </div>
      </div>
      {edited && (
        <EditBuffForm
          key={edited.id}
          buff={edited}
          onDone={() => setEditing(null)}
        />
      )}
    </div>
  );
}

/** Columns that fit the track's width, following resizes. */
function useTrackSize(ref: React.RefObject<HTMLDivElement | null>): TrackSize {
  const [size, setSize] = useState<TrackSize>(12);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => {
      const label = el.querySelector<HTMLElement>(
        "[data-testid=effects-track] > :nth-child(2)",
      );
      setSize(trackSize(el.clientWidth - (label?.offsetWidth ?? 0)));
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);
  return size;
}

function place(run: ColumnRun, row: number): CSSProperties {
  return { gridColumn: `${run.from + 2} / ${run.to + 3}`, gridRow: row };
}

function EffectRow({
  buff: b,
  row,
  turn,
  start,
  size,
  onEdit,
}: {
  buff: BuffEffect;
  row: number;
  turn: number;
  start: number;
  size: number;
  onEdit: () => void;
}) {
  const { t } = useTranslation();
  const { triggerBuff, dismissBuff } = useBuffsStore();
  const { run, busy } = useTableStore();
  const inactive = b.applies === false;
  const lastTurn = b.duration === 1;
  const span = effectSpan({
    turn,
    start,
    size,
    duration: b.duration,
    startRound: b.startRound,
  });
  const reason =
    b.effects.min_str !== undefined
      ? t("buffs.inactive", { min: b.effects.min_str })
      : t("buffs.replaced");
  const reasonHint =
    b.effects.min_str !== undefined
      ? t("buffs.inactiveHint", { min: b.effects.min_str })
      : t("buffs.replacedHint");
  const effectsText =
    !inactive &&
    (b.effects.weapon_buff !== undefined || b.effects.multiplier !== undefined)
      ? [
          b.effects.weapon_buff !== undefined &&
            t("buffs.effWeapon", { n: b.effects.weapon_buff }),
          b.effects.multiplier !== undefined &&
            t("buffs.effDice", { n: b.effects.multiplier }),
        ]
          .filter(Boolean)
          .join(" · ")
      : null;

  return (
    // display: contents keeps the row's cells on the parent grid while the row stays one element
    // (data-testid="buff") to scope its buttons.
    <div
      className="contents"
      data-testid="buff"
      data-buff={b.name}
      data-applies={String(!inactive)}
      data-last-turn={String(lastTurn)}
    >
      <div
        className="flex min-w-0 items-center gap-1 pr-2"
        style={{ gridColumn: 1, gridRow: row }}
      >
        <span
          className="min-w-0 flex-1 truncate"
          title={[b.name, effectsText, b.note].filter(Boolean).join(" · ")}
        >
          <span
            className={cn(
              "text-body font-medium",
              lastTurn && "text-warning",
              inactive && "text-muted-foreground",
            )}
          >
            {b.name}
          </span>
          {(b.note || effectsText) && (
            <span className="text-label text-muted-foreground">
              {" · "}
              {effectsText && (
                <span data-testid="buff-effects">{effectsText}</span>
              )}
              {effectsText && b.note && " · "}
              {b.note && (
                <span className="italic" data-testid="buff-note">
                  {b.note}
                </span>
              )}
            </span>
          )}
        </span>
        {b.charges > 0 && (
          <Button
            size="icon-xs"
            variant="ghost"
            disabled={busy}
            aria-label={t("buffs.absorb", { name: b.name })}
            title={t("buffs.absorb", { name: b.name })}
            onClick={() => void run(() => triggerBuff(b.id))}
          >
            <Shield />
          </Button>
        )}
        <Button
          size="icon-xs"
          variant="ghost"
          aria-label={t("buffs.edit", { name: b.name })}
          title={t("buffs.edit", { name: b.name })}
          onClick={onEdit}
        >
          <PencilSimple />
        </Button>
        <Button
          size="icon-xs"
          variant="ghost"
          disabled={busy}
          aria-label={t("buffs.dismiss", { name: b.name })}
          title={t("buffs.dismiss", { name: b.name })}
          onClick={() => void run(() => dismissBuff(b.id))}
        >
          <X />
        </Button>
      </div>
      {span.past && (
        <div
          aria-hidden
          className="h-0.5 self-center rounded-full bg-neu-700"
          style={place(span.past, row)}
        />
      )}
      {span.remaining && (
        <div
          className={cn(
            "flex h-5 min-w-0 items-center gap-1 overflow-hidden rounded-full px-2 text-label whitespace-nowrap tabular-nums",
            inactive
              ? "border border-dashed border-neu-500 text-muted-foreground"
              : lastTurn
                ? "bg-warning/20 text-warning ring-1 ring-warning ring-inset"
                : b.duration > 0
                  ? "bg-amb-800 text-amb-200 ring-1 ring-amb-600 ring-inset"
                  : "text-amb-200",
            span.overflows &&
              "[mask-image:linear-gradient(to_right,black_75%,transparent)]",
          )}
          style={place(span.remaining, row)}
          title={
            inactive ? reasonHint : lastTurn ? t("effects.lastTurn") : undefined
          }
          data-testid="effect-bar"
        >
          {inactive && <LockSimple aria-hidden className="shrink-0" />}
          {b.duration > 0 && (
            <span data-testid="buff-rounds">
              {formatTurnsLeft(t, b.duration)}
            </span>
          )}
          {b.duration === 0 && b.charges > 0 && (
            <Pips left={b.charges} total={b.chargesTotal} />
          )}
          {b.charges > 0 && (
            <span data-testid="buff-charges">
              {b.chargesTotal >= b.charges
                ? t("effects.chargesOf", {
                    count: b.charges,
                    total: b.chargesTotal,
                  })
                : t("effects.charges", { count: b.charges })}
              {b.duration === 0 && ` · ${t("effects.untilSpent")}`}
            </span>
          )}
          {b.duration === 0 && b.charges === 0 && !inactive && (
            <span>{t("effects.untilDismissed")}</span>
          )}
          {inactive && <span className="italic">{reason}</span>}
        </div>
      )}
    </div>
  );
}

/**
 * One diamond per charge (at most 12 drawn): filled while left, hollow once used. `total` is what the
 * server kept from the start (0: unknown, only the ones left are drawn).
 */
function Pips({ left, total }: { left: number; total: number }) {
  const size = Math.min(Math.max(total, left), 12);
  return (
    <span aria-hidden className="flex shrink-0 items-center gap-1">
      {Array.from({ length: size }, (_, i) => (
        <span
          key={i}
          data-testid={i < left ? "pip-left" : "pip-used"}
          className={cn(
            "size-2 rotate-45 rounded-[1px]",
            i < left ? "bg-amb-400" : "border border-amb-600",
          )}
        />
      ))}
    </span>
  );
}
