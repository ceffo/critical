import { oklchToLinearRgb } from "./theme";

/**
 * WCAG 2 contrast arithmetic for the theme tests (`theme.test.ts`) and the UI-review report
 * (`ui-review/contrast.spec.ts`): oklch literals to linear sRGB, alpha compositing in gamma sRGB
 * (as a browser paints `color-mix(… transparent)` over a backdrop), relative luminance and the
 * contrast ratio. Not imported by the app.
 */

export type Rgb = [number, number, number];

export function parseOklch(css: string): [number, number, number] {
  const m = /^oklch\(([\d.]+) ([\d.]+) ([\d.]+)\)$/.exec(css);
  if (!m) throw new Error(`not an oklch literal: ${css}`);
  return [Number(m[1]), Number(m[2]), Number(m[3])];
}

const clamp01 = (x: number) => Math.min(1, Math.max(0, x));
export const linear = (css: string): Rgb =>
  oklchToLinearRgb(...parseOklch(css)).map(clamp01) as Rgb;
const toGamma = (x: number) =>
  x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055;
const toLinear = (x: number) =>
  x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;

/** `fg` at `alpha` over `bg`, composited in gamma sRGB. */
export function over(fg: Rgb, alpha: number, bg: Rgb): Rgb {
  return fg.map((c, i) => {
    const g = toGamma(c) * alpha + toGamma(bg[i]) * (1 - alpha);
    return toLinear(g);
  }) as Rgb;
}

export const luminance = ([r, g, b]: Rgb) =>
  0.2126 * r + 0.7152 * g + 0.0722 * b;

export const ratio = (a: Rgb, b: Rgb) => {
  const [la, lb] = [luminance(a), luminance(b)];
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
};

/** A linear-sRGB colour as a `#rrggbb` literal (for swatch captions). */
export const hex = (c: Rgb) =>
  "#" +
  c
    .map((x) =>
      Math.round(clamp01(toGamma(x)) * 255)
        .toString(16)
        .padStart(2, "0"),
    )
    .join("");
