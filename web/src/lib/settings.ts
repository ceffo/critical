/**
 * Per-device display preferences (ADR 0009 step 6): language, appearance and ambiance, plus the
 * journal split of the Act activity (ADR 0022 step 5.4: its height and whether it is collapsed),
 * kept in localStorage under one versioned key. Presentation only; never sent to the API.
 */
import {
  DEFAULT_AMBIANCE,
  isAmbiance,
  isMode,
  type AmbianceId,
  type Mode,
} from "./theme";

export const LANGUAGES = ["en", "fr"] as const;
export type Language = (typeof LANGUAGES)[number];

export interface Settings {
  lang: Language;
  mode: Mode;
  amb: AmbianceId;
  /** The journal's height in CSS px once dragged; null = the default split. */
  journalHeight: number | null;
  journalCollapsed: boolean;
}

export const SETTINGS_KEY = "critical.settings.v1";
/** Keys written before the settings popover; read once as the initial value, then removed. */
export const LEGACY_THEME_KEY = "critical.theme";
export const LEGACY_LANGUAGE_KEY = "critical.language";

export const DEFAULT_SETTINGS: Settings = {
  lang: "en",
  mode: "system",
  amb: DEFAULT_AMBIANCE,
  journalHeight: null,
  journalCollapsed: false,
};

function isHeight(v: unknown): v is number {
  return typeof v === "number" && Number.isFinite(v) && v > 0;
}

export function isLanguage(v: unknown): v is Language {
  return LANGUAGES.includes(v as Language);
}

type Store = Pick<Storage, "getItem" | "setItem" | "removeItem">;

function storage(): Store | null {
  try {
    return window.localStorage;
  } catch {
    return null; // storage unavailable (privacy mode)
  }
}

/**
 * The saved settings, each field validated on its own (a bad value falls back to its default).
 * Without saved settings, the legacy theme / language keys give the initial value; the result
 * is then saved under the new key and the legacy keys removed, so they are read only once.
 */
export function loadSettings(store: Store | null = storage()): Settings {
  if (!store) return { ...DEFAULT_SETTINGS };
  let raw: string | null;
  try {
    raw = store.getItem(SETTINGS_KEY);
  } catch {
    return { ...DEFAULT_SETTINGS };
  }
  if (raw !== null) {
    let saved: Partial<Record<keyof Settings, unknown>> = {};
    try {
      const parsed: unknown = JSON.parse(raw);
      if (parsed && typeof parsed === "object") saved = parsed;
    } catch {
      // corrupt value: defaults
    }
    return {
      lang: isLanguage(saved.lang) ? saved.lang : DEFAULT_SETTINGS.lang,
      mode: isMode(saved.mode) ? saved.mode : DEFAULT_SETTINGS.mode,
      amb: isAmbiance(saved.amb) ? saved.amb : DEFAULT_SETTINGS.amb,
      journalHeight: isHeight(saved.journalHeight)
        ? Math.round(saved.journalHeight)
        : DEFAULT_SETTINGS.journalHeight,
      journalCollapsed: saved.journalCollapsed === true,
    };
  }

  const theme = store.getItem(LEGACY_THEME_KEY);
  const lang = store.getItem(LEGACY_LANGUAGE_KEY);
  const migrated: Settings = {
    ...DEFAULT_SETTINGS,
    lang: isLanguage(lang) ? lang : DEFAULT_SETTINGS.lang,
    mode: isMode(theme) ? theme : DEFAULT_SETTINGS.mode,
  };
  if (theme !== null || lang !== null) {
    saveSettings(migrated, store);
    store.removeItem(LEGACY_THEME_KEY);
    store.removeItem(LEGACY_LANGUAGE_KEY);
  }
  return migrated;
}

export function saveSettings(s: Settings, store: Store | null = storage()) {
  try {
    store?.setItem(
      SETTINGS_KEY,
      JSON.stringify({
        lang: s.lang,
        mode: s.mode,
        amb: s.amb,
        journalHeight: s.journalHeight,
        journalCollapsed: s.journalCollapsed,
      }),
    );
  } catch {
    // ignore (quota, privacy mode)
  }
}
