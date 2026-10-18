import type { Mix, Theme } from "../src/lib/theme";

/**
 * The new-themes design round (critical-4jaw, docs/design/new-themes/README.md): the candidate role
 * tables, written as the app's `Theme` (web/src/lib/theme.ts) so the implementer copies them into
 * `THEMES`. They are painted over the real app by themes-round.spec.ts and held to the rules of
 * ADR 0026 by the app's own theme.test.ts (vitest.candidates.ts adds them to `THEMES` first).
 */

const NEUTRAL_L = [0.96, 0.91, 0.85, 0.76, 0.66, 0.56, 0.46, 0.36, 0.27];
const STAGE_DARK = { band: 9, subBand: 5.5, elevated: 5, stageOutline: 40 };
const STAGE_LIGHT = { band: 7, subBand: 4, elevated: 0, stageOutline: 40 };
const MIX_DARK: Mix = { muted: 76, border: 32, input: 42, ...STAGE_DARK };
const MIX_LIGHT: Mix = { muted: 74, border: 34, input: 48, ...STAGE_LIGHT };

export const CANDIDATES: Theme[] = [
  {
    // Green: forest-night surfaces, an emerald primary (160). Success moves to lime (128), 32° off.
    id: "forest",
    dark: {
      ramps: {
        amb: { h: 160, c: 0.13 },
        sec: { h: 88, c: 0.15 },
        ter: { h: 300, c: 0.12 },
      },
      neutral: { h: 160, c: 0.02, l: NEUTRAL_L },
      ground: [0.1, 0.03, 165],
      surface: [0.25, 0.04, 160],
      raised: [0.335, 0.028, 160],
      hover: [0.405, 0.028, 160],
      selected: [0.39, 0.075, 160],
      ink: [0.95, 0.012, 150],
      tint: [0.71, 0.13, 160],
      highlight: [0.9, 0.15, 88],
      highlightFill: [0.9, 0.15, 88],
      magic: [0.78, 0.12, 300],
      danger: [0.74, 0.18, 28],
      success: [0.82, 0.17, 128],
      warning: [0.86, 0.13, 70],
      onAccent: [0.15, 0.03, 160],
      onHighlight: [0.15, 0.03, 160],
      mix: MIX_DARK,
      shadowAlpha: "0.65",
    },
    light: {
      ramps: {
        amb: { h: 160, c: 0.13 },
        sec: { h: 88, c: 0.15 },
        ter: { h: 300, c: 0.12 },
      },
      neutral: { h: 160, c: 0.02, l: [...NEUTRAL_L].reverse() },
      ground: [0.875, 0.04, 155],
      surface: [0.99, 0.01, 140],
      raised: [0.91, 0.03, 155],
      hover: [0.88, 0.03, 155],
      selected: [0.875, 0.065, 160],
      ink: [0.2, 0.035, 160],
      tint: [0.42, 0.11, 160],
      highlight: [0.5, 0.12, 88],
      highlightFill: [0.82, 0.15, 88],
      magic: [0.46, 0.13, 300],
      danger: [0.5, 0.19, 27],
      success: [0.47, 0.13, 128],
      warning: [0.5, 0.14, 62],
      onAccent: [0.985, 0.006, 150],
      onHighlight: [0.15, 0.03, 160],
      mix: MIX_LIGHT,
      shadowAlpha: "0.18",
    },
  },
  {
    // Red: wine-dark surfaces, a crimson-rose primary (355). Danger moves to vermilion (40), 45° off.
    id: "ember",
    dark: {
      ramps: {
        amb: { h: 355, c: 0.17 },
        sec: { h: 90, c: 0.15 },
        ter: { h: 200, c: 0.12 },
      },
      neutral: { h: 15, c: 0.02, l: NEUTRAL_L },
      ground: [0.1, 0.035, 15],
      surface: [0.25, 0.05, 12],
      raised: [0.335, 0.035, 12],
      hover: [0.405, 0.035, 12],
      selected: [0.39, 0.1, 355],
      ink: [0.95, 0.012, 30],
      tint: [0.725, 0.16, 355],
      highlight: [0.9, 0.15, 90],
      highlightFill: [0.9, 0.15, 90],
      magic: [0.78, 0.11, 200],
      danger: [0.74, 0.17, 40],
      success: [0.82, 0.16, 150],
      warning: [0.86, 0.14, 75],
      onAccent: [0.15, 0.03, 15],
      onHighlight: [0.15, 0.03, 15],
      mix: MIX_DARK,
      shadowAlpha: "0.65",
    },
  },
  {
    // True amber: near-black warm surfaces, amber ink and primary (70), like an amber terminal.
    // Warning moves to orange (42), 28° off; the highlight to lemon (105).
    id: "amber",
    dark: {
      ramps: {
        amb: { h: 70, c: 0.16 },
        sec: { h: 105, c: 0.15 },
        ter: { h: 210, c: 0.11 },
      },
      neutral: { h: 65, c: 0.02, l: NEUTRAL_L },
      ground: [0.1, 0.02, 60],
      surface: [0.25, 0.035, 65],
      raised: [0.335, 0.03, 65],
      hover: [0.405, 0.03, 65],
      selected: [0.39, 0.075, 70],
      ink: [0.95, 0.045, 85],
      tint: [0.71, 0.15, 70],
      highlight: [0.9, 0.16, 105],
      highlightFill: [0.9, 0.16, 105],
      magic: [0.78, 0.11, 210],
      danger: [0.74, 0.18, 22],
      success: [0.82, 0.15, 150],
      warning: [0.86, 0.11, 42],
      onAccent: [0.15, 0.03, 60],
      onHighlight: [0.15, 0.03, 60],
      mix: MIX_DARK,
      shadowAlpha: "0.65",
    },
  },
];

/** Every candidate in each mode it exists in, as captured. */
export const CANDIDATE_MODES = CANDIDATES.flatMap((t) =>
  (["dark", "light"] as const)
    .filter((m) => t[m])
    .map((mode) => ({ id: t.id, mode })),
);
