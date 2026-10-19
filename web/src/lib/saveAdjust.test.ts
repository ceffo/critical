import { describe, expect, it } from "vitest";
import type { SaveBreakdown } from "@/types/combat";
import {
  adjustmentChanged,
  adjustmentInEffect,
  MAX_ADJUSTMENT,
  stepAdjustment,
} from "./saveAdjust";

describe("stepAdjustment", () => {
  it.each([
    [0, 1, 1],
    [0, -1, -1],
    [3, 1, 4],
    [MAX_ADJUSTMENT, 1, MAX_ADJUSTMENT],
    [-MAX_ADJUSTMENT, -1, -MAX_ADJUSTMENT],
  ])("%i %+i → %i", (value, step, want) => {
    expect(stepAdjustment(value, step)).toBe(want);
  });
});

describe("adjustmentChanged", () => {
  const save = (adjustment: number, adjustmentReason?: string) =>
    ({ adjustment, adjustmentReason }) as SaveBreakdown;

  it.each([
    ["untouched card", save(0), 0, "", false],
    ["a reason alone changes nothing", save(0), 0, "Gaia fire", false],
    ["new adjustment", save(0), 4, "", true],
    [
      "same adjustment and reason",
      save(4, "Gaia fire"),
      4,
      " Gaia fire ",
      false,
    ],
    ["new reason", save(4, "Gaia fire"), 4, "Gaia lightning", true],
    ["back to 0", save(4, "Gaia fire"), 0, "Gaia fire", true],
  ])("%s", (_name, r, value, reason, want) => {
    expect(adjustmentChanged(r, value, reason)).toBe(want);
  });
});

// ADR 0027 step 8.2: the "adjust after the roll" line is closed by default and open while the check
// carries an adjustment (a save, or a skill check whose adjustment is absent when none was made).
describe("adjustmentInEffect", () => {
  it.each([
    [{ adjustment: 0 }, false],
    [{}, false],
    [{ adjustment: 4 }, true],
    [{ adjustment: -5 }, true],
  ])("%o → %s", (check, want) => {
    expect(adjustmentInEffect(check)).toBe(want);
  });
});
