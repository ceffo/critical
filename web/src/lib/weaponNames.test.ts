import { describe, expect, it } from "vitest";
import type { TFunction } from "i18next";
import { weaponName } from "./weaponNames";
import type { EquipmentCatalogue } from "@/types/items";

const t = ((key: string, o?: { defaultValue?: string }) =>
  key === "weapons.types.dagger"
    ? "Dagger"
    : (o?.defaultValue ?? key)) as unknown as TFunction;

const catalogue = {
  weapons: [
    {
      code: "heavy_horse_lance",
      name: { en: "Heavy horse lance", fr: "Lance de cavalerie lourde" },
    },
  ],
} as unknown as EquipmentCatalogue;

describe("weaponName (ADR 0037 step 21.6)", () => {
  it("names a code from the catalogue in the UI language", () => {
    expect(weaponName(t, "heavy_horse_lance", catalogue, "fr")).toBe(
      "Lance de cavalerie lourde",
    );
    expect(weaponName(t, "heavy_horse_lance", catalogue, "en")).toBe(
      "Heavy horse lance",
    );
  });
  it("falls back to the type's word, then to the code", () => {
    expect(weaponName(t, "dagger", catalogue, "en")).toBe("Dagger");
    expect(weaponName(t, "katana", null, "en")).toBe("katana");
  });
});
