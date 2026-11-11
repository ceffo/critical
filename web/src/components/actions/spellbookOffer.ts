import type { SpellbookRules } from "@/types/character";
import type { SpellbookEntry } from "@/types/spells";

/**
 * The refusals of the server's spellbook check (ADR 0030 step 20.1) the lookup words up front.
 * They are the server's own error codes, so the reason on screen is the one a click would get.
 */
export type SpellbookRefusal =
  | "spell_level_above_class"
  | "spell_level_above_intelligence"
  | "spellbook_level_full";

export type SpellbookOffer =
  | { kind: "none" }
  | { kind: "known" }
  | { kind: "learnOrAdd" }
  | { kind: "refused"; code: SpellbookRefusal };

/**
 * What the lookup's detail offers on a spell (ADR 0030 steps 20 and 20.6): nothing for a class
 * without a spellbook (the server's derived `spellbook` block is null) or a spell that is not a
 * wizard spell; "In the spellbook" once it is there; the server's refusal when the spell's level
 * is above the class's or the Intelligence's highest, or the book already holds the most spells
 * of that level (same order as the server's check, from the same served limits); else Learn and Add.
 */
export function spellbookOffer(
  rules: SpellbookRules | null,
  wizard: boolean,
  book: SpellbookEntry[],
  spell: { id: string; level: number },
): SpellbookOffer {
  if (!rules || !wizard) return { kind: "none" };
  if (book.some((e) => e.id === spell.id)) return { kind: "known" };
  if (spell.level > rules.classMaxSpellLevel)
    return { kind: "refused", code: "spell_level_above_class" };
  if (spell.level > rules.intMaxSpellLevel)
    return { kind: "refused", code: "spell_level_above_intelligence" };
  const same = book.filter((e) => e.level === spell.level).length;
  if (!rules.noMaximum && same >= rules.maxSpellsPerLevel)
    return { kind: "refused", code: "spellbook_level_full" };
  return { kind: "learnOrAdd" };
}
