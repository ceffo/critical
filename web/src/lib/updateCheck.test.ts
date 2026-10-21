import { describe, expect, it } from "vitest";
import {
  isNewer,
  nextStreak,
  parseRelease,
  updateAvailable,
} from "./updateCheck";

describe("which versions can be compared", () => {
  it.each([
    ["1.4.2", [1, 4, 2]],
    ["v0.61.1", [0, 61, 1]],
    ["unknown", null],
    ["", null],
    ["0.61.1+43d0ef8", null],
    ["0.61.1+43d0ef8.dirty", null],
    ["1.4", null],
  ])("%s", (v, want) => {
    expect(parseRelease(v)).toEqual(want);
  });
});

describe("a newer version", () => {
  it.each([
    ["1.4.2", "1.4.3", true],
    ["1.4.9", "1.5.0", true],
    ["1.9.9", "2.0.0", true],
    ["0.9.0", "0.10.0", true], // numbers, not text
    ["1.4.2", "1.4.2", false],
    ["1.4.3", "1.4.2", false], // a rollback shows nothing
    ["1.4.2", "unknown", false],
    ["unknown", "1.4.2", false],
    ["1.4.2+abc", "1.4.3", false], // a dev build never triggers
    ["1.4.2", "1.4.3+abc.dirty", false],
  ])("%s then %s: %s", (running, answer, want) => {
    expect(isNewer(running, answer)).toBe(want);
  });
});

// Feeds answers one by one and returns whether the notice shows after each.
function run(running: string, answers: (string | null)[]): boolean[] {
  let streak = 0;
  return answers.map((a) => {
    streak = nextStreak(streak, running, a);
    return updateAvailable(streak);
  });
}

describe("the notice needs two consecutive newer answers", () => {
  it("one newer answer shows nothing, the second shows it", () => {
    expect(run("1.4.2", ["1.4.3", "1.4.3"])).toEqual([false, true]);
  });
  it("a rolling deploy (newer, then the old machine) starts over", () => {
    expect(run("1.4.2", ["1.4.3", "1.4.2", "1.4.3"])).toEqual([
      false,
      false,
      false,
    ]);
  });
  it("a failed read starts over", () => {
    expect(run("1.4.2", ["1.4.3", null, "1.4.3"])).toEqual([
      false,
      false,
      false,
    ]);
  });
  it.each([
    ["an older answer", "1.4.1"],
    ["an equal one", "1.4.2"],
    ["unknown", "unknown"],
  ])("%s never shows it", (_n, a) => {
    expect(run("1.4.2", [a, a, a])).toEqual([false, false, false]);
  });
  it("a dev build never shows it", () => {
    expect(run("1.4.2+abc.dirty", ["1.4.3", "1.4.3"])).toEqual([false, false]);
  });
});
