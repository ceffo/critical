import type { SharedCharacter } from "@/types/character";

/**
 * Who the banner names: the viewed character's name and its owner's display name, from the others'
 * list. `owner` is "" when the owner has no display name (the banner words that itself, in the
 * player's language) and `name` is null while the list has not answered or no longer holds the
 * character (the sheet's own name is shown then).
 */
export function whoIs(
  others: readonly SharedCharacter[] | null,
  id: number,
): { name: string | null; owner: string } {
  const c = others?.find((o) => o.id === id);
  return { name: c?.name ?? null, owner: c?.owner.name.trim() ?? "" };
}
