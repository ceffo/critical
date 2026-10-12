import { describe, expect, it } from "vitest";
import { formatFactor, formatSigned, hpPercent, hpTone } from "./format";

describe("formatSigned", () => {
  it.each([
    [3, "+3"],
    [-5, "−5"],
    [0, "0"],
  ])("%d → %s", (n, want) => expect(formatSigned(n)).toBe(want));
});

describe("hpPercent / hpTone", () => {
  it("clamps to 0..100", () => {
    expect(hpPercent(-4, 50)).toBe(0);
    expect(hpPercent(60, 50)).toBe(100);
    expect(hpPercent(10, 0)).toBe(0);
  });
  it.each([
    [51, "good"],
    [50, "warn"],
    [25, "warn"],
    [24.9, "bad"],
  ] as const)("%d%% → %s", (pct, want) => expect(hpTone(pct)).toBe(want));
});

describe("formatFactor", () => {
  it.each([
    [2, "×2"],
    [1.5, "×1.5"],
  ])("%d → %s", (n, want) => expect(formatFactor(n)).toBe(want));
});
