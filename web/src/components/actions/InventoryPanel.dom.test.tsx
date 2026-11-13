import { afterEach, describe, expect, it } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { pieceFixture } from "@/test/pieceFixture";
import { useItemsStore } from "@/stores/items";
import { InventoryPanel } from "./InventoryPanel";

afterEach(() => useItemsStore.setState({ items: [] }));

const plate = pieceFixture({
  id: "plate",
  name: { en: "Plate mail" },
  category: "armour",
  usable: false,
  baseAc: 3,
});
// A weapon that kept an old count above 1, behaviour and effect dice: stays as stored, no controls.
const sword = pieceFixture({
  id: "sword",
  name: { en: "Longsword" },
  category: "weapon",
  usable: false,
  behaviour: "charges",
  count: 3,
  effectDice: "1d8",
});
const potion = pieceFixture({
  id: "potion",
  name: { en: "Potion of healing" },
  category: "magic",
  magicKind: "potion",
  count: 2,
});

describe("InventoryPanel rows (a weapon, an armour and a shield are never used)", () => {
  it("shows no use, add, count or behaviour on a weapon or an armour, but on a potion", async () => {
    useItemsStore.setState({ items: [plate, sword, potion] });
    await renderApp(<InventoryPanel />);
    for (const id of ["plate", "sword"]) {
      const row = screen.getByTestId(`item-${id}`);
      expect(within(row).queryByTestId("item-count")).toBeNull();
      expect(within(row).queryByTestId("item-behaviour")).toBeNull();
      expect(within(row).queryByRole("button", { name: /^Use / })).toBeNull();
      expect(
        within(row).queryByRole("button", { name: /^Add one / }),
      ).toBeNull();
    }
    // The dice chip of a weapon that already had effect dice is kept.
    expect(
      within(screen.getByTestId("item-sword")).getByTestId("item-dice"),
    ).toBeInTheDocument();
    const row = screen.getByTestId("item-potion");
    expect(within(row).getByTestId("item-count")).toHaveTextContent("2");
    expect(
      within(row).getAllByRole("button", { name: /potion/i }).length,
    ).toBeGreaterThan(0);
  });
});
