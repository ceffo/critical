import { describe, expect, it } from "vitest";
import { copyName } from "./copyName";

const en = (name: string) => `${name} (copy)`;
const fr = (name: string) => `${name} (copie)`;

describe("copyName", () => {
  it("wraps the source's name in the wording of the player's language", () => {
    expect(copyName("Ilyana", en, 40)).toBe("Ilyana (copy)");
    expect(copyName("Ilyana", fr, 40)).toBe("Ilyana (copie)");
  });

  it("keeps the name whole when the limit is not known or is met exactly", () => {
    expect(copyName("x".repeat(60), en, null)).toBe(`${"x".repeat(60)} (copy)`);
    expect(copyName("x".repeat(33), en, 40)).toBe(`${"x".repeat(33)} (copy)`);
  });

  it("gives up the end of a long name so the suffix stays whole and the result fits", () => {
    const got = copyName("x".repeat(40), en, 40);
    expect(got).toBe(`${"x".repeat(33)} (copy)`);
    expect([...got]).toHaveLength(40);
    expect(copyName("é".repeat(40), fr, 40)).toBe(`${"é".repeat(32)} (copie)`);
  });

  it("drops the space the cut leaves before the suffix, and never goes under an empty name", () => {
    expect(copyName(`${"x".repeat(32)} yyyyyyy`, en, 40)).toBe(
      `${"x".repeat(32)} (copy)`,
    );
    expect(copyName("ab", (n) => `${n} (a very long suffix)`, 10)).toBe(
      " (a very long suffix)",
    );
  });
});
