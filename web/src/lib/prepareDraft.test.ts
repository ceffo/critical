import { describe, expect, it } from "vitest";
import { splitPrepared } from "./prepareDraft";
import type { SpellbookEntry } from "@/types/spells";

const book = [{ id: "alarm" }, { id: "haste" }] as SpellbookEntry[];

describe("splitPrepared", () => {
  it("keeps spellbook ids (duplicates too) and drops the others once", () => {
    expect(splitPrepared(["alarm", "alarm", "shield", "haste", "shield", ""], book)).toEqual({
      kept: ["alarm", "alarm", "haste"],
      dropped: ["shield", ""],
    });
  });
  it("is empty for an empty preparation", () => {
    expect(splitPrepared([], book)).toEqual({ kept: [], dropped: [] });
  });
});
