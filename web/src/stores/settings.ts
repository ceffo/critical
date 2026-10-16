import { create } from "zustand";
import i18n from "@/i18n";
import { loadSettings, saveSettings, type Settings } from "@/lib/settings";
import { applyTheme, watchSystemScheme } from "@/lib/theme";

interface SettingsStore extends Settings {
  /** Change one or more preferences: applied live and saved. */
  update: (patch: Partial<Settings>) => void;
}

function apply(s: Pick<Settings, "lang" | "mode" | "amb">) {
  applyTheme(s.mode, s.amb);
  document.documentElement.lang = s.lang;
  if (i18n.language !== s.lang) void i18n.changeLanguage(s.lang);
}

/** Display preferences of this device (ADR 0009); see lib/settings.ts for storage. */
export const useSettingsStore = create<SettingsStore>((set, get) => ({
  ...loadSettings(),
  update: (patch) => {
    set(patch);
    const { lang, mode, amb, journalHeight, journalCollapsed } = get();
    saveSettings({ lang, mode, amb, journalHeight, journalCollapsed });
    apply({ lang, mode, amb });
  },
}));

/** Apply the saved preferences now and follow OS light/dark changes while in "system" mode. */
export function startSettings() {
  const { lang, mode, amb } = useSettingsStore.getState();
  apply({ lang, mode, amb });
  watchSystemScheme(() => {
    const s = useSettingsStore.getState();
    if (s.mode === "system") applyTheme(s.mode, s.amb);
  });
}
