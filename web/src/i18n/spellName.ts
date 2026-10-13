import type { TFunction } from "i18next";

/**
 * Localised spell name by the spell's stable id (key in the `spells` namespace).
 * A missing key in the active language falls back to English (i18next fallbackLng),
 * and a key missing everywhere falls back to `englishName` (then the id itself).
 */
export function spellName(
  t: TFunction,
  id: string,
  englishName?: string,
): string {
  // "hold_person_priest" / "fortify_l4" are keys of other spells of the same
  // name: the translated name is the one of the base id.
  const base = id.replace(/(_priest)?(_l\d+)?$/, "") || id;
  const fallback =
    base === id
      ? (englishName ?? id)
      : t(base, { ns: "spells", defaultValue: englishName ?? id });
  return t(id, { ns: "spells", defaultValue: fallback });
}

/**
 * Stable spell id derived from the English name ("Magic Missile" -> "magic_missile").
 * Interim: the spell database has no explicit id yet; replace with the real id when it does.
 */
export function spellId(englishName: string): string {
  return englishName
    .toLowerCase()
    .replace(/['’]/g, "")
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "");
}
