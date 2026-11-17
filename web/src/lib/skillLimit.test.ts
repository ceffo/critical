import { describe, expect, it } from "vitest";
import { pointsLimit } from "./skillLimit";

describe("pointsLimit", () => {
  it.each([
    // [points, rawPercent, cap, max, expected]
    [15, 65, 0, 95, 45], // a bard: 95 - (65 - 15)
    [20, 40, 0, 95, 75],
    [10, 45, 0, 95, 60],
    [0, 40, 30, 95, 30], // a thief's cap per skill is lower than the room under 95
    [0, 80, 30, 95, 15], // the room under 95 is lower than the cap
    [5, 100, 30, 95, 0], // nothing left under the maximum
  ])(
    "points %i raw %i cap %i max %i -> %i",
    (points, rawPercent, cap, max, want) => {
      expect(pointsLimit({ points, rawPercent }, cap, max)).toBe(want);
    },
  );
});
