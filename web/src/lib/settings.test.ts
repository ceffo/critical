import { describe, expect, it } from "vitest";
import {
  DEFAULT_SETTINGS,
  LEGACY_AMBIANCES,
  LEGACY_LANGUAGE_KEY,
  LEGACY_THEME_KEY,
  PAINT_KEY,
  SETTINGS_KEY,
  loadSettings,
  savePaint,
  saveSettings,
} from "./settings";
import { themeVars } from "./theme";

function memory(init: Record<string, string> = {}) {
  const data = new Map(Object.entries(init));
  return {
    data,
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
    removeItem: (k: string) => void data.delete(k),
  };
}

const saved = (store: ReturnType<typeof memory>) =>
  JSON.parse(store.data.get(SETTINGS_KEY)!) as Record<string, unknown>;

describe("settings", () => {
  it("defaults to English, system appearance and Arcane dark and light", () => {
    expect(loadSettings(memory())).toEqual({
      lang: "en",
      mode: "system",
      darkTheme: "arcane",
      lightTheme: "arcane",
      journalHeight: null,
      journalCollapsed: false,
    });
    expect(loadSettings(null)).toEqual(DEFAULT_SETTINGS);
  });

  it("round-trips through storage, both themes and the journal split included", () => {
    const store = memory();
    const s = {
      lang: "fr",
      mode: "light",
      darkTheme: "deep",
      lightTheme: "parchment",
      journalHeight: 320,
      journalCollapsed: true,
    } as const;
    saveSettings(s, store);
    expect(saved(store)).toEqual(s);
    expect(loadSettings(store)).toEqual(s);
  });

  it("keeps both theme choices when the mode switches", () => {
    const store = memory();
    saveSettings(
      { ...DEFAULT_SETTINGS, darkTheme: "plain", lightTheme: "parchment" },
      store,
    );
    for (const mode of ["dark", "light", "system"] as const) {
      saveSettings({ ...loadSettings(store), mode }, store);
      expect(loadSettings(store)).toMatchObject({
        mode,
        darkTheme: "plain",
        lightTheme: "parchment",
      });
    }
  });

  // ADR 0026 step 4.3: one case per row of the mapping table.
  const MAPPED = [
    ["ocean", "deep", "arcane"],
    ["ambre", "arcane", "parchment"],
    ["sylve", "arcane", "arcane"],
    ["sang", "arcane", "arcane"],
    ["obsidienne", "plain", "plain"],
    ["arcane", "arcane", "arcane"],
  ] as const;
  for (const [amb, darkTheme, lightTheme] of MAPPED)
    it(`maps a saved "${amb}" to ${darkTheme} dark and ${lightTheme} light, once, and saves it back`, () => {
      const store = memory({
        [SETTINGS_KEY]: JSON.stringify({ lang: "fr", mode: "dark", amb }),
      });
      expect(loadSettings(store)).toMatchObject({
        lang: "fr",
        mode: "dark",
        darkTheme,
        lightTheme,
      });
      expect(saved(store)).toEqual({
        lang: "fr",
        mode: "dark",
        darkTheme,
        lightTheme,
        journalHeight: null,
        journalCollapsed: false,
      });
      // Saved back without `amb`: a later choice is not re-mapped.
      saveSettings({ ...loadSettings(store), darkTheme: "plain" }, store);
      expect(loadSettings(store).darkTheme).toBe("plain");
    });

  it("has a row for every ambiance that existed", () => {
    expect(Object.keys(LEGACY_AMBIANCES).sort()).toEqual(
      MAPPED.map(([amb]) => amb).sort(),
    );
  });

  it("gives the defaults for an unknown or corrupt saved ambiance, and saves them back", () => {
    for (const amb of ["sepia", 3, null, "toString"]) {
      const store = memory({
        [SETTINGS_KEY]: JSON.stringify({ mode: "light", amb }),
      });
      expect(loadSettings(store)).toMatchObject({
        mode: "light",
        darkTheme: "arcane",
        lightTheme: "arcane",
      });
      expect("amb" in saved(store)).toBe(false);
    }
  });

  it("falls back to the mode's default for a choice that is not a theme of that mode", () => {
    expect(
      loadSettings(
        memory({
          [SETTINGS_KEY]: JSON.stringify({
            darkTheme: "parchment",
            lightTheme: "deep",
          }),
        }),
      ),
    ).toMatchObject({ darkTheme: "arcane", lightTheme: "arcane" });
    expect(
      loadSettings(
        memory({
          [SETTINGS_KEY]: JSON.stringify({
            darkTheme: "plain",
            lightTheme: "sylve",
          }),
        }),
      ),
    ).toMatchObject({ darkTheme: "plain", lightTheme: "arcane" });
  });

  it("migrates the legacy theme and language keys once", () => {
    const store = memory({
      [LEGACY_THEME_KEY]: "dark",
      [LEGACY_LANGUAGE_KEY]: "fr",
    });
    const migrated = {
      lang: "fr",
      mode: "dark",
      darkTheme: "arcane",
      lightTheme: "arcane",
      journalHeight: null,
      journalCollapsed: false,
    };
    expect(loadSettings(store)).toEqual(migrated);
    expect(store.data.has(LEGACY_THEME_KEY)).toBe(false);
    expect(store.data.has(LEGACY_LANGUAGE_KEY)).toBe(false);
    expect(saved(store)).toEqual(migrated);
  });

  it("ignores legacy keys once new settings exist", () => {
    const store = memory({
      [SETTINGS_KEY]: JSON.stringify({
        lang: "en",
        mode: "light",
        darkTheme: "deep",
        lightTheme: "plain",
      }),
      [LEGACY_THEME_KEY]: "dark",
      [LEGACY_LANGUAGE_KEY]: "fr",
    });
    expect(loadSettings(store)).toEqual({
      lang: "en",
      mode: "light",
      darkTheme: "deep",
      lightTheme: "plain",
      journalHeight: null,
      journalCollapsed: false,
    });
  });

  it("falls back per field on invalid or corrupt values", () => {
    expect(
      loadSettings(
        memory({
          [SETTINGS_KEY]: JSON.stringify({
            lang: "de",
            mode: "dark",
            darkTheme: 3,
            journalHeight: -5,
            journalCollapsed: "yes",
          }),
        }),
      ),
    ).toEqual({
      lang: "en",
      mode: "dark",
      darkTheme: "arcane",
      lightTheme: "arcane",
      journalHeight: null,
      journalCollapsed: false,
    });
    expect(loadSettings(memory({ [SETTINGS_KEY]: "{not json" }))).toEqual(
      DEFAULT_SETTINGS,
    );
    expect(loadSettings(memory({ [LEGACY_THEME_KEY]: "sepia" }))).toEqual(
      DEFAULT_SETTINGS,
    );
  });

  it("saves the chosen themes' variables for the first paint (ADR 0026 step 4.5)", () => {
    const store = memory();
    savePaint({ darkTheme: "deep", lightTheme: "parchment" }, store);
    expect(JSON.parse(store.data.get(PAINT_KEY)!)).toEqual({
      dark: themeVars("deep", false),
      light: themeVars("parchment", true),
    });
  });
});
