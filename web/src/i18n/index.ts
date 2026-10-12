import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import enCommon from "./locales/en/common.json";
import enSpells from "./locales/en/spells.json";
import frCommon from "./locales/fr/common.json";
import frSpells from "./locales/fr/spells.json";

export const LANGUAGES = ["en", "fr"] as const;
export type Language = (typeof LANGUAGES)[number];
export const LANGUAGE_STORAGE_KEY = "critical.language";

export const resources = {
  en: { common: enCommon, spells: enSpells },
  fr: { common: frCommon, spells: frSpells },
} as const;

function isLanguage(v: string | null): v is Language {
  return LANGUAGES.includes(v as Language);
}

/** Saved preference (localStorage), else English. Presentation-only; never sent to the API. */
export function initialLanguage(): Language {
  try {
    const saved = localStorage.getItem(LANGUAGE_STORAGE_KEY);
    if (isLanguage(saved)) return saved;
  } catch {
    // storage unavailable (tests, privacy mode)
  }
  return "en";
}

export function setLanguage(lang: Language) {
  try {
    localStorage.setItem(LANGUAGE_STORAGE_KEY, lang);
  } catch {
    // ignore
  }
  return i18n.changeLanguage(lang);
}

void i18n.use(initReactI18next).init({
  resources,
  lng: initialLanguage(),
  fallbackLng: "en",
  defaultNS: "common",
  ns: ["common", "spells"],
  interpolation: { escapeValue: false },
});

export default i18n;
