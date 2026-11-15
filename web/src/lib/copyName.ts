/**
 * The name of the copy of a character (ADR 0029 step 12.1): the wording of `characters.copyName` around
 * the source's name, which the web builds (the backend builds no display text). The server keeps a name
 * to `max` characters (the creation options' `maxNameLength`, null while unknown): a long source name
 * gives up the end of its own text so the suffix stays whole, and the player can rename the copy.
 */
export function copyName(
  name: string,
  wording: (name: string) => string,
  max: number | null,
): string {
  const full = wording(name);
  if (max === null) return full;
  const over = [...full].length - max;
  if (over <= 0) return full;
  const base = [...name].slice(0, Math.max(0, [...name].length - over));
  return wording(base.join("").trimEnd());
}
