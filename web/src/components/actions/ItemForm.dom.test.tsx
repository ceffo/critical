import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp, type Language } from "@/test/render";
import { draftFromItem, emptyDraft } from "@/lib/itemForm";
import { pieceFixture } from "@/test/pieceFixture";
import { equipmentRules } from "@/test/equipmentRules";
import { useEquipmentRulesStore } from "@/stores/equipmentRules";
import { ItemForm } from "./ItemForm";

// The typed add form (ADR 0033 step 1): step 1 is what the piece is, step 2 only its category's
// fields; Back keeps what was typed.

async function form(lng: Language = "en") {
  useEquipmentRulesStore.setState({ rules: equipmentRules });
  const onSave = vi.fn().mockResolvedValue(null);
  const view = await renderApp(
    <ItemForm
      initial={emptyDraft()}
      busy={false}
      onSave={onSave}
      onCancel={() => {}}
    />,
    { lng },
  );
  return { ...view, onSave };
}

const field = (name: string) =>
  document.querySelector(`[data-field='${name}']`);

async function startStep2(
  view: Awaited<ReturnType<typeof form>>,
  name: string,
  category: string,
) {
  await view.user.type(screen.getByLabelText("Name"), name);
  await view.user.selectOptions(screen.getByLabelText("Category"), category);
  await view.user.click(screen.getByRole("button", { name: "Next" }));
}

describe("ItemForm, adding in two steps", () => {
  it("step 1 shows the name and the category only", async () => {
    await form();
    expect(field("name")).not.toBeNull();
    expect(field("category")).not.toBeNull();
    expect(field("count")).toBeNull();
    expect(field("baseAc")).toBeNull();
    expect(field("damageSm")).toBeNull();
  });

  it("a weapon shows weapon fields, no armour field", async () => {
    const v = await form();
    await startStep2(v, "Axe", "weapon");
    for (const f of ["damageSm", "weaponType", "magicToHit", "twoHanded"])
      expect(field(f), f).not.toBeNull();
    for (const f of ["baseAc", "shieldKind", "ammoKind"])
      expect(field(f), f).toBeNull();
    expect(field("name")).toBeNull();
  });

  // ADR 0033 step 3: only a bow offers its Strength rating; typed 18/50, it is sent.
  it("a bow offers its Strength rating, another weapon none", async () => {
    const v = await form();
    await startStep2(v, "Yew bow", "weapon");
    expect(field("strengthRating")).toBeNull();
    await v.user.selectOptions(
      screen.getByLabelText("Weapon type"),
      "long_bow",
    );
    expect(field("strengthRating")).not.toBeNull();
    await v.user.selectOptions(
      screen.getByLabelText("Melee or missile"),
      "missile",
    );
    await v.user.type(screen.getByLabelText("Bow's Strength"), "18/50");
    await v.user.click(screen.getByRole("button", { name: "Add" }));
    expect(v.onSave.mock.calls[0][0]).toMatchObject({
      weaponType: "long_bow",
      strengthRating: { score: 18, exceptional: 50 },
    });
    await v.user.selectOptions(
      screen.getByLabelText("Weapon type"),
      "light_crossbow",
    );
    expect(field("strengthRating")).toBeNull();
  });

  it("an armour shows armour fields and its +N, no weapon field", async () => {
    const v = await form();
    await startStep2(v, "Plate", "armour");
    for (const f of ["baseAc", "skillColumn", "armorRow", "plus"])
      expect(field(f), f).not.toBeNull();
    for (const f of ["damageSm", "weaponType", "shieldKind"])
      expect(field(f), f).toBeNull();
  });

  it("a plain item shows its quantity; the shared fields sit under More options", async () => {
    const v = await form();
    await startStep2(v, "Rope", "gear");
    expect(field("count")).not.toBeNull();
    for (const f of ["baseAc", "damageSm", "magicKind", "plus"])
      expect(field(f), f).toBeNull();
    const more = screen.getByTestId("item-more") as HTMLDetailsElement;
    expect(more.open).toBe(false);
    await v.user.click(screen.getByText("More options"));
    for (const f of ["component", "notes"])
      expect(more.contains(field(f)), f).toBe(true);
    // ADR 0049 step 13: the behaviour and the effect dice are gone from the item form.
    for (const f of ["behaviour", "maxCount", "effectDice", "effectSave"])
      expect(field(f), f).toBeNull();
    expect(screen.getByLabelText("Quantity")).toHaveValue("1");
  });

  it("Back keeps what was typed, and Add sends it", async () => {
    const v = await form();
    await startStep2(v, "Chain", "armour");
    await v.user.clear(screen.getByLabelText("Base AC"));
    await v.user.type(screen.getByLabelText("Base AC"), "5");
    await v.user.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByLabelText("Name")).toHaveValue("Chain");
    expect(screen.getByLabelText("Category")).toHaveValue("armour");
    await v.user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByLabelText("Base AC")).toHaveValue("5");
    await v.user.click(screen.getByRole("button", { name: "Add" }));
    expect(v.onSave).toHaveBeenCalledOnce();
    expect(v.onSave.mock.calls[0][0]).toMatchObject({ baseAc: 5 });
  });

  it("Enter in the name moves to step 2 (keyboard flow)", async () => {
    const v = await form();
    await v.user.type(screen.getByLabelText("Name"), "Rope{Enter}");
    expect(field("count")).not.toBeNull();
    expect(screen.getByTestId("item-summary")).toHaveTextContent("Rope");
  });

  it("speaks French", async () => {
    const v = await form("fr");
    await v.user.type(screen.getByLabelText("Nom"), "Corde");
    await v.user.click(screen.getByRole("button", { name: "Suivant" }));
    expect(screen.getByRole("button", { name: "Retour" })).toBeVisible();
    expect(screen.getByText("Plus d'options")).toBeVisible();
  });
});

// ADR 0049 steps 2 and 16: the quantity is shown where the category has one; the abilities are
// for every category.
describe("ItemForm, quantity and abilities", () => {
  it.each(["weapon", "armour", "shield"])(
    "a %s has no quantity field",
    async (category) => {
      const v = await form();
      await startStep2(v, "Piece", category);
      expect(field("count")).toBeNull();
    },
  );

  it.each(["ammunition", "gear"])("%s has a quantity", async (category) => {
    const v = await form();
    await startStep2(v, "Piece", category);
    expect(field("count")).not.toBeNull();
  });

  it("a magic ring has none, a potion has one", async () => {
    const v = await form();
    await startStep2(v, "Thing", "magic");
    await v.user.selectOptions(screen.getByLabelText("Magic kind"), "ring");
    expect(field("count")).toBeNull();
    await v.user.selectOptions(screen.getByLabelText("Magic kind"), "potion");
    expect(field("count")).not.toBeNull();
  });

  it("adds a per-day ability to a sword and sends it", async () => {
    const v = await form();
    await startStep2(v, "Holy sword", "weapon");
    expect(screen.getByText("No ability.")).toBeVisible();
    await v.user.click(
      screen.getByRole("button", { name: "Add an ability to this item" }),
    );
    // A weapon has no quantity, so "With the item" is not offered.
    const kind = screen.getByLabelText("Limit");
    expect(
      Array.from((kind as HTMLSelectElement).options).map((o) => o.value),
    ).toEqual(["unlimited", "per_day", "charges"]);
    await v.user.type(
      screen.getByLabelText("Name", {
        selector: "[data-testid='ability-name']",
      }),
      "Turn zombies",
    );
    const max = screen.getByLabelText("Per day");
    await v.user.clear(max);
    await v.user.type(max, "2");
    // Left follows the max of a new ability.
    expect(screen.getByLabelText("Left")).toHaveValue("2");
    await v.user.click(
      screen.getByRole("button", {
        name: "Roll dice when this ability is used",
      }),
    );
    await v.user.type(screen.getByLabelText("Dice rolled on use"), "2d6");
    await v.user.click(screen.getByRole("button", { name: "Add" }));
    expect(v.onSave.mock.calls[0][0].abilities).toEqual([
      {
        id: "",
        name: { en: "Turn zombies" },
        limit: { kind: "per_day", max: 2, left: 2 },
        effects: [{ kind: "dice", dice: "2d6" }],
      },
    ]);
  });

  // ADR 0049 steps 9 and 12: a potion's heal is dice, or full (the gap to max HP, no dice to type);
  // the save belongs to a dice effect only.
  it.each([
    [
      "en",
      "Add an ability to this item",
      "Regain hit points when this ability is used",
      "Hit points regained",
      "Fill to max HP",
      "Save against the effect",
      "Add",
    ],
    [
      "fr",
      "Ajouter une capacité à cet objet",
      "Regagner des points de vie quand cette capacité est utilisée",
      "Points de vie regagnés",
      "Jusqu'aux PV max",
      "JS contre l'effet",
      "Ajouter",
    ],
  ] as const)(
    "adds a heal to a potion in %s: dice, or full with no dice",
    async (lng, addAbility, addHeal, diceLabel, fullLabel, saveLabel, save) => {
      const v = await form(lng);
      await v.user.type(
        screen.getByLabelText(lng === "en" ? "Name" : "Nom"),
        "Potion of healing",
      );
      await v.user.selectOptions(
        screen.getByLabelText(lng === "en" ? "Category" : "Catégorie"),
        "magic",
      );
      await v.user.click(
        screen.getByRole("button", { name: lng === "en" ? "Next" : "Suivant" }),
      );
      await v.user.selectOptions(
        screen.getByLabelText(
          lng === "en" ? "Magic kind" : "Type d’objet magique",
        ),
        "potion",
      );
      await v.user.click(screen.getByRole("button", { name: addAbility }));
      await v.user.click(screen.getByRole("button", { name: addHeal }));
      expect(screen.queryByLabelText(saveLabel)).toBeNull();
      await v.user.type(screen.getByLabelText(diceLabel), "2d4+2");
      await v.user.click(screen.getByRole("button", { name: save }));
      expect(v.onSave.mock.calls[0][0].abilities).toEqual([
        {
          id: "",
          limit: { kind: "with_item" },
          effects: [{ kind: "heal", dice: "2d4+2" }],
        },
      ]);
      v.onSave.mockClear();
      await v.user.click(screen.getByLabelText(fullLabel));
      expect(screen.getByLabelText(diceLabel)).toBeDisabled();
      await v.user.click(screen.getByRole("button", { name: save }));
      expect(v.onSave.mock.calls[0][0].abilities[0].effects).toEqual([
        { kind: "heal", full: true },
      ]);
    },
  );

  // ADR 0049 steps 9, 10 and 12: a potion's buff is a name, rounds and modifiers in the buffs'
  // effect keys; the rounds are typed, the modifiers rows of key and value (a Strength one too).
  it.each([
    [
      "en",
      "Add an ability to this item",
      "Put a timed effect on the list when this ability is used",
      "Effect name",
      "Rounds",
      "Add",
    ],
    [
      "fr",
      "Ajouter une capacité à cet objet",
      "Poser un effet temporaire sur la liste quand cette capacité est utilisée",
      "Nom de l'effet",
      "Tours",
      "Ajouter",
    ],
  ] as const)(
    "adds a buff to a potion in %s: name, rounds and its modifiers",
    async (lng, addAbility, addBuff, nameLabel, roundsLabel, save) => {
      const v = await form(lng);
      await v.user.type(
        screen.getByLabelText(lng === "en" ? "Name" : "Nom"),
        "Potion of heroism",
      );
      await v.user.selectOptions(
        screen.getByLabelText(lng === "en" ? "Category" : "Catégorie"),
        "magic",
      );
      await v.user.click(
        screen.getByRole("button", { name: lng === "en" ? "Next" : "Suivant" }),
      );
      await v.user.selectOptions(
        screen.getByLabelText(
          lng === "en" ? "Magic kind" : "Type d’objet magique",
        ),
        "potion",
      );
      await v.user.click(screen.getByRole("button", { name: addAbility }));
      await v.user.click(screen.getByRole("button", { name: addBuff }));
      await v.user.type(screen.getByLabelText(nameLabel), "Heroism");
      await v.user.clear(screen.getByLabelText(roundsLabel));
      await v.user.type(screen.getByLabelText(roundsLabel), "60");
      await v.user.type(
        screen.getByTestId("ability-buff-mod-value"),
        "{Backspace}2",
      );
      await v.user.selectOptions(
        screen.getByTestId("ability-buff-mod-key"),
        "to_hit",
      );
      await v.user.click(screen.getByTestId("ability-buff-add-mod"));
      await v.user.selectOptions(
        screen.getAllByTestId("ability-buff-mod-key")[1],
        "str_bonus",
      );
      await v.user.click(screen.getByRole("button", { name: save }));
      expect(v.onSave.mock.calls[0][0].abilities).toEqual([
        {
          id: "",
          limit: { kind: "with_item" },
          effects: [
            {
              kind: "buff",
              name: "Heroism",
              rounds: 60,
              buff: { to_hit: 2, str_bonus: 1 },
            },
          ],
        },
      ]);
      // A rounds that is no whole number is not sent.
      v.onSave.mockClear();
      await v.user.clear(screen.getByLabelText(roundsLabel));
      await v.user.type(screen.getByLabelText(roundsLabel), "a lot");
      await v.user.click(screen.getByRole("button", { name: save }));
      expect(v.onSave).not.toHaveBeenCalled();
    },
  );

  it("offers an ability spent with the item on a stock, and speaks French", async () => {
    const v = await form("fr");
    await v.user.type(screen.getByLabelText("Nom"), "Fiole");
    await v.user.selectOptions(screen.getByLabelText("Catégorie"), "gear");
    await v.user.click(screen.getByRole("button", { name: "Suivant" }));
    expect(screen.getByText("Capacités")).toBeVisible();
    await v.user.click(
      screen.getByRole("button", { name: "Ajouter une capacité à cet objet" }),
    );
    expect(screen.getByLabelText("Limite")).toHaveValue("with_item");
    expect(screen.getByRole("option", { name: "Avec l'objet" })).toBeVisible();
    expect(screen.getByLabelText("Quantité")).toBeVisible();
  });

  // The refusal reads the rules' bound (one source), worded in both languages.
  it.each([
    ["en", "Quantity", "Enter a whole number from 0 to 999"],
    ["fr", "Quantité", "Entrez un nombre entier de 0 à 999"],
  ] as const)("refuses 1000 in %s", async (lng, label, message) => {
    const v = await form(lng);
    await v.user.type(
      screen.getByLabelText(lng === "en" ? "Name" : "Nom"),
      "Rope{Enter}",
    );
    const q = screen.getByLabelText(label);
    await v.user.clear(q);
    await v.user.type(q, "1000");
    await v.user.click(
      screen.getByRole("button", { name: lng === "en" ? "Add" : "Ajouter" }),
    );
    expect(screen.getByTestId("field-error")).toHaveTextContent(message);
    expect(v.onSave).not.toHaveBeenCalled();
  });

  // The rules could not be read: the form says so, and a stored with_item ability keeps its option
  // so a save does not drop it.
  it("with the rules unread, shows an error and keeps a stored with_item ability", async () => {
    useEquipmentRulesStore.setState({ rules: null, failed: false });
    // The rules request fails (the network is down).
    const down = vi
      .spyOn(globalThis, "fetch")
      .mockRejectedValue(new Error("down"));
    const onSave = vi.fn().mockResolvedValue(null);
    const flask = pieceFixture({
      id: "flask",
      name: { en: "Flask" },
      category: "gear",
      abilities: [
        {
          id: "use",
          limit: { kind: "with_item" },
          effects: [],
          usable: true,
        },
      ],
    });
    const v = await renderApp(
      <ItemForm
        initial={draftFromItem(flask, "en")}
        editing={flask}
        busy={false}
        onSave={onSave}
        onCancel={() => {}}
      />,
    );
    expect(await screen.findByTestId("rules-error")).toBeVisible();
    down.mockRestore();
    expect(screen.getByLabelText("Limit")).toHaveValue("with_item");
    await v.user.type(
      screen.getByLabelText("Name", { selector: "[data-field='name'] input" }),
      "s",
    );
    await v.user.click(screen.getByRole("button", { name: "Save" }));
    // The ability is unchanged, so no abilities key is sent and nothing drops it.
    expect(onSave.mock.calls[0][0]).not.toHaveProperty("abilities");
  });
});
