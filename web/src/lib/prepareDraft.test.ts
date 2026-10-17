import { describe, expect, it } from "vitest";
import {
  addOne,
  draftRows,
  levelBudgets,
  overBudget,
  removeOne,
  splitPrepared,
} from "./prepareDraft";
import type { LevelLimit, SpellbookEntry } from "@/types/spells";

const book = [{ id: "alarm" }, { id: "haste" }] as SpellbookEntry[];

describe("splitPrepared", () => {
  it("keeps spellbook ids (duplicates too) and drops the others once", () => {
    expect(
      splitPrepared(["alarm", "alarm", "shield", "haste", "shield", ""], book),
    ).toEqual({
      kept: ["alarm", "alarm", "haste"],
      dropped: ["shield", ""],
    });
  });
  it("is empty for an empty preparation", () => {
    expect(splitPrepared([], book)).toEqual({ kept: [], dropped: [] });
  });
});

describe("draftRows", () => {
  it("counts the copies of each spell, first added first", () => {
    expect(draftRows(["haste", "alarm", "haste"])).toEqual([
      { id: "haste", count: 2 },
      { id: "alarm", count: 1 },
    ]);
  });
});

describe("addOne / removeOne", () => {
  it("adds a copy, or the spell with a count of 1", () => {
    expect(addOne([], "alarm")).toEqual(["alarm"]);
    expect(draftRows(addOne(["alarm"], "alarm"))).toEqual([
      { id: "alarm", count: 2 },
    ]);
  });
  it("removes one copy; the last one takes the spell out", () => {
    const two = ["alarm", "haste", "alarm"];
    expect(removeOne(two, "alarm")).toEqual(["alarm", "haste"]);
    expect(removeOne(removeOne(two, "alarm"), "alarm")).toEqual(["haste"]);
    expect(removeOne(["haste"], "alarm")).toEqual(["haste"]);
  });
});

const levelled = [
  { id: "alarm", level: 1 },
  { id: "identify", level: 1 },
  { id: "haste", level: 3 },
  { id: "wish", level: 9 },
] as SpellbookEntry[];
const limits: LevelLimit[] = [
  { level: 1, max: 1 },
  { level: 2, max: 2 },
  { level: 3, max: 0 },
];

describe("levelBudgets", () => {
  it("lists the levels with a limit above 0 and those the draft holds, with the server's max", () => {
    expect(levelBudgets(["alarm", "haste"], levelled, limits)).toEqual([
      { level: 1, count: 1, max: 1 },
      { level: 2, count: 0, max: 2 },
      { level: 3, count: 1, max: 0 },
    ]);
  });
  it("counts copies and ignores ids outside the spellbook", () => {
    expect(
      levelBudgets(["alarm", "alarm", "shield"], levelled, limits),
    ).toEqual([
      { level: 1, count: 2, max: 1 },
      { level: 2, count: 0, max: 2 },
    ]);
  });
});

describe("overBudget", () => {
  it("is empty at or under every limit: Enregistrer is enabled", () => {
    expect(
      overBudget(
        levelBudgets(["alarm", "wish"], levelled, [
          ...limits,
          { level: 9, max: 1 },
        ]),
      ),
    ).toEqual([]);
  });
  it("names each level over its limit: Enregistrer is disabled", () => {
    expect(
      overBudget(
        levelBudgets(["alarm", "identify", "haste"], levelled, limits),
      ),
    ).toEqual([1, 3]);
  });
  it("is enabled again once the extra copy is removed", () => {
    const over = ["alarm", "alarm"];
    expect(overBudget(levelBudgets(over, levelled, limits))).toEqual([1]);
    expect(
      overBudget(levelBudgets(removeOne(over, "alarm"), levelled, limits)),
    ).toEqual([]);
  });
});
