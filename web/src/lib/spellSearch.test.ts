import { describe, expect, it } from "vitest";
import { baseKey, fold, nameMatches } from "./spellSearch";

// Runs against the shipped fr spells catalogue, by structure (keys present in it).
describe("nameMatches", () => {
  it("finds a French name by part of it, accents and case ignored", () => {
    expect(nameMatches("MAINS").prefix).toContain("burning_hands");
    expect(nameMatches("brulantes").contains).toContain("burning_hands");
  });

  it("ranks a full French name exact", () => {
    expect(nameMatches("mains brûlantes").exact).toEqual(["burning_hands"]);
  });

  it("puts each key in its best tier only", () => {
    const m = nameMatches("boule de feu");
    expect(m.exact).toContain("fireball");
    expect(m.prefix).not.toContain("fireball");
    expect(m.contains).not.toContain("fireball");
  });

  it("matches nothing for blank text or an unknown name", () => {
    expect(nameMatches("  ")).toEqual({ exact: [], prefix: [], contains: [] });
    expect(nameMatches("zzzz")).toEqual({ exact: [], prefix: [], contains: [] });
  });
});

describe("fold / baseKey", () => {
  it("folds accents and case", () => {
    expect(fold("Détection")).toBe("detection");
  });
  it("strips the _priest / _l<n> suffixes", () => {
    expect(baseKey("hold_person_priest")).toBe("hold_person");
    expect(baseKey("fortify_l4")).toBe("fortify");
    expect(baseKey("cure_wounds_priest_l4")).toBe("cure_wounds");
    expect(baseKey("fireball")).toBe("fireball");
  });
});
