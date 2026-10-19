import type { CharacterSummary } from "@/types/character";

/**
 * The card's "race · alignment" line: only what is set, joined by " · ". What is still to choose
 * (a blank code, a stored free text the server blanked) is the tag's, never a blank or a stray
 * separator here. `word` words a code by its i18n key.
 */
export function identityLine(
  c: Pick<CharacterSummary, "race" | "alignment">,
  word: (key: string, code: string) => string,
): string {
  return [
    c.race && word("details.races", c.race),
    c.alignment && word("details.alignments", c.alignment),
  ]
    .filter(Boolean)
    .join(" · ");
}

/** The tag's i18n key for what is still to choose, null when nothing is. */
export function toPickKey(missing: CharacterSummary["missing"]): string | null {
  return missing.length === 0 ? null : `roster.toPick.${missing.join("_")}`;
}
