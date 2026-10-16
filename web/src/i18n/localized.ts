import type { LocalizedText } from "@/types/localized";

/** The language every text falls back to (ADR 0023): the books' language, which every text has. */
export const FALLBACK_LANG = "en";

/**
 * `text` in the UI language `lang`: its body in `lang`, else in the language without its region
 * ("fr" for "fr-CA"), else in the fallback language; "" for a text in none of them.
 */
export function localized(
  lang: string,
  text: LocalizedText | undefined,
): string {
  if (!text) return "";
  return text[lang] || text[lang.split("-")[0]] || text[FALLBACK_LANG] || "";
}
