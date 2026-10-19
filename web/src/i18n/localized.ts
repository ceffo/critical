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
  return localizedPick(lang, text).body;
}

/** `localized`, with the language the body was found in ("" when the text is in none). */
export function localizedPick(
  lang: string,
  text: LocalizedText | undefined,
): { lang: string; body: string } {
  if (!text) return { lang: "", body: "" };
  const any = Object.keys(text)
    .filter((l) => text[l])
    .sort()[0];
  for (const l of [lang, lang.split("-")[0], FALLBACK_LANG, any]) {
    if (l && text[l]) return { lang: l, body: text[l] };
  }
  return { lang: "", body: "" };
}

/**
 * The language the editors write a typed text in (ADR 0023 step 11): the UI language without its
 * region.
 */
export function editLang(lang: string): string {
  return lang.split("-")[0] || FALLBACK_LANG;
}
