import { useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import { LockSimple, PencilSimple, Plus, Shield, X } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { formatTurnsLeft } from "@/lib/passTime";
import { effectSpan, trackSize, windowStart, type ColumnRun, type TrackSize } from "@/lib/timeline";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useTableStore } from "@/stores/table";
import type { BuffEffect } from "@/types/buffs";
import { AddBuffForm, EditBuffForm } from "./BuffForms";

/** The label column, then one column per turn of the window. */
const LABEL_COLUMN = "clamp(10rem, 13vw, 15rem)";

/**
 * The effects track: one row per active buff over a window of turns that follows the current one.
 * A timed buff is a thin line for the turns it has run and a pill for what is left (from the
 * server's `duration`), in the warning colour on its last turn; a charge buff shows its charges as
 * pips; a buff that does not apply is dashed with a lock and the reason. Edit and remove per row,
 * Add below the label. The number of turn columns follows the width available (8, 12 or 16).
 */
export function EffectsTimeline() {
  const { t } = useTranslation();
  const buffs = useBuffsStore((s) => s.buffs);
  const turn = useBuffsStore((s) => s.round);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const edited = buffs.find((b) => b.id === editing);
  const trackRef = useRef<HTMLDivElement>(null);
  const size = useTrackSize(trackRef);
  const start = windowStart(turn);
  const current = turn - start;
  const columns = Array.from({ length: size }, (_, i) => start + i);
  const grid: CSSProperties = {
    gridTemplateColumns: `${LABEL_COLUMN} repeat(${size}, minmax(0, 1fr))`,
    gridTemplateRows: `auto repeat(${Math.max(buffs.length, 1)}, minmax(1.75rem, auto))`,
  };

  return (
    <div className="flex min-w-0 flex-1 flex-col gap-2" role="group" aria-label={t("effects.title")}>
      <div ref={trackRef} className="max-h-[clamp(9rem,22vh,16rem)] min-w-0 overflow-y-auto">
        <div className="relative grid items-center gap-y-1" style={grid} data-testid="effects-track" data-columns={size}>
          {/* The current turn, highlighted across every row. */}
          <div
            aria-hidden
            className="pointer-events-none self-stretch rounded-md bg-tint/10 ring-1 ring-tint/30 ring-inset"
            style={{ gridColumn: current + 2, gridRow: "1 / -1" }}
          />
          <div className="flex items-center gap-2" style={{ gridColumn: 1, gridRow: 1 }}>
            <span className="text-sm font-medium">{t("effects.title")}</span>
            <Button
              size="xs"
              variant="ghost"
              onClick={() => {
                setEditing(null);
                setAdding(true);
              }}
            >
              <Plus />
              {t("buffs.add")}
            </Button>
          </div>
          {columns.map((n, i) => (
            <span
              key={n}
              className={cn(
                "text-center text-xs tabular-nums",
                i === current ? "font-semibold text-amb-300" : "text-muted-foreground",
                i < current && "opacity-40",
              )}
              style={{ gridColumn: i + 2, gridRow: 1 }}
              aria-current={i === current ? "step" : undefined}
            >
              {t("effects.column", { n })}
            </span>
          ))}
          {buffs.map((b, row) => (
            <EffectRow
              key={b.id}
              buff={b}
              row={row + 2}
              turn={turn}
              start={start}
              size={size}
              onEdit={() => {
                setAdding(false);
                setEditing(b.id);
              }}
            />
          ))}
          {buffs.length === 0 && (
            <span className="text-sm text-muted-foreground italic" style={{ gridColumn: "1 / -1", gridRow: 2 }}>
              {t("effects.none")}
            </span>
          )}
        </div>
      </div>
      {edited ? (
        <EditBuffForm key={edited.id} buff={edited} onDone={() => setEditing(null)} />
      ) : (
        adding && <AddBuffForm onDone={() => setAdding(false)} />
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
      const label = el.querySelector<HTMLElement>("[data-testid=effects-track] > :nth-child(2)");
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
  const span = effectSpan({ turn, start, size, duration: b.duration, startRound: b.startRound });
  const reason =
    b.effects.min_str !== undefined ? t("buffs.inactive", { min: b.effects.min_str }) : t("buffs.replaced");
  const reasonHint =
    b.effects.min_str !== undefined ? t("buffs.inactiveHint", { min: b.effects.min_str }) : t("buffs.replacedHint");
  const effectsText =
    !inactive && (b.effects.weapon_buff !== undefined || b.effects.multiplier !== undefined)
      ? [
          b.effects.weapon_buff !== undefined && t("buffs.effWeapon", { n: b.effects.weapon_buff }),
          b.effects.multiplier !== undefined && t("buffs.effDice", { n: b.effects.multiplier }),
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
      <div className="flex min-w-0 items-center gap-1 pr-2" style={{ gridColumn: 1, gridRow: row }}>
        <div className="flex min-w-0 flex-1 flex-col leading-tight">
          <span
            className={cn("truncate text-sm font-medium", lastTurn && "text-warning", inactive && "text-muted-foreground")}
            title={b.name}
          >
            {b.name}
          </span>
          {(b.note || effectsText) && (
            <span className="truncate text-xs text-muted-foreground" title={[effectsText, b.note].filter(Boolean).join(" · ")}>
              {effectsText && <span data-testid="buff-effects">{effectsText}</span>}
              {effectsText && b.note && " · "}
              {b.note && (
                <span className="italic" data-testid="buff-note">
                  {b.note}
                </span>
              )}
            </span>
          )}
        </div>
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
        <div aria-hidden className="h-0.5 self-center rounded-full bg-neu-700" style={place(span.past, row)} />
      )}
      {span.remaining && (
        <div
          className={cn(
            "flex h-[1.35rem] min-w-0 items-center gap-1.5 overflow-hidden rounded-full px-2 text-xs whitespace-nowrap tabular-nums",
            inactive
              ? "border border-dashed border-neu-500 text-muted-foreground"
              : lastTurn
                ? "bg-warning/20 text-warning ring-1 ring-warning ring-inset"
                : b.duration > 0
                  ? "bg-amb-800 text-amb-200 ring-1 ring-amb-600 ring-inset"
                  : "text-amb-200",
            span.overflows && "[mask-image:linear-gradient(to_right,black_75%,transparent)]",
          )}
          style={place(span.remaining, row)}
          title={inactive ? reasonHint : lastTurn ? t("effects.lastTurn") : undefined}
          data-testid="effect-bar"
        >
          {inactive && <LockSimple aria-hidden className="shrink-0" />}
          {b.duration > 0 && <span data-testid="buff-rounds">{formatTurnsLeft(t, b.duration)}</span>}
          {b.duration === 0 && b.charges > 0 && <Pips count={b.charges} />}
          {b.charges > 0 && (
            <span data-testid="buff-charges">
              {t("effects.charges", { count: b.charges })}
              {b.duration === 0 && ` · ${t("effects.untilSpent")}`}
            </span>
          )}
          {b.duration === 0 && b.charges === 0 && !inactive && <span>{t("effects.untilDismissed")}</span>}
          {inactive && <span className="italic">{reason}</span>}
        </div>
      )}
    </div>
  );
}

/** One diamond per charge left (at most 12 drawn). */
function Pips({ count }: { count: number }) {
  return (
    <span aria-hidden className="flex shrink-0 items-center gap-1">
      {Array.from({ length: Math.min(count, 12) }, (_, i) => (
        <span key={i} className="size-2 rotate-45 rounded-[1px] bg-amb-400" />
      ))}
    </span>
  );
}
