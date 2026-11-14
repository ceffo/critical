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
    expect(postUseAbility).toHaveBeenCalledWith("wand", "bolt", undefined);
  });

  // ADR 0049 steps 9 and 10: an ability whose spell effect asks something (a to-hit, one of several
  // items, a mode, a target) opens the Use dialog, from what the server serves per effect; one that asks
  // nothing is used at once with no body, and the dialog sends the cast's own field names.
  describe("the Use dialog of a spell effect", () => {
    const answered = () =>
      postUseAbility.mockResolvedValue({
        itemId: "scroll",
        abilityId: "use",
        name: "Scroll",
        type: "",
        limit: "with_item",
        remaining: 0,
        removed: true,
        effects: [],
        logEntryId: 2,
      });
    const scrollOf = (asks: object) =>
      pieceFixture({
        id: "scroll",
        name: { en: "Scroll" },
        category: "magic",
        magicKind: "scroll",
        count: 1,
        abilities: [
          ability({
            id: "use",
            limit: { kind: "with_item" },
            effects: [
              { kind: "dice", dice: "1d4" },
              { kind: "spell", spellId: "chromatic_orb", level: 13, asks },
            ],
          }),
        ],
      });

    it("answers a to-hit with the target AC, by the effect's index", async () => {
      answered();
      useItemsStore.setState({ items: [scrollOf({ attack_roll: true })] });
      const { user } = await renderApp(<ItemsTab onManage={() => {}} />);
      await user.click(screen.getByRole("button", { name: /^Use / }));
      expect(postUseAbility).not.toHaveBeenCalled();
      const dialog = await screen.findByTestId("use-ability-dialog");
      const confirm = within(dialog).getByTestId("use-ability-confirm");
      expect(confirm).toBeDisabled();
      await user.type(within(dialog).getByTestId("use-ability-ac"), "4");
      expect(confirm).toBeEnabled();
      await user.click(confirm);
      expect(postUseAbility).toHaveBeenCalledWith("scroll", "use", {
        spells: [{ effect: 1, targetAc: 4 }],
      });
    });

    it("offers the items, modes, splits and target the server serves, and sends the picks", async () => {
      answered();
      useItemsStore.setState({
        items: [
          scrollOf({
            consume_item_type: "gem",
            items: [
              { id: "ruby", name: { en: "Ruby" }, count: 2 },
              { id: "opal", name: { en: "Opal" }, count: 1 },
            ],
            modes: [{ key: "ice_plane" }, { key: "hemisphere" }],
            slot_splits: [
              [1, 1, 1],
              [1, 2],
            ],
            cast_target: { default: "self", alternate: "other" },
          }),
        ],
      });
      const { user } = await renderApp(<ItemsTab onManage={() => {}} />);
      await user.click(screen.getByRole("button", { name: /^Use / }));
      const dialog = await screen.findByTestId("use-ability-dialog");
      await user.selectOptions(
        within(dialog).getByTestId("use-ability-item"),
        "opal",
      );
      await user.selectOptions(
        within(dialog).getByTestId("use-ability-mode"),
        "hemisphere",
      );
      await user.selectOptions(
        within(dialog).getByTestId("use-ability-split"),
        "1,2",
      );
      await user.selectOptions(
        within(dialog).getByTestId("use-ability-target"),
        "other",
      );
      await user.click(within(dialog).getByTestId("use-ability-confirm"));
      expect(postUseAbility).toHaveBeenCalledWith("scroll", "use", {
        spells: [
          {
            effect: 1,
            itemId: "opal",
            mode: "hemisphere",
            extraSlots: [1, 2],
            onSelf: false,
          },
        ],
      });
    });

    it("opens no dialog when nothing is left to choose, and sends no body", async () => {
      answered();
      useItemsStore.setState({
        items: [
          scrollOf({
            consume_item_type: "gem",
            items: [{ id: "ruby", name: { en: "Ruby" }, count: 2 }],
            cast_target: { default: "self", alternate: "self" },
          }),
        ],
      });
      const { user } = await renderApp(<ItemsTab onManage={() => {}} />);
      await user.click(screen.getByRole("button", { name: /^Use / }));
      expect(screen.queryByTestId("use-ability-dialog")).toBeNull();
      expect(postUseAbility).toHaveBeenCalledWith("scroll", "use", undefined);
    });

    it("cancelling uses nothing", async () => {
      answered();
      useItemsStore.setState({ items: [scrollOf({ attack_roll: true })] });
      const { user } = await renderApp(<ItemsTab onManage={() => {}} />);
      await user.click(screen.getByRole("button", { name: /^Use / }));
      const dialog = await screen.findByTestId("use-ability-dialog");
      await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
      expect(postUseAbility).not.toHaveBeenCalled();
    });
  });
});
