import { useTranslation } from "react-i18next";
import { useSpellNamesStore } from "@/stores/spellNames";
import type { Spell, SpellNames } from "@/types/spells";

/**
 * A spell's name in the player's language, by its catalogue id; `englishName` is the name the
 * caller already holds (a log payload, a slot), used when the catalogue does not know the id.
 */
export type SpellNamer = (id: string, englishName?: string) => string;

/** Whether `lang` shows the catalogue's French texts. */
function isFrench(lang: string): boolean {
  return lang.startsWith("fr");
}

/**
 * The namer for `lang` over the catalogue's names (GET /api/spells/names, ADR 0021): the French
 * name in French when the catalogue has one, else English; the catalogue's English name first,
 * then the supplied one, then the id.
 */
export function spellNamer(lang: string, names: SpellNames): SpellNamer {
  return (id, englishName) => {
    const n = names[id];
    if (isFrench(lang) && n?.name_fr) return n.name_fr;
    return n?.name ?? englishName ?? id;
  };
}

/** Names from what the caller holds only: the English name, else the id (tests, no catalogue). */
export const englishSpellName: SpellNamer = (id, englishName) =>
  englishName ?? id;

/** The namer for the current language; re-renders when the language or the names change. */
export function useSpellName(): SpellNamer {
  const { i18n } = useTranslation();
  const names = useSpellNamesStore((s) => s.names);
  return spellNamer(i18n.language, names);
}

/** A spell's name and description in `lang`, French when the catalogue has it, per field. */
export function spellTexts(
  lang: string,
  spell: Pick<Spell, "name" | "name_fr" | "description" | "description_fr">,
): { name: string; description: string } {
  const fr = isFrench(lang);
  return {
    name: (fr && spell.name_fr) || spell.name,
    description: (fr && spell.description_fr) || spell.description,
  };
}

/**
 * The spell's name in the language the UI is not in, when it has one and it differs: English in
 * the French UI, the French name in the English UI. Shown in grey after the name
 * ("Mains brûlantes · Burning Hands").
 */
export function otherSpellName(
  lang: string,
  spell: Pick<Spell, "name" | "name_fr">,
): string | null {
  const shown = spellTexts(lang, { ...spell, description: "" }).name;
  const other = isFrench(lang) ? spell.name : spell.name_fr;
  return other && other !== shown ? other : null;
}

/**
 * Spell id derived from the English name ("Magic Missile" -> "magic_missile"), for slots saved
 * before slots carried the spell's id.
 */
export function spellId(englishName: string): string {
  return englishName
    .toLowerCase()
    .replace(/['’]/g, "")
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}
