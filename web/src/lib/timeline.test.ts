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
  ])("round %d starts at %d", (round, start) => {
    expect(windowStart(round)).toBe(start);
  });
});

describe("effectSpan", () => {
  it("a timed buff covers the current column and the rounds it has left; the past runs from its start", () => {
    // Round 5, window from 3: current column 2. Started round 1 (before the window), 4 rounds left.
    expect(
      effectSpan({ round: 5, start: 3, size: 12, duration: 4, startRound: 1 }),
    ).toEqual({
      past: { from: 0, to: 1 },
      remaining: { from: 2, to: 5 },
      overflows: false,
    });
  });

  it("a 1-round buff is only the current column", () => {
    expect(
      effectSpan({ round: 2, start: 1, size: 8, duration: 1, startRound: 2 }),
    ).toEqual({
      past: null,
      remaining: { from: 1, to: 1 },
      overflows: false,
    });
  });

  it("in the 5-column window a 3-round buff at round 4 fills the current column and the two ahead", () => {
    // Round 4, window 2..6: two rounds of history, the current one, two ahead.
    expect(
      effectSpan({
        round: 4,
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
      effectSpan({ round: 1, start: 1, size: 8, duration: 150, startRound: 1 }),
    ).toEqual({
      past: null,
      remaining: { from: 0, to: 7 },
      overflows: true,
    });
  });

  it("an untimed buff (charges or until dismissed) runs to the edge", () => {
    expect(
      effectSpan({ round: 0, start: 0, size: 8, duration: 0, startRound: 0 }),
    ).toEqual({
      past: null,
      remaining: { from: 0, to: 7 },
      overflows: true,
    });
  });

  it("a buff added at round 5, then End combat: no past part, the pill starts at column 0", () => {
    expect(
      effectSpan({
        round: 0,
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
