import type { MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { Sparkle } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useFilStore } from "@/stores/fil";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { groupByRound, isUndone } from "@/lib/groupByRound";
import { formatSigned, formatTime } from "@/lib/format";
import { entryText } from "@/lib/logSummary";
import { useSpellName } from "@/i18n/spellName";
import {
  cappedHeal,
  cardEntryId,
  cardSource,
  filterEntries,
  rowValue,
  turnLabel,
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
 * Undo and Redo, then the entries grouped by turn, newest first, one line each in the columns
 * turn · value · icon · entry · time. The turn label sits beside the first entry of its turn; the
 * current turn is tinted. A click on an entry shows it in the result panel. The log is a stack:
 * Undo pops the newest entry, values are never edited (and there is no note to add).
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
        <h2 className="text-title">{t("log.title")}</h2>
        <div
          role="radiogroup"
          aria-label={t("fil.filter.label")}
          className="flex rounded-lg bg-neu-900 p-0.5"
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
                  ? "bg-card font-medium"
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
      <div className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-2 pb-3">
        {groups.map((g) => (
          <section
            key={g.key}
            className={cn(
              // The turn column, then the turn's entries. A group is marked by its label and, for
              // the current turn, a tint; never a rule between groups (ADR 0022 step 4.1).
              "grid grid-cols-[8.5rem_minmax(0,1fr)] items-start gap-x-3 rounded-lg px-2 py-0.5",
              g.current && "bg-amb-900/40",
            )}
            aria-label={turnLabel(t, g.round)}
            data-current={g.current}
          >
            <h3 className="section-label flex min-h-7 flex-wrap items-center gap-x-1.5">
              <span className={cn(g.current && "text-tint")}>
                {turnLabel(t, g.round)}
              </span>
              {g.current && (
                <span className="rounded-md bg-amb-900 px-1.5 text-tint">
                  {t("log.current")}
                </span>
              )}
            </h3>
            {g.entries.length === 0 ? (
              <p className="flex min-h-7 items-center text-body text-muted-foreground">
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
                  if (tx.length === 1) return <div key={tx[0].id}>{rows}</div>;
                  // One request recorded several entries (ADR 0018): framed as one unit, and
                  // undone as one.
                  const undone = isUndone(tx);
                  return (
                    <div
                      key={tx[0].id}
                      className={cn(
                        "my-0.5 rounded-lg border-l-2 bg-neu-900/30",
                        undone ? "border-neu-700" : "border-amb-500",
                      )}
                      data-testid="log-transaction"
                      data-undone={undone}
                    >
                      {undone && (
                        <p className="px-2 pt-0.5 text-label text-danger">
                          {t("log.status.voided")}
                        </p>
                      )}
                      {rows}
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        ))}
      </div>
    </section>
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
  const reroll = useDiceStore((s) => s.roll);
  const rolling = useDiceStore((s) => s.rolling);
  const diceFormula = dicePayload(e)?.formula;
  const voided = e.status === "voided";
  const amended = e.status === "amended";
  const isHp = e.kind === "hp_change";
  const text = entryText(e, t, spellName);
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
        // One line: value · icon · entry (the select button, on the row's subgrid), then the row
        // action and the time. A long entry wraps inside its cell; nothing is drawn under it.
        "group/row relative grid cursor-pointer grid-cols-[3.5rem_1.125rem_minmax(0,1fr)_auto] items-center gap-x-2 rounded-md px-2 py-1",
        selected
          ? "bg-amb-900 before:absolute before:inset-y-1 before:left-0 before:w-0.5 before:rounded-full before:bg-amb-500"
          : "hover:bg-neu-900/60",
      )}
      data-status={e.status}
      data-entry-id={e.id}
      data-selected={selected}
      onClick={onRowClick}
    >
      <button
        type="button"
        className="col-span-3 grid grid-cols-subgrid items-center rounded-sm text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        aria-pressed={selected}
        data-testid="log-select"
        onClick={onSelect}
      >
        <span
          className={cn(
            "text-right text-body font-semibold tabular-nums",
            voided
              ? "text-muted-foreground line-through"
              : value && VALUE_TONES[value.tone],
          )}
          data-testid={value ? (isHp ? "log-delta" : "log-value") : undefined}
        >
          {value?.text}
        </span>
        <span className="flex text-muted-foreground" title={kindLabel}>
          <KindIcon aria-hidden className="size-[1.125rem]" />
          <span className="sr-only">{kindLabel}</span>
        </span>
        <span className="min-w-0 text-body break-words">
          <span className={cn(voided && "text-muted-foreground line-through")}>
            {text}
          </span>
          {amended && (
            <span className="ml-2 rounded-md bg-neu-900 px-1.5 text-label text-muted-foreground">
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
            <span className="ml-2 text-label text-danger">
              {t("log.status.voided")}
            </span>
          )}
        </span>
      </button>
      <span className="flex items-center justify-end gap-2">
        {!voided && diceFormula !== undefined && (
          // On hover, on keyboard focus in the row, and on the selected row (ADR 0022 step 5.3).
          <Button
            size="xs"
            variant="ghost"
            className={cn(
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
        <span className="text-label text-muted-foreground tabular-nums">
          {formatTime(e.createdAt, i18n.language)}
        </span>
      </span>
    </li>
  );
}
