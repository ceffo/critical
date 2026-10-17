import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base } from "@playwright/test";
import { AMBIANCES, oklchToLinearRgb, themeVars } from "../src/lib/theme";

/**
 * The theme audit's numbers (docs/design/ui-review/README.md): the WCAG 2 contrast ratio of every
 * token pair that carries text, a border, a focus ring or a status colour, for every ambiance in
 * both modes, written to docs/design/ui-review/contrast.md. Computed from `themeVars` (the same
 * values `lib/theme.test.ts` checks for AA), so the table is reproducible without a browser.
 * Semi-transparent tokens (`--border` = ink at 18 %, `--input` at 22 %, `--ink-muted` at 70 %) are
 * composited over their backdrop in gamma sRGB, as a browser paints them.
 */

type Rgb = [number, number, number];
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../..");
const outFile = path.join(root, "docs", "design", "ui-review", "contrast.md");

function parse(css: string): [number, number, number] {
  const m = /^oklch\(([\d.]+) ([\d.]+) ([\d.]+)\)$/.exec(css);
  if (!m) throw new Error(`not an oklch literal: ${css}`);
  return [Number(m[1]), Number(m[2]), Number(m[3])];
}
const clamp01 = (x: number) => Math.min(1, Math.max(0, x));
const linear = (css: string): Rgb =>
  oklchToLinearRgb(...parse(css)).map(clamp01) as Rgb;
const toGamma = (x: number) =>
  x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055;
const toLinear = (x: number) =>
  x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
/** `fg` at `alpha` over `bg`, composited in gamma sRGB. */
function over(fg: Rgb, alpha: number, bg: Rgb): Rgb {
  return fg.map((c, i) => {
    const g = toGamma(c) * alpha + toGamma(bg[i]) * (1 - alpha);
    return toLinear(g);
  }) as Rgb;
}
const luminance = ([r, g, b]: Rgb) => 0.2126 * r + 0.7152 * g + 0.0722 * b;
const ratio = (a: Rgb, b: Rgb) => {
  const [la, lb] = [luminance(a), luminance(b)];
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
};

interface Row {
  pair: string;
  /** The WCAG threshold the pair must reach: 4.5 (text), 3 (non-text) or 0 (informational). */
  min: number;
  value: (v: Record<string, string>) => number;
}

const T = (v: Record<string, string>, k: string) => linear(v[`--${k}`]);
const muted = (v: Record<string, string>, on: string) =>
  over(T(v, "ink"), 0.7, T(v, on));
const border = (v: Record<string, string>, on: string, alpha = 0.18) =>
  over(T(v, "ink"), alpha, T(v, on));

const ROWS: Row[] = [
  {
    pair: "ink on ground",
    min: 4.5,
    value: (v) => ratio(T(v, "ink"), T(v, "ground")),
  },
  {
    pair: "ink on surface",
    min: 4.5,
    value: (v) => ratio(T(v, "ink"), T(v, "surface")),
  },
  {
    pair: "ink on neu-900",
    min: 4.5,
    value: (v) => ratio(T(v, "ink"), T(v, "neu-900")),
  },
  {
    pair: "ink-muted on surface",
    min: 4.5,
    value: (v) => ratio(muted(v, "surface"), T(v, "surface")),
  },
  {
    pair: "ink-muted on neu-900",
    min: 4.5,
    value: (v) => ratio(muted(v, "neu-900"), T(v, "neu-900")),
  },
  {
    pair: "ink-muted on ground",
    min: 4.5,
    value: (v) => ratio(muted(v, "ground"), T(v, "ground")),
  },
  {
    pair: "surface vs ground (card on page)",
    min: 0,
    value: (v) => ratio(T(v, "surface"), T(v, "ground")),
  },
  {
    pair: "neu-900 vs surface (raised step)",
    min: 0,
    value: (v) => ratio(T(v, "neu-900"), T(v, "surface")),
  },
  {
    pair: "border (ink 18%) vs surface",
    min: 3,
    value: (v) => ratio(border(v, "surface"), T(v, "surface")),
  },
  {
    pair: "border (ink 18%) vs ground (card outline)",
    min: 3,
    value: (v) => ratio(border(v, "ground"), T(v, "ground")),
  },
  {
    pair: "input border (ink 22%) vs surface",
    min: 3,
    value: (v) => ratio(border(v, "surface", 0.22), T(v, "surface")),
  },
  {
    pair: "amb-700 (result outline) vs surface",
    min: 3,
    value: (v) => ratio(T(v, "amb-700"), T(v, "surface")),
  },
  {
    pair: "amb-900 (selected row, rail pill) vs surface",
    min: 0,
    value: (v) => ratio(T(v, "amb-900"), T(v, "surface")),
  },
  {
    pair: "tint (ring / focus) vs surface",
    min: 3,
    value: (v) => ratio(T(v, "tint"), T(v, "surface")),
  },
  {
    pair: "tint as text on surface",
    min: 4.5,
    value: (v) => ratio(T(v, "tint"), T(v, "surface")),
  },
  {
    pair: "tint as text on neu-900",
    min: 4.5,
    value: (v) => ratio(T(v, "tint"), T(v, "neu-900")),
  },
  {
    pair: "tint as text on amb-900 (rail pill)",
    min: 4.5,
    value: (v) => ratio(T(v, "tint"), T(v, "amb-900")),
  },
  {
    pair: "highlight as text on surface",
    min: 4.5,
    value: (v) => ratio(T(v, "highlight"), T(v, "surface")),
  },
  {
    pair: "magic as text on surface",
    min: 4.5,
    value: (v) => ratio(T(v, "magic"), T(v, "surface")),
  },
  {
    pair: "danger as text on surface",
    min: 4.5,
    value: (v) => ratio(T(v, "danger"), T(v, "surface")),
  },
  {
    pair: "success as text on surface",
    min: 4.5,
    value: (v) => ratio(T(v, "success"), T(v, "surface")),
  },
  {
    pair: "warning as text on surface",
    min: 4.5,
    value: (v) => ratio(T(v, "warning"), T(v, "surface")),
  },
  {
    pair: "on-accent on tint (primary button)",
    min: 4.5,
    value: (v) => ratio(T(v, "on-accent"), T(v, "tint")),
  },
  {
    pair: "on-accent on danger",
    min: 4.5,
    value: (v) => ratio(T(v, "on-accent"), T(v, "danger")),
  },
  {
    pair: "on-accent on success",
    min: 4.5,
    value: (v) => ratio(T(v, "on-accent"), T(v, "success")),
  },
  {
    pair: "on-highlight on highlight-fill",
    min: 4.5,
    value: (v) => ratio(T(v, "on-highlight"), T(v, "highlight-fill")),
  },
  {
    pair: "danger vs tint (red off the primary)",
    min: 3,
    value: (v) => ratio(T(v, "danger"), T(v, "tint")),
  },
  {
    pair: "success vs tint (green off the primary)",
    min: 3,
    value: (v) => ratio(T(v, "success"), T(v, "tint")),
  },
  {
    pair: "highlight vs tint (gold off the primary)",
    min: 3,
    value: (v) => ratio(T(v, "highlight"), T(v, "tint")),
  },
  {
    pair: "magic vs tint (magic off the primary)",
    min: 3,
    value: (v) => ratio(T(v, "magic"), T(v, "tint")),
  },
];

const AMB_NAMES: Record<string, string> = {
  arcane: "Arcane",
  sylve: "Grove (Sylve)",
  sang: "Blood (Sang)",
  ocean: "Ocean",
  ambre: "Amber (Ambre)",
  obsidienne: "Obsidian (Obsidienne)",
};

base("contrast report: every theme token pair, both modes", async () => {
  const lines: string[] = [
    "# Theme contrast report",
    "",
    "Generated by `just ui-shots` (`web/ui-review/contrast.spec.ts`) from `web/src/lib/theme.ts`",
    "`themeVars`. WCAG 2 contrast ratios (relative luminance, sRGB). Thresholds: 4.5:1 for text",
    "(AA), 3:1 for non-text (borders, focus rings, fills, one accent off another; WCAG 1.4.11),",
    "none for the informational rows (surface steps, tinted rows). `✗` marks a pair under its",
    "threshold. Semi-transparent tokens are composited over their backdrop in gamma sRGB.",
    "",
  ];
  const fmt = (r: number, min: number) =>
    `${r.toFixed(2)}${min && r < min ? " ✗" : ""}`;
  for (const mode of ["dark", "light"] as const) {
    lines.push(`## ${mode === "dark" ? "Dark" : "Light"} mode`, "");
    lines.push(
      `| Pair | min | ${AMBIANCES.map((a) => AMB_NAMES[a.id]).join(" | ")} |`,
      `|---|---|${AMBIANCES.map(() => "---").join("|")}|`,
    );
    const vars = AMBIANCES.map((a) => themeVars(a.id, mode === "light"));
    for (const row of ROWS)
      lines.push(
        `| ${row.pair} | ${row.min || "–"} | ${vars
          .map((v) => fmt(row.value(v), row.min))
          .join(" | ")} |`,
      );
    lines.push("");
    // The raw role values, for the mock-up builder.
    lines.push(`### ${mode} role values`, "");
    const roles = [
      "ground",
      "surface",
      "neu-900",
      "ink",
      "tint",
      "highlight",
      "highlight-fill",
      "magic",
      "danger",
      "success",
      "warning",
      "amb-700",
      "amb-900",
    ];
    lines.push(
      `| Token | ${AMBIANCES.map((a) => AMB_NAMES[a.id]).join(" | ")} |`,
      `|---|${AMBIANCES.map(() => "---").join("|")}|`,
    );
    for (const r of roles)
      lines.push(
        `| ${r} | ${vars.map((v) => `\`${v[`--${r}`]}\``).join(" | ")} |`,
      );
    lines.push("");
  }
  await mkdir(path.dirname(outFile), { recursive: true });
  await writeFile(outFile, lines.join("\n"));
});
