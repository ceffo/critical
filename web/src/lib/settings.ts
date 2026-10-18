/**
 * Per-device display preferences (ADR 0009 step 6, ADR 0026 step 4): language, appearance mode, a
 * dark theme and a light theme, plus the journal split of the Act activity (ADR 0022 step 5.4: its
 * height and whether it is collapsed), kept in localStorage under one versioned key. Presentation
 * only; never sent to the API.
 */
import {
  DEFAULT_DARK_THEME,
  DEFAULT_LIGHT_THEME,
  isDarkTheme,
  isLightTheme,
  isMode,
  type DarkThemeId,
  type LightThemeId,
  type Mode,
  themeVars,
} from "./theme";

export const LANGUAGES = ["en", "fr"] as const;
export type Language = (typeof LANGUAGES)[number];

export interface Settings {
  lang: Language;
  mode: Mode;
  /** The theme the dark mode applies (Arcane, Deep, Plain). */
  darkTheme: DarkThemeId;
  /** The theme the light mode applies (Arcane, Parchment, Plain). */
  lightTheme: LightThemeId;
  /** The journal's height in CSS px once dragged; null = the default split. */
  journalHeight: number | null;
  journalCollapsed: boolean;
}

export const SETTINGS_KEY = "critical.settings.v1";
/**
 * The chosen themes' variables, dark and light, saved for index.html's inline script: it paints
 * them before the app runs (ADR 0026 step 4.5), so a Deep or Parchment user sees no flash of the
 * default theme. A copy of `themeVars`, rewritten whenever the settings apply.
 */
export const PAINT_KEY = "critical.paint.v1";
/** Keys written before the settings popover; read once as the initial value, then removed. */
export const LEGACY_THEME_KEY = "critical.theme";
export const LEGACY_LANGUAGE_KEY = "critical.language";

/**
 * A saved single ambiance (the `amb` field before ADR 0026) as the pair it becomes (step 4.3),
 * mapped once at load and saved back without it. Any other value gives the defaults.
 */
export const LEGACY_AMBIANCES: Record<
  string,
  { darkTheme: DarkThemeId; lightTheme: LightThemeId }
> = {
  ocean: { darkTheme: "deep", lightTheme: "arcane" },
  ambre: { darkTheme: "arcane", lightTheme: "parchment" },
  sylve: { darkTheme: "arcane", lightTheme: "arcane" },
  sang: { darkTheme: "arcane", lightTheme: "arcane" },
  obsidienne: { darkTheme: "plain", lightTheme: "plain" },
  arcane: { darkTheme: "arcane", lightTheme: "arcane" },
};

export const DEFAULT_SETTINGS: Settings = {
  lang: "en",
  mode: "system",
  darkTheme: DEFAULT_DARK_THEME,
  lightTheme: DEFAULT_LIGHT_THEME,
  journalHeight: null,
  journalCollapsed: false,
};

/**
 * The change picking `id` in the list of the shown appearance makes (ADR 0026 step 4.6): the
 * light choice in light, the dark choice in dark, never the other one. A theme the appearance
 * does not have changes nothing.
 */
export function themePatch(light: boolean, id: string): Partial<Settings> {
  if (light) return isLightTheme(id) ? { lightTheme: id } : {};
  return isDarkTheme(id) ? { darkTheme: id } : {};
}

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
 * A saved `amb` (one ambiance for both modes, before ADR 0026) is mapped to a dark and a light
 * theme by `LEGACY_AMBIANCES` and the result saved back without it, so it is mapped only once.
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
    let saved: Partial<Record<keyof Settings | "amb", unknown>> = {};
    try {
      const parsed: unknown = JSON.parse(raw);
      if (parsed && typeof parsed === "object") saved = parsed;
    } catch {
      // corrupt value: defaults
    }
    const legacy = !("amb" in saved)
      ? null
      : typeof saved.amb === "string" &&
          Object.hasOwn(LEGACY_AMBIANCES, saved.amb)
        ? LEGACY_AMBIANCES[saved.amb]
        : {
            darkTheme: DEFAULT_SETTINGS.darkTheme,
            lightTheme: DEFAULT_SETTINGS.lightTheme,
          };
    const darkTheme = saved.darkTheme ?? legacy?.darkTheme;
    const lightTheme = saved.lightTheme ?? legacy?.lightTheme;
    const settings: Settings = {
      lang: isLanguage(saved.lang) ? saved.lang : DEFAULT_SETTINGS.lang,
      mode: isMode(saved.mode) ? saved.mode : DEFAULT_SETTINGS.mode,
      darkTheme: isDarkTheme(darkTheme)
        ? darkTheme
        : DEFAULT_SETTINGS.darkTheme,
      lightTheme: isLightTheme(lightTheme)
        ? lightTheme
        : DEFAULT_SETTINGS.lightTheme,
      journalHeight: isHeight(saved.journalHeight)
        ? Math.round(saved.journalHeight)
        : DEFAULT_SETTINGS.journalHeight,
      journalCollapsed: saved.journalCollapsed === true,
    };
    if (legacy) saveSettings(settings, store);
    return settings;
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
        darkTheme: s.darkTheme,
        lightTheme: s.lightTheme,
        journalHeight: s.journalHeight,
        journalCollapsed: s.journalCollapsed,
      }),
    );
  } catch {
    // ignore (quota, privacy mode)
  }
}

/** Save the variables index.html paints before the app runs, for both modes (see `PAINT_KEY`). */
export function savePaint(
  s: Pick<Settings, "darkTheme" | "lightTheme">,
  store: Store | null = storage(),
) {
  try {
    store?.setItem(
      PAINT_KEY,
      JSON.stringify({
        dark: themeVars(s.darkTheme, false),
        light: themeVars(s.lightTheme, true),
      }),
    );
  } catch {
    // ignore (quota, privacy mode)
  }
}
