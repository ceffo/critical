import { useState, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { Sparkle } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useFilStore } from "@/stores/fil";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { useTableStore } from "@/stores/table";
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
import { fieldClass } from "./fieldStyles";
import { RedoButton } from "./RedoButton";
import { UndoButton } from "./UndoButton";
import { KIND_ICONS, VALUE_TONES } from "./journalStyle";
import { useDiceStore } from "@/stores/dice";
import { dicePayload } from "@/lib/logSummary";

const FILTERS: LogFilter[] = ["all", "rolls"];

/**
 * The journal (GET /api/log): a filter (all / rolls), Undo, and the entries grouped by turn, newest
 * first. A click on an entry shows it in the detail card. The log is a stack: only the newest
 * non-voided entry offers Void (pop); any entry can get a note, values are never edited.
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
      <div className="flex shrink-0 items-center gap-2 px-3 pt-3 pb-2">
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
        <span className="ml-auto flex gap-1">
          <UndoButton />
          <RedoButton />
        </span>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto px-3 pb-3">
        {groups.map((g) => (
          <section
            key={g.key}
            className="mb-2"
            aria-label={turnLabel(t, g.round)}
          >
            <h3 className="sticky top-0 z-10 flex items-center gap-2 bg-card px-2 py-1">
              <span className="text-body font-medium">
                {turnLabel(t, g.round)}
              </span>
              {g.current && (
                <span className="rounded-md bg-amb-900 px-1.5 text-label text-amb-300">
                  {t("log.current")}
                </span>
              )}
            </h3>
            {g.entries.length === 0 ? (
              <p className="px-2 text-body text-muted-foreground">
                {entries.length === 0
                  ? t("log.empty")
                  : filter === "rolls"
                    ? t("fil.noRollsThisTurn")
                    : t("log.nothingThisRound")}
              </p>
            ) : (
              <div className="flex flex-col gap-1">
                {g.transactions.map((tx) => {
                  const rows = (
                    <ul className="flex flex-col gap-1">
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
                        "rounded-lg border-l-2 bg-neu-900/30 py-0.5 pl-1",
                        undone ? "border-neu-700" : "border-amb-500",
                      )}
                      data-testid="log-transaction"
                      data-undone={undone}
                    >
                      {undone && (
                        <p className="px-2 text-label text-danger">
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

type Mode = "view" | "amend";

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
  const [mode, setMode] = useState<Mode>("view");
  const voided = e.status === "voided";
  const amended = e.status === "amended";
  const isHp = e.kind === "hp_change";
  const text = entryText(e, t, spellName);
  const capped = cappedHeal(e);
  const value = rowValue(e);
  const KindIcon = KIND_ICONS[e.kind] ?? Sparkle;
  const kindLabel = t(`log.kinds.${e.kind}`, { defaultValue: e.kind });
  // A click anywhere on the row but its own controls shows the entry in the card.
  const onRowClick = (ev: MouseEvent<HTMLLIElement>) => {
    if ((ev.target as HTMLElement).closest("button, input, form, a")) return;
    onSelect();
  };

  return (
    <li
      className={cn(
        "relative cursor-pointer rounded-lg px-2 py-1",
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
        className="grid w-full grid-cols-[1.125rem_minmax(0,1fr)_auto] items-center gap-x-2 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        aria-pressed={selected}
        data-testid="log-select"
        onClick={onSelect}
      >
        <span className="flex text-muted-foreground" title={kindLabel}>
          <KindIcon aria-hidden className="size-[1.125rem]" />
          <span className="sr-only">{kindLabel}</span>
        </span>
        <span
          className={cn(
            "min-w-0 truncate text-body",
            voided && "text-muted-foreground line-through",
          )}
          title={text}
        >
          {text}
        </span>
        {value && (
          <span
            className={cn(
              "text-body font-semibold tabular-nums",
              voided
                ? "text-muted-foreground line-through"
                : VALUE_TONES[value.tone],
            )}
            data-testid={isHp ? "log-delta" : "log-value"}
          >
            {value.text}
          </span>
        )}
      </button>
      <div className="flex min-h-6 flex-wrap items-center gap-x-2 pl-[1.625rem] text-label text-muted-foreground">
        <span className="tabular-nums">
          {formatTime(e.createdAt, i18n.language)}
        </span>
        {amended && (
          <>
            <span className="rounded-md bg-neu-900 px-1.5">
              {t("log.status.amended")}
            </span>
            {isHp && (
              <span>{t("log.was", { delta: formatSigned(e.hpDelta) })}</span>
            )}
          </>
        )}
        {capped && (
          <span data-testid="log-capped">
            {t(capped.hpRequested < 0 ? "log.floored" : "log.capped", {
              requested: formatSigned(capped.hpRequested),
            })}
          </span>
        )}
        {voided && !(grouped && undoneGroup) && (
          <span className="text-danger">{t("log.status.voided")}</span>
        )}
        {!voided && mode === "view" && (
          <span className="ml-auto flex gap-1">
            {diceFormula !== undefined && (
              <Button
                size="xs"
                variant="ghost"
                disabled={rolling}
                title={t("dice.rerollTitle", { formula: diceFormula })}
                onClick={() => void reroll(diceFormula)}
              >
                {t("dice.reroll")}
              </Button>
            )}
            <Button size="xs" variant="ghost" onClick={() => setMode("amend")}>
              {t("log.amend")}
            </Button>
          </span>
        )}
      </div>
      {e.note && (
        <p
          className="mt-1 pl-[1.625rem] text-label italic text-muted-foreground"
          data-testid="log-note"
        >
          {e.note}
        </p>
      )}
      {mode === "amend" && !voided && (
        <AmendForm entry={e} onDone={() => setMode("view")} />
      )}
    </li>
  );
}

function AmendForm({
  entry: e,
  onDone,
}: {
  entry: LogEntry;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const { amendEntry, busy } = useTableStore();
  const [note, setNote] = useState(e.note);

  return (
    <form
      className="mt-2 flex items-end gap-2 rounded-md bg-neu-900/60 p-2"
      aria-label={t("log.amendLabel")}
      onSubmit={async (ev) => {
        ev.preventDefault();
        if (await amendEntry(e.id, note.trim())) onDone();
      }}
    >
      <label className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-label text-muted-foreground">
          {t("log.note")}
        </span>
        <input
          className={fieldClass}
          type="text"
          maxLength={500}
          value={note}
          onChange={(ev) => setNote(ev.target.value)}
          autoFocus
        />
      </label>
      <Button size="xs" type="button" variant="ghost" onClick={onDone}>
        {t("log.cancel")}
      </Button>
      <Button size="xs" type="submit" disabled={busy}>
        {t("log.save")}
      </Button>
    </form>
  );
}
