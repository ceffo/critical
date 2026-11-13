import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { pieceFixture } from "@/test/pieceFixture";
import { useItemsStore } from "@/stores/items";
import type { ItemAbility } from "@/types/items";
import { ItemsTab } from "./ItemsTab";

const postUseAbility = vi.hoisted(() => vi.fn());
vi.mock("@/api/items", async (orig) => ({
  ...(await orig<typeof import("@/api/items")>()),
  postUseAbility,
}));

afterEach(() => {
  useItemsStore.setState({ items: [] });
  postUseAbility.mockReset();
});

const ability = (a: Partial<ItemAbility> & Pick<ItemAbility, "limit">) => ({
  id: "ability_1",
  effects: [],
  usable: true,
  ...a,
});

const plate = pieceFixture({
  id: "plate",
  name: { en: "Plate mail" },
  category: "armour",
  usable: false,
  abilities: [],
});
const sword = pieceFixture({
  id: "sword",
  name: { en: "Longsword" },
  category: "weapon",
  usable: false,
  abilities: [],
});
const rope = pieceFixture({
  id: "rope",
  name: { en: "Rope" },
  abilities: [],
});
const arrows = pieceFixture({
  id: "arrows",
  name: { en: "Arrows" },
  category: "ammunition",
  ammoKind: "arrow",
  // Whatever it holds, ammunition is fired from the Weapons tab.
  abilities: [ability({ limit: { kind: "unlimited" } })],
});
const flasks = pieceFixture({
  id: "flasks",
  name: { en: "Fire flasks" },
  count: 3,
  abilities: [ability({ id: "throw", limit: { kind: "with_item" } })],
});
const wand = pieceFixture({
  id: "wand",
  name: { en: "Wand of lightning" },
  category: "magic",
  magicKind: "wand",
  equipState: "carried",
  state: "carried",
  abilities: [
    ability({
      id: "bolt",
      name: { en: "Lightning bolt" },
      limit: { kind: "charges", max: 10, left: 3, cost: 1 },
    }),
  ],
});
const gem = pieceFixture({
  id: "gem",
  name: { en: "Power Gem" },
  abilities: [
    ability({
      limit: { kind: "per_day", max: 2, left: 0 },
      usable: false,
      reason: "exhausted",
    }),
  ],
});
const ring = pieceFixture({
  id: "ring",
  name: { en: "Ring of light" },
  category: "magic",
  magicKind: "ring",
  equipState: "worn",
  state: "worn",
  abilities: [ability({ limit: { kind: "unlimited" } })],
});

const rows = () => screen.getAllByTestId(/^ability-.*-/, { exact: false });

describe("ItemsTab lists abilities (ADR 0049 step 14)", () => {
  it("ADR 0049 step 14: one row per ability in item list order, never an item with none or ammunition", async () => {
    useItemsStore.setState({
      items: [plate, sword, rope, arrows, flasks, wand, gem, ring],
    });
    await renderApp(<ItemsTab onManage={() => {}} />);
    const list = screen.getByRole("list", { name: "Item abilities" });
    expect(
      within(list)
        .getAllByRole("listitem")
        .map((li) => li.getAttribute("data-testid")),
    ).toEqual([
      "ability-flasks-throw",
      "ability-wand-bolt",
      "ability-gem-ability_1",
      "ability-ring-ability_1",
    ]);
    expect(rows()).toHaveLength(4);
  });

  it("ADR 0049 step 14: shows the item and ability names and the limit as the player reads it", async () => {
    useItemsStore.setState({ items: [flasks, wand, gem, ring] });
    await renderApp(<ItemsTab onManage={() => {}} />);
    const limit = (id: string) =>
      within(screen.getByTestId(id)).getByTestId("ability-limit");
    expect(limit("ability-flasks-throw")).toHaveTextContent("× 3");
    expect(limit("ability-wand-bolt")).toHaveTextContent("3 / 10 charges");
    expect(limit("ability-gem-ability_1")).toHaveTextContent("0 / 2 per day");
    expect(limit("ability-ring-ability_1")).toHaveTextContent("∞");
    expect(
      screen.getByText(/Wand of lightning · Lightning bolt/),
    ).toBeVisible();
  });

  it("ADR 0049 step 14: an exhausted ability stays listed, its button disabled, the reason as text", async () => {
    useItemsStore.setState({ items: [gem] });
    await renderApp(<ItemsTab onManage={() => {}} />);
    const row = screen.getByTestId("ability-gem-ability_1");
    expect(within(row).getByRole("button", { name: /^Use / })).toBeDisabled();
    expect(within(row).getByTestId("ability-reason")).toHaveTextContent(
      "Used up until the long rest",
    );
  });

  it("ADR 0049 step 14: the reason is worded in French", async () => {
    useItemsStore.setState({ items: [gem] });
    await renderApp(<ItemsTab onManage={() => {}} />, { lng: "fr" });
    expect(screen.getByTestId("ability-reason")).toHaveTextContent(
      "Épuisé jusqu'au repos long",
    );
    expect(screen.getByTestId("ability-limit")).toHaveTextContent(
      "0 / 2 par jour",
    );
  });

  it("ADR 0049 step 14: the web does not decide: a usable ability with nothing left is enabled", async () => {
    const odd = pieceFixture({
      id: "odd",
      name: { en: "Odd" },
      abilities: [
        ability({ limit: { kind: "charges", max: 5, left: 0 }, usable: true }),
      ],
    });
    useItemsStore.setState({ items: [odd] });
    await renderApp(<ItemsTab onManage={() => {}} />);
    expect(screen.getByRole("button", { name: /^Use / })).toBeEnabled();
  });

  it.each([
    ["en", /Add abilities to your items in the Inventory/],
    ["fr", /Ajoutez des capacités à vos objets dans l'Inventaire/],
  ] as const)(
    "ADR 0049 step 14: the empty state points to the Inventory in %s",
    async (lng, re) => {
      useItemsStore.setState({ items: [plate, sword, rope] });
      await renderApp(<ItemsTab onManage={() => {}} />, { lng });
      expect(screen.getByText(re)).toBeInTheDocument();
      expect(screen.queryByRole("list")).toBeNull();
    },
  );

  it("ADR 0049 step 17: an unequipped item is tagged and its Use stays enabled; an equipped one has no tag", async () => {
    useItemsStore.setState({ items: [wand, ring] });
    await renderApp(<ItemsTab onManage={() => {}} />);
    const stowed = screen.getByTestId("ability-wand-bolt");
    expect(within(stowed).getByTestId("ability-stowed")).toHaveTextContent(
      "not equipped",
    );
    expect(within(stowed).getByRole("button", { name: /^Use / })).toBeEnabled();
    expect(
      within(screen.getByTestId("ability-ring-ability_1")).queryByTestId(
        "ability-stowed",
      ),
    ).toBeNull();
  });

  it("ADR 0049 step 14: Use calls the ability's endpoint with the item and ability ids", async () => {
    postUseAbility.mockResolvedValue({
      itemId: "wand",
      abilityId: "bolt",
      name: "Wand of lightning",
      type: "wand",
      limit: "charges",
      left: 2,
      max: 10,
      remaining: 2,
      removed: false,
      effects: [],
      logEntryId: 1,
    });
    useItemsStore.setState({ items: [wand] });
    const { user } = await renderApp(<ItemsTab onManage={() => {}} />);
    await user.click(screen.getByRole("button", { name: /^Use / }));
    expect(postUseAbility).toHaveBeenCalledWith("wand", "bolt");
  });
});
