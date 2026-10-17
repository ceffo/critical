import {
  AMBIANCES,
  STEPS,
  oklchToLinearRgb,
  themeVars,
  type AmbianceId,
} from "../src/lib/theme";
import { hex, linear, over, parseOklch, ratio, type Rgb } from "./wcag";

/**
 * The palettes of the mock-up round (docs/design/ui-review/mockups/README.md): for each option of
 * the UI review's "Recommendations", the token values it proposes, per theme and mode. Nothing here
 * reaches the app: mockups.spec.ts paints these values over the running app (a stylesheet that
 * overrides the custom properties `lib/theme.ts` sets), together with the option's component rules
 * (docs/design/ui-review/mockups/css/*.css), and writes the measured contrast of every theme.
 *
 * Every number below is a proposal for the ADR that follows the user's pick (ADR 0020 step 2's
 * table); the app's own generator is untouched.
 */

export type Vars = Record<string, string>;

/** How strong the ink is in the three translucent tokens (muted text, outlines, field borders). */
export interface Mix {
  muted: number;
  border: number;
  input: number;
}

export interface Theme {
  /** Stable id, used in file names and as `data-mock-theme` on <html>. */
  id: string;
  name: string;
  mode: "dark" | "light";
  /** The app ambiance the capture selects before painting the overrides. */
  base: AmbianceId;
  /** What the option does with it: keep as is (with the new tokens), retune, drop, or a new theme. */
  status: "keep" | "retune" | "drop" | "new";
  why: string;
  vars: Vars;
  mix: Mix;
}

const n3 = (x: number) => Number(x.toFixed(3)).toString();
const inGamut = (l: number, c: number, h: number) =>
  oklchToLinearRgb(l, c, h).every((x) => x >= 0 && x <= 1);
/** An oklch literal, chroma clamped into sRGB (lightness never moves), as `themeVars` does. */
export function ok(l: number, c: number, h: number): string {
  let lo = 0;
  let hi = c;
  if (!inGamut(l, c, h)) {
    for (let i = 0; i < 20; i++) {
      const mid = (lo + hi) / 2;
      if (inGamut(l, mid, h)) lo = mid;
      else hi = mid;
    }
    c = Math.floor(lo * 1000) / 1000;
  }
  return `oklch(${n3(l)} ${n3(c)} ${h})`;
}

const ambiance = (id: AmbianceId) => AMBIANCES.find((a) => a.id === id)!;

/** Today's translucent mixes (ADR 0020 step 6). */
export const TODAY_MIX: Mix = { muted: 70, border: 18, input: 22 };
/** Direction 1's: muted text a little stronger (meta only), outlines and field edges visible. */
const LADDER_MIX: Record<"dark" | "light", Mix> = {
  dark: { muted: 76, border: 32, input: 42 },
  light: { muted: 74, border: 34, input: 48 },
};

/**
 * Direction 1, the contrast ladder: the page darker, every card lighter, raised blocks one more
 * step (dark L .10 / .25 / .335, light .89 / .995 / .915), and the six accents spread on a lightness
 * ladder so emphasis is read before hue. Dark: tint .65 < danger .74 < magic .78 < success .82 <
 * warning .86 < highlight .90 < ink .95 (every accent stays AA as text on the card, which bounds the
 * ladder below at L ~.62: danger cannot get 1.8:1 off tint, it stands out by hue and as a fill).
 * Light mirrored: ink .18, tint .40, magic .47, danger .50, success .52, highlight .555, warning
 * .565. Hues unchanged.
 */
export function ladder(id: AmbianceId, light: boolean): Vars {
  const v = { ...themeVars(id, light) };
  const a = ambiance(id);
  const { h, c } = a.primary;
  const near = c < 0.02;
  const nc = Math.min(0.03, c * 0.22) + 0.006;
  const sec = a.secondary;
  if (!light) {
    v["--ground"] = ok(0.1, nc + 0.012, h);
    v["--surface"] = ok(0.25, nc + 0.01, h);
    v["--neu-900"] = ok(0.335, nc * 0.8, h);
    v["--neu-800"] = ok(0.405, nc * 0.8, h);
    v["--tint"] = near ? ok(0.86, 0.02, h) : ok(0.65, Math.min(c, 0.14), h);
    v["--highlight"] = ok(0.9, Math.min(sec.c, 0.15), sec.h);
    v["--highlight-fill"] = v["--highlight"];
    v["--magic"] = ok(0.78, Math.min(a.tertiary.c, 0.12), a.tertiary.h);
    v["--danger"] = ok(0.74, 0.19, 25);
    v["--success"] = ok(0.82, 0.16, 150);
    v["--warning"] = ok(0.86, 0.13, 65);
    v["--amb-800"] = ok(0.39, Math.min(c, 0.17) * 0.6, h);
  } else {
    v["--ground"] = ok(0.89, nc + 0.008, h);
    v["--surface"] = ok(0.995, 0.004, h);
    v["--neu-900"] = ok(0.915, nc * 0.8, h);
    v["--neu-800"] = ok(0.885, nc * 0.8, h);
    v["--ink"] = ok(0.18, nc, h);
    v["--tint"] = near ? ok(0.3, 0.02, h) : ok(0.4, Math.min(c, 0.2), h);
    v["--highlight"] = ok(0.555, Math.min(sec.c, 0.12), sec.h);
    v["--highlight-fill"] = ok(0.82, Math.min(sec.c, 0.15), sec.h);
    v["--magic"] = ok(0.47, Math.min(a.tertiary.c, 0.1), a.tertiary.h);
    v["--danger"] = ok(0.5, 0.2, 25);
    v["--success"] = ok(0.52, 0.14, 150);
    v["--warning"] = ok(0.565, 0.15, 62);
    v["--amb-800"] = ok(0.88, Math.min(c, 0.17) * 0.45, h);
  }
  return v;
}

/** The nine-step ramps of ADR 0020 step 1, for the hand-made themes of direction 4. */
const RAMP_L = [0.96, 0.91, 0.84, 0.76, 0.68, 0.58, 0.47, 0.36, 0.26];
const RAMP_CF = [0.2, 0.4, 0.65, 0.9, 1, 1, 0.9, 0.75, 0.55];
function ramps(
  hues: { amb: [number, number]; sec: [number, number]; ter: [number, number] },
  neutral: { h: number; c: number; l: number[] },
  light: boolean,
): Vars {
  const v: Vars = {};
  for (const [prefix, [h, c]] of Object.entries(hues))
    STEPS.forEach((step, i) => {
      const j = light ? 8 - i : i;
      v[`--${prefix}-${step}`] = ok(RAMP_L[j], c * RAMP_CF[j], h);
    });
  STEPS.forEach((step, i) => {
    v[`--neu-${step}`] = ok(neutral.l[i], neutral.c, neutral.h);
  });
  return v;
}

/**
 * Direction 4's Parchment (light): warm paper, brown-black ink, a lapis primary (blue ink: far from
 * every semantic hue), oxblood danger, moss success, ochre warning, gilt highlight, a violet magic:
 * the palette of an illuminated manuscript.
 */
function parchment(): Vars {
  const v = ramps(
    { amb: [262, 0.12], sec: [86, 0.12], ter: [305, 0.11] },
    // light order: neu-100 the darkest, neu-900 the raised step under a card
    { h: 75, c: 0.028, l: [0.3, 0.4, 0.5, 0.58, 0.66, 0.74, 0.8, 0.855, 0.89] },
    true,
  );
  return {
    ...v,
    "--ground": ok(0.865, 0.034, 80),
    "--surface": ok(0.97, 0.02, 88),
    "--ink": ok(0.24, 0.035, 55),
    "--tint": ok(0.37, 0.12, 262),
    "--highlight": ok(0.53, 0.11, 82),
    "--highlight-fill": ok(0.8, 0.135, 85),
    "--magic": ok(0.46, 0.12, 310),
    "--danger": ok(0.48, 0.17, 27),
    "--success": ok(0.5, 0.12, 140),
    "--warning": ok(0.545, 0.13, 60),
    "--on-accent": ok(0.97, 0.02, 88),
    "--on-highlight": ok(0.2, 0.035, 55),
    "--amb-800": ok(0.88, 0.05, 262),
    "--shadow-alpha": "0.18",
  };
}

/**
 * Direction 4's Deep (dark, from Ocean): ink-blue ground, a cyan-blue primary, coral danger, jade
 * success, gold highlight, violet magic; each accent at its own lightness.
 */
function deep(): Vars {
  const v = ramps(
    { amb: [235, 0.12], sec: [85, 0.14], ter: [300, 0.12] },
    {
      h: 245,
      c: 0.022,
      l: [0.96, 0.91, 0.85, 0.76, 0.66, 0.56, 0.46, 0.405, 0.335],
    },
    false,
  );
  return {
    ...v,
    "--ground": ok(0.1, 0.035, 250),
    "--surface": ok(0.25, 0.04, 245),
    "--ink": ok(0.95, 0.01, 240),
    "--tint": ok(0.65, 0.12, 232),
    "--highlight": ok(0.9, 0.14, 85),
    "--highlight-fill": ok(0.9, 0.14, 85),
    "--magic": ok(0.78, 0.12, 300),
    "--danger": ok(0.74, 0.18, 30),
    "--success": ok(0.82, 0.14, 168),
    "--warning": ok(0.86, 0.13, 65),
    "--on-accent": ok(0.15, 0.03, 245),
    "--on-highlight": ok(0.15, 0.03, 245),
    "--amb-800": ok(0.39, 0.07, 235),
    "--shadow-alpha": "0.65",
  };
}

/**
 * Direction 2's chrome: the primary ramp painted as the neutral one (selected rows, pills, the rail,
 * tracks, the weapon in hand all become grey steps); `tint` keeps its hue for the ring, the mark and
 * the four primary actions (the option's stylesheet routes the rest to neutrals).
 */
function neutralChrome(v: Vars): Vars {
  const out = { ...v };
  for (const step of STEPS) out[`--amb-${step}`] = v[`--neu-${step}`];
  return out;
}

const NAMES: Record<AmbianceId, string> = {
  arcane: "Arcane",
  sylve: "Grove",
  sang: "Blood",
  ocean: "Ocean",
  ambre: "Amber",
  obsidienne: "Obsidian",
};

/** Hue distance in degrees. */
const dh = (a: number, b: number) => {
  const d = Math.abs(a - b) % 360;
  return d > 180 ? 360 - d : d;
};

/** The six ambiances under a token function, with the verdict of the option for each. */
function sixAmbiances(
  make: (id: AmbianceId, light: boolean) => Vars,
  verdict: (id: AmbianceId, light: boolean) => [Theme["status"], string],
): Theme[] {
  const out: Theme[] = [];
  for (const light of [false, true])
    for (const a of AMBIANCES) {
      const [status, why] = verdict(a.id, light);
      out.push({
        id: `${a.id}-${light ? "light" : "dark"}`,
        name: NAMES[a.id],
        mode: light ? "light" : "dark",
        base: a.id,
        status,
        why,
        vars: make(a.id, light),
        mix: light ? LADDER_MIX.light : LADDER_MIX.dark,
      });
    }
  return out;
}

/** Where the primary hue lands on a semantic one, a token change cannot fix it (review, cause 5). */
function hueClash(id: AmbianceId): string | null {
  const a = ambiance(id);
  if (a.primary.c < 0.02) return null;
  for (const [name, hue] of [
    ["danger", 25],
    ["success", 150],
    ["warning", 55],
  ] as const)
    if (dh(a.primary.h, hue) < 25)
      return `primary hue ${a.primary.h} is ${dh(a.primary.h, hue)}° from ${name}`;
  return null;
}

export interface Option {
  id: string;
  /** Short name used in captions. */
  name: string;
  /** The component stylesheets (docs/design/ui-review/mockups/css/<name>.css), in order. */
  css: string[];
  /** Its theme set: every theme it keeps, retunes, adds or drops, with its tokens. */
  themes: () => Theme[];
  /** The two themes the screens are captured in. */
  capture: { dark: string; light: string };
  /** Extra captures (one screen in another theme of the set). */
  extra?: { theme: string; screen: string; viewport: string }[];
}

const ladderVerdict = (
  id: AmbianceId,
  light: boolean,
): [Theme["status"], string] => {
  const clash = hueClash(id);
  if (clash)
    return [
      "drop",
      `${clash}: the ladder separates it by lightness, the hue still says "danger/success/warning" on every button and bar`,
    ];
  if (id === "ambre")
    return light
      ? [
          "keep",
          "the parchment-like light mode; its violet secondary changes the crit/max code",
        ]
      : [
          "drop",
          "gold primary 17° from warning and on the Arcane highlight gold",
        ];
  return ["keep", "hues ≥ 40° from every semantic hue"];
};

export const OPTIONS: Option[] = [
  {
    id: "today",
    name: "Today",
    css: [],
    themes: () =>
      AMBIANCES.flatMap((a) =>
        [false, true].map((light): Theme => ({
          id: `${a.id}-${light ? "light" : "dark"}`,
          name: NAMES[a.id],
          mode: light ? "light" : "dark",
          base: a.id,
          status: "keep",
          why: "today",
          vars: themeVars(a.id, light),
          mix: TODAY_MIX,
        })),
      ),
    capture: { dark: "arcane-dark", light: "arcane-light" },
  },
  {
    id: "o1-ladder",
    name: "1 · Contrast ladder",
    css: ["ladder"],
    themes: () => sixAmbiances(ladder, ladderVerdict),
    capture: { dark: "arcane-dark", light: "arcane-light" },
  },
  {
    id: "o12-neutral-chrome",
    name: "1 + 2 · Neutral chrome",
    css: ["ladder", "neutral-chrome"],
    themes: () =>
      sixAmbiances(
        (id, light) => neutralChrome(ladder(id, light)),
        (id) =>
          hueClash(id)
            ? [
                "keep",
                "its hue only paints the mark, the ring and four buttons, so it no longer reads as a status",
              ]
            : [
                "keep",
                "colour is reserved for meaning; the ambiance tints the greys and the primary actions",
              ],
      ),
    capture: { dark: "arcane-dark", light: "arcane-light" },
    extra: [
      { theme: "sang-dark", screen: "act-weapons", viewport: "1440x900" },
    ],
  },
  {
    id: "o13-raised-stage",
    name: "1 + 3 · Raised stage",
    css: ["ladder", "raised-stage"],
    themes: () => sixAmbiances(ladder, ladderVerdict),
    capture: { dark: "arcane-dark", light: "arcane-light" },
  },
  {
    id: "o4-bespoke",
    name: "4 · Bespoke themes (Parchment, Deep)",
    css: ["ladder", "bespoke"],
    themes: () => {
      const keep = (
        id: AmbianceId,
        light: boolean,
        name: string,
        why: string,
        status: Theme["status"] = "retune",
      ): Theme => ({
        id: `${id}-${light ? "light" : "dark"}`,
        name,
        mode: light ? "light" : "dark",
        base: id,
        status,
        why,
        vars: ladder(id, light),
        mix: light ? LADDER_MIX.light : LADDER_MIX.dark,
      });
      return [
        keep("arcane", false, "Arcane", "the brand, hand-tuned on the ladder"),
        {
          id: "deep-dark",
          name: "Deep",
          mode: "dark",
          base: "ocean",
          status: "new",
          why: "from Ocean dark: ink blue, coral danger, jade success, each accent on its own lightness",
          vars: deep(),
          mix: LADDER_MIX.dark,
        },
        keep(
          "obsidienne",
          false,
          "Plain",
          "Obsidian renamed: no colour but the semantic ones",
        ),
        keep(
          "arcane",
          true,
          "Arcane",
          "the most legible light screen today, kept",
        ),
        {
          id: "parchment-light",
          name: "Parchment",
          mode: "light",
          base: "ambre",
          status: "new",
          why: "from Amber light: warm paper, brown ink, lapis primary, oxblood / moss / ochre / gilt",
          vars: parchment(),
          mix: { muted: 76, border: 34, input: 50 },
        },
        keep("obsidienne", true, "Plain", "Obsidian light renamed"),
        ...(["sylve", "sang", "ocean", "ambre"] as const).flatMap((id) =>
          [false, true].map((light): Theme => ({
            ...keep(id, light, NAMES[id], ""),
            status: "drop",
            why:
              id === "ocean"
                ? light
                  ? "Ocean light: pale blue, no identity left once Deep exists"
                  : "becomes Deep"
                : id === "ambre"
                  ? light
                    ? "becomes Parchment"
                    : "gold primary on the warning hue"
                  : `${hueClash(id)}`,
          })),
        ),
      ];
    },
    capture: { dark: "deep-dark", light: "parchment-light" },
    extra: [
      { theme: "arcane-dark", screen: "act-weapons", viewport: "1440x900" },
    ],
  },
];

/** The stylesheet that paints a theme over the app: its tokens, as overrides of the inline ones. */
export function themeCss(theme: Theme): string {
  const lines = Object.entries(theme.vars).map(
    ([k, v]) => `  ${k}: ${v} !important;`,
  );
  lines.push(
    `  --ink-muted: color-mix(in oklab, var(--ink) ${theme.mix.muted}%, transparent) !important;`,
    `  --border: color-mix(in oklab, var(--ink) ${theme.mix.border}%, transparent) !important;`,
    `  --input: color-mix(in oklab, var(--ink) ${theme.mix.input}%, transparent) !important;`,
  );
  return `:root:root {\n${lines.join("\n")}\n}\n`;
}

// --- Measurements --------------------------------------------------------------------------------

export interface Pair {
  label: string;
  /**
   * "text" (AA 4.5:1), "edge" (3:1, WCAG 1.4.11), "step" (a tone step, judged by its OKLab
   * lightness difference: WCAG ratios flatten dark tones, L .12 vs .235 is a clear step at 1.2:1),
   * "apart" (one accent off the primary, the review's 1.8:1 floor).
   */
  kind: "text" | "edge" | "step" | "apart";
  min: number;
  fg: Rgb;
  bg: Rgb;
  /** For a tone step: the OKLab lightness difference, which is what the eye reads in dark tones. */
  dL?: number;
}

export function pairs(t: Theme): Pair[] {
  const T = (k: string) => linear(t.vars[`--${k}`]);
  const L = (k: string) => parseOklch(t.vars[`--${k}`])[0];
  const surface = T("surface");
  const ground = T("ground");
  const raised = T("neu-900");
  const ink = T("ink");
  const muted = (on: Rgb) => over(ink, t.mix.muted / 100, on);
  const edge = (on: Rgb, a: number) => over(ink, a / 100, on);
  const p = (
    label: string,
    kind: Pair["kind"],
    min: number,
    fg: Rgb,
    bg: Rgb,
  ): Pair => ({ label, kind, min, fg, bg });
  return [
    p("ink on surface", "text", 4.5, ink, surface),
    p("muted on surface", "text", 4.5, muted(surface), surface),
    p("muted on raised", "text", 4.5, muted(raised), raised),
    p("tint on surface", "text", 4.5, T("tint"), surface),
    p("highlight on surface", "text", 4.5, T("highlight"), surface),
    p("magic on surface", "text", 4.5, T("magic"), surface),
    p("danger on surface", "text", 4.5, T("danger"), surface),
    p("success on surface", "text", 4.5, T("success"), surface),
    p("warning on surface", "text", 4.5, T("warning"), surface),
    p("danger on raised", "text", 4.5, T("danger"), raised),
    p("on-accent on tint", "text", 4.5, T("on-accent"), T("tint")),
    p(
      "on-highlight on highlight-fill",
      "text",
      4.5,
      T("on-highlight"),
      T("highlight-fill"),
    ),
    {
      ...p("card on page", "step", 0.1, surface, ground),
      dL: L("surface") - L("ground"),
    },
    {
      ...p("raised on card", "step", 0.07, raised, surface),
      dL: L("neu-900") - L("surface"),
    },
    p("card outline on page", "edge", 0, edge(ground, t.mix.border), ground),
    p("field edge on card", "edge", 3, edge(surface, t.mix.input), surface),
    p("tint (ring, latest) on card", "edge", 3, T("tint"), surface),
    p("danger off tint", "apart", 1.8, T("danger"), T("tint")),
    p("success off tint", "apart", 1.8, T("success"), T("tint")),
    p("highlight off tint", "apart", 1.8, T("highlight"), T("tint")),
    p("magic off tint", "apart", 1.8, T("magic"), T("tint")),
    p("danger off success", "apart", 0, T("danger"), T("success")),
  ];
}

/** The ratio a pair is judged by (ΔL for a tone step, absolute: light mode steps go down). */
export const measured = (pr: Pair) =>
  pr.dL !== undefined ? Math.abs(pr.dL) : ratio(pr.fg, pr.bg);
export const wcagRatio = (pr: Pair) => ratio(pr.fg, pr.bg);
export { hex };
