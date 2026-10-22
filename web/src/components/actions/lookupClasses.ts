import type { SpellClass } from "@/api/spells_db";
import type { SpellbookRules } from "@/types/character";

const CLASSES: { value: SpellClass; key: string }[] = [
  { value: "mine", key: "classMine" },
  { value: "wizard", key: "classWizard" },
  { value: "priest", key: "classPriest" },
  { value: "all", key: "classAll" },
];

/**
 * The lookup's class toggle for a character: "Mine" (the spellbook) only for a class that has one
 * (ADR 0040: the class's spellbook feature, which the server derives as a non-null `spellbook`
 * block). `rules` is that block; the web never lists the classes.
 */
export function lookupClasses(rules: SpellbookRules | null) {
  return CLASSES.filter((c) => c.value !== "mine" || rules !== null);
}

/**
 * The class the lookup shows: a "Mine" left selected by a character that has a spellbook falls
 * back to Wizard (the lookup's default) when the active character has none.
 */
export function effectiveClass(
  cls: SpellClass,
  rules: SpellbookRules | null,
): SpellClass {
  return cls === "mine" && rules === null ? "wizard" : cls;
}
