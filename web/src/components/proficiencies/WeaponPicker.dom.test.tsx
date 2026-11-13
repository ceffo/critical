import { afterEach, describe, expect, it } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useEquipmentCatalogueStore } from "@/stores/equipmentCatalogue";
import type { EquipmentCatalogue } from "@/types/items";
import { WeaponPicker } from "./WeaponPicker";

const weapon = (
  code: string,
  fr: string,
  en: string,
  weaponKind: "melee" | "missile",
) => ({ code, name: { fr, en }, weaponKind });

// A catalogue read once (the inventory picker's), seeded so the picker fetches nothing.
const catalogue = {
  weapons: [
    weapon("long_bow", "Arc long", "Long bow", "missile"),
    weapon("quarterstaff", "Bâton", "Quarterstaff", "melee"),
    weapon("heavy_horse_lance", "Lance d’arçon lourde", "Heavy lance", "melee"),
  ],
  ammunition: [],
  armour: [],
  shields: [],
} as unknown as EquipmentCatalogue;

const options = [
  { code: "long_bow", specializationSlots: 3 },
  { code: "quarterstaff", specializationSlots: 2 },
  { code: "heavy_horse_lance", specializationSlots: 2 },
];

describe("WeaponPicker grouping", () => {
  afterEach(() => useEquipmentCatalogueStore.setState({ catalogue: null }));

  it.each([
    ["fr", "Mêlée", "Tir", "Bâton"],
    ["en", "Melee", "Missile", "Quarterstaff"],
  ] as const)(
    "lists the weapons by %s name under their family headings",
    async (lng, melee, missile, staff) => {
      useEquipmentCatalogueStore.setState({ catalogue });
      await renderApp(
        <WeaponPicker
          weapons={options}
          picks={[]}
          onChange={() => {}}
          figures={null}
          errorFor={() => null}
        />,
        { lng },
      );
      const headings = screen.getAllByRole("heading").map((h) => h.textContent);
      expect(headings).toEqual([melee, missile]);
      const group = screen.getByTestId("weapon-group-melee");
      expect(within(group).getByText(staff)).toBeInTheDocument();
      expect(
        within(group).queryByText(lng === "fr" ? "Arc long" : "Long bow"),
      ).toBeNull();
      // Two picks of one family are two rows: the staff and the lance.
      expect(within(group).getAllByTestId("weapon-add")).toHaveLength(2);
    },
  );
});
