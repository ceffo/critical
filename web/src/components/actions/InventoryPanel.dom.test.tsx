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
// A weapon that kept an old count above 1: shown as stored, no controls.
const sword = pieceFixture({
  id: "sword",
  name: { en: "Longsword" },
  category: "weapon",
  usable: false,
  count: 3,
  abilities: [
    {
      id: "turn",
      name: { en: "Turn zombies" },
      limit: { kind: "per_day", max: 2, left: 1 },
      effects: [{ kind: "dice", dice: "1d8" }],
      usable: true,
    },
  ],
});
const potion = pieceFixture({
  id: "potion",
  name: { en: "Potion of healing" },
  category: "magic",
  magicKind: "potion",
  count: 2,
});

// ADR 0049 steps 2 and 16: the server says which pieces have a quantity the player moves; the row
// has no Use, and shows the abilities as chips with their limit.
describe("InventoryPanel rows (quantity by category, abilities as chips)", () => {
  it("has no spend or add on a weapon or an armour, but on a potion; no Use anywhere", async () => {
    useItemsStore.setState({ items: [plate, sword, potion] });
    await renderApp(<InventoryPanel />);
    for (const id of ["plate", "sword"]) {
      const row = screen.getByTestId(`item-${id}`);
      expect(
        within(row).queryByRole("button", {
          name: /^(Use|Spend one|Add one) /,
        }),
      ).toBeNull();
    }
    // A stored count above 1 on a one-object piece is shown, not editable.
    expect(
      within(screen.getByTestId("item-sword")).getByTestId("item-count"),
    ).toHaveTextContent("× 3");
    expect(
      within(screen.getByTestId("item-plate")).queryByTestId("item-count"),
    ).toBeNull();
    const row = screen.getByTestId("item-potion");
    expect(within(row).getByTestId("item-count")).toHaveTextContent("2");
    expect(
      within(row).getByRole("button", { name: "Spend one Potion of healing" }),
    ).toBeEnabled();
    expect(
      within(row).getByRole("button", { name: "Add one Potion of healing" }),
    ).toBeEnabled();
    expect(screen.queryByRole("button", { name: /^Use / })).toBeNull();
  });

  it("shows the ability as a chip with its limit and its dice", async () => {
    useItemsStore.setState({ items: [sword] });
    await renderApp(<InventoryPanel />);
    const chip = screen.getByTestId("ability-chip-turn");
    expect(chip).toHaveTextContent("Turn zombies");
    expect(within(chip).getByTestId("ability-chip-limit")).toHaveTextContent(
      "1 / 2 per day",
    );
    expect(within(chip).getByTestId("item-dice")).toHaveTextContent("1d8");
  });

  it("speaks French", async () => {
    useItemsStore.setState({ items: [sword] });
    await renderApp(<InventoryPanel />, { lng: "fr" });
    expect(screen.getByTestId("ability-chip-limit")).toHaveTextContent(
      "1 / 2 par jour",
    );
  });

  // ADR 0049 step 21.3: the amber band counts the pieces whose `review` codes the server serves
  // (slice 2's), and Check words each reason.
  it("the review band reads the served review codes", async () => {
    const wand = pieceFixture({
      id: "wand",
      name: { en: "Wand of lightning" },
      category: "magic",
      magicKind: "wand",
      usable: false,
      review: ["charges_total", "limit_clamped"],
    });
    useItemsStore.setState({ items: [wand, potion] });
    const v = await renderApp(<InventoryPanel />);
    expect(screen.getByTestId("review-band")).toHaveTextContent("1");
    await v.user.click(screen.getByRole("button", { name: /Check/ }));
    const list = screen.getByTestId("inventory-review-list");
    expect(list).toHaveTextContent("Only the charges left were known");
    expect(list).toHaveTextContent("brought down");
  });
});
