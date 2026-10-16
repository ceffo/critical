import { describe, expect, it } from "vitest";
import { inventoryNamer, recordedNames } from "./inventoryName";

const gem = {
  id: "power_gem_dispel",
  name: { en: "Power Gem", fr: "Gemme de pouvoir" },
};
const sword = {
  id: "longsword",
  name: { en: "Long Sword +2", fr: "Épée longue +2" },
};

// ADR 0023 step 12: a result or a log entry names its item or weapon by id, in the player's
// language, while the inventory has it; else by the name recorded with it.
describe("inventoryNamer", () => {
  const fr = inventoryNamer([gem], [sword], "fr");

  it("names by id in the player's language", () => {
    expect(fr.item("power_gem_dispel", "Power Gem")).toBe("Gemme de pouvoir");
    expect(fr.weapon("longsword", "Long Sword +2")).toBe("Épée longue +2");
  });

  it("keeps the recorded name for an id the inventory no longer has, or none", () => {
    expect(fr.item("wand_lightning", "Wand of lightning")).toBe(
      "Wand of lightning",
    );
    expect(fr.weapon(undefined, "Dagger +3")).toBe("Dagger +3");
  });

  it("recordedNames names as recorded", () => {
    expect(recordedNames.item("power_gem_dispel", "Power Gem")).toBe(
      "Power Gem",
    );
  });
});
