// The lookup's class toggle offers "Mine" only to a class with a spellbook (mage, bard: the
// server's derived block), and a stale "Mine" falls back to a valid option.
import { describe, expect, it } from "vitest";
import type { SpellbookRules } from "@/types/character";
import { effectiveClass, lookupClasses } from "./lookupClasses";

const rules: SpellbookRules = {
  classMaxSpellLevel: 9,
  intMaxSpellLevel: 7,
  learnChance: 65,
  maxSpellsPerLevel: 11,
  noMaximum: false,
};

describe("lookupClasses", () => {
  it("offers Mine first to a character with a spellbook", () => {
    expect(lookupClasses(rules).map((c) => c.value)).toEqual([
      "mine",
      "wizard",
      "priest",
      "all",
    ]);
  });
  it("leaves Mine out for a character without one", () => {
    expect(lookupClasses(null).map((c) => c.value)).toEqual([
      "wizard",
      "priest",
      "all",
    ]);
  });
});

describe("effectiveClass", () => {
  it("falls back to Wizard when Mine is selected without a spellbook", () => {
    expect(effectiveClass("mine", null)).toBe("wizard");
  });
  it("keeps Mine with a spellbook and every other class either way", () => {
    expect(effectiveClass("mine", rules)).toBe("mine");
    expect(effectiveClass("priest", null)).toBe("priest");
    expect(effectiveClass("all", rules)).toBe("all");
  });
});
