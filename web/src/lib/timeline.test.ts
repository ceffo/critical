import { describe, expect, it } from "vitest";
import { TRACK_COLUMNS, effectSpan, windowStart } from "./timeline";

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
    expect(
      effectSpan({ turn: 5, start: 3, size: 12, duration: 4, startRound: 1 }),
    ).toEqual({
      past: { from: 0, to: 1 },
      remaining: { from: 2, to: 5 },
      overflows: false,
    });
  });

  it("a 1-turn buff is only the current column", () => {
    expect(
      effectSpan({ turn: 2, start: 1, size: 8, duration: 1, startRound: 2 }),
    ).toEqual({
      past: null,
      remaining: { from: 1, to: 1 },
      overflows: false,
    });
  });

  it("in the 5-column window a 3-turn buff at turn 4 fills the current column and the two ahead", () => {
    // Turn 4, window 2..6: two turns of history, the current one, two ahead.
    expect(
      effectSpan({
        turn: 4,
        start: windowStart(4),
        size: TRACK_COLUMNS,
        duration: 3,
        startRound: 3,
      }),
    ).toEqual({
      past: { from: 1, to: 1 },
      remaining: { from: 2, to: 4 },
      overflows: false,
    });
  });

  it("a long buff runs past the window", () => {
    expect(
      effectSpan({ turn: 1, start: 1, size: 8, duration: 150, startRound: 1 }),
    ).toEqual({
      past: null,
      remaining: { from: 0, to: 7 },
      overflows: true,
    });
  });

  it("an untimed buff (charges or until dismissed) runs to the edge", () => {
    expect(
      effectSpan({ turn: 0, start: 0, size: 8, duration: 0, startRound: 0 }),
    ).toEqual({
      past: null,
      remaining: { from: 0, to: 7 },
      overflows: true,
    });
  });

  it("a buff added at turn 5, then End combat: no past part, the pill starts at column 0", () => {
    expect(
      effectSpan({
        turn: 0,
        start: windowStart(0),
        size: 8,
        duration: 3,
        startRound: 5,
      }),
    ).toEqual({
      past: null,
      remaining: { from: 0, to: 2 },
      overflows: false,
    });
  });
});
