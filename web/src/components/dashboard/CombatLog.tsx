import { useState, type MouseEvent } from "react";
import { useTranslation } from "react-i18next";
import { Campfire, CaretDown, CaretRight } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { useBuffsStore } from "@/stores/buffs";
import { useJournalStore } from "@/stores/journal";
import { useSettingsStore } from "@/stores/settings";
import { useLogStore } from "@/stores/log";
import { useRollStore } from "@/stores/roll";
import { groupByRound, isUndone, opensRound } from "@/lib/groupByRound";
import { formatSigned, formatTime } from "@/lib/format";
import { useSpellName } from "@/i18n/spellName";
import { useInventoryName } from "@/i18n/inventoryName";
import { cardEntryId, cardSource } from "@/journal/card";
import {
  entryCapped,
  entrySummary,
  entryText,
  filterEntries,
  rerollFormula,
  rowValue,
} from "@/journal/entry";
import { LOG_FILTERS } from "@/journal/filter";
import { EntryIcon } from "@/journal/EntryIcon";
import { roundLabel, roundMarker } from "@/journal/rounds";
import { cn } from "@/lib/utils";
import type { LogEntry } from "@/types/log";
import { RedoButton } from "./RedoButton";
import { UndoButton } from "./UndoButton";
import { VALUE_TONES } from "./journalStyle";
import { useDiceStore } from "@/stores/dice";

/**
 * The journal (GET /api/log, ADR 0022 step 5): a head with the Hide setup / All / Rolls filter (saved with the display settings; the default hides the
 * kinds whose view says `bookkeeping`), the entry count,
 * Undo and Redo, then the entries grouped by round, newest first, one 24 px line each: value · icon
 * · entry, the time right-aligned last (ADR 0027 step 9). A narrow marker ("T3", an icon out of
 * combat) sits beside a round's first entry, the current round's in the tint; round blocks touch and
 * alternate between the card's tone and the sub-band. A round's opening rows (the advance and the
 * initiative) are plain rows of its block. A click on an entry shows it in the result panel. The log is a stack: Undo pops the newest entry, values are never edited (and there is no
 * note to add).
 */
export function CombatLog() {
  const { t } = useTranslation();
  const entries = useLogStore((s) => s.entries);
  const round = useBuffsStore((s) => s.round);
  const last = useRollStore((s) => s.last);
  const { selectedId, select } = useJournalStore();
  const filter = useSettingsStore((s) => s.journalFilter);
  const update = useSettingsStore((s) => s.update);
  const source = cardSource(last, entries, selectedId);
  const shownId = cardEntryId(source);
  // The shown row is the latest result's (outline) or one the player picked (fill): never both.
  const shownIsLatest =
    source.kind === "live" || (source.kind === "entry" && source.latest);
  const shown = filterEntries(entries, filter);
  const hidden = filter === "hideSetup" ? entries.length - shown.length : 0;
  const groups = groupByRound(shown, round);

  return (
    <section
      className="zone-card flex min-h-0 min-w-0 flex-1 flex-col"
      aria-label={t("log.title")}
    >
      {/* The journal's head in the header band (ADR 0026 step 6.2), in place of the top padding. */}
      <div className="card-band mb-1 flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 px-4 py-2">
        <h2 className="section-label">{t("log.title")}</h2>
        <div
          role="radiogroup"
          aria-label={t("journal.filter.label")}
          className="flex rounded-lg bg-raised p-0.5"
        >
          {LOG_FILTERS.map((f) => (
            <button
              key={f}
              type="button"
              role="radio"
              aria-checked={filter === f}
              title={
                f === "hideSetup"
                  ? t("journal.filter.hideSetupTitle")
                  : undefined
              }
              onClick={() => update({ journalFilter: f })}
              className={cn(
                "h-6 rounded-md px-3 text-body transition-colors",
                filter === f
                  ? "chip-chosen font-medium"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {t(`journal.filter.${f}`)}
            </button>
          ))}
        </div>
        <span
          className="text-body text-muted-foreground tabular-nums"
          data-testid="journal-count"
        >
          {t("shell.journal.count", { count: entries.length })}
        </span>
        {hidden > 0 && (
          <span
            className="text-body text-muted-foreground"
            data-testid="journal-hidden"
          >
            {t("journal.setupHidden", { count: hidden })}
          </span>
        )}
        <span className="ml-auto flex gap-1">
          <UndoButton />
          <RedoButton />
        </span>
      </div>
      {/* The scroller is its rows' containing block (relative): an absolute box inside, such as a
          round marker's sr-only label, then scrolls and clips with it instead of stretching the page. */}
      <div className="relative flex min-h-0 flex-1 flex-col overflow-y-auto pb-2">
        {/* From 2200 px wide the rounds flow in two balanced columns, so the same height shows twice
            the rows instead of a line a metre long (ADR 0017 step 1, wide screens). */}
        <div className="min-[2200px]:columns-2 min-[2200px]:gap-x-4">
          {groups.map((g, i) => {
            const marker = roundMarker(t, g.round, g.current);
            return (
              <section
                key={g.key}
                className={cn(
                  // The narrow round marker, then the round's entries. Round blocks touch; every other
                  // one is on the sub-band tone, never a rule between them (ADR 0027 step 9.1).
                  "grid grid-cols-[2.25rem_minmax(0,1fr)] items-start gap-x-1 px-2",
                  i % 2 === 1 && "bg-sub-band",
                )}
                aria-label={roundLabel(t, g.round)}
                data-current={g.current}
                data-testid="journal-round"
              >
                <h3
                  className={cn(
                    "flex h-6 items-center justify-center text-label font-semibold tabular-nums",
                    // The current round's one mark: its marker in the tint (ADR 0027 step 9.1).
                    g.current ? "text-tint" : "text-muted-foreground",
                  )}
                  title={marker.full}
                  data-testid="journal-round-marker"
                >
                  {marker.short ? (
                    <span aria-hidden>{marker.short}</span>
                  ) : (
                    <Campfire aria-hidden className="size-4" />
                  )}
                  <span className="sr-only">{marker.full}</span>
                </h3>
                {g.entries.length === 0 ? (
                  <p className="flex min-h-6 items-center px-2 text-body text-muted-foreground">
                    {entries.length === 0
                      ? t("log.empty")
                      : filter === "rolls"
                        ? t("journal.noRollsThisRound")
                        : hidden > 0 && shown.length === 0
                          ? t("journal.onlySetupHidden")
                          : t("log.nothingThisRound")}
                  </p>
                ) : (
                  <div className="flex min-w-0 flex-col">
                    {g.transactions.map((tx) => {
                      // A round's opening rows read as the round's own (ADR 0027 step 9.6), each with
                      // its own Undone tag; any other multi-entry transaction is framed.
                      const framed = tx.length > 1 && !opensRound(tx);
                      const rows = (
                        <ul className="flex flex-col">
                          {tx.map((e) => (
                            <LogRow
                              key={e.id}
                              entry={e}
                              selected={e.id === shownId && !shownIsLatest}
                              latest={e.id === shownId && shownIsLatest}
                              onSelect={() => select(e.id)}
                              grouped={framed}
                              undoneGroup={isUndone(tx)}
                            />
                          ))}
                        </ul>
                      );
                      if (!framed) return <div key={tx[0].id}>{rows}</div>;
                      // One request recorded several entries (ADR 0018): framed as one unit, and
                      // undone as one.
                      const undone = isUndone(tx);
                      return (
                        <div
                          key={tx[0].id}
                          className={cn(
                            "border-l-2",
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
  latest,
  onSelect,
  grouped,
  undoneGroup,
}: {
  entry: LogEntry;
  /** A row the player picked: the selected tone, no bar (the bar is the undo group's). */
  selected: boolean;
  /** The latest result's row: the 2 px tint outline. */
  latest: boolean;
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
  const diceFormula = rerollFormula(e);
  const voided = e.status === "voided";
  const amended = e.status === "amended";
  const isHp = e.kind === "hp_change";
  // A kind with a short form shows its summary; the detail opens under it (a button, so by keyboard).
  const short = entrySummary(e, t, spellName, names);
  const [open, setOpen] = useState(false);
  const text = short ? short.summary : entryText(e, t, spellName, names);
  const capped = entryCapped(e);
  const value = rowValue(e);
  const kindLabel = t(`log.kinds.${e.kind}`, { defaultValue: e.kind });
  // A click anywhere on the row but its own controls shows the entry in the result panel.
  const onRowClick = (ev: MouseEvent<HTMLLIElement>) => {
    if ((ev.target as HTMLElement).closest("button, input, form, a")) return;
    onSelect();
  };

  return (
    <li
      className={cn(
        // One 24 px line (ADR 0027 step 9.2): value · icon · entry (the select button), the row
        // action, then the time right-aligned in the last column (step 9.4). A long entry wraps
        // inside its row; nothing is drawn under it.
        "group/row relative flex min-h-6 cursor-pointer items-start gap-x-2 rounded-md px-2",
        selected ? "selected-fill" : "hover:bg-raised-hover",
        latest && "latest",
      )}
      data-status={e.status}
      data-kind={e.kind}
      data-entry-id={e.id}
      data-selected={selected || latest}
      data-latest={latest}
      onClick={onRowClick}
    >
      <button
        type="button"
        className="flex min-h-6 min-w-0 flex-1 items-start gap-x-2 rounded-sm py-[3px] text-left"
        aria-pressed={selected || latest}
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
          <EntryIcon entry={e} className="size-[1.125rem]" />
          <span className="sr-only">{kindLabel}</span>
        </span>
        <span className="min-w-0 flex-1 text-body break-words">
          <span className={cn(voided && "text-muted-foreground line-through")}>
            {text}
          </span>
          {short && open && (
            <span
              className="mt-0.5 block text-muted-foreground"
              data-testid="log-detail"
            >
              {short.detail.map((line, i) => (
                <span key={i} className="block">
                  {line}
                </span>
              ))}
            </span>
          )}
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
      </button>
      {short && (
        <button
          type="button"
          className="flex h-6 shrink-0 items-center rounded-sm text-muted-foreground hover:text-foreground"
          aria-expanded={open}
          aria-label={t(open ? "log.summary.collapse" : "log.summary.expand")}
          title={t(open ? "log.summary.collapse" : "log.summary.expand")}
          data-testid="log-expand"
          onClick={() => setOpen((v) => !v)}
        >
          {open ? (
            <CaretDown aria-hidden className="size-[1.125rem]" />
          ) : (
            <CaretRight aria-hidden className="size-[1.125rem]" />
          )}
        </button>
      )}
      {diceFormula !== null && (
        // On hover, on keyboard focus in the row, and on the selected row (ADR 0022 step 5.3).
        <Button
          size="xs"
          variant="ghost"
          className={cn(
            "h-6 shrink-0",
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
      {/* The time in its own right-aligned column, last (ADR 0027 step 9.4). */}
      <span
        className="shrink-0 text-right text-label leading-6 text-muted-foreground tabular-nums"
        data-testid="log-time"
      >
        {formatTime(e.createdAt, i18n.language)}
      </span>
    </li>
  );
}
