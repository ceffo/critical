import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import {
  DARK_THEMES,
  DEFAULT_DARK_THEME,
  DEFAULT_LIGHT_THEME,
  LIGHT_THEMES,
  NEAR_NEUTRAL_CHROMA,
  STEPS,
  THEMES,
  effectiveTheme,
  isLight,
  oklchToLinearRgb,
  palette,
  themeVars,
} from "./theme";
import { linear, luminance, over, parseOklch, ratio, type Rgb } from "./wcag";
import { resources } from "@/i18n";

/** Every theme in every mode it exists in (ADR 0026 step 3.1). */
const CASES = THEMES.flatMap((t) =>
  [false, true]
    .filter((light) => palette(t.id, light))
    .map((light) => ({
      id: t.id,
      light,
      name: `${t.id} ${light ? "light" : "dark"}`,
      p: palette(t.id, light)!,
      v: themeVars(t.id, light),
    })),
);

describe("themes (ADR 0026 step 3)", () => {
  it("are Arcane, Deep, Parchment, Plain, Forest, Ember and Amber, each in its approved modes", () => {
    expect(THEMES.map((t) => t.id)).toEqual([
      "arcane",
      "deep",
      "parchment",
      "plain",
      "forest",
      "ember",
      "amber",
    ]);
    expect(DARK_THEMES).toEqual([
      "arcane",
      "deep",
      "plain",
      "forest",
      "ember",
      "amber",
    ]);
    expect(LIGHT_THEMES).toEqual(["arcane", "parchment", "plain", "forest"]);
    expect([DEFAULT_DARK_THEME, DEFAULT_LIGHT_THEME]).toEqual([
      "arcane",
      "arcane",
    ]);
  });

  it("give the same set of variables in every mode they exist in", () => {
    const keys = Object.keys(themeVars("arcane", false)).sort();
    for (const c of CASES)
      expect(Object.keys(c.v).sort(), c.name).toEqual(keys);
  });

  it("paint the role table: ramps reversed in light mode, the roles on their tokens", () => {
    const v = themeVars("arcane", false);
    // Primary C = 0.17, H = 285; factors [.2 .4 .65 .9 1 1 .9 .75 .55], chroma clamped to sRGB.
    expect(v["--amb-100"]).toBe("oklch(0.96 0.019 285)");
    expect(v["--amb-500"]).toBe("oklch(0.68 0.17 285)");
    // The selected tone, the raised and hover steps replace their ramp steps (step 1.1).
    expect(v["--amb-800"]).toBe("oklch(0.39 0.102 285)");
    expect(v["--neu-900"]).toBe("oklch(0.335 0.029 285)");
    expect(v["--neu-800"]).toBe("oklch(0.405 0.029 285)");
    expect(v["--ground"]).toBe("oklch(0.1 0.048 285)");
    expect(v["--surface"]).toBe("oklch(0.25 0.046 285)");
    // The translucent mixes of step 1.2.
    expect(v["--ink-muted"]).toBe(
      "color-mix(in oklab, var(--ink) 76%, transparent)",
    );
    expect(v["--border"]).toBe(
      "color-mix(in oklab, var(--ink) 32%, transparent)",
    );
    expect(v["--input"]).toBe(
      "color-mix(in oklab, var(--ink) 42%, transparent)",
    );
    const light = themeVars("arcane", true);
    STEPS.forEach((s, i) => {
      const mirror = STEPS[STEPS.length - 1 - i];
      for (const p of ["sec", "ter"])
        expect(light[`--${p}-${s}`]).toBe(v[`--${p}-${mirror}`]);
    });
    expect(light["--input"]).toBe(
      "color-mix(in oklab, var(--ink) 48%, transparent)",
    );
    expect(themeVars("parchment", true)["--input"]).toBe(
      "color-mix(in oklab, var(--ink) 50%, transparent)",
    );
  });

  it("fall back to the mode's default for a theme without that mode", () => {
    expect(themeVars("deep", true)).toEqual(themeVars("arcane", true));
    expect(themeVars("parchment", false)).toEqual(themeVars("arcane", false));
    expect(themeVars("sylve", false)).toEqual(themeVars("arcane", false));
  });

  const hueDistance = (a: number, b: number) => {
    const d = Math.abs(a - b) % 360;
    return d > 180 ? 360 - d : d;
  };

  for (const c of CASES) {
    it(`${c.name}: the primary stays 25° off its own danger, success and warning (step 3.5)`, () => {
      const primary = c.p.ramps.amb;
      if (primary.c < NEAR_NEUTRAL_CHROMA) return; // exempt
      for (const role of ["danger", "success", "warning"] as const)
        expect(
          hueDistance(primary.h, c.p[role][2]),
          `primary vs ${role}`,
        ).toBeGreaterThanOrEqual(25);
    });

    it(`${c.name}: a near-neutral primary highlights in grey, the highlight stays gold (step 3.6)`, () => {
      if (c.p.ramps.amb.c >= NEAR_NEUTRAL_CHROMA) return;
      const [l, ch] = parseOklch(c.v["--tint"]);
      expect(ch).toBeLessThanOrEqual(NEAR_NEUTRAL_CHROMA);
      expect(l).toBe(c.light ? 0.3 : 0.86);
      expect(c.p.highlight[2]).toBe(c.p.ramps.sec.h);
      expect(hueDistance(c.p.highlight[2], 88)).toBeLessThanOrEqual(10);
    });
  }

  it("have an English and a French name each", () => {
    for (const lang of ["en", "fr"] as const) {
      const names = (
        resources[lang].common as {
          settings: { themes: Record<string, string> };
        }
      ).settings.themes;
      expect(Object.keys(names).sort(), lang).toEqual(
        THEMES.map((t) => t.id).sort(),
      );
      for (const t of THEMES)
        expect(names[t.id], `${lang} ${t.id}`).toBeTruthy();
    }
  });
});

describe("effective theme (ADR 0026 step 4.2)", () => {
  it("is the dark or the light choice, by the mode and the OS preference", () => {
    expect(effectiveTheme("dark", false, "deep", "parchment")).toEqual({
      id: "deep",
      light: false,
    });
    expect(effectiveTheme("light", true, "deep", "parchment")).toEqual({
      id: "parchment",
      light: true,
    });
    expect(effectiveTheme("system", true, "deep", "parchment").id).toBe("deep");
    expect(effectiveTheme("system", false, "deep", "parchment").id).toBe(
      "parchment",
    );
  });

  it("follows the OS only in system mode", () => {
    expect(isLight("light", true)).toBe(true);
    expect(isLight("dark", false)).toBe(false);
    expect(isLight("system", true)).toBe(false);
    expect(isLight("system", false)).toBe(true);
  });
});

// --- The contrast ladder and its contrast rules (ADR 0026 steps 1 and 2, ADR 0020 step 4) --------

/** Whether an oklch literal is inside sRGB (with the 3-decimal rounding's tolerance). */
function inGamut(css: string): boolean {
  const raw = oklchToLinearRgb(...parseOklch(css));
  return raw.every((x) => x >= -0.002 && x <= 1.002);
}

const L = (v: Record<string, string>, k: string) => parseOklch(v[k])[0];
const T = (v: Record<string, string>, k: string) => linear(v[k]);
/** The ink at `pct` % over a backdrop, as the browser paints `color-mix(… transparent)`. */
const inkOver = (v: Record<string, string>, pct: number, bg: string) =>
  over(T(v, "--ink"), pct / 100, T(v, bg));

const ACCENTS = [
  "--tint",
  "--highlight",
  "--magic",
  "--danger",
  "--success",
  "--warning",
];
const AA_TEXT = 4.5;
const UI = 3;

describe("contrast ladder (ADR 0026 step 1)", () => {
  for (const { name, light, v, p } of CASES) {
    it(`${name}: the tones keep their order (dark page < card < raised, light page < raised < card)`, () => {
      const [page, card, raised] = ["--ground", "--surface", "--neu-900"].map(
        (k) => L(v, k),
      );
      if (light) expect(page < raised && raised < card).toBe(true);
      else expect(page < card && card < raised).toBe(true);
    });

    if (!light)
      it(`${name}: the accents sit on the ladder tint < danger < magic < success < warning < highlight < ink (step 1.4)`, () => {
        // A near-neutral theme's tint is grey, not an accent (step 3.6): it stays out of the order.
        const order = [
          ...(p.ramps.amb.c < NEAR_NEUTRAL_CHROMA ? [] : ["--tint"]),
          "--danger",
          "--magic",
          "--success",
          "--warning",
          "--highlight",
          "--ink",
        ];
        const ls = order.map((k) => L(v, k));
        for (let i = 1; i < ls.length; i++)
          expect(ls[i], `${order[i - 1]} < ${order[i]}`).toBeGreaterThan(
            ls[i - 1],
          );
      });
  }
});

describe("contrast (ADR 0026 step 2, ADR 0020 step 4)", () => {
  for (const { name, v, p, light } of CASES) {
    it(`${name}: every token is inside the sRGB gamut`, () => {
      for (const [k, css] of Object.entries(v))
        if (css.startsWith("oklch(")) expect(inGamut(css), k).toBe(true);
    });

    it(`${name}: card on page ΔL ≥ .10 and raised on card ΔL ≥ .07 (step 2.1)`, () => {
      expect(
        Math.abs(L(v, "--surface") - L(v, "--ground")),
        "card on page",
      ).toBeGreaterThanOrEqual(0.1 - 1e-9);
      expect(
        Math.abs(L(v, "--neu-900") - L(v, "--surface")),
        "raised on card",
      ).toBeGreaterThanOrEqual(0.07 - 1e-9);
    });

    it(`${name}: the field edge and the focus ring reach 3:1 on the card (step 2.2)`, () => {
      expect(
        ratio(inkOver(v, p.mix.input, "--surface"), T(v, "--surface")),
        "field edge on card",
      ).toBeGreaterThanOrEqual(UI);
      expect(
        ratio(T(v, "--tint"), T(v, "--surface")),
        "tint on card",
      ).toBeGreaterThanOrEqual(UI);
    });

    it(`${name}: ink and muted ink reach AA on page, card and the raised step (step 2.3)`, () => {
      for (const bg of ["--ground", "--surface", "--neu-900"]) {
        expect(
          ratio(T(v, "--ink"), T(v, bg)),
          `ink on ${bg}`,
        ).toBeGreaterThanOrEqual(AA_TEXT);
        expect(
          ratio(inkOver(v, p.mix.muted, bg), T(v, bg)),
          `muted on ${bg}`,
        ).toBeGreaterThanOrEqual(AA_TEXT);
      }
    });

    it(`${name}: every accent reads as text on the card and the raised step`, () => {
      for (const role of ACCENTS)
        for (const bg of ["--surface", "--neu-900"])
          expect(
            ratio(T(v, role), T(v, bg)),
            `${role} on ${bg}: ${v[role]} on ${v[bg]}`,
          ).toBeGreaterThanOrEqual(AA_TEXT);
    });

    it(`${name}: on-accent reads on every accent used as a fill, on-highlight on the highlight fill`, () => {
      for (const role of ACCENTS)
        expect(
          ratio(T(v, "--on-accent"), T(v, role)),
          `on-accent on ${role}`,
        ).toBeGreaterThanOrEqual(AA_TEXT);
      expect(
        ratio(T(v, "--on-highlight"), T(v, "--highlight-fill")),
        "on-highlight on highlight-fill",
      ).toBeGreaterThanOrEqual(AA_TEXT);
    });

    it(`${name}: the AC shield's number reads on its fill, its outline stands off it (ADR 0035 item 4)`, () => {
      expect(
        ratio(T(v, "--ink"), T(v, "--amb-800")),
        `ink on amb-800: ${v["--ink"]} on ${v["--amb-800"]}`,
      ).toBeGreaterThanOrEqual(AA_TEXT);
      for (const edge of ["--tint", "--warning"])
        expect(
          ratio(T(v, edge), T(v, "--amb-800")),
          `${edge} edge on amb-800`,
        ).toBeGreaterThanOrEqual(UI);
    });

    it(`${name}: the accent fills stand 3:1 off the card`, () => {
      for (const role of ACCENTS)
        expect(
          ratio(T(v, role), T(v, "--surface")),
          `${role} vs surface`,
        ).toBeGreaterThanOrEqual(UI);
      if (!light)
        expect(
          ratio(T(v, "--highlight-fill"), T(v, "--surface")),
          "highlight-fill vs surface",
        ).toBeGreaterThanOrEqual(UI);
    });
  }

  it("covers the ten theme-modes of ADR 0026 step 3.1", () => {
    expect(CASES.map((c) => c.name)).toEqual([
      "arcane dark",
      "arcane light",
      "deep dark",
      "parchment light",
      "plain dark",
      "plain light",
      "forest dark",
      "forest light",
      "ember dark",
      "amber dark",
    ]);
  });
});

// --- The raised stage's tones (ADR 0026 step 6) ---------------------------------------------------

describe("raised stage (ADR 0026 step 6)", () => {
  it("paints the band, sub-band, elevated tone and stage outline from the ink", () => {
    const dark = themeVars("arcane", false);
    expect(dark["--band"]).toBe(
      "color-mix(in oklab, var(--ink) 9%, transparent)",
    );
    expect(dark["--sub-band"]).toBe(
      "color-mix(in oklab, var(--ink) 5.5%, transparent)",
    );
    expect(dark["--elevated"]).toBe(
      "color-mix(in oklab, var(--ink) 5%, var(--surface))",
    );
    expect(dark["--stage-outline"]).toBe(
      "color-mix(in oklab, var(--ink) 40%, transparent)",
    );
    const light = themeVars("arcane", true);
    expect(light["--band"]).toBe(
      "color-mix(in oklab, var(--ink) 7%, transparent)",
    );
    expect(light["--sub-band"]).toBe(
      "color-mix(in oklab, var(--ink) 4%, transparent)",
    );
    // Light mode's focused card is the card itself (step 6.1).
    expect(light["--elevated"]).toBe("var(--surface)");
  });

  for (const { name, v, p, light } of CASES) {
    const card = T(v, "--surface");
    /** The tones a band's or a focused card's text sits on (step 6). */
    const elevated = over(T(v, "--ink"), p.mix.elevated / 100, card);
    const tones: [string, Rgb][] = [
      ["elevated", elevated],
      ["band on card", over(T(v, "--ink"), p.mix.band / 100, card)],
      ["band on elevated", over(T(v, "--ink"), p.mix.band / 100, elevated)],
      ["sub-band on card", over(T(v, "--ink"), p.mix.subBand / 100, card)],
    ];

    it(`${name}: card ≤ elevated, card < sub-band < band, each a step towards the ink (steps 6.1-6.3)`, () => {
      const lum = (c: Rgb) => luminance(c);
      const sign = light ? -1 : 1; // towards the ink: lighter in dark, darker in light
      const [el, band, sub] = [tones[0][1], tones[1][1], tones[3][1]].map(
        (c) => sign * (lum(c) - lum(card)),
      );
      expect(el, "elevated").toBeGreaterThanOrEqual(-1e-9);
      if (!light) expect(el, "elevated, dark").toBeGreaterThan(0);
      expect(sub, "sub-band").toBeGreaterThan(0);
      expect(band, "band past sub-band").toBeGreaterThan(sub);
    });

    it(`${name}: ink and muted ink read on every band and the focused card, accents where they are text`, () => {
      for (const [tone, bg] of tones) {
        expect(
          ratio(T(v, "--ink"), bg),
          `ink on ${tone}`,
        ).toBeGreaterThanOrEqual(AA_TEXT);
        expect(
          ratio(over(T(v, "--ink"), p.mix.muted / 100, bg), bg),
          `muted on ${tone}`,
        ).toBeGreaterThanOrEqual(AA_TEXT);
        // The act panel's band (on the focused card) holds the tabs: ink text and the chosen tab's
        // tint mark, a non-text mark at 3:1. Every other band can hold accent text.
        const onTabs = tone === "band on elevated";
        for (const role of onTabs ? ["--tint"] : ACCENTS)
          expect(
            ratio(T(v, role), bg),
            `${role} on ${tone}`,
          ).toBeGreaterThanOrEqual(onTabs ? UI : AA_TEXT);
      }
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

describe("index.css defaults (ADR 0026 step 1.6)", () => {
  const css = readFileSync(join(__dirname, "..", "index.css"), "utf8");
  it("paint the default light and dark themes exactly as themeVars computes them", () => {
    expect(block(css, ":root")).toEqual(themeVars(DEFAULT_LIGHT_THEME, true));
    expect(block(css, ".dark")).toEqual(themeVars(DEFAULT_DARK_THEME, false));
  });
});
