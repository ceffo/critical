import { describe, expect, it } from "vitest";
import type { ProficiencyOption } from "@/types/character";
import {
  addPick,
  fold,
  catalogueRows,
  groupBands,
  servedCatalogue,
  removePick,
  setSlots,
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

const served = (
  cls = "thief",
  figures?: Parameters<typeof servedCatalogue>[2],
) => servedCatalogue(options, cls, figures);

describe("catalogueRows", () => {
  it("lists every row by name with the server's cost", () => {
    const rows = catalogueRows(served(), nameOf, "", "fr");
    expect(rows.map((r) => [r.option.id, r.cost])).toEqual([
      ["tumbling", 7],
      ["cooking", 5],
      ["riding_land", 4],
      ["tracking", 9],
    ]);
  });

  it("offers only what the server gives a cost for (it gives Languages, Modern none)", () => {
    const ids = catalogueRows(served(), nameOf, "", "fr").map(
      (r) => r.option.id,
    );
    expect(ids).not.toContain("languages_modern");
  });

  it("searches the name, accents and case ignored", () => {
    const rows = catalogueRows(served(), nameOf, " EQUI", "fr");
    expect(rows.map((r) => r.option.id)).toEqual(["riding_land"]);
    expect(fold("Équitation")).toBe("equitation");
  });

  it("offers nothing for a class the server sent no costs for", () => {
    expect(catalogueRows(served("mage"), nameOf, "", "fr")).toEqual([]);
  });
});

describe("groupBands", () => {
  it("puts the character's groups first, then the others by name, a row under each of its groups", () => {
    const rows = catalogueRows(served(), nameOf, "", "fr");
    const bands = groupBands(rows, ["rogue", "general"], (g) => g, "fr");
    expect(
      bands.map((b) => [b.group, b.own, b.rows.map((r) => r.option.id)]),
    ).toEqual([
      ["rogue", true, ["tumbling"]],
      ["general", true, ["cooking", "riding_land"]],
      ["warrior", false, ["tracking"]],
    ]);
  });

  it("leaves out a group the search emptied", () => {
    const rows = catalogueRows(served(), nameOf, "pist", "fr");
    expect(
      groupBands(rows, ["rogue", "general"], (g) => g, "fr").map(
        (b) => b.group,
      ),
    ).toEqual(["warrior"]);
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

// ADR 0042 step 15: the costs and the groups are the server's for the
// character (every class entry judged together), read as served, not the first class's table.
describe("servedCatalogue", () => {
  const creation = {
    proficiencies: [opt("spellcraft", ["Wizard"])],
    proficiencyGroups: { fighter: ["Warrior"] },
    proficiencyCosts: { fighter: { spellcraft: 2 } },
  };

  it("reads the server's groups and costs directly", () => {
    const c = servedCatalogue(creation, "fighter", {
      groups: ["Warrior", "Wizard"],
      catalogue: [
        {
          id: "spellcraft",
          cost: 1,
          ability: "int",
          modifier: -2,
          target: 14,
        },
      ],
    });
    expect(c.groups).toEqual(["Warrior", "Wizard"]);
    expect(c.costs).toEqual({ spellcraft: 1 });
  });

  it("carries the served target of a new pick to its row, with no arithmetic here", () => {
    const c = servedCatalogue(creation, "fighter", {
      groups: [],
      catalogue: [
        {
          id: "spellcraft",
          cost: 1,
          ability: "int",
          modifier: -2,
          target: 14,
        },
      ],
    });
    expect(catalogueRows(c, (id) => id, "", "en")[0].target).toBe(14);
  });

  it("falls back to the creation options' figures when the server serves none", () => {
    const c = servedCatalogue(creation, "fighter", undefined);
    expect(c.groups).toEqual(["Warrior"]);
    expect(c.costs).toEqual({ spellcraft: 2 });
  });
});
