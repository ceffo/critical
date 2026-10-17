/**
 * Theme system (ADR 0009, ADR 0020, the themes and the contrast ladder of ADR 0026): the appearance
 * mode picks the dark or the light theme the device chose, and the theme's role table for that mode
 * gives the CSS custom properties set on the document root. `index.css` maps them onto the Tailwind
 * `@theme` tokens and the shadcn variables, so every component follows. Components use roles
 * (`tint`, `highlight`, `magic`, `danger`, …) and ramp steps, never raw colours.
 */

export const MODES = ["dark", "light", "system"] as const;
export type Mode = (typeof MODES)[number];

/** A hue and its peak chroma (oklch). */
export interface Hue {
  h: number;
  c: number;
}

/** An oklch colour: lightness, chroma, hue. Chroma is clamped into sRGB when painted. */
export type Lch = readonly [number, number, number];

/** The strength of the ink in the translucent tokens (ADR 0026 step 1.2), in percent. */
export interface Mix {
  /** `--ink-muted`: timestamps, units, hints (meta only, step 1.3). */
  muted: number;
  /** `--border`: the card outline. */
  border: number;
  /** `--input`: the field edge (3:1 on the card, step 2.2). */
  input: number;
}

/** One theme in one mode: its ramps, its surfaces and its roles (ADR 0026 step 1). */
export interface Palette {
  /** The primary (`amb-*`), secondary (`sec-*`) and tertiary (`ter-*`) ramps' hues. */
  ramps: { amb: Hue; sec: Hue; ter: Hue };
  /** The neutral ramp `neu-100..700`, in this mode's order (light: 100 is the darkest). */
  neutral: { h: number; c: number; l: readonly number[] };
  ground: Lch;
  surface: Lch;
  /** The raised step (`neu-900`): tiles, rows, filled chips. */
  raised: Lch;
  /** One neutral step past raised (`neu-800`): the hover of a raised row or chip. */
  hover: Lch;
  /** The selected tone (`amb-800`): the primary hue at low chroma. */
  selected: Lch;
  ink: Lch;
  tint: Lch;
  highlight: Lch;
  /** The highlight as a fill (crit badge, a maximum die), with `on-highlight` text on it. */
  highlightFill: Lch;
  magic: Lch;
  danger: Lch;
  success: Lch;
  warning: Lch;
  /** Text on any accent used as a fill. */
  onAccent: Lch;
  onHighlight: Lch;
  mix: Mix;
  shadowAlpha: string;
  /** Ramp steps painted by hand (Plain's near-neutral primary keeps its light steps grey). */
  overrides?: Record<string, Lch>;
}

export interface Theme {
  id: string;
  dark?: Palette;
  light?: Palette;
}

/** Ramp steps 100..900 (index 0..8). */
export const STEPS = [100, 200, 300, 400, 500, 600, 700, 800, 900] as const;
/** Palette ramps: lightness and chroma factor per step (dark mode order, 100 is the lightest). */
const RAMP_L = [0.96, 0.91, 0.84, 0.76, 0.68, 0.58, 0.47, 0.36, 0.26];
const RAMP_CF = [0.2, 0.4, 0.65, 0.9, 1, 1, 0.9, 0.75, 0.55];
/** The neutral ramp's lightness, dark mode order; light mode reverses it (ADR 0009 step 4). */
const NEUTRAL_L = [0.96, 0.91, 0.85, 0.76, 0.66, 0.56, 0.46, 0.36, 0.27];
const NEUTRAL_L_LIGHT = [...NEUTRAL_L].reverse();

/** The translucent mixes of ADR 0026 step 1.2. */
const MIX_DARK: Mix = { muted: 76, border: 32, input: 42 };
const MIX_LIGHT: Mix = { muted: 74, border: 34, input: 48 };
const MIX_PARCHMENT: Mix = { muted: 76, border: 34, input: 50 };

/**
 * The four themes of ADR 0026 step 3, a hand-tuned role table per mode. Values are the mock-up
 * round's (docs/design/ui-review/mockups/themes.md, "4 · Bespoke themes"), retuned where a rule of
 * steps 1-2 asked for it (the ADR's dated amendment lists them). The order is the popover's.
 */
export const THEMES = [
  {
    // The brand: violet-indigo primary (285), gold highlight (88), teal magic (200).
    id: "arcane",
    dark: {
      ramps: {
        amb: { h: 285, c: 0.17 },
        sec: { h: 88, c: 0.15 },
        ter: { h: 200, c: 0.12 },
      },
      neutral: { h: 285, c: 0.022, l: NEUTRAL_L },
      ground: [0.1, 0.048, 285],
      surface: [0.25, 0.046, 285],
      raised: [0.335, 0.029, 285],
      hover: [0.405, 0.029, 285],
      selected: [0.39, 0.102, 285],
      ink: [0.95, 0.008, 285],
      tint: [0.71, 0.14, 285],
      highlight: [0.9, 0.15, 88],
      highlightFill: [0.9, 0.15, 88],
      magic: [0.78, 0.12, 200],
      danger: [0.74, 0.19, 25],
      success: [0.82, 0.16, 150],
      warning: [0.86, 0.13, 65],
      onAccent: [0.15, 0.03, 285],
      onHighlight: [0.15, 0.03, 285],
      mix: MIX_DARK,
      shadowAlpha: "0.65",
    },
    light: {
      ramps: {
        amb: { h: 285, c: 0.17 },
        sec: { h: 88, c: 0.15 },
        ter: { h: 200, c: 0.12 },
      },
      neutral: { h: 285, c: 0.022, l: NEUTRAL_L_LIGHT },
      ground: [0.89, 0.044, 285],
      surface: [0.995, 0.004, 285],
      raised: [0.915, 0.029, 285],
      hover: [0.885, 0.029, 285],
      selected: [0.88, 0.077, 285],
      ink: [0.18, 0.036, 285],
      tint: [0.4, 0.17, 285],
      highlight: [0.5, 0.12, 88],
      highlightFill: [0.82, 0.15, 88],
      magic: [0.47, 0.1, 200],
      danger: [0.5, 0.2, 25],
      success: [0.485, 0.14, 150],
      warning: [0.51, 0.15, 62],
      onAccent: [0.985, 0.005, 285],
      onHighlight: [0.15, 0.03, 285],
      mix: MIX_LIGHT,
      shadowAlpha: "0.18",
    },
  },
  {
    // From Ocean dark: ink-blue page, cyan-blue primary (232), coral danger (30), jade success
    // (168), gold highlight (85), violet magic (300). Dark only.
    id: "deep",
    dark: {
      ramps: {
        amb: { h: 235, c: 0.12 },
        sec: { h: 85, c: 0.14 },
        ter: { h: 300, c: 0.12 },
      },
      neutral: {
        h: 245,
        c: 0.022,
        l: [0.96, 0.91, 0.85, 0.76, 0.66, 0.56, 0.46, 0.405, 0.335],
      },
      ground: [0.1, 0.035, 250],
      surface: [0.25, 0.04, 245],
      raised: [0.335, 0.022, 245],
      hover: [0.405, 0.022, 245],
      selected: [0.39, 0.07, 235],
      ink: [0.95, 0.01, 240],
      tint: [0.7, 0.12, 232],
      highlight: [0.9, 0.14, 85],
      highlightFill: [0.9, 0.14, 85],
      magic: [0.78, 0.12, 300],
      danger: [0.74, 0.18, 30],
      success: [0.82, 0.14, 168],
      warning: [0.86, 0.13, 65],
      onAccent: [0.15, 0.03, 245],
      onHighlight: [0.15, 0.03, 245],
      mix: MIX_DARK,
      shadowAlpha: "0.65",
    },
  },
  {
    // From Amber light: warm paper, brown-black ink, lapis primary (262), oxblood danger (27), moss
    // success (140), ochre warning (60), gilt highlight (82), violet magic (310). Light only.
    id: "parchment",
    light: {
      ramps: {
        amb: { h: 262, c: 0.12 },
        sec: { h: 86, c: 0.12 },
        ter: { h: 305, c: 0.11 },
      },
      neutral: {
        h: 75,
        c: 0.028,
        l: [0.3, 0.4, 0.5, 0.58, 0.66, 0.74, 0.8, 0.855, 0.89],
      },
      ground: [0.865, 0.034, 80],
      surface: [0.97, 0.02, 88],
      raised: [0.89, 0.028, 75],
      hover: [0.855, 0.028, 75],
      selected: [0.88, 0.05, 262],
      ink: [0.24, 0.035, 55],
      tint: [0.37, 0.12, 262],
      highlight: [0.485, 0.11, 82],
      highlightFill: [0.8, 0.135, 85],
      magic: [0.46, 0.12, 310],
      danger: [0.48, 0.17, 27],
      success: [0.47, 0.12, 140],
      warning: [0.49, 0.13, 60],
      onAccent: [0.97, 0.02, 88],
      onHighlight: [0.2, 0.035, 55],
      mix: MIX_PARCHMENT,
      shadowAlpha: "0.18",
    },
  },
  {
    // From Obsidian: near-neutral, no colour but the semantic ones and the gold highlight; it
    // highlights in grey (ADR 0026 step 3.6).
    id: "plain",
    dark: {
      ramps: {
        amb: { h: 265, c: 0.014 },
        sec: { h: 88, c: 0.13 },
        ter: { h: 200, c: 0.07 },
      },
      neutral: { h: 265, c: 0.005, l: NEUTRAL_L },
      ground: [0.1, 0.021, 265],
      surface: [0.25, 0.019, 265],
      raised: [0.335, 0.007, 265],
      hover: [0.405, 0.007, 265],
      selected: [0.39, 0.008, 265],
      ink: [0.95, 0.008, 265],
      tint: [0.86, 0.02, 265],
      highlight: [0.9, 0.13, 88],
      highlightFill: [0.9, 0.13, 88],
      magic: [0.78, 0.07, 200],
      danger: [0.74, 0.19, 25],
      success: [0.82, 0.16, 150],
      warning: [0.86, 0.13, 65],
      onAccent: [0.15, 0.03, 265],
      onHighlight: [0.15, 0.03, 265],
      mix: MIX_DARK,
      shadowAlpha: "0.65",
      overrides: {
        "--amb-200": [0.95, 0.01, 265],
        "--amb-300": [0.92, 0.01, 265],
      },
    },
    light: {
      ramps: {
        amb: { h: 265, c: 0.014 },
        sec: { h: 88, c: 0.13 },
        ter: { h: 200, c: 0.07 },
      },
      neutral: { h: 265, c: 0.005, l: NEUTRAL_L_LIGHT },
      ground: [0.89, 0.017, 265],
      surface: [0.995, 0.004, 265],
      raised: [0.915, 0.007, 265],
      hover: [0.885, 0.007, 265],
      selected: [0.88, 0.006, 265],
      ink: [0.18, 0.009, 265],
      tint: [0.3, 0.02, 265],
      highlight: [0.5, 0.12, 88],
      highlightFill: [0.82, 0.13, 88],
      magic: [0.47, 0.07, 200],
      danger: [0.5, 0.2, 25],
      success: [0.485, 0.14, 150],
      warning: [0.51, 0.15, 62],
      onAccent: [0.985, 0.005, 265],
      onHighlight: [0.15, 0.03, 265],
      mix: MIX_LIGHT,
      shadowAlpha: "0.18",
    },
  },
] as const satisfies readonly Theme[];

export type ThemeId = (typeof THEMES)[number]["id"];
type WithMode<M extends "dark" | "light"> = Extract<
  (typeof THEMES)[number],
  Record<M, Palette>
>["id"];
/** The themes offered as the dark choice (ADR 0026 step 4.1): Arcane, Deep, Plain. */
export type DarkThemeId = WithMode<"dark">;
/** The themes offered as the light choice: Arcane, Parchment, Plain. */
export type LightThemeId = WithMode<"light">;

export const DARK_THEMES = THEMES.filter((t) => "dark" in t).map(
  (t) => t.id,
) as DarkThemeId[];
export const LIGHT_THEMES = THEMES.filter((t) => "light" in t).map(
  (t) => t.id,
) as LightThemeId[];

/** The default pair (ADR 0026 step 3.3). */
export const DEFAULT_DARK_THEME: DarkThemeId = "arcane";
export const DEFAULT_LIGHT_THEME: LightThemeId = "arcane";

/** A primary under this chroma is near-neutral: it highlights in grey (ADR 0026 step 3.6). */
export const NEAR_NEUTRAL_CHROMA = 0.02;

export function isMode(v: unknown): v is Mode {
  return MODES.includes(v as Mode);
}

export function isDarkTheme(v: unknown): v is DarkThemeId {
  return DARK_THEMES.includes(v as DarkThemeId);
}

export function isLightTheme(v: unknown): v is LightThemeId {
  return LIGHT_THEMES.includes(v as LightThemeId);
}

/** The role table of a theme in a mode, or undefined when the theme has no such mode. */
export function palette(id: string, light: boolean): Palette | undefined {
  const t: Theme | undefined = THEMES.find((x) => x.id === id);
  return light ? t?.light : t?.dark;
}

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

/** An oklch literal with 3 decimals, its chroma clamped to the sRGB gamut (never its lightness). */
const oklch = (l: number, c: number, h: number) =>
  `oklch(${n3(l)} ${n3(clampChroma(l, c, h))} ${h})`;
const lit = ([l, c, h]: Lch) => oklch(l, c, h);

const inkMix = (pct: number) =>
  `color-mix(in oklab, var(--ink) ${pct}%, transparent)`;

/**
 * The custom properties of a theme in dark or light mode. A theme without that mode falls back to
 * the mode's default theme. Light mode reverses the ramps (step 100 becomes the darkest), so a
 * class like `bg-amb-900` stays a quiet tint and `text-amb-300` a readable colour in either mode.
 * `theme.test.ts` holds every table to the rules of ADR 0026 steps 1-3.
 */
export function themeVars(id: string, light: boolean): Record<string, string> {
  const p =
    palette(id, light) ??
    palette(light ? DEFAULT_LIGHT_THEME : DEFAULT_DARK_THEME, light)!;
  const v: Record<string, string> = {};
  for (const [prefix, hue] of Object.entries(p.ramps))
    STEPS.forEach((step, i) => {
      const j = light ? 8 - i : i;
      v[`--${prefix}-${step}`] = oklch(RAMP_L[j], hue.c * RAMP_CF[j], hue.h);
    });
  STEPS.forEach((step, i) => {
    v[`--neu-${step}`] = oklch(p.neutral.l[i], p.neutral.c, p.neutral.h);
  });
  for (const [k, c] of Object.entries(p.overrides ?? {})) v[k] = lit(c);
  v["--neu-900"] = lit(p.raised);
  v["--neu-800"] = lit(p.hover);
  v["--amb-800"] = lit(p.selected);
  v["--ground"] = lit(p.ground);
  v["--surface"] = lit(p.surface);
  v["--ink"] = lit(p.ink);
  v["--tint"] = lit(p.tint);
  v["--highlight"] = lit(p.highlight);
  v["--highlight-fill"] = lit(p.highlightFill);
  v["--magic"] = lit(p.magic);
  v["--danger"] = lit(p.danger);
  v["--success"] = lit(p.success);
  v["--warning"] = lit(p.warning);
  v["--on-accent"] = lit(p.onAccent);
  v["--on-highlight"] = lit(p.onHighlight);
  v["--ink-muted"] = inkMix(p.mix.muted);
  v["--border"] = inkMix(p.mix.border);
  v["--input"] = inkMix(p.mix.input);
  v["--shadow-alpha"] = p.shadowAlpha;
  return v;
}

/** Whether the effective appearance is light, given the mode and the OS preference. */
export function isLight(mode: Mode, systemPrefersDark: boolean): boolean {
  return mode === "light" || (mode === "system" && !systemPrefersDark);
}

/** The theme that applies (ADR 0026 step 4.2): the light or the dark choice, by the mode. */
export function effectiveTheme(
  mode: Mode,
  systemPrefersDark: boolean,
  dark: DarkThemeId,
  light: LightThemeId,
): { id: ThemeId; light: boolean } {
  const l = isLight(mode, systemPrefersDark);
  return { id: l ? light : dark, light: l };
}

const darkQuery = () => window.matchMedia("(prefers-color-scheme: dark)");

export function systemPrefersDark(): boolean {
  try {
    return darkQuery().matches;
  } catch {
    return true;
  }
}

/** Set `.dark`, `color-scheme` and the effective theme's variables on the document root. */
export function applyTheme(mode: Mode, dark: DarkThemeId, light: LightThemeId) {
  const root = document.documentElement;
  const e = effectiveTheme(mode, systemPrefersDark(), dark, light);
  root.classList.toggle("dark", !e.light);
  root.style.colorScheme = e.light ? "light" : "dark";
  root.dataset.theme = e.id;
  for (const [k, val] of Object.entries(themeVars(e.id, e.light))) {
    root.style.setProperty(k, val);
  }
}

/** Call `onChange` when the OS light/dark preference changes; returns the unsubscribe. */
export function watchSystemScheme(onChange: () => void): () => void {
  const q = darkQuery();
  q.addEventListener("change", onChange);
  return () => q.removeEventListener("change", onChange);
}
