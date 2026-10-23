import { useState, type CSSProperties } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import {
  CaretDown,
  CaretUp,
  LockSimple,
  PencilSimple,
  Shield,
  X,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { buffName, useSpellName } from "@/i18n/spellName";
import { powerEffectsText, powerRows, type PowerRow } from "@/lib/itemPowers";
import { formatRoundsLeft } from "@/lib/passTime";
import {
  TRACK_COLUMNS,
  effectSpan,
  windowStart,
  type ColumnRun,
} from "@/lib/timeline";
import { cn } from "@/lib/utils";
import { useBuffsStore } from "@/stores/buffs";
import { useItemsStore } from "@/stores/items";
import { useTableStore } from "@/stores/table";
import type { BuffEffect } from "@/types/buffs";
import { AddEffect } from "./AddEffect";
import { EditBuffForm } from "./BuffForms";

/**
 * The name column, then one column per round of the window (ADR 0027 step 4.6): the name column is as
 * wide as the longest effect name, the hidden ones included, so it does not jump when the expander
 * opens; past the cap (a player's very long custom name) the name wraps, never cut. The round columns
 * share the rest of the width.
 */
const NAME_COLUMN = "fit-content(clamp(14rem, 22vw, 26rem))";

/** Effect rows shown at rest; more than this and an expander shows the rest (ADR 0017 step 6). */
const REST_ROWS = 3;

/** A row of the track: an active buff, or a carried item's power that is on (ADR 0016 step 13). */
type TrackRow =
  | { kind: "buff"; key: string; buff: BuffEffect }
  | { kind: "power"; key: string; power: PowerRow };

/**
 * The effects track: one row per active buff over a window of rounds that follows the current one,
 * then one per item power that is on (named by its item, nothing to edit or dismiss).
 * A timed buff is a thin line for the rounds it has run and a pill for what is left (from the
 * server's `duration`), in the warning colour on its last round; a charge buff shows its charges as
 * pips; a buff that does not apply is dashed with a lock and the reason. Absorb, edit and remove
 * show on the row's hover and keyboard focus (ADR 0027 step 4.4); Add stays next to the label. Five
 * round columns at every width (step 4.3), sharing the width the header leaves. At rest the track shows its round row and up to 3 effect rows, no scrollbar; with more effects an
 * expander (aria-expanded) grows the track in place to show them all, capped at 40% of the viewport
 * height where it scrolls (ADR 0017 step 6).
 */
export function EffectsTimeline() {
  const { t, i18n } = useTranslation();
  const buffs = useBuffsStore((s) => s.buffs);
  const powers = useBuffsStore((s) => s.powers);
  const items = useItemsStore((s) => s.items);
  const round = useBuffsStore((s) => s.round);
  const [editing, setEditing] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);
  const edited = buffs.find((b) => b.id === editing);
  const size = TRACK_COLUMNS;
  const rows: TrackRow[] = [
    ...buffs.map((b): TrackRow => ({ kind: "buff", key: b.id, buff: b })),
    ...powerRows(powers, items, i18n.language).map((p): TrackRow => ({
      kind: "power",
      key: `power:${p.key}`,
      power: p,
    })),
  ];
  const hidden = Math.max(rows.length - REST_ROWS, 0);
  const shown = expanded ? rows : rows.slice(0, REST_ROWS);
  const start = windowStart(round);
  const current = round - start;
  // Out of combat (round 0) no round is counted: the numbers row and the current-round highlight go,
  // the row itself stays so the effect rows do not move (ADR 0027 step 4.3).
  const inCombat = round > 0;
  const columns = Array.from({ length: size }, (_, i) => start + i);
  const grid: CSSProperties = {
    gridTemplateColumns: `${NAME_COLUMN} repeat(${size}, minmax(3rem, 1fr))`,
    // An effect row grows when its name wraps (names are never truncated).
    gridTemplateRows: `1.5rem repeat(${Math.max(shown.length, 1)}, minmax(1.5rem, auto))`,
  };

  return (
    <div
      className="flex flex-1 basis-0 flex-col gap-2"
      role="group"
      aria-label={t("effects.title")}
    >
      <div className={cn(expanded && "max-h-[40vh] overflow-y-auto")}>
        <div
          className="relative grid items-center"
          style={grid}
          data-testid="effects-track"
          data-columns={size}
        >
          {/* The current round, highlighted across every row (in combat only). */}
          {inCombat && (
            <div
              aria-hidden
              data-testid="effects-current-column"
              className="pointer-events-none self-stretch rounded-md bg-tint/10 ring-1 ring-tint/30 ring-inset"
              style={{ gridColumn: current + 2, gridRow: "1 / -1" }}
            />
          )}
          <div
            className="sticky top-0 z-10 flex h-full items-center gap-2 bg-card"
            style={{ gridColumn: 1, gridRow: 1 }}
          >
            <span className="section-label">{t("effects.title")}</span>
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
                // Opaque, so effect rows scrolled under the sticky round row stay hidden; the current
                // column keeps its highlight tint.
                inCombat && i === current
                  ? "bg-[color-mix(in_oklab,var(--tint)_10%,var(--card))]"
                  : "bg-card",
                inCombat && i === current
                  ? "font-semibold text-tint"
                  : "text-muted-foreground",
                i < current && "text-muted-foreground/40",
              )}
              style={{ gridColumn: i + 2, gridRow: 1 }}
              aria-current={inCombat && i === current ? "step" : undefined}
              data-testid="effects-round-number"
            >
              {inCombat ? t("effects.column", { n }) : null}
            </span>
          ))}
          {shown.map((r, row) =>
            r.kind === "buff" ? (
              <EffectRow
                key={r.key}
                buff={r.buff}
                row={row + 2}
                round={round}
                start={start}
                size={size}
                onEdit={() => {
                  setEditing(r.buff.id);
                }}
              />
            ) : (
              <ItemPowerRow
                key={r.key}
                power={r.power}
                row={row + 2}
                round={round}
                start={start}
                size={size}
              />
            ),
          )}
          {/* Every row's label, hidden and zero-high, sizes the name column (shown or not). */}
          {rows.map((r) => (
            <span
              key={`ghost:${r.key}`}
              aria-hidden
              className="invisible h-0 pr-2"
              style={{ gridColumn: 1, gridRow: 1 }}
              data-testid="effect-name-ghost"
            >
              {r.kind === "buff" ? (
                <BuffLabel buff={r.buff} ghost />
              ) : (
                <PowerLabel power={r.power} ghost />
              )}
            </span>
          ))}
          {rows.length === 0 && (
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

function place(run: ColumnRun, row: number): CSSProperties {
  return { gridColumn: `${run.from + 2} / ${run.to + 3}`, gridRow: row };
}

function EffectRow({
  buff: b,
  row,
  round,
  start,
  size,
  onEdit,
}: {
  buff: BuffEffect;
  row: number;
  round: number;
  start: number;
  size: number;
  onEdit: () => void;
}) {
  const { t } = useTranslation();
  const name = buffName(useSpellName(), b);
  const { triggerBuff } = useBuffsStore();
  const { run, busy, dismissBuff } = useTableStore();
  const inactive = b.applies === false;
  const lastRound = b.duration === 1;
  const span = effectSpan({
    round,
    start,
    size,
    duration: b.duration,
    startRound: b.startRound,
  });
  // ADR 0004 step 20.5: a spell cast on someone else or on an area only counts down here.
  const reason = b.onOther
    ? t("buffs.onOther")
    : b.effects.min_str !== undefined
      ? t("buffs.inactive", { min: b.effects.min_str })
      : t("buffs.replaced");
  const reasonHint = b.onOther
    ? t("buffs.onOtherHint")
    : b.effects.min_str !== undefined
      ? t("buffs.inactiveHint", { min: b.effects.min_str })
      : t("buffs.replacedHint");
  return (
    // display: contents keeps the row's cells on the parent grid while the row stays one element
    // (data-testid="buff") to scope its buttons.
    <div
      className="group/row contents"
      data-testid="buff"
      data-buff={b.name}
      data-applies={String(!inactive)}
      data-last-round={String(lastRound)}
    >
      <div
        className="relative flex min-w-0 items-center pr-2"
        style={{ gridColumn: 1, gridRow: row }}
      >
        <BuffLabel buff={b} />
        {/* Every-fight actions, out of the way at rest (ADR 0027 steps 1.4 and 4.4): they show on
            the row's hover and on keyboard focus, over the end of the name cell. */}
        <div
          className="absolute inset-y-0 right-0 flex items-center gap-0.5 bg-card pl-1 opacity-0 transition-opacity group-focus-within/row:opacity-100 group-hover/row:opacity-100"
          data-testid="buff-actions"
        >
          {b.charges > 0 && (
            <Button
              size="icon-xs"
              variant="ghost"
              disabled={busy}
              aria-label={t("buffs.absorb", { name })}
              title={t("buffs.absorb", { name })}
              onClick={() => void run(() => triggerBuff(b.id))}
            >
              <Shield />
            </Button>
          )}
          <Button
            size="icon-xs"
            variant="ghost"
            aria-label={t("buffs.edit", { name })}
            title={t("buffs.edit", { name })}
            onClick={onEdit}
          >
            <PencilSimple />
          </Button>
          {/* ADR 0004 step 21: a row whose spell has end effects asks how it ended. */}
          {b.asksEndCause ? (
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    size="icon-xs"
                    variant="ghost"
                    disabled={busy}
                    aria-label={t("buffs.dismiss", { name })}
                    title={t("buffs.endHint", { name })}
                    data-testid="buff-stop"
                  />
                }
              >
                <X />
              </DropdownMenuTrigger>
              <DropdownMenuContent>
                <DropdownMenuItem
                  data-testid="buff-end-voluntary"
                  onClick={() => void dismissBuff(b.id, "voluntary")}
                >
                  {t("buffs.endBy")}
                </DropdownMenuItem>
                <DropdownMenuItem
                  data-testid="buff-end-dispelled"
                  onClick={() => void dismissBuff(b.id, "dispelled")}
                >
                  {t("buffs.endDispelled")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <Button
              size="icon-xs"
              variant="ghost"
              disabled={busy}
              aria-label={t("buffs.dismiss", { name })}
              title={t("buffs.dismiss", { name })}
              onClick={() => void dismissBuff(b.id)}
            >
              <X />
            </Button>
          )}
        </div>
      </div>
      {span.past && (
        <div
          aria-hidden
          className="h-0.5 self-center rounded-full bg-input"
          style={place(span.past, row)}
        />
      )}
      {span.remaining && (
        <div
          className={cn(
            "flex h-5 min-w-0 items-center gap-1 overflow-hidden rounded-full px-2 text-label whitespace-nowrap tabular-nums",
            inactive
              ? "border border-dashed border-neu-500 text-muted-foreground"
              : lastRound
                ? "bg-warning/20 text-warning ring-1 ring-warning ring-inset"
                : b.duration > 0
                  ? "bg-selected text-amb-200 ring-1 ring-tint/70 ring-inset"
                  : "text-amb-200",
            span.overflows &&
              "[mask-image:linear-gradient(to_right,black_75%,transparent)]",
          )}
          style={place(span.remaining, row)}
          title={
            inactive
              ? reasonHint
              : lastRound
                ? t("effects.lastRound")
                : undefined
          }
          data-testid="effect-bar"
        >
          {inactive && <LockSimple aria-hidden className="shrink-0" />}
          {b.duration > 0 && (
            <span data-testid="buff-rounds">
              {formatRoundsLeft(t, b.duration)}
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

/** What a buff changes, beside its name ("weapon +2 · dice ×2"); none when it does not apply. */
function buffEffectsText(t: TFunction, b: BuffEffect): string | null {
  if (b.applies === false) return null;
  const parts = [
    // A mind link (ADR 0044 step 3): the row's name is the opponent; say what is held on it.
    b.target &&
      (b.tangents
        ? t("buffs.tangentsHeld", { count: b.tangents })
        : t("buffs.contactHeld")),
    b.effects.weapon_buff !== undefined &&
      t("buffs.effWeapon", { n: b.effects.weapon_buff }),
    b.effects.multiplier !== undefined &&
      t("buffs.effDice", { n: b.effects.multiplier }),
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(" · ") : null;
}

/**
 * A buff's name cell text: the name (warning on its last round, muted when it does not apply), then
 * what it changes and the player's note. Also drawn, hidden, to size the name column.
 */
function BuffLabel({ buff: b, ghost }: { buff: BuffEffect; ghost?: boolean }) {
  const { t } = useTranslation();
  const name = buffName(useSpellName(), b);
  const effectsText = buffEffectsText(t, b);
  return (
    <span
      className="min-w-0 flex-1 break-words"
      data-testid={ghost ? undefined : "effect-name"}
      title={
        ghost
          ? undefined
          : [name, effectsText, b.note].filter(Boolean).join(" · ")
      }
    >
      <span
        className={cn(
          "text-body font-medium",
          b.duration === 1 && "text-warning",
          b.applies === false && "text-muted-foreground",
        )}
      >
        {name}
      </span>
      {(b.note || effectsText) && (
        <span className="text-label text-muted-foreground">
          {" · "}
          {effectsText && (
            <span data-testid={ghost ? undefined : "buff-effects"}>
              {effectsText}
            </span>
          )}
          {effectsText && b.note && " · "}
          {b.note && (
            <span
              className="italic"
              data-testid={ghost ? undefined : "buff-note"}
            >
              {b.note}
            </span>
          )}
        </span>
      )}
    </span>
  );
}

/** An item power's name cell text: the item's name and the power's effects. */
function PowerLabel({ power: p, ghost }: { power: PowerRow; ghost?: boolean }) {
  const { t } = useTranslation();
  const effectsText = powerEffectsText(t, p.effects);
  return (
    <span
      className="min-w-0 flex-1 break-words"
      data-testid={ghost ? undefined : "effect-name"}
      title={
        ghost
          ? undefined
          : [t("effects.power", { name: p.name }), effectsText]
              .filter(Boolean)
              .join(" · ")
      }
    >
      <span className="text-body font-medium">{p.name}</span>
      {effectsText && (
        <span className="text-label text-muted-foreground">
          {" · "}
          <span data-testid={ghost ? undefined : "power-effects"}>
            {effectsText}
          </span>
        </span>
      )}
    </span>
  );
}

/**
 * An item power that is on: the item's name and the power's effects, a bar from the current round to
 * the edge, filled like a buff's pill and fading out (it lasts while its trigger holds). No counters, no edit or dismiss (ADR 0016 step 13).
 */
function ItemPowerRow({
  power: p,
  row,
  round,
  start,
  size,
}: {
  power: PowerRow;
  row: number;
  round: number;
  start: number;
  size: number;
}) {
  const { t } = useTranslation();
  const span = effectSpan({
    round,
    start,
    size,
    duration: 0,
    startRound: round,
  });
  const lasts =
    p.minStr !== undefined
      ? t("effects.powerOnFrom", { min: p.minStr })
      : t("effects.powerOn");
  return (
    <div className="contents" data-testid="item-power" data-item={p.itemId}>
      <div
        className="flex min-w-0 items-center pr-2"
        style={{ gridColumn: 1, gridRow: row }}
      >
        <PowerLabel power={p} />
      </div>
      {span.remaining && (
        <div
          className={cn(
            "flex h-5 min-w-0 items-center gap-1 overflow-hidden rounded-full px-2 text-label whitespace-nowrap",
            // A filled pill like a timed buff's, but a softer fill and no counter: it lasts while
            // its trigger holds, so the pill runs to the edge and fades out there.
            "bg-selected/60 text-amb-200 ring-1 ring-tint/40 ring-inset",
            span.overflows &&
              "[mask-image:linear-gradient(to_right,black_75%,transparent)]",
          )}
          style={place(span.remaining, row)}
          data-testid="effect-bar"
        >
          <span>{lasts}</span>
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
            i < left ? "bg-tint" : "border border-tint",
          )}
        />
      ))}
    </span>
  );
}
