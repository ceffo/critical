import type { LocalizedText } from "@/types/localized";

/** The language every text falls back to (ADR 0023): the books' language, which every text has. */
export const FALLBACK_LANG = "en";

/**
 * `text` in the UI language `lang`: its body in `lang`, else in the language without its region
 * ("fr" for "fr-CA"), else in the fallback language, else in its first language in code order (a
 * name the player typed in another language, ADR 0023 step 12); "" for a text in none.
 */
export function localized(
  lang: string,
  text: LocalizedText | undefined,
): string {
  if (!text) return "";
  const any = Object.keys(text)
    .filter((l) => text[l])
    .sort()[0];
  return (
    text[lang] ||
    text[lang.split("-")[0]] ||
    text[FALLBACK_LANG] ||
    (any ? text[any] : "") ||
    ""
  );
}

/**
 * The language the editors write a typed text in (ADR 0023 step 11): the UI language without its
 * region.
 */
export function editLang(lang: string): string {
  return lang.split("-")[0] || FALLBACK_LANG;
}
