import { describe, expect, it } from "vitest";
import type { ClassEntry } from "@/types/character";
import { primaryEntry } from "./classEntries";

const active = (cls: string, level: number): ClassEntry => ({
  class: cls,
  level,
  startingXp: 0,
  state: "active",
  leftAtLevel: 0,
});
const frozen = (cls: string, leftAt: number): ClassEntry => ({
  class: cls,
  level: leftAt,
  startingXp: 0,
  state: "frozen",
  leftAtLevel: leftAt,
});

// The same rows as the Go test TestView_Step3_PrimaryIsTheFirstActive (ADR 0045 step 5).
describe("primaryEntry", () => {
  it.each([
    ["a single active entry", [active("mage", 3)], "mage"],
    [
      "several active entries",
      [active("fighter", 5), active("mage", 4)],
      "fighter",
    ],
    [
      "a frozen entry first",
      [frozen("fighter", 9), active("mage", 10)],
      "mage",
    ],
    ["no active entry", [frozen("fighter", 9)], "fighter"],
  ])("%s", (_name, entries, want) => {
    expect(primaryEntry(entries)?.class).toBe(want);
  });

  it("is undefined without entries", () => {
    expect(primaryEntry(undefined)).toBeUndefined();
    expect(primaryEntry([])).toBeUndefined();
  });
});
