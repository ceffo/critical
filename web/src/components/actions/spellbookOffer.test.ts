// The lookup detail's spellbook offer (ADR 0030 steps 20 and 20.6): Learn and Add only for a class
// with a spellbook (mage, bard: the server's derived block) on a wizard spell not yet in the book.
import { describe, expect, it } from "vitest";
import type { SpellbookRules } from "@/types/character";
import { spellbookOffer } from "./spellbookOffer";

const rules: SpellbookRules = {
  classMaxSpellLevel: 9,
  intMaxSpellLevel: 7,
  learnChance: 65,
  maxSpellsPerLevel: 11,
  noMaximum: false,
};

describe("spellbookOffer", () => {
  it("offers Learn and Add to a mage or a bard on a wizard spell", () => {
    expect(spellbookOffer(rules, true, false)).toBe("learnOrAdd");
  });
  it("shows the spell in the spellbook once it is there", () => {
    expect(spellbookOffer(rules, true, true)).toBe("known");
  });
  it("offers nothing to a class without a spellbook", () => {
    expect(spellbookOffer(null, true, false)).toBe("none");
  });
  it("offers nothing on a priest spell", () => {
    expect(spellbookOffer(rules, false, false)).toBe("none");
  });
});
