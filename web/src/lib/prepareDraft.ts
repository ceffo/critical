import type { SpellbookEntry } from "@/types/spells";

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
