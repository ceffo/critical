/**
 * Theme system (ADR 0009, palettes and roles by ADR 0020): an appearance mode and an ambiance
 * (three hues: primary, secondary, tertiary) give the CSS custom properties set on the document
 * root. `index.css` maps them onto the Tailwind `@theme` tokens and the shadcn variables, so every
 * component follows. Components use roles (`tint`, `highlight`, `magic`, `danger`, …) and ramp
 * steps, never raw colours.
 */

export const MODES = ["dark", "light", "system"] as const;
export type Mode = (typeof MODES)[number];

/** A hue and its peak chroma (oklch). */
export interface Hue {
  h: number;
  c: number;
}

/**
 * An ambiance names its three palettes (ADR 0020 step 1): primary (chrome, selection, actions),
 * secondary (highlights: the brand "!", key values, rewards) and tertiary (magic: spells, slots).
 * Hues keep clear of the semantic ones (danger 25, success 150, warning 55) where the identity
 * allows; Sang's crimson primary is the accepted exception.
 */
export const AMBIANCES = [
  {
    id: "arcane",
    primary: { h: 285, c: 0.17 },
    secondary: { h: 88, c: 0.15 },
    tertiary: { h: 200, c: 0.12 },
  },
  {
    id: "sylve",
    primary: { h: 152, c: 0.13 },
    secondary: { h: 70, c: 0.15 },
    tertiary: { h: 300, c: 0.13 },
  },
  {
    id: "sang",
    primary: { h: 15, c: 0.17 },
    secondary: { h: 85, c: 0.15 },
    tertiary: { h: 250, c: 0.13 },
  },
  {
    id: "ocean",
    primary: { h: 240, c: 0.14 },
    secondary: { h: 75, c: 0.14 },
    tertiary: { h: 330, c: 0.13 },
  },
  {
    id: "ambre",
    primary: { h: 72, c: 0.14 },
    secondary: { h: 300, c: 0.14 },
    tertiary: { h: 200, c: 0.12 },
  },
  {
    id: "obsidienne",
    primary: { h: 265, c: 0.014 },
    secondary: { h: 88, c: 0.13 },
    tertiary: { h: 200, c: 0.07 },
  },
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
/** Palette ramps: lightness and chroma factor per step (dark mode order, 100 is the lightest). */
const RAMP_L = [0.96, 0.91, 0.84, 0.76, 0.68, 0.58, 0.47, 0.36, 0.26];
const RAMP_CF = [0.2, 0.4, 0.65, 0.9, 1, 1, 0.9, 0.75, 0.55];
/** Neutral ramp lightness per step (dark mode order); its chroma is a faint tint of the primary. */
const NEUTRAL_L = [0.96, 0.91, 0.85, 0.76, 0.66, 0.56, 0.46, 0.36, 0.27];

/** A near-neutral primary (Obsidienne) highlights in grey, not in colour. */
const NEAR_NEUTRAL_CHROMA = 0.02;

/** Semantic hues (ADR 0020 step 3), the same in every ambiance. */
export const SEMANTIC = {
  danger: 25,
  success: 150,
  warning: 55,
} as const;

const n3 = (x: number) => Number(x.toFixed(3)).toString();

/** oklch -> linear sRGB (CSS Color 4 matrices), unclipped. */
export function oklchToLinearRgb(
  l: number,
  c: number,
  h: number,
): [number, number, number] {
  const a = c * Math.cos((h * Math.PI) / 180);
  const b = c * Math.sin((h * Math.PI) / 180);
  const L = (l + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const M = (l - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const S = (l - 0.0894841775 * a - 1.291485548 * b) ** 3;
  return [
    4.0767416621 * L - 3.3077115913 * M + 0.2309699292 * S,
    -1.2684380046 * L + 2.6097574011 * M - 0.3413193965 * S,
    -0.0041960863 * L - 0.7034186147 * M + 1.707614701 * S,
  ];
}

const inGamut = (l: number, c: number, h: number) =>
  oklchToLinearRgb(l, c, h).every((x) => x >= 0 && x <= 1);

/** The chroma, at most `c`, that keeps (l, h) inside the sRGB gamut (ADR 0020 step 4). */
function clampChroma(l: number, c: number, h: number): number {
  if (inGamut(l, c, h)) return c;
  let lo = 0;
  let hi = c;
  for (let i = 0; i < 20; i++) {
    const mid = (lo + hi) / 2;
    if (inGamut(l, mid, h)) lo = mid;
    else hi = mid;
  }
  return Math.floor(lo * 1000) / 1000; // rounded down, so the 3-decimal literal stays inside
}

/** An oklch literal with 3 decimals, its chroma clamped to the sRGB gamut. */
const oklch = (l: number, c: number, h: number) =>
  `oklch(${n3(l)} ${n3(clampChroma(l, c, h))} ${h})`;

/**
 * The custom properties for an ambiance in dark or light mode. Light mode reverses the ramps
 * (step 100 becomes the darkest), so a class like `bg-amb-900` stays a quiet tint and
 * `text-amb-300` stays a readable colour in either mode.
 *
 * Role colours are tuned per mode for WCAG AA on the surfaces (ADR 0020 step 4): every accent is
 * readable as text on `surface` and on `neu-900`, and `on-accent` is readable on every accent
 * used as a fill (dark ink on the bright accents of dark mode, near-white on the deep accents of
 * light mode).
 */
export function themeVars(
  ambiance: AmbianceId,
  light: boolean,
): Record<string, string> {
  const a = AMBIANCES.find((x) => x.id === ambiance) ?? AMBIANCES[0];
  const { h, c } = a.primary;
  const nearNeutral = c < NEAR_NEUTRAL_CHROMA;
  const nc = Math.min(0.03, c * 0.22) + 0.006; // neutral tint chroma
  const v: Record<string, string> = {};

  const ramp = (prefix: string, hue: Hue) =>
    STEPS.forEach((step, i) => {
      const j = light ? 8 - i : i;
      v[`--${prefix}-${step}`] = oklch(RAMP_L[j], hue.c * RAMP_CF[j], hue.h);
    });
  ramp("amb", a.primary);
  ramp("sec", a.secondary);
  ramp("ter", a.tertiary);
  STEPS.forEach((step, i) => {
    const j = light ? 8 - i : i;
    v[`--neu-${step}`] = oklch(NEUTRAL_L[j], nc * 0.6, h);
  });
  if (nearNeutral && !light) {
    v["--amb-200"] = oklch(0.95, 0.01, h);
    v["--amb-300"] = oklch(0.92, 0.01, h);
  }

  // Accent roles: one lightness per mode, chroma capped so every hue stays in the sRGB gamut.
  const accent = (
    hue: Hue,
    lDark: number,
    cDark: number,
    lLight: number,
    cLight: number,
  ) =>
    light
      ? oklch(lLight, Math.min(hue.c, cLight), hue.h)
      : oklch(lDark, Math.min(hue.c, cDark), hue.h);
  v["--tint"] = nearNeutral
    ? light
      ? oklch(0.3, 0.02, h)
      : oklch(0.86, 0.02, h)
    : accent(a.primary, 0.72, 0.14, 0.46, 0.2);
  v["--highlight"] = accent(a.secondary, 0.82, 0.15, 0.53, 0.11);
  // A highlight used as a fill (crit badge, a maximum die) stays bright gold in light mode too,
  // with dark text on it: a gold that takes white text would be olive.
  v["--highlight-fill"] = light
    ? oklch(0.8, Math.min(a.secondary.c, 0.15), a.secondary.h)
    : v["--highlight"];
  v["--magic"] = accent(a.tertiary, 0.78, 0.12, 0.48, 0.1);
  v["--danger"] = light
    ? oklch(0.52, 0.19, SEMANTIC.danger)
    : oklch(0.72, 0.15, SEMANTIC.danger);
  v["--success"] = light
    ? oklch(0.5, 0.13, SEMANTIC.success)
    : oklch(0.78, 0.15, SEMANTIC.success);
  v["--warning"] = light
    ? oklch(0.54, 0.14, SEMANTIC.warning)
    : oklch(0.78, 0.14, SEMANTIC.warning);
  // Text on any accent used as a fill; on the highlight fill it is always dark.
  v["--on-accent"] = light ? oklch(0.985, 0.005, h) : oklch(0.15, 0.03, h);
  v["--on-highlight"] = oklch(0.15, 0.03, h);

  if (light) {
    v["--ground"] = oklch(0.93, nc, h);
    v["--surface"] = oklch(0.985, 0.005, h);
    v["--ink"] = oklch(0.2, nc, h);
  } else {
    v["--ground"] = oklch(nearNeutral ? 0.13 : 0.15, nc + 0.012, h);
    v["--surface"] = oklch(nearNeutral ? 0.18 : 0.205, nc + 0.008, h);
    v["--ink"] = oklch(0.95, 0.008, h);
  }
  v["--shadow-alpha"] = light ? "0.18" : "0.65";
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
