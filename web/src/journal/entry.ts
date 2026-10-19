// An entry read through its kind's view (registry.ts): the journal row, the collapsed bar, the
// header's last result and the undo tooltips ask here, never switch on the kind themselves.
import type { TFunction } from "i18next";
import { Sparkle, type Icon } from "@phosphor-icons/react";
import { englishSpellName, type SpellNamer } from "@/i18n/spellName";
import { type InventoryNamer, recordedNames } from "@/i18n/inventoryName";
import type { HpBreakdown, LogEntry } from "@/types/log";
import type { AnyKindView, RowValue, ShortResult, Words } from "./kindView";
import { viewOf } from "./registry";

/** The journal filter: everything, or rolls only. */
export type LogFilter = "all" | "rolls";

/** An entry's view and its decoded payload (null: an unknown kind, or a payload it cannot read). */
export function readEntry(e: LogEntry): {
  view: AnyKindView | null;
  payload: unknown;
} {
  const view = viewOf(e.kind);
  return { view, payload: view ? view.decode(e) : null };
}

function words(
  t: TFunction,
  spellName: SpellNamer,
  names: InventoryNamer,
): Words {
  return { t, spellName, names };
}

/**
 * The entry's text in the player's language, worded from its structured payload (ADR 0023). The
 * server's summary is only the player's own text (an HP change's source) or, on entries logged
 * before the payloads existed, an older English line. Spell names come from `spellName`
 * (useSpellName in a component), item and weapon names from `names` (useInventoryName).
 */
export function entryText(
  e: LogEntry,
  t: TFunction,
  spellName: SpellNamer = englishSpellName,
  names: InventoryNamer = recordedNames,
): string {
  const { view, payload } = readEntry(e);
  if (!view || payload === null) return e.effectiveSummary;
  return view.text(payload, words(t, spellName, names), e);
}

/** The journal row's value: the number that sums the entry up; null when none. */
export function rowValue(e: LogEntry): RowValue | null {
  const { view, payload } = readEntry(e);
  if (!view?.value || payload === null) return null;
  return view.value(payload, e);
}

/** The kind's icon; a kind this web does not know gets the generic one. */
export function entryIcon(e: LogEntry): Icon {
  return viewOf(e.kind)?.icon ?? Sparkle;
}

/** What an HP change asked for when the server cut it; null otherwise. */
export function entryCapped(e: LogEntry): HpBreakdown | null {
  const { view, payload } = readEntry(e);
  if (!view?.capped || payload === null) return null;
  return view.capped(payload, e);
}

/** The formula the row offers to roll again (a free dice roll still in force); null otherwise. */
export function rerollFormula(e: LogEntry): string | null {
  if (e.status === "voided") return null;
  const { view, payload } = readEntry(e);
  if (!view?.reroll || payload === null) return null;
  return view.reroll(payload);
}

/**
 * The "rolls" filter: entries that carry dice the server rolled (attacks, saves, item uses,
 * initiative, skill and ability checks, free dice rolls, exposures, turning, and casts with a damage
 * roll or a to-hit d20). Each kind says so in its view.
 */
export function isRolledEntry(e: LogEntry): boolean {
  const { view, payload } = readEntry(e);
  return view?.rolled?.(payload) ?? false;
}

export function filterEntries(
  entries: LogEntry[],
  filter: LogFilter,
): LogEntry[] {
  return filter === "all" ? entries : entries.filter(isRolledEntry);
}

/** The header's last result in short (ADR 0027 step 4.5), from the entry's structured payload. */
export function shortResult(
  e: LogEntry,
  t: TFunction,
  spellName: SpellNamer,
  names: InventoryNamer,
): ShortResult {
  const value = rowValue(e);
  const { view, payload } = readEntry(e);
  if (view?.short && payload !== null)
    return view.short(payload, words(t, spellName, names), e, value);
  return {
    value,
    what: t(`log.kinds.${e.kind}`, { defaultValue: e.kind }),
    verdict: null,
  };
}
