// The lookup detail's spellbook offer (ADR 0030 steps 20 and 20.6): Learn and Add only for a class
// with a spellbook (mage, bard: the server's derived block) on a wizard spell the server would take;
// else the server's refusal, with the server's own code.
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import type { SpellbookRules } from "@/types/character";
import type { SpellbookEntry } from "@/types/spells";
import { spellbookOffer, type SpellbookRefusal } from "./spellbookOffer";

const rules: SpellbookRules = {
  classMaxSpellLevel: 9,
  intMaxSpellLevel: 7,
  learnChance: 65,
  maxSpellsPerLevel: 2,
  noMaximum: false,
  schoolAdjustment: 0,
  otherAdjustment: 0,
};
const entry = (id: string, level: number): SpellbookEntry => ({
  id,
  name: id,
  level,
});
const spell = { id: "s", level: 3 };

describe("spellbookOffer", () => {
  it("offers Learn and Add to a mage or a bard on a wizard spell", () => {
    expect(spellbookOffer(rules, true, [], spell)).toEqual({
      kind: "learnOrAdd",
    });
  });
  it("shows the spell in the spellbook once it is there", () => {
    expect(spellbookOffer(rules, true, [entry("s", 3)], spell)).toEqual({
      kind: "known",
    });
  });
  it("offers nothing to a class without a spellbook", () => {
    expect(spellbookOffer(null, true, [], spell)).toEqual({ kind: "none" });
  });
  it("offers nothing on a priest spell", () => {
    expect(spellbookOffer(rules, false, [], spell)).toEqual({ kind: "none" });
  });

  const cases: [
    string,
    SpellbookRules,
    SpellbookEntry[],
    number,
    SpellbookRefusal,
  ][] = [
    [
      "above the class",
      { ...rules, classMaxSpellLevel: 6 },
      [],
      7,
      "spell_level_above_class",
    ],
    ["above the Intelligence", rules, [], 8, "spell_level_above_intelligence"],
    [
      "the class check comes first",
      { ...rules, classMaxSpellLevel: 4, intMaxSpellLevel: 3 },
      [],
      5,
      "spell_level_above_class",
    ],
    [
      "level full",
      rules,
      [entry("a", 3), entry("b", 3)],
      3,
      "spellbook_level_full",
    ],
  ];
  it.each(cases)(
    "refuses %s with the server's code",
    (_n, r, book, level, code) => {
      expect(spellbookOffer(r, true, book, { id: "s", level })).toEqual({
        kind: "refused",
        code,
      });
    },
  );
  it("a book with no maximum is never full", () => {
    const book = [entry("a", 3), entry("b", 3), entry("c", 3)];
    expect(
      spellbookOffer({ ...rules, noMaximum: true }, true, book, spell),
    ).toEqual({ kind: "learnOrAdd" });
  });
  it("takes a spell at the limit", () => {
    expect(
      spellbookOffer(rules, true, [entry("a", 2)], { id: "s", level: 7 }),
    ).toEqual({ kind: "learnOrAdd" });
  });

  it("the refusal codes are the ones the server answers", () => {
    // The Go side maps learnsvc's refusals to these codes (internal/render); reading the table
    // keeps the web's wording keyed by exactly what a click would get.
    const render = readFileSync(
      new URL("../../../../api/internal/render/render.go", import.meta.url),
      "utf8",
    );
    const mapped = (target: string) =>
      new RegExp(`target: learnsvc\\.${target},[^\\n]*code: "([a-z_]+)"`).exec(
        render,
      )?.[1];
    expect(mapped("ErrAboveClass")).toBe("spell_level_above_class");
    expect(mapped("ErrAboveIntelligence")).toBe(
      "spell_level_above_intelligence",
    );
    expect(mapped("ErrLevelFull")).toBe("spellbook_level_full");
  });
});
