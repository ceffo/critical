import type { LevelLimit, SpellbookEntry } from "@/types/spells";

/**
 * Split today's prepared spell ids into those still in the spellbook (the editable draft) and
 * those that are not (listed as "will be removed", never sent: the API refuses unknown ids).
 */
export function splitPrepared(
  prepared: string[],
  spellbook: SpellbookEntry[],
): { kept: string[]; dropped: string[] } {
  const known = new Set(spellbook.map((s) => s.id));
  const kept: string[] = [];
  const dropped: string[] = [];
  for (const id of prepared) {
    if (known.has(id)) kept.push(id);
    else if (!dropped.includes(id)) dropped.push(id);
  }
  return { kept, dropped };
}

/** One prepared spell of the draft and its copies, in the order it was first added. */
export interface DraftRow {
  id: string;
  count: number;
}

/** The draft's spells with their copy counts (ADR 0027 step 11.4), first added first. */
export function draftRows(draft: string[]): DraftRow[] {
  const rows = new Map<string, DraftRow>();
  for (const id of draft) {
    const row = rows.get(id);
    if (row) row.count++;
    else rows.set(id, { id, count: 1 });
  }
  return [...rows.values()];
}

/** One more copy of `id` (the grimoire's add and the row's +). */
export function addOne(draft: string[], id: string): string[] {
  return [...draft, id];
}

/** One copy of `id` less (the row's −): the last copy removed takes the spell out of the draft. */
export function removeOne(draft: string[], id: string): string[] {
  const at = draft.lastIndexOf(id);
  return at < 0 ? draft : [...draft.slice(0, at), ...draft.slice(at + 1)];
}

/** A level of the draft: how many spells are prepared at it and how many the server allows. */
export interface LevelBudget {
  level: number;
  count: number;
  max: number;
}

/**
 * The draft's levels (ADR 0027 step 11.3 and 11.5): every level the server gives the character a
 * preparation limit above 0, plus any level the draft holds a spell at, ascending. `max` is the
 * server's count; nothing is computed here but the number of copies per level.
 */
export function levelBudgets(
  draft: string[],
  spellbook: SpellbookEntry[],
  limits: LevelLimit[],
): LevelBudget[] {
  const levelOf = new Map(spellbook.map((s) => [s.id, s.level]));
  const counts = new Map<number, number>();
  for (const l of limits) if (l.max > 0) counts.set(l.level, 0);
  for (const id of draft) {
    const level = levelOf.get(id);
    if (level !== undefined) counts.set(level, (counts.get(level) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([level, count]) => ({
      level,
      count,
      max: limits.find((l) => l.level === level)?.max ?? 0,
    }))
    .sort((a, b) => a.level - b.level);
}

/**
 * The levels over budget (ADR 0027 step 11.5): "Enregistrer" stays disabled while this is not empty.
 * The browser only compares the two numbers the server gave; the server still refuses the save.
 */
export function overBudget(budgets: LevelBudget[]): number[] {
  return budgets.filter((b) => b.count > b.max).map((b) => b.level);
}
