import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test as base } from "@playwright/test";
import { THEMES, palette, themeVars, type Mix } from "../src/lib/theme";
import { linear, over, ratio } from "../src/lib/wcag";

/**
 * The theme audit's numbers (docs/design/ui-review/README.md): the WCAG 2 contrast ratio of every
 * token pair that carries text, a border, a focus ring or a status colour, for every theme in each
 * mode it exists in, written to docs/design/ui-review/contrast.md. Computed from `themeVars` (the
 * same values `lib/theme.test.ts` checks against ADR 0026's rules), so the table is reproducible
 * without a browser. Semi-transparent tokens (`--ink-muted`, `--border`, `--input`: the ink at the
 * theme's mix) are composited over their backdrop in gamma sRGB, as a browser paints them.
 */

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../..");
const outFile = path.join(root, "docs", "design", "ui-review", "contrast.md");

/** A theme in one mode: its variables and its translucent mixes. */
interface Col {
  v: Record<string, string>;
  mix: Mix;
}

interface Row {
  pair: string;
  /** The threshold the pair must reach: 4.5 (text), 3 (non-text), a ΔL floor, 0 (informational). */
  min: number;
  value: (c: Col) => number;
}

const T = ({ v }: Col, k: string) => linear(v[`--${k}`]);
const dL = ({ v }: Col, a: string, b: string) =>
  Math.abs(
    Number(/^oklch\(([\d.]+)/.exec(v[`--${a}`])![1]) -
      Number(/^oklch\(([\d.]+)/.exec(v[`--${b}`])![1]),
  );
const muted = (c: Col, on: string) =>
  over(T(c, "ink"), c.mix.muted / 100, T(c, on));
const border = (c: Col, on: string, which: "border" | "input" = "border") =>
  over(T(c, "ink"), c.mix[which] / 100, T(c, on));

const ROWS: Row[] = [
  {
    pair: "ink on ground",
    min: 4.5,
    value: (c) => ratio(T(c, "ink"), T(c, "ground")),
  },
  {
    pair: "ink on surface",
    min: 4.5,
    value: (c) => ratio(T(c, "ink"), T(c, "surface")),
  },
  {
    pair: "ink on neu-900",
    min: 4.5,
    value: (c) => ratio(T(c, "ink"), T(c, "neu-900")),
  },
  {
    pair: "ink-muted on surface",
    min: 4.5,
    value: (c) => ratio(muted(c, "surface"), T(c, "surface")),
  },
  {
    pair: "ink-muted on neu-900",
    min: 4.5,
    value: (c) => ratio(muted(c, "neu-900"), T(c, "neu-900")),
  },
  {
    pair: "ink-muted on ground",
    min: 4.5,
    value: (c) => ratio(muted(c, "ground"), T(c, "ground")),
  },
  {
    pair: "surface vs ground (card on page), ΔL",
    min: 0.1,
    value: (c) => dL(c, "surface", "ground"),
  },
  {
    pair: "neu-900 vs surface (raised step), ΔL",
    min: 0.07,
    value: (c) => dL(c, "neu-900", "surface"),
  },
  {
    pair: "border (card outline) vs surface",
    min: 0,
    value: (c) => ratio(border(c, "surface"), T(c, "surface")),
  },
  {
    pair: "border (card outline) vs ground",
    min: 0,
    value: (c) => ratio(border(c, "ground"), T(c, "ground")),
  },
  {
    pair: "input (field edge) vs surface",
    min: 3,
    value: (c) => ratio(border(c, "surface", "input"), T(c, "surface")),
  },
  {
    pair: "amb-700 (result outline) vs surface",
    min: 0,
    value: (c) => ratio(T(c, "amb-700"), T(c, "surface")),
  },
  {
    pair: "amb-900 (selected row, rail pill) vs surface",
    min: 0,
    value: (c) => ratio(T(c, "amb-900"), T(c, "surface")),
  },
  {
    pair: "tint (ring / focus) vs surface",
    min: 3,
    value: (c) => ratio(T(c, "tint"), T(c, "surface")),
  },
  {
    pair: "tint as text on surface",
    min: 4.5,
    value: (c) => ratio(T(c, "tint"), T(c, "surface")),
  },
  {
    pair: "tint as text on neu-900",
    min: 4.5,
    value: (c) => ratio(T(c, "tint"), T(c, "neu-900")),
  },
  {
    pair: "tint as text on amb-900 (rail pill)",
    min: 0,
    value: (c) => ratio(T(c, "tint"), T(c, "amb-900")),
  },
  {
    pair: "highlight as text on surface",
    min: 4.5,
    value: (c) => ratio(T(c, "highlight"), T(c, "surface")),
  },
  {
    pair: "magic as text on surface",
    min: 4.5,
    value: (c) => ratio(T(c, "magic"), T(c, "surface")),
  },
  {
    pair: "danger as text on surface",
    min: 4.5,
    value: (c) => ratio(T(c, "danger"), T(c, "surface")),
  },
  {
    pair: "success as text on surface",
    min: 4.5,
    value: (c) => ratio(T(c, "success"), T(c, "surface")),
  },
  {
    pair: "warning as text on surface",
    min: 4.5,
    value: (c) => ratio(T(c, "warning"), T(c, "surface")),
  },
  {
    pair: "on-accent on tint (primary button)",
    min: 4.5,
    value: (c) => ratio(T(c, "on-accent"), T(c, "tint")),
  },
  {
    pair: "on-accent on danger",
    min: 4.5,
    value: (c) => ratio(T(c, "on-accent"), T(c, "danger")),
  },
  {
    pair: "on-accent on success",
    min: 4.5,
    value: (c) => ratio(T(c, "on-accent"), T(c, "success")),
  },
  {
    pair: "on-highlight on highlight-fill",
    min: 4.5,
    value: (c) => ratio(T(c, "on-highlight"), T(c, "highlight-fill")),
  },
  {
    pair: "danger vs tint (red off the primary)",
    min: 0,
    value: (c) => ratio(T(c, "danger"), T(c, "tint")),
  },
  {
    pair: "success vs tint (green off the primary)",
    min: 0,
    value: (c) => ratio(T(c, "success"), T(c, "tint")),
  },
  {
    pair: "highlight vs tint (gold off the primary)",
    min: 0,
    value: (c) => ratio(T(c, "highlight"), T(c, "tint")),
  },
  {
    pair: "magic vs tint (magic off the primary)",
    min: 0,
    value: (c) => ratio(T(c, "magic"), T(c, "tint")),
  },
];

const NAMES: Record<string, string> = {
  arcane: "Arcane",
  deep: "Deep (Abysses)",
  parchment: "Parchment (Parchemin)",
  plain: "Plain (Sobre)",
};

base("contrast report: every theme token pair, both modes", async () => {
  const lines: string[] = [
    "# Theme contrast report",
    "",
    "Generated by `just ui-shots` (`web/ui-review/contrast.spec.ts`) from `web/src/lib/theme.ts`",
    "`themeVars`, every theme in each mode it exists in (ADR 0026). WCAG 2 contrast ratios",
    "(relative luminance, sRGB). Thresholds: 4.5:1 for text (AA), 3:1 for the field edge, the focus",
    "ring and the accent fills (WCAG 1.4.11); tone steps by their OKLab lightness difference (ΔL ≥ .10",
    "card on page, ≥ .07 raised on card, ADR 0026 step 2.1); none for the informational rows (the card",
    "outline, tinted rows, one accent off another). `✗` marks a pair under its threshold.",
    "Semi-transparent tokens are composited over their backdrop in gamma sRGB.",
    "",
  ];
  // A ratio is ≥ 1; a ΔL is under 1 and shown to three decimals.
  const fmt = (r: number, min: number) =>
    `${r < 1 ? r.toFixed(3) : r.toFixed(2)}${min && r < min - 1e-9 ? " ✗" : ""}`;
  for (const mode of ["dark", "light"] as const) {
    lines.push(`## ${mode === "dark" ? "Dark" : "Light"} mode`, "");
    const light = mode === "light";
    const themes = THEMES.filter((t) => palette(t.id, light));
    const cols: Col[] = themes.map((t) => ({
      v: themeVars(t.id, light),
      mix: palette(t.id, light)!.mix,
    }));
    lines.push(
      `| Pair | min | ${themes.map((t) => NAMES[t.id]).join(" | ")} |`,
      `|---|---|${themes.map(() => "---").join("|")}|`,
    );
    for (const row of ROWS)
      lines.push(
        `| ${row.pair} | ${row.min || "–"} | ${cols
          .map((c) => fmt(row.value(c), row.min))
          .join(" | ")} |`,
      );
    lines.push("");
    // The raw role values.
    lines.push(`### ${mode} role values`, "");
    const roles = [
      "ground",
      "surface",
      "neu-900",
      "neu-800",
      "ink",
      "tint",
      "highlight",
      "highlight-fill",
      "magic",
      "danger",
      "success",
      "warning",
      "amb-700",
      "amb-800",
      "amb-900",
    ];
    lines.push(
      `| Token | ${themes.map((t) => NAMES[t.id]).join(" | ")} |`,
      `|---|${themes.map(() => "---").join("|")}|`,
    );
    for (const r of roles)
      lines.push(
        `| ${r} | ${cols.map(({ v }) => `\`${v[`--${r}`]}\``).join(" | ")} |`,
      );
    lines.push("");
  }
  await mkdir(path.dirname(outFile), { recursive: true });
  await writeFile(outFile, lines.join("\n"));
});
