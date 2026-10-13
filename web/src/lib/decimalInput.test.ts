import { describe, expect, it } from "vitest";
import { parseDecimal } from "./decimalInput";

describe("parseDecimal", () => {
  it.each([
    ["2", 2],
    ["1.5", 1.5],
    ["1,5", 1.5],
    [" 2.25 ", 2.25],
    ["1", 1],
    ["0.99", 0.99],
  ])("reads %j as %d", (input, want) => {
    expect(parseDecimal(input)).toBe(want);
  });

  it.each(["", "  ", "abc", "1.5.2", "1,5,2", "1e3", "-2", "×2"])("rejects %j", (input) => {
    expect(parseDecimal(input)).toBeNaN();
  });
});
