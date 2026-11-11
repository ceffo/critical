import { describe, expect, it } from "vitest";
import { actFeatures, lookupLabelKey } from "./actFeatures";

const daily = [{ behaviour: "daily" as const }];
const stock = [{ behaviour: "stock" as const }];

describe("actFeatures", () => {
  it("a character with no slots, no powers and no per-day item has no Spells tab", () => {
    expect(actFeatures(0, false, stock).showSpellsTab).toBe(false);
  });
  it("slots show the Spells tab", () => {
    expect(actFeatures(3, false, []).showSpellsTab).toBe(true);
  });
  it("a per-day item keeps the Spells tab, where the Long rest refilling it lives", () => {
    const f = actFeatures(0, false, daily);
    expect(f.showSpellsTab).toBe(true);
    expect(f.hasSpells).toBe(false);
  });
  it("names the rail's activity by its content", () => {
    expect(lookupLabelKey(actFeatures(2, false, []))).toBe("spells");
    expect(lookupLabelKey(actFeatures(0, true, []))).toBe("powers");
    expect(lookupLabelKey(actFeatures(2, true, []))).toBe("spellsAndPowers");
    expect(lookupLabelKey(actFeatures(0, false, daily))).toBe("reference");
  });
});
