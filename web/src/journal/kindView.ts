// The journal's one record per entry kind (docs/frontend.md, "Journal registry"): how an entry of
// that kind is decoded, worded and shown. Display only: verdicts, totals and dice are the server's,
// read from the payloads as recorded (docs/all.md, "Core Architectural Rule"; ADR 0023: the words
// are built here from structured payloads).
import type { ReactNode } from "react";
import type { TFunction } from "i18next";
import type { Icon } from "@phosphor-icons/react";
import type { LogEntry } from "@/types/log";
import type { HpBreakdown } from "@/types/log";
import type { SpellNamer } from "@/i18n/spellName";
import type { InventoryNamer } from "@/i18n/inventoryName";

/** What words an entry: the translator, and the namers of spells and inventory (by id, ADR 0023). */
export interface Words {
  t: TFunction;
  spellName: SpellNamer;
  names: InventoryNamer;
}

/**
 * The value column of a journal row: the number that sums the entry up. A roll total is in ink;
 * `crit` (gold) only when the entry's payload says the roll was a crit (ADR 0026 step 5.8). No
 * payload flags a maximum today, so none is inferred.
 */
export interface RowValue {
  text: string;
  tone: "damage" | "heal" | "pass" | "fail" | "ink" | "crit" | "muted";
}

/** The verdict of a short result: a word in sentence case, coloured by its tone. */
export interface ShortVerdict {
  text: string;
  tone: "pass" | "fail" | "crit";
}

/**
 * The header's last result in short (ADR 0027 step 4.5): "<value> · <what> · <verdict>". The value
 * is the journal row's number, the what a word or a name, the verdict only for a roll that has one.
 */
export interface ShortResult {
  value: RowValue | null;
  what: string;
  verdict: ShortVerdict | null;
}

/** The result card's body of one kind: its payload, its entry (null while the log has not caught
 * up with a live result) and whether it is the latest result, the only one that takes corrections. */
export interface CardProps<P> {
  payload: P;
  /** The entry's journal text, for a body that shows it. */
  text: string;
  entry: LogEntry | null;
  interactive: boolean;
}

/**
 * One entry kind's view. `decode` reads the entry's payload (the one in effect for kinds that are
 * corrected after the roll) once; every other member works on what it returned. An entry whose
 * payload cannot be read (an older shape) is worded by the server's summary and shown as text.
 * Members are methods so that one kind's view, typed by its payload, fits the registry.
 */
export interface KindView<P, L = never> {
  /** The kind's payload, or null when the entry's payload is not one. */
  decode(e: LogEntry): P | null;
  /** The kind's icon in the journal, its bar and the result card. */
  icon: Icon;
  /** The journal's text in the player's language. */
  text(p: P, w: Words, e: LogEntry): string;
  /** The journal row's value; none when absent. */
  value?(p: P, e: LogEntry): RowValue | null;
  /** The journal's Rolls filter: the entry carries dice the server rolled. Absent: never. */
  rolled?(p: P | null): boolean;
  /** The result card's subtitle (weapon, save, spell, item…); empty when absent. */
  subtitle?(p: P, w: Words): string;
  /** The header's last result in short; absent: the row value and the kind's label. */
  short?(p: P, w: Words, e: LogEntry, value: RowValue | null): ShortResult;
  /** The result card's body with its corrections; absent: the journal's text. */
  card?(props: CardProps<P>): ReactNode;
  /** The payload of this session's last roll of the kind, from the roll endpoint's answer. */
  live?(result: L): P;
  /** What a heal or damage asked for when the server cut it (max HP, the floor); null otherwise. */
  capped?(p: P, e: LogEntry): HpBreakdown | null;
  /** The formula the journal row offers to roll again (a free dice roll). */
  reroll?(p: P): string;
  /**
   * Shown as the header's last result although not a roll kind (a free dice roll, ADR 0022 step
   * 2.2). The roll kinds are the server's (`entry.roll`).
   */
  lastResult?: boolean;
}

/** A kind's view as the registry holds it, whatever its payload. */
export type AnyKindView = KindView<unknown, unknown>;
