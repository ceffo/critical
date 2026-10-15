import { describe, expect, it } from "vitest";
import { MAX_TARGET_AC, adjustFormKey, attackSettled, hitsAc, parseTargetAc, stepTargetAc, tellGmWithCrit } from "./attackAdjust";

describe("stepTargetAc", () => {
  it.each([
    [5, 1, 6],
    [5, -1, 4],
    [0, -1, -1],
    [MAX_TARGET_AC, 1, MAX_TARGET_AC],
    [-MAX_TARGET_AC, -1, -MAX_TARGET_AC],
  ])("%i %+i = %i", (ac, step, want) => {
    expect(stepTargetAc(ac, step)).toBe(want);
  });
});

describe("stepTargetAc with the served highest AC", () => {
  it.each([
    [9, 1, 10],
    [10, 1, 10],
    [-MAX_TARGET_AC, -1, -MAX_TARGET_AC],
  ])("%i %+i = %i at most 10", (ac, step, want) => {
    expect(stepTargetAc(ac, step, 10)).toBe(want);
  });
});

describe("parseTargetAc with the served highest AC", () => {
  it.each([
    ["10", 10],
    ["-20", -20],
  ])("%j is %i", (text, want) => {
    expect(parseTargetAc(text, 10)).toBe(want);
  });

  it.each(["11", "20"])("%j is above the highest AC", (text) => {
    expect(parseTargetAc(text, 10)).toBeNull();
  });
});

describe("parseTargetAc", () => {
  it.each([
    ["7", 7],
    [" -3 ", -3],
    ["−2", -2],
    ["0", 0],
    ["20", 20],
    ["-20", -20],
  ])("%j is %i", (text, want) => {
    expect(parseTargetAc(text)).toBe(want);
  });

  it.each(["", "-", "1.5", "abc", "21", "-21"])("%j is not an AC", (text) => {
    expect(parseTargetAc(text)).toBeNull();
  });
});

describe("adjustFormKey", () => {
  it("changes when the AC goes back to unknown, so the form restarts empty", () => {
    expect(adjustFormKey({ logEntryId: 7, targetAc: null })).not.toBe(adjustFormKey({ logEntryId: 7, targetAc: 4 }));
  });
  it("changes with the attack and the AC, and is stable otherwise", () => {
    expect(adjustFormKey({ logEntryId: 7, targetAc: 4 })).toBe(adjustFormKey({ logEntryId: 7, targetAc: 4 }));
    expect(adjustFormKey({ logEntryId: 8, targetAc: 4 })).not.toBe(adjustFormKey({ logEntryId: 7, targetAc: 4 }));
    expect(adjustFormKey({ logEntryId: 7, targetAc: 5 })).not.toBe(adjustFormKey({ logEntryId: 7, targetAc: 4 }));
  });
});

describe("attackSettled", () => {
  it.each([
    [{ acKnown: true }, true],
    [{}, true], // logged before unknown ACs: the AC was known
    [{ acKnown: false }, false],
    [{ acKnown: false, verdict: "miss" }, true],
  ])("%j → %s", (r, want) => {
    expect(attackSettled(r)).toBe(want);
  });
});

describe("hitsAc", () => {
  it("reads hitsAc, else the older targetACNeeded", () => {
    expect(hitsAc({ hitsAc: -3, targetACNeeded: -3 })).toBe(-3);
    expect(hitsAc({ targetACNeeded: 4 })).toBe(4);
  });
});

describe("tellGmWithCrit", () => {
  it.each([
    [true, true],
    [false, false],
    [undefined, true],
  ])("critPossible %s shows the crit part: %s", (critPossible, want) => {
    expect(tellGmWithCrit({ critPossible })).toBe(want);
  });
});
