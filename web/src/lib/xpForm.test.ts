import { describe, expect, it } from "vitest";
import { parseXpAmount } from "./xpForm";

describe("parseXpAmount", () => {
  it("reads whole numbers, ignoring thousands spaces", () => {
    expect(parseXpAmount("1200")).toBe(1200);
    expect(parseXpAmount(" 12 500 ")).toBe(12500);
    expect(parseXpAmount("12 500")).toBe(12500);
    expect(parseXpAmount("0")).toBe(0);
  });
  it("rejects anything else", () => {
    for (const s of ["", "  ", "-5", "1.5", "1,5", "12a", "1e3"])
      expect(parseXpAmount(s)).toBeNull();
  });
});
