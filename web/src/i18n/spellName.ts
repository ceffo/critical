import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import i18n from "@/i18n";
import { localized } from "@/i18n/localized";
import { markedField } from "@/lib/valueMarks";
import { useSpellNamesStore } from "@/stores/spellNames";
import type { Spell, SpellNames } from "@/types/spells";

/**
 * A spell's name in the player's language, by its catalogue id; `englishName` is the name the
 * caller already holds (a log payload, a slot), used when the catalogue does not know the id.
 */
export type SpellNamer = (id: string, englishName?: string) => string;

/**
 * The namer for `lang` over the catalogue's names (GET /api/spells/names, ADR 0021): the
 * catalogue's name in `lang`, falling back to English (ADR 0023); for an id the catalogue lacks,
 * the supplied name, then the id.
 */
export function spellNamer(lang: string, names: SpellNames): SpellNamer {
  return (id, englishName) =>
    localized(lang, names[id]?.name) || englishName || id;
}

/**
 * A buff's name in the player's language: the catalogue's name of the spell it comes from, the
 * proficiency's name when a successful check put it on (ADR 0037 step 20, the Skills tab's
 * `skills.names` key, in `t`'s language: the current one when the caller passes none), or its
 * stored name otherwise (hand-made, or stored before buffs kept their source).
 */
export function buffName(
  spellName: SpellNamer,
  buff: { name: string; spellId?: string; proficiencyId?: string },
  t: TFunction = i18n.t,
): string {
  if (buff.spellId) return spellName(buff.spellId, buff.name);
  if (buff.proficiencyId)
    return t(`skills.names.${buff.proficiencyId}`, {
      defaultValue: buff.name,
    });
  return buff.name;
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

/**
 * A spell's name and description in `lang`, each falling back to English on its own. The
 * description carries the API's value marks (lib/valueMarks); `plainText` drops them.
 */
export function spellTexts(
  lang: string,
  spell: Pick<Spell, "name" | "description" | "marked">,
): { name: string; description: string } {
  return {
    name: localized(lang, spell.name),
    description: markedField(
      lang,
      spell.description,
      spell.marked,
      "description",
    ),
  };
}

/** The spell's name in the UI language, falling back to English. */
export function spellName(lang: string, spell: Pick<Spell, "name">): string {
  return localized(lang, spell.name);
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
