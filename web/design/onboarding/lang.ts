// The mock-ups' language: read once from `?lang=` (fr by default, ADR 0027 step 2: French first).
export type Lang = "fr" | "en";

export const LANG: Lang =
  new URLSearchParams(location.search).get("lang") === "en" ? "en" : "fr";

/** The French or the English wording of a mock-up text. */
export const tr = (fr: string, en: string): string => (LANG === "fr" ? fr : en);

/** A language map's wording in the mock-ups' language (the shape the API serves, ADR 0023). */
export const lt = (m: { fr: string; en: string }): string => m[LANG];
