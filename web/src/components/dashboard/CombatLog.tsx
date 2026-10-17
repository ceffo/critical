import type { MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { Campfire, Sparkle } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useFilStore } from "@/stores/fil";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { groupByRound, isUndone } from "@/lib/groupByRound";
import { formatSigned, formatTime } from "@/lib/format";
import { entryText } from "@/lib/logSummary";
import { useSpellName } from "@/i18n/spellName";
import { useInventoryName } from "@/i18n/inventoryName";
import {
  cappedHeal,
  cardEntryId,
  cardSource,
  filterEntries,
  rowValue,
  turnLabel,
  turnMarker,
  type LogFilter,
} from "@/lib/filCard";
import { cn } from "@/lib/utils";
import type { LogEntry } from "@/types/log";
import { RedoButton } from "./RedoButton";
import { UndoButton } from "./UndoButton";
import { KIND_ICONS, VALUE_TONES } from "./journalStyle";
import { useDiceStore } from "@/stores/dice";
import { dicePayload } from "@/lib/logSummary";

const FILTERS: LogFilter[] = ["all", "rolls"];

/**
 * The journal (GET /api/log, ADR 0022 step 5): a head with the All / Rolls filter, the entry count,
 * Undo and Redo, then the entries grouped by turn, newest first, one 22 px line each: value · icon
 * · entry with the time right after it (ADR 0027 step 9). A narrow marker ("T3", an icon out of
 * combat) sits beside a turn's first entry, the current turn's in the tint; turn blocks touch and
 * alternate between the card's tone and the sub-band. A click on an entry shows it in the result
 * panel. The log is a stack: Undo pops the newest entry, values are never edited (and there is no
 * note to add).
 */
export function CombatLog() {
  const { t } = useTranslation();
  const entries = useLogStore((s) => s.entries);
  const round = useBuffsStore((s) => s.round);
  const last = useRollStore((s) => s.last);
  const { selectedId, filter, setFilter, select } = useFilStore();
  const shownId = cardEntryId(cardSource(last, entries, selectedId));
  const groups = groupByRound(filterEntries(entries, filter), round);

  return (
    <section
      className="zone-card flex min-h-0 min-w-0 flex-1 flex-col"
      aria-label={t("log.title")}
    >
      <div className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 px-4 pt-3.5 pb-2">
        <h2 className="section-label">{t("log.title")}</h2>
        <div
          role="radiogroup"
          aria-label={t("fil.filter.label")}
          className="flex rounded-lg bg-raised p-0.5"
        >
          {FILTERS.map((f) => (
            <button
              key={f}
              type="button"
              role="radio"
              aria-checked={filter === f}
              onClick={() => setFilter(f)}
              className={cn(
                "h-6 rounded-md px-3 text-body transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50",
                filter === f
                  ? "chip-chosen font-medium"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {t(`fil.filter.${f}`)}
            </button>
          ))}
        </div>
        <span
          className="text-body text-muted-foreground tabular-nums"
          data-testid="journal-count"
        >
          {t("shell.journal.count", { count: entries.length })}
        </span>
        <span className="ml-auto flex gap-1">
          <UndoButton />
          <RedoButton />
        </span>
      </div>
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto pb-2">
        {groups.map((g, i) => {
          const marker = turnMarker(t, g.round, g.current);
          return (
            <section
              key={g.key}
              className={cn(
                // The narrow turn marker, then the turn's entries. Turn blocks touch; every other
                // one is on the sub-band tone, never a rule between them (ADR 0027 step 9.1).
                "grid grid-cols-[2.25rem_minmax(0,1fr)] items-start gap-x-1 px-2",
                i % 2 === 1 && "bg-sub-band",
              )}
              aria-label={turnLabel(t, g.round)}
              data-current={g.current}
              data-testid="journal-turn"
            >
              <h3
                className={cn(
                  "flex h-[1.375rem] items-center justify-center text-label font-semibold tabular-nums",
                  // The current turn's one mark: its marker in the tint (ADR 0027 step 9.1).
                  g.current ? "text-tint" : "text-muted-foreground",
                )}
                title={marker.full}
                data-testid="journal-turn-marker"
              >
                {marker.short ? (
                  <span aria-hidden>{marker.short}</span>
                ) : (
                  <Campfire aria-hidden className="size-4" />
                )}
                <span className="sr-only">{marker.full}</span>
              </h3>
              {g.entries.length === 0 ? (
                <p className="flex min-h-[1.375rem] items-center px-2 text-body text-muted-foreground">
                  {entries.length === 0
                    ? t("log.empty")
                    : filter === "rolls"
                      ? t("fil.noRollsThisTurn")
                      : t("log.nothingThisRound")}
                </p>
              ) : (
                <div className="flex min-w-0 flex-col">
                  {g.transactions.map((tx) => {
                    const rows = (
                      <ul className="flex flex-col">
                        {tx.map((e) => (
                          <LogRow
                            key={e.id}
                            entry={e}
                            selected={e.id === shownId}
                            onSelect={() => select(e.id)}
                            grouped={tx.length > 1}
                            undoneGroup={isUndone(tx)}
                          />
                        ))}
                      </ul>
                    );
                    if (tx.length === 1)
                      return <div key={tx[0].id}>{rows}</div>;
                    // One request recorded several entries (ADR 0018): framed as one unit, and
                    // undone as one.
                    const undone = isUndone(tx);
                    return (
                      <div
                        key={tx[0].id}
                        className={cn(
                          "rounded-md border-l-2 bg-raised",
                          undone ? "border-input" : "border-tint",
                        )}
                        data-testid="log-transaction"
                        data-undone={undone}
                      >
                        {undone && (
                          <p className="px-2 pt-0.5">
                            <VoidedTag />
                          </p>
                        )}
                        {rows}
                      </div>
                    );
                  })}
                </div>
              )}
            </section>
          );
        })}
      </div>
    </section>
  );
}

/** "Undone" as a muted tag, never red (ADR 0026 step 5.8, ADR 0027 step 9.5). */
function VoidedTag({ className }: { className?: string }) {
  const { t } = useTranslation();
  return (
    <span
      className={cn("chip px-1.5 text-label text-muted-foreground", className)}
    >
      {t("log.status.voided")}
    </span>
  );
}

function LogRow({
  entry: e,
  selected,
  onSelect,
  grouped,
  undoneGroup,
}: {
  entry: LogEntry;
  selected: boolean;
  onSelect: () => void;
  /** In a multi-entry transaction: an undone one says so once on its frame, not on each row. */
  grouped: boolean;
  undoneGroup: boolean;
}) {
  const { t, i18n } = useTranslation();
  const spellName = useSpellName();
  const names = useInventoryName();
  const reroll = useDiceStore((s) => s.roll);
  const rolling = useDiceStore((s) => s.rolling);
  const diceFormula = dicePayload(e)?.formula;
  const voided = e.status === "voided";
  const amended = e.status === "amended";
  const isHp = e.kind === "hp_change";
  const text = entryText(e, t, spellName, names);
  const capped = cappedHeal(e);
  const value = rowValue(e);
  const KindIcon = KIND_ICONS[e.kind] ?? Sparkle;
  const kindLabel = t(`log.kinds.${e.kind}`, { defaultValue: e.kind });
  // A click anywhere on the row but its own controls shows the entry in the result panel.
  const onRowClick = (ev: MouseEvent<HTMLLIElement>) => {
    if ((ev.target as HTMLElement).closest("button, input, form, a")) return;
    onSelect();
  };

  return (
    <li
      className={cn(
        // One 22 px line (ADR 0027 step 9.2): value · icon · entry with the time right after it (the
        // select button), then the row action at the row's end. A long entry wraps inside its row;
        // nothing is drawn under it.
        "group/row relative flex cursor-pointer items-start gap-x-2 rounded-md px-2 py-0.5",
        selected ? "selected" : "hover:bg-raised-hover",
      )}
      data-status={e.status}
      data-entry-id={e.id}
      data-selected={selected}
      onClick={onRowClick}
    >
      <button
        type="button"
        className="flex min-w-0 flex-1 items-start gap-x-2 rounded-sm text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        aria-pressed={selected}
        data-testid="log-select"
        onClick={onSelect}
      >
        <span
          className={cn(
            "w-10 shrink-0 text-right text-body font-semibold tabular-nums",
            voided
              ? "text-muted-foreground line-through"
              : value && VALUE_TONES[value.tone],
          )}
          data-testid={value ? (isHp ? "log-delta" : "log-value") : undefined}
          data-tone={value && !voided ? value.tone : undefined}
        >
          {value?.text}
        </span>
        <span
          className="flex h-[1.125rem] shrink-0 items-center text-muted-foreground"
          title={kindLabel}
        >
          <KindIcon aria-hidden className="size-[1.125rem]" />
          <span className="sr-only">{kindLabel}</span>
        </span>
        <span className="min-w-0 text-body break-words">
          <span className={cn(voided && "text-muted-foreground line-through")}>
            {text}
          </span>
          {amended && (
            <span className="chip ml-2 px-1.5 text-label text-muted-foreground">
              {t("log.status.amended")}
              {isHp && ` · ${t("log.was", { delta: formatSigned(e.hpDelta) })}`}
            </span>
          )}
          {capped && (
            <span
              className="ml-2 text-label text-muted-foreground"
              data-testid="log-capped"
            >
              {t(capped.hpRequested < 0 ? "log.floored" : "log.capped", {
                requested: formatSigned(capped.hpRequested),
              })}
            </span>
          )}
          {voided && !(grouped && undoneGroup) && (
            <VoidedTag className="ml-2" />
          )}
        </span>
        {/* The time right after the entry text (ADR 0027 step 9.4). */}
        <span className="shrink-0 text-label leading-[1.125rem] text-muted-foreground tabular-nums">
          {formatTime(e.createdAt, i18n.language)}
        </span>
      </button>
      {!voided && diceFormula !== undefined && (
        // On hover, on keyboard focus in the row, and on the selected row (ADR 0022 step 5.3).
        <Button
          size="xs"
          variant="ghost"
          className={cn(
            "h-[1.125rem] shrink-0",
            "invisible group-hover/row:visible group-focus-within/row:visible",
            selected && "visible",
          )}
          disabled={rolling}
          title={t("dice.rerollTitle", { formula: diceFormula })}
          data-testid="log-reroll"
          onClick={() => void reroll(diceFormula)}
        >
          {t("dice.reroll")}
        </Button>
      )}
    </li>
  );
}
