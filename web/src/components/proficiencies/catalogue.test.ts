import { describe, expect, it } from "vitest";
import type { ProficiencyOption } from "@/types/character";
import {
  addPick,
  fold,
  optionsForCharacter,
  removePick,
  setSlots,
  splitCatalogue,
} from "./catalogue";

const opt = (id: string, groups: string[], slots = 1): ProficiencyOption => ({
  id,
  ability: "dex",
  modifier: 0,
  slots,
  groups,
});

const NAMES: Record<string, string> = {
  tumbling: "Acrobatie",
  riding_land: "Équitation, à terre",
  tracking: "Pistage",
  cooking: "Cuisine",
  languages_modern: "Langues vivantes",
};

// The costs are deliberately not Table 37's: the web shows the server's figure, never derives one.
const options = {
  proficiencies: [
    opt("tumbling", ["rogue"]),
    opt("tracking", ["warrior"], 2),
    opt("riding_land", ["general"]),
    opt("cooking", ["general"]),
    opt("languages_modern", ["general"]),
  ],
  proficiencyGroups: { thief: ["rogue", "general"] },
  proficiencyCosts: {
    thief: { tumbling: 7, tracking: 9, riding_land: 4, cooking: 5 },
  },
};
const nameOf = (id: string) => NAMES[id] ?? id;

describe("splitCatalogue", () => {
  it("lists the class's groups first, each part by name, with the server's cost", () => {
    const { own, other } = splitCatalogue(options, "thief", nameOf, "", "fr");
    expect(own.map((r) => [r.option.id, r.cost])).toEqual([
      ["tumbling", 7],
      ["cooking", 5],
      ["riding_land", 4],
    ]);
    expect(other.map((r) => [r.option.id, r.cost])).toEqual([["tracking", 9]]);
  });

  it("offers only what the server gives a cost for (it gives Languages, Modern none)", () => {
    const { own, other } = splitCatalogue(options, "thief", nameOf, "", "fr");
    expect([...own, ...other].map((r) => r.option.id)).not.toContain(
      "languages_modern",
    );
  });

  it("searches the name, accents and case ignored", () => {
    const { own, other } = splitCatalogue(
      options,
      "thief",
      nameOf,
      " EQUI",
      "fr",
    );
    expect([...own, ...other].map((r) => r.option.id)).toEqual(["riding_land"]);
    expect(fold("Équitation")).toBe("equitation");
  });

  it("offers nothing for a class the server sent no costs for", () => {
    const { own, other } = splitCatalogue(options, "mage", nameOf, "", "fr");
    expect([...own, ...other]).toEqual([]);
  });
});

describe("picks", () => {
  it("adds a pick at the server's cost, once, and never without a cost", () => {
    const one = addPick([], "tumbling", 7);
    expect(one).toEqual([{ id: "tumbling", slots: 7 }]);
    expect(addPick(one, "tumbling", 7)).toBe(one);
    expect(addPick(one, "cooking", undefined)).toBe(one);
  });

  it("sets one pick's slots and removes one", () => {
    const picks = [
      { id: "tumbling", slots: 1 },
      { id: "cooking", slots: 1 },
    ];
    expect(setSlots(picks, "cooking", 3)).toEqual([
      { id: "tumbling", slots: 1 },
      { id: "cooking", slots: 3 },
    ]);
    expect(removePick(picks, "tumbling")).toEqual([
      { id: "cooking", slots: 1 },
    ]);
  });
});

// ADR 0042 step 15: the costs and the groups are the server's for the character, not the creation
// options' table of the first class.
describe("optionsForCharacter", () => {
  const creation = {
    proficiencies: [opt("spellcraft", ["Wizard"])],
    proficiencyGroups: { fighter: ["Warrior"] },
    proficiencyCosts: { fighter: { spellcraft: 2 } },
  };

  it("files the server's groups and costs under the class the picker reads", () => {
    const o = optionsForCharacter(creation, "fighter", {
      groups: ["Warrior", "Wizard"],
      catalogue: [{ id: "spellcraft", cost: 1 }],
    });
    expect(o.proficiencyGroups).toEqual({ fighter: ["Warrior", "Wizard"] });
    expect(o.proficiencyCosts).toEqual({ fighter: { spellcraft: 1 } });
    const { own } = splitCatalogue(o, "fighter", (id) => id, "", "fr");
    expect(own.map((r) => [r.option.id, r.cost])).toEqual([["spellcraft", 1]]);
  });

  it("keeps the creation options when the server serves no figures", () => {
    expect(optionsForCharacter(creation, "fighter", undefined)).toBe(creation);
    expect(optionsForCharacter(creation, "fighter", {})).toBe(creation);
  });
});
