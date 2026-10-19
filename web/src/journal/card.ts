// What the result card shows (ADR 0022 step 2.5): the latest result by default, an older entry the
// player picked in the journal (read-only), and the header's last result. The roll kinds, the
// entries whose result takes the card, are the server's: `entry.roll` (ADR 0018 step 10).
import type { TFunction } from "i18next";
import type { SpellNamer } from "@/i18n/spellName";
import type { InventoryNamer } from "@/i18n/inventoryName";
import type { RollResult, SaveBreakdown } from "@/types/combat";
import type { UseResult } from "@/types/items";
import type { CastResult, ExposureResult } from "@/types/spells";
import type { AbilityCheck, SkillCheck } from "@/types/skills";
import type { LogEntry, LogKind } from "@/types/log";
import type { AnyKindView } from "./kindView";
import { entryText, readEntry } from "./entry";
import { viewOf } from "./registry";
import { entryWhen } from "./rounds";

/** The last roll of this session as the roll endpoints returned it, by the kind it is logged as. */
export type LiveResult =
  | { kind: "attack"; result: RollResult }
  | { kind: "save"; result: SaveBreakdown }
  | { kind: "cast"; result: CastResult }
  | { kind: "item_use"; result: UseResult }
  | { kind: "skill_check"; result: SkillCheck }
  | { kind: "ability_check"; result: AbilityCheck }
  | { kind: "exposure"; result: ExposureResult };

/**
 * The newest entry of the roll kinds, voided ones included (the server's "latest roll"): the card
 * shows it by default, and only it takes corrections. An API older than the `roll` mark has none:
 * the card then shows only this session's live result.
 */
export function latestResult(entries: readonly LogEntry[]): LogEntry | null {
  return entries.find((e) => e.roll === true) ?? null;
}

/**
 * The header's last result (ADR 0022 step 2.2): the newest roll-kind entry, or entry its kind shows
 * there (a free dice roll), still in force, so a roll made from any activity shows there. Undone
 * entries give way to the one before them.
 */
export function lastResultEntry(entries: readonly LogEntry[]): LogEntry | null {
  return (
    entries.find(
      (e) =>
        e.status !== "voided" &&
        (e.roll === true || viewOf(e.kind)?.lastResult === true),
    ) ?? null
  );
}

/** The log entry recording a live result. */
export function liveEntryId(last: LiveResult): number | undefined {
  return last.result.logEntryId;
}

/**
 * What the card shows:
 * - an older entry the player selected in the journal (read-only);
 * - else the latest result: this session's last roll while it is the newest result entry (or the
 *   log has not caught up with it yet), otherwise the newest result entry from its payload;
 * - else nothing.
 * `latest` marks the latest result, the only one that offers corrections.
 */
export type CardSource =
  | { kind: "empty" }
  | { kind: "live"; last: LiveResult; entry: LogEntry | null }
  | { kind: "entry"; entry: LogEntry; latest: boolean };

export function cardSource(
  last: LiveResult | null,
  entries: readonly LogEntry[],
  selectedId: number | null,
): CardSource {
  const latest = latestResult(entries);
  const liveId = last ? liveEntryId(last) : undefined;
  if (
    selectedId !== null &&
    selectedId !== latest?.id &&
    selectedId !== liveId
  ) {
    const selected = entries.find((e) => e.id === selectedId);
    if (selected) return { kind: "entry", entry: selected, latest: false };
  }
  if (
    last &&
    (latest === null || liveId === undefined || liveId >= latest.id)
  ) {
    return {
      kind: "live",
      last,
      entry: entries.find((e) => e.id === liveId) ?? null,
    };
  }
  if (latest) return { kind: "entry", entry: latest, latest: true };
  return { kind: "empty" };
}

/** The journal row highlighted for the card: the selected entry, else the latest result's. */
export function cardEntryId(source: CardSource): number | null {
  if (source.kind === "entry") return source.entry.id;
  if (source.kind === "live") return liveEntryId(source.last) ?? null;
  return null;
}

/**
 * The card's content: the kind's view and payload (from the live answer or the entry), the header
 * (kind tag, subtitle, "T3 · 21:31") and the journal's text. `view` is null, or `payload` null,
 * when the card shows only the text (an unknown kind, an unreadable payload).
 */
export interface CardContent {
  kind: LogKind;
  view: AnyKindView | null;
  payload: unknown;
  entry: LogEntry | null;
  tag: string;
  subtitle: string;
  /** Empty while the log has not caught up with a live result. */
  when: string;
  text: string;
}

export function cardContent(
  source: Exclude<CardSource, { kind: "empty" }>,
  t: TFunction,
  lang: string,
  spellName: SpellNamer,
  names: InventoryNamer,
): CardContent {
  const entry = source.entry;
  let kind: LogKind;
  let view: AnyKindView | null;
  let payload: unknown;
  if (source.kind === "live") {
    kind = source.last.kind;
    view = viewOf(kind);
    payload = view?.live ? view.live(source.last.result) : null;
  } else {
    kind = source.entry.kind;
    ({ view, payload } = readEntry(source.entry));
  }
  const w = { t, spellName, names };
  return {
    kind,
    view,
    payload,
    entry,
    tag: t(`log.kinds.${kind}`, { defaultValue: kind }),
    subtitle:
      view?.subtitle && payload !== null ? view.subtitle(payload, w) : "",
    when: entry ? entryWhen(t, entry, lang) : "",
    text: entry ? entryText(entry, t, spellName, names) : "",
  };
}
