import { describe, expect, it } from "vitest";
import { effectSpan, trackSize, windowStart } from "./timeline";

describe("trackSize", () => {
  it.each([
    [400, 8],
    [500, 8],
    [719, 8],
    [720, 12],
    [959, 12],
    [960, 16],
    [2000, 16],
  ])("%d px gives %d columns", (width, n) => {
    expect(trackSize(width)).toBe(n);
  });
});

describe("windowStart", () => {
  it.each([
    [0, 0],
    [1, 1],
    [2, 1],
    [3, 1],
    [4, 2],
    [10, 8],
  ])("turn %d starts at %d", (turn, start) => {
    expect(windowStart(turn)).toBe(start);
  });
});

describe("effectSpan", () => {
  it("a timed buff covers the current column and the turns it has left; the past runs from its start", () => {
    // Turn 5, window from 3: current column 2. Started turn 1 (before the window), 4 turns left.
    expect(effectSpan({ turn: 5, start: 3, size: 12, duration: 4, startRound: 1 })).toEqual({
      past: { from: 0, to: 1 },
      remaining: { from: 2, to: 5 },
      overflows: false,
    });
  });

  it("a 1-turn buff is only the current column", () => {
    expect(effectSpan({ turn: 2, start: 1, size: 8, duration: 1, startRound: 2 })).toEqual({
      past: null,
      remaining: { from: 1, to: 1 },
      overflows: false,
    });
  });

  it("a long buff runs past the window", () => {
    expect(effectSpan({ turn: 1, start: 1, size: 8, duration: 150, startRound: 1 })).toEqual({
      past: null,
      remaining: { from: 0, to: 7 },
      overflows: true,
    });
  });

  it("an untimed buff (charges or until dismissed) runs to the edge", () => {
    expect(effectSpan({ turn: 0, start: 0, size: 8, duration: 0, startRound: 0 })).toEqual({
      past: null,
      remaining: { from: 0, to: 7 },
      overflows: true,
    });
  });

  it("a start round above the turn (after End combat) has no past part", () => {
    expect(effectSpan({ turn: 0, start: 0, size: 8, duration: 3, startRound: 6 }).past).toBeNull();
  });
});
