import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  AMBIANCES,
  STEPS,
  isLight,
  oklchToLinearRgb,
  themeVars,
} from "./theme";

describe("themeVars", () => {
  it("computes the three Sylve ramps in dark mode (lightness x chroma factor)", () => {
    const v = themeVars("sylve", false);
    // Primary C = 0.13, H = 152; factors [.2 .4 .65 .9 1 1 .9 .75 .55], chroma to 3 decimals,
    // clamped to the sRGB gamut on the dark steps.
    expect(STEPS.map((s) => v[`--amb-${s}`])).toEqual([
      "oklch(0.96 0.026 152)",
      "oklch(0.91 0.052 152)",
      "oklch(0.84 0.085 152)",
      "oklch(0.76 0.117 152)",
      "oklch(0.68 0.13 152)",
      "oklch(0.58 0.13 152)",
      "oklch(0.47 0.117 152)",
      "oklch(0.36 0.094 152)",
      "oklch(0.26 0.068 152)",
    ]);
    // Secondary (amber 70) and tertiary (violet 300) follow the same curve on their own hue;
    // a step outside the sRGB gamut is pulled back by chroma, never by lightness.
    expect(v["--sec-500"]).toBe("oklch(0.68 0.147 70)");
    expect(v["--ter-500"]).toBe("oklch(0.68 0.13 300)");
    // Neutral ramp: a faint tint of the primary hue.
    expect(v["--neu-100"]).toBe("oklch(0.96 0.021 152)");
    expect(v["--ground"]).toBe("oklch(0.15 0.039 152)");
    // Roles: the accents at their dark-mode lightness, chroma capped per role.
    expect(v["--tint"]).toBe("oklch(0.72 0.13 152)");
    expect(v["--highlight"]).toBe("oklch(0.82 0.143 70)");
    expect(v["--magic"]).toBe("oklch(0.78 0.12 300)");
    expect(v["--on-accent"]).toBe("oklch(0.15 0.03 152)");
  });

  it("reverses the ramps in light mode and darkens the text and accents", () => {
    const dark = themeVars("sylve", false);
    const light = themeVars("sylve", true);
    STEPS.forEach((s, i) => {
      const mirror = STEPS[STEPS.length - 1 - i];
      for (const p of ["amb", "sec", "ter", "neu"])
        expect(light[`--${p}-${s}`]).toBe(dark[`--${p}-${mirror}`]);
    });
    expect(light["--tint"]).toBe("oklch(0.46 0.121 152)");
    expect(light["--highlight"]).toBe("oklch(0.53 0.11 70)");
    expect(light["--ink"]).toBe("oklch(0.2 0.035 152)");
    expect(light["--on-accent"]).toBe("oklch(0.985 0.005 152)");
    // The highlight fill stays a bright gold with dark text in light mode.
    expect(light["--highlight-fill"]).toBe("oklch(0.8 0.15 70)");
    expect(light["--on-highlight"]).toBe("oklch(0.15 0.03 152)");
  });

  it("keeps the semantic hues in every ambiance, tuned per mode", () => {
    for (const a of AMBIANCES) {
      expect(themeVars(a.id, false)["--danger"]).toBe("oklch(0.72 0.15 25)");
      expect(themeVars(a.id, true)["--danger"]).toBe("oklch(0.52 0.19 25)");
      expect(themeVars(a.id, false)["--success"]).toBe("oklch(0.78 0.15 150)");
      expect(themeVars(a.id, true)["--success"]).toBe("oklch(0.5 0.13 150)");
    }
  });

  it("highlights a near-neutral ambiance in grey, not in colour", () => {
    expect(themeVars("obsidienne", false)["--tint"]).toBe(
      "oklch(0.86 0.02 265)",
    );
    expect(themeVars("obsidienne", true)["--tint"]).toBe("oklch(0.3 0.02 265)");
  });

  it("gives every ambiance the same set of variables in both modes", () => {
    const keys = Object.keys(themeVars("arcane", false)).sort();
    for (const a of AMBIANCES)
      for (const light of [false, true])
        expect(Object.keys(themeVars(a.id, light)).sort()).toEqual(keys);
  });
});

describe("isLight", () => {
  it("follows the OS only in system mode", () => {
    expect(isLight("light", true)).toBe(true);
    expect(isLight("dark", false)).toBe(false);
    expect(isLight("system", true)).toBe(false);
    expect(isLight("system", false)).toBe(true);
  });
});

// --- WCAG contrast (ADR 0020 step 4) -------------------------------------------------------

type Rgb = [number, number, number];

/** The linear sRGB of an oklch literal; `inGamut` is false when a channel had to be clipped. */
function toLinearRgb(css: string): { rgb: Rgb; inGamut: boolean } {
  const m = /^oklch\(([\d.]+) ([\d.]+) ([\d.]+)\)$/.exec(css);
  if (!m) throw new Error(`not an oklch literal: ${css}`);
  const raw = oklchToLinearRgb(Number(m[1]), Number(m[2]), Number(m[3]));
  const tolerance = 0.002; // rounding of the 3-decimal literals
  const inGamut = raw.every((x) => x >= -tolerance && x <= 1 + tolerance);
  return { rgb: raw.map((x) => Math.min(1, Math.max(0, x))) as Rgb, inGamut };
}

const luminance = ([r, g, b]: Rgb) => 0.2126 * r + 0.7152 * g + 0.0722 * b;

function contrast(fg: string, bg: string): number {
  const a = luminance(toLinearRgb(fg).rgb);
  const b = luminance(toLinearRgb(bg).rgb);
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

/** `color-mix(in oklab, ink 70%, transparent)` over a surface: a 70/30 mix in oklab. */
function mutedOn(ink: string, surface: string): string {
  const p = (css: string) => /^oklch\(([\d.]+) ([\d.]+) ([\d.]+)\)$/.exec(css)!;
  const [i, s] = [p(ink), p(surface)];
  // Chroma is small for both; mixing L alone is within 0.01 of the oklab mix.
  const L = 0.7 * Number(i[1]) + 0.3 * Number(s[1]);
  return `oklch(${L.toFixed(3)} ${i[2]} ${i[3]})`;
}

describe("contrast (ADR 0020 step 4)", () => {
  const AA_TEXT = 4.5;
  const AA_LARGE = 3;
  const ACCENTS = [
    "--tint",
    "--highlight",
    "--magic",
    "--danger",
    "--success",
    "--warning",
  ];

  for (const a of AMBIANCES)
    for (const light of [false, true]) {
      const name = `${a.id} ${light ? "light" : "dark"}`;
      const v = themeVars(a.id, light);

      it(`${name}: every token is inside the sRGB gamut`, () => {
        for (const [k, css] of Object.entries(v))
          if (css.startsWith("oklch("))
            expect(toLinearRgb(css).inGamut, k).toBe(true);
      });

      it(`${name}: text and muted text reach AA on ground, surface and the raised step`, () => {
        for (const bg of ["--ground", "--surface", "--neu-900"]) {
          expect(
            contrast(v["--ink"], v[bg]),
            `ink on ${bg}`,
          ).toBeGreaterThanOrEqual(AA_TEXT);
          expect(
            contrast(mutedOn(v["--ink"], v[bg]), v[bg]),
            `muted on ${bg}`,
          ).toBeGreaterThanOrEqual(AA_TEXT);
        }
      });

      it(`${name}: every accent reads as text on surface and the raised step`, () => {
        for (const role of ACCENTS)
          for (const bg of ["--surface", "--neu-900"])
            expect(
              contrast(v[role], v[bg]),
              `${role} on ${bg}`,
            ).toBeGreaterThanOrEqual(AA_TEXT);
      });

      it(`${name}: on-accent reads on every accent used as a fill`, () => {
        for (const role of ACCENTS)
          expect(
            contrast(v["--on-accent"], v[role]),
            `on-accent on ${role}`,
          ).toBeGreaterThanOrEqual(AA_TEXT);
        expect(
          contrast(v["--on-highlight"], v["--highlight-fill"]),
          "on-highlight on highlight-fill",
        ).toBeGreaterThanOrEqual(AA_TEXT);
      });

      it(`${name}: the accent fills stand off the surface (UI contrast)`, () => {
        for (const role of ACCENTS)
          expect(
            contrast(v[role], v["--surface"]),
            `${role} vs surface`,
          ).toBeGreaterThanOrEqual(AA_LARGE);
      });
    }
});

/** The `--name: value;` declarations of the first CSS block opened by `selector {`. */
function block(css: string, selector: string): Record<string, string> {
  const start = css.indexOf(`\n${selector} {`);
  const body = css.slice(start, css.indexOf("}", start));
  return Object.fromEntries(
    [...body.matchAll(/^\s*(--[\w-]+):\s*([^;]+);/gm)].map((m) => [m[1], m[2]]),
  );
}

describe("index.css defaults", () => {
  const css = readFileSync(join(__dirname, "..", "index.css"), "utf8");
  it("paint Arcane light and dark exactly as themeVars computes them", () => {
    expect(block(css, ":root")).toEqual(themeVars("arcane", true));
    expect(block(css, ".dark")).toEqual(themeVars("arcane", false));
  });
});
