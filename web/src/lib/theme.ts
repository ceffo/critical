export const THEMES = ["light", "dark", "system"] as const;
export type Theme = (typeof THEMES)[number];
export const THEME_STORAGE_KEY = "critical.theme";

function isTheme(v: string | null): v is Theme {
  return THEMES.includes(v as Theme);
}

/** Saved preference (localStorage), else follow the OS. */
export function getTheme(): Theme {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    if (isTheme(saved)) return saved;
  } catch {
    // storage unavailable
  }
  return "system";
}

const query = () => window.matchMedia("(prefers-color-scheme: dark)");

/** Toggle `.dark` on the document root for the given preference. */
export function applyTheme(theme: Theme) {
  const dark = theme === "dark" || (theme === "system" && query().matches);
  document.documentElement.classList.toggle("dark", dark);
}

export function setTheme(theme: Theme) {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // ignore
  }
  applyTheme(theme);
}

export function nextTheme(theme: Theme): Theme {
  return THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length];
}

/** Follow OS changes live while the preference is "system". */
export function watchSystemTheme() {
  query().addEventListener("change", () => {
    if (getTheme() === "system") applyTheme("system");
  });
}
