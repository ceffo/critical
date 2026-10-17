import { create } from "zustand";
import i18n from "@/i18n";
import {
  loadSettings,
  savePaint,
  saveSettings,
  type Settings,
} from "@/lib/settings";
import { applyTheme, watchSystemScheme } from "@/lib/theme";

interface SettingsStore extends Settings {
  /** Change one or more preferences: applied live and saved. */
  update: (patch: Partial<Settings>) => void;
}

type Display = Pick<Settings, "lang" | "mode" | "darkTheme" | "lightTheme">;

function apply(s: Display) {
  applyTheme(s.mode, s.darkTheme, s.lightTheme);
  savePaint(s);
  document.documentElement.lang = s.lang;
  if (i18n.language !== s.lang) void i18n.changeLanguage(s.lang);
}

/** Display preferences of this device (ADR 0009, ADR 0026); see lib/settings.ts for storage. */
export const useSettingsStore = create<SettingsStore>((set, get) => ({
  ...loadSettings(),
  update: (patch) => {
    set(patch);
    const s = get();
    saveSettings(s);
    apply(s);
  },
}));

/**
 * Apply the saved preferences now and, in "system" mode, switch between the dark and the light
 * theme live when the OS preference changes (ADR 0026 step 4.2).
 */
export function startSettings() {
  apply(useSettingsStore.getState());
  watchSystemScheme(() => {
    const s = useSettingsStore.getState();
    if (s.mode === "system") applyTheme(s.mode, s.darkTheme, s.lightTheme);
  });
}
