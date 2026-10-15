// Display helpers for the Moi column: what the log already says, picked out for a glance. No game
// rules here (docs/all.md, "Core Architectural Rule"): HP and save verdicts come from the server.
import type { LogEntry } from "@/types/log";
import type { SaveCategory } from "@/types/combat";
import { exposurePayload } from "./logSummary";

/**
 * The newest HP change still in force: the delta the server applied and the player's source, or
 * for a spell's exposure the Bard took (ADR 0008 step 16) the spell.
 */
export interface LastHpChange {
  delta: number;
  source: string;
  exposure?: { spellId: string; spell: string };
}

/**
 * The newest non-voided entry with a non-zero applied HP delta, an HP change or an exposure the
 * Bard took (entries are newest first, as `GET /api/log` returns them). A heal capped to nothing at
 * full HP is skipped: it changed nothing.
 */
export function lastHpChange(entries: readonly LogEntry[]): LastHpChange | null {
  const e = entries.find(
    (x) =>
      (x.kind === "hp_change" || x.kind === "exposure") &&
      x.status !== "voided" &&
      x.effectiveHpDelta !== 0,
  );
  if (!e) return null;
  const p = exposurePayload(e);
  return p
    ? { delta: e.effectiveHpDelta, source: "", exposure: { spellId: p.spellId, spell: p.spell } }
    : { delta: e.effectiveHpDelta, source: e.effectiveSummary };
}

/** The damage to draw as a ghost segment on the HP bar: the newest HP change, when it is damage. */
export function lastDamage(entries: readonly LogEntry[]): number {
  const last = lastHpChange(entries);
  return last && last.delta < 0 ? -last.delta : 0;
}

/** The newest save rolled in a category, as the server logged it (adjusted saves included). */
export interface LastSave {
  d20: number;
  passed: boolean;
}

/** Newest non-voided save per category, from the save entries' effective payload. */
export function lastSaves(entries: readonly LogEntry[]): Partial<Record<SaveCategory, LastSave>> {
  const out: Partial<Record<SaveCategory, LastSave>> = {};
  for (const e of entries) {
    if (e.kind !== "save" || e.status === "voided") continue;
    const p = (e.effectivePayload ?? e.payload) as {
      category?: unknown;
      d20Roll?: unknown;
      passed?: unknown;
    } | null;
    if (!p || typeof p.category !== "string" || typeof p.passed !== "boolean") continue;
    if (typeof p.d20Roll !== "number") continue;
    const cat = p.category as SaveCategory;
    if (!(cat in out)) out[cat] = { d20: p.d20Roll, passed: p.passed };
  }
  return out;
}
