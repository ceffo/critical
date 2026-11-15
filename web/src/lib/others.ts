import type { SharedCharacter } from "@/types/character";

/** One owner's characters in the others' list. */
export interface OwnerGroup {
  id: string;
  /** The owner's display name, "" when none is known (the list words that itself). */
  name: string;
  characters: SharedCharacter[];
}

/** The others' list grouped by owner, in the order the server gave (by owner name, then id). */
export function groupByOwner(list: readonly SharedCharacter[]): OwnerGroup[] {
  const groups: OwnerGroup[] = [];
  const at = new Map<string, OwnerGroup>();
  for (const c of list) {
    let g = at.get(c.owner.id);
    if (!g) {
      g = { id: c.owner.id, name: c.owner.name.trim(), characters: [] };
      at.set(c.owner.id, g);
      groups.push(g);
    }
    g.characters.push(c);
  }
  return groups;
}
