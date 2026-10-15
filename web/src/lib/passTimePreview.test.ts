import { describe, expect, it } from "vitest";
import { hoursToAsk, previewFor, type KeyedAnswer } from "./passTimePreview";

const ok = (hours: number, rounds: number): KeyedAnswer => ({
  hours,
  answer: { kind: "ok", rounds, expiring: [] },
});

describe("previewFor", () => {
  it("typing 1, then 1, then 1,5: the answer for 1 never shows for 1,5", () => {
    expect(previewFor("1", null)).toEqual({ kind: "pending" });
    expect(previewFor("1", ok(1, 60))).toMatchObject({ kind: "ok", rounds: 60 });
    // "1," is not a number yet: the hint, not the answer for 1.
    expect(previewFor("1,", ok(1, 60))).toEqual({ kind: "invalid" });
    expect(previewFor("1,5", ok(1, 60))).toEqual({ kind: "pending" });
    expect(previewFor("1,5", ok(1.5, 90))).toMatchObject({ kind: "ok", rounds: 90 });
  });

  it("an answer for older input arriving last is ignored", () => {
    // The 1.5 answer landed, then a late answer for 1 replaced it: still pending, Confirm off.
    expect(previewFor("1.5", ok(1, 60)).kind).toBe("pending");
  });

  it("the server's refusal is shown for the hours it was about", () => {
    const refused: KeyedAnswer = { hours: 0.01, answer: { kind: "error", message: "less than one round" } };
    expect(previewFor("0.01", refused)).toEqual({ kind: "error", message: "less than one round" });
  });

  it("empty and non-numbers never ask the server", () => {
    expect(previewFor("  ", null)).toEqual({ kind: "empty" });
    expect(hoursToAsk("abc")).toBeNull();
    expect(hoursToAsk("0")).toBeNull();
    expect(hoursToAsk("1,5")).toBe(1.5);
  });
});
