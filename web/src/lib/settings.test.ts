import { describe, expect, it } from "vitest";
import {
  DEFAULT_SETTINGS,
  LEGACY_LANGUAGE_KEY,
  LEGACY_THEME_KEY,
  SETTINGS_KEY,
  loadSettings,
  saveSettings,
} from "./settings";

function memory(init: Record<string, string> = {}) {
  const data = new Map(Object.entries(init));
  return {
    data,
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
    removeItem: (k: string) => void data.delete(k),
  };
}

describe("settings", () => {
  it("defaults to English, system appearance and Arcane", () => {
    expect(loadSettings(memory())).toEqual({
      lang: "en",
      mode: "system",
      amb: "arcane",
    });
    expect(loadSettings(null)).toEqual(DEFAULT_SETTINGS);
  });

  it("round-trips through storage", () => {
    const store = memory();
    saveSettings({ lang: "fr", mode: "light", amb: "sang" }, store);
    expect(JSON.parse(store.data.get(SETTINGS_KEY)!)).toEqual({
      lang: "fr",
      mode: "light",
      amb: "sang",
    });
    expect(loadSettings(store)).toEqual({
      lang: "fr",
      mode: "light",
      amb: "sang",
    });
  });

  it("migrates the legacy theme and language keys once", () => {
    const store = memory({
      [LEGACY_THEME_KEY]: "dark",
      [LEGACY_LANGUAGE_KEY]: "fr",
    });
    expect(loadSettings(store)).toEqual({
      lang: "fr",
      mode: "dark",
      amb: "arcane",
    });
    expect(store.data.has(LEGACY_THEME_KEY)).toBe(false);
    expect(store.data.has(LEGACY_LANGUAGE_KEY)).toBe(false);
    expect(JSON.parse(store.data.get(SETTINGS_KEY)!)).toEqual({
      lang: "fr",
      mode: "dark",
      amb: "arcane",
    });
  });

  it("ignores legacy keys once new settings exist", () => {
    const store = memory({
      [SETTINGS_KEY]: JSON.stringify({
        lang: "en",
        mode: "light",
        amb: "ocean",
      }),
      [LEGACY_THEME_KEY]: "dark",
      [LEGACY_LANGUAGE_KEY]: "fr",
    });
    expect(loadSettings(store)).toEqual({
      lang: "en",
      mode: "light",
      amb: "ocean",
    });
  });

  it("falls back per field on invalid or corrupt values", () => {
    expect(
      loadSettings(
        memory({
          [SETTINGS_KEY]: JSON.stringify({ lang: "de", mode: "dark", amb: 3 }),
        }),
      ),
    ).toEqual({ lang: "en", mode: "dark", amb: "arcane" });
    expect(loadSettings(memory({ [SETTINGS_KEY]: "{not json" }))).toEqual(
      DEFAULT_SETTINGS,
    );
    expect(loadSettings(memory({ [LEGACY_THEME_KEY]: "sepia" }))).toEqual(
      DEFAULT_SETTINGS,
    );
  });
});
