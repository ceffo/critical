import { describe, expect, it } from "vitest";
import { hpBarModel } from "./hpBar";

describe("hpBarModel", () => {
  // Band selection: the fill colour by threshold on the served current / max.
  it.each([
    [85, "good"],
    [43, "good"],
    [42, "warn"], // 49.4 %
    [22, "warn"], // 25.9 %
    [21, "bad"], // 24.7 %
    [0, "bad"],
    [-4, "bad"],
  ] as const)("%d / 85 → %s", (current, tone) =>
    expect(hpBarModel(current, 85, -10, 0).tone).toBe(tone),
  );

  it("puts the dying zone floor..0 on the left of the floor..max scale", () => {
    const m = hpBarModel(70, 90, -10, 0);
    expect(m.dyingPct).toBeCloseTo(10);
    expect(m.deepPct).toBe(0);
  });

  it("fills the dying zone from 0 down to current HP below 0", () => {
    expect(hpBarModel(-4, 90, -10, 0).deepPct).toBeCloseTo(40);
    expect(hpBarModel(-10, 90, -10, 0).deepPct).toBe(100);
    expect(hpBarModel(-4, 90, -10, 0).fillPct).toBe(0);
  });

  it("draws no dying zone without a floor below 0", () => {
    expect(hpBarModel(5, 10, 0, 0).dyingPct).toBe(0);
  });

  it("draws the last damage after the fill, clipped at max", () => {
    const m = hpBarModel(70, 100, -10, 9);
    expect(m.fillPct).toBe(70);
    expect(m.ghostPct).toBeCloseTo(9);
    expect(hpBarModel(95, 100, -10, 20).ghostPct).toBe(5);
    expect(hpBarModel(70, 100, -10, 0).ghostPct).toBe(0);
  });
});
