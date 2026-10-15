/**
 * Nocturne theme system (ADR 0009): an appearance mode and an ambiance (hue + chroma) give the
 * CSS custom properties set on the document root. `index.css` maps them onto the Tailwind
 * `@theme` tokens and the shadcn variables, so every component follows.
 */

export const MODES = ["dark", "light", "system"] as const;
export type Mode = (typeof MODES)[number];

export const AMBIANCES = [
  { id: "arcane", h: 289, c: 0.125 },
  { id: "sylve", h: 152, c: 0.11 },
  { id: "sang", h: 22, c: 0.15 },
  { id: "ocean", h: 240, c: 0.11 },
  { id: "ambre", h: 72, c: 0.12 },
  { id: "obsidienne", h: 265, c: 0.014 },
] as const;
export type AmbianceId = (typeof AMBIANCES)[number]["id"];
export const DEFAULT_AMBIANCE: AmbianceId = "arcane";

export function isMode(v: unknown): v is Mode {
  return MODES.includes(v as Mode);
}

export function isAmbiance(v: unknown): v is AmbianceId {
  return AMBIANCES.some((a) => a.id === v);
}

/** Ramp steps 100..900 (index 0..8). */
export const STEPS = [100, 200, 300, 400, 500, 600, 700, 800, 900] as const;
/** Accent ramp: lightness and chroma factor per step (dark mode order). */
const ACCENT_L = [0.97, 0.93, 0.87, 0.78, 0.68, 0.58, 0.48, 0.38, 0.27];
const ACCENT_CF = [0.15, 0.3, 0.55, 0.85, 1, 0.95, 0.8, 0.6, 0.4];
/** Neutral ramp lightness per step (dark mode order); its chroma is a faint tint of the hue. */
const NEUTRAL_L = [0.97, 0.93, 0.87, 0.77, 0.66, 0.56, 0.46, 0.37, 0.28];

/** A near-neutral ambiance highlights in light grey (or dark grey in light mode), not in colour. */
const NEAR_NEUTRAL_CHROMA = 0.02;

const n3 = (x: number) => Number(x.toFixed(3)).toString();
const oklch = (l: number, c: number, h: number) => `oklch(${n3(l)} ${n3(c)} ${h})`;

/**
 * The custom properties for an ambiance in dark or light mode. Light mode reverses both ramps
 * (step 100 becomes the darkest), so a class like `bg-amb-900` stays a quiet tint and
 * `text-amb-300` stays a readable highlight in either mode.
 */
export function themeVars(ambiance: AmbianceId, light: boolean): Record<string, string> {
  const a = AMBIANCES.find((x) => x.id === ambiance) ?? AMBIANCES[0];
  const { h, c } = a;
  const nearNeutral = c < NEAR_NEUTRAL_CHROMA;
  const nc = Math.min(0.03, c * 0.22) + 0.006; // neutral tint chroma
  const v: Record<string, string> = {};

  STEPS.forEach((step, i) => {
    const j = light ? 8 - i : i;
    v[`--amb-${step}`] = oklch(ACCENT_L[j], c * ACCENT_CF[j], h);
    v[`--neu-${step}`] = oklch(NEUTRAL_L[j], nc * 0.6, h);
  });
  if (nearNeutral && !light) {
    v["--amb-200"] = oklch(0.95, 0.01, h);
    v["--amb-300"] = oklch(0.92, 0.01, h);
  }

  if (nearNeutral) v["--tint"] = light ? oklch(0.3, 0.02, h) : oklch(0.86, 0.02, h);
  else v["--tint"] = light ? oklch(0.52, c, h) : oklch(0.68, c, h);

  if (light) {
    v["--ground"] = oklch(0.965, nc * 0.5, h);
    v["--surface"] = oklch(0.995, 0.002, h);
    v["--ink"] = oklch(0.24, nc, h);
  } else {
    v["--ground"] = oklch(nearNeutral ? 0.16 : 0.19, nc + 0.012, h);
    v["--surface"] = oklch(nearNeutral ? 0.215 : 0.245, nc + 0.008, h);
    v["--ink"] = oklch(0.94, 0.006, h);
  }

  // State colours: L' = lA + lB * L, so a light state colour becomes a dark one in light mode.
  v["--lA"] = light ? "1.3" : "0";
  v["--lB"] = light ? "-1" : "1";
  v["--shadow-alpha"] = light ? "0.16" : "0.65";
  return v;
}

/** Whether the effective appearance is light, given the mode and the OS preference. */
export function isLight(mode: Mode, systemPrefersDark: boolean): boolean {
  return mode === "light" || (mode === "system" && !systemPrefersDark);
}

const darkQuery = () => window.matchMedia("(prefers-color-scheme: dark)");

export function systemPrefersDark(): boolean {
  try {
    return darkQuery().matches;
  } catch {
    return true;
  }
}

/** Set `.dark`, `color-scheme` and the theme variables on the document root. */
export function applyTheme(mode: Mode, ambiance: AmbianceId) {
  const root = document.documentElement;
  const light = isLight(mode, systemPrefersDark());
  root.classList.toggle("dark", !light);
  root.style.colorScheme = light ? "light" : "dark";
  root.dataset.ambiance = ambiance;
  for (const [k, val] of Object.entries(themeVars(ambiance, light))) {
    root.style.setProperty(k, val);
  }
}

/** Call `onChange` when the OS light/dark preference changes; returns the unsubscribe. */
export function watchSystemScheme(onChange: () => void): () => void {
  const q = darkQuery();
  q.addEventListener("change", onChange);
  return () => q.removeEventListener("change", onChange);
}
