/**
 * Mirrors the API's language maps (ADR 0023): a text's body by language code, holding only the
 * languages the catalogue has (`{}` for a text in none). Never a field per language: pick one with
 * `localized` (src/i18n/localized.ts), which falls back to English per field.
 */
export type LocalizedText = Readonly<Record<string, string>>;
