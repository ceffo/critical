import type { SpellbookRules } from "@/types/character";

/**
 * What the lookup's detail offers on a spell (ADR 0030 steps 20 and 20.6): nothing for a class
 * without a spellbook (the server's derived `spellbook` block is null) or a spell that is not a
 * wizard spell; "In the spellbook" once it is there; else Learn and Add. The limits are the
 * server's: an offered button may still be refused, worded from its code.
 */
export function spellbookOffer(
  rules: SpellbookRules | null,
  wizard: boolean,
  known: boolean,
): "none" | "known" | "learnOrAdd" {
  if (!rules || !wizard) return "none";
  return known ? "known" : "learnOrAdd";
}
