import type { Character, CreationOptions } from "@/types/character";

/**
 * The classes entry `index` of the character may be switched to (ADR 0050 step 15.1), as the
 * options let the form offer them: another class of the entry's group that keeps a spellbook, which
 * no other entry holds already. The server decides the rest (a specialist on one side, minimums,
 * alignment) and a refusal shows in the dialog.
 */
export function switchTargets(
  character: Character,
  options: CreationOptions | null,
  index: number,
): string[] {
  const classes = options?.classes ?? [];
  const entry = character.classes[index];
  const own = classes.find((c) => c.class === entry?.class);
  if (!entry || !own?.spellbook || !own.group) return [];
  const held = new Set(character.classes.map((e) => e.class));
  return classes
    .filter((c) => c.spellbook && c.group === own.group && !held.has(c.class))
    .map((c) => c.class);
}

/** The entries of the character with at least one class to switch to. */
export function switchableEntries(
  character: Character,
  options: CreationOptions | null,
): number[] {
  return character.classes
    .map((_, i) => i)
    .filter((i) => switchTargets(character, options, i).length > 0);
}
