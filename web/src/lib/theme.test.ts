import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { AMBIANCES, STEPS, isLight, themeVars } from "./theme";

describe("themeVars", () => {
  it("computes the Sylve accent ramp in dark mode (lightness x chroma factor)", () => {
    const v = themeVars("sylve", false);
    // C = 0.11, H = 152; factors [.15 .3 .55 .85 1 .95 .8 .6 .4], chroma to 3 decimals.
    expect(STEPS.map((s) => v[`--amb-${s}`])).toEqual([
      "oklch(0.97 0.017 152)",
      "oklch(0.93 0.033 152)",
      "oklch(0.87 0.061 152)",
      "oklch(0.78 0.093 152)",
      "oklch(0.68 0.11 152)",
      "oklch(0.58 0.104 152)",
      "oklch(0.48 0.088 152)",
      "oklch(0.38 0.066 152)",
      "oklch(0.27 0.044 152)",
    ]);
    expect(v["--tint"]).toBe("oklch(0.68 0.11 152)");
    // Neutral ramp: a faint tint of the hue.
    expect(v["--neu-100"]).toBe("oklch(0.97 0.018 152)");
    expect(v["--ground"]).toBe("oklch(0.19 0.042 152)");
    expect(v["--lA"]).toBe("0");
    expect(v["--lB"]).toBe("1");
  });

  it("reverses both ramps in light mode and darkens the text", () => {
    const dark = themeVars("sylve", false);
    const light = themeVars("sylve", true);
    STEPS.forEach((s, i) => {
      const mirror = STEPS[STEPS.length - 1 - i];
      expect(light[`--amb-${s}`]).toBe(dark[`--amb-${mirror}`]);
      expect(light[`--neu-${s}`]).toBe(dark[`--neu-${mirror}`]);
    });
    expect(light["--tint"]).toBe("oklch(0.52 0.11 152)");
    expect(light["--ink"]).toBe("oklch(0.24 0.03 152)");
    // State colours flip their lightness: L' = 1.3 - L.
    expect(light["--lA"]).toBe("1.3");
    expect(light["--lB"]).toBe("-1");
  });

  it("highlights a near-neutral ambiance in grey, not in colour", () => {
    expect(themeVars("obsidienne", false)["--tint"]).toBe("oklch(0.86 0.02 265)");
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
