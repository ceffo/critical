import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import enCommon from "./locales/en/common.json";
import frCommon from "./locales/fr/common.json";
import { loadSettings } from "@/lib/settings";

export { LANGUAGES, type Language } from "@/lib/settings";

export const resources = {
  en: { common: enCommon },
  fr: { common: frCommon },
} as const;

void i18n.use(initReactI18next).init({
  resources,
  // The settings store switches it later (stores/settings.ts); presentation only, never sent to the API.
  lng: loadSettings().lang,
  fallbackLng: "en",
  defaultNS: "common",
  ns: ["common"],
  interpolation: { escapeValue: false },
});

export default i18n;
