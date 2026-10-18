import { describe, expect, it } from "vitest";
import { parseWhole } from "./wholeNumber";

describe("parseWhole", () => {
  it("reads a whole number, blank as null", () => {
    expect(parseWhole(" 16 ")).toBe(16);
    expect(parseWhole("-1")).toBe(-1);
    expect(parseWhole("0")).toBe(0);
    expect(parseWhole("  ")).toBeNull();
  });

  it("is NaN for anything else", () => {
    for (const raw of ["2.5", "1e3", "12a", "+3", "1 000"])
      expect(parseWhole(raw)).toBeNaN();
  });
});
