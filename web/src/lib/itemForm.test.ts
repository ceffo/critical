import { describe, expect, it } from "vitest";
import type { Item } from "@/types/items";
import {
  changedFields,
  draftFromItem,
  draftToBody,
  emptyDraft,
  hasField,
  parseInteger,
} from "./itemForm";

/** A piece as GET /api/items serves it: every category's fields, zero for the others. */
function piece(p: Partial<Item>): Item {
  return {
    id: "x",
    name: { en: "X" },
    category: "gear",
    magicKind: "",
    catalogueCode: "",
    component: "",
    spellId: "",
    state: "carried",
    equipState: "",
    type: "",
    behaviour: "stock",
    magicalBonus: 0,
    ammoKind: "",
    count: 1,
    maxCount: 1,
    effectDice: "",
    effectSave: "",
    notes: "",
    powers: [],
    damageTypes: [],
    damageSm: "",
    damageL: "",
    weaponType: "",
    weaponKind: "",
    size: "",
    speed: 0,
    magicToHit: 0,
    magicDamage: 0,
    twoHanded: false,
    fires: "",
    specialized: false,
    nonproficientPenalty: 0,
    baseAc: 0,
    skillColumn: "",
    armorRow: "",
    shieldKind: "",
    acMelee: 0,
    acMissile: 0,
    review: [],
    warnings: [],
    ...p,
  };
}

const wand = piece({
  id: "wand_lightning",
  name: { en: "Wand of lightning", fr: "Baguette de foudre" },
  category: "magic",
  magicKind: "wand",
  type: "wand",
  behaviour: "charges",
  count: 4,
  maxCount: 4,
  effectDice: "6d6",
  effectSave: "rod_staff_wand",
});

const sword = piece({
  id: "longsword",
  name: { en: "Long Sword +2" },
  category: "weapon",
  state: "wielded",
  equipState: "wielded",
  damageSm: "1d8",
  damageL: "1d12",
  damageTypes: ["s"],
  weaponType: "long_sword",
  weaponKind: "melee",
  size: "m",
  speed: 5,
  magicToHit: 2,
  magicDamage: 2,
});

describe("parseInteger", () => {
  it("accepts whole numbers only", () => {
    expect(parseInteger("6")).toBe(6);
    expect(parseInteger(" -3 ")).toBe(-3);
    expect(parseInteger("+2")).toBe(2);
    for (const bad of ["", "x", "1.5", "1e3", "2x6"])
      expect(parseInteger(bad)).toBeNaN();
  });
});

describe("draftToBody", () => {
  it("builds the add body for Arrows +3: the shared fields and ammunition's own (ADR 0033 step 4)", () => {
    const { body, errors } = draftToBody(
      {
        ...emptyDraft("ammunition"),
        name: "Arrows +3",
        count: "6",
        magicalBonus: "3",
        ammoKind: "arrow",
        damageSm: "1d6",
        damageL: "1d6",
      },
      "en",
    );
    expect(errors).toEqual({});
    expect(body).toEqual({
      name: { en: "Arrows +3" },
      category: "ammunition",
      component: "",
      behaviour: "stock",
      count: 6,
      magicalBonus: 3,
      ammoKind: "arrow",
      damageSm: "1d6",
      damageL: "1d6",
      damageTypes: [],
      effectDice: "",
      effectSave: "",
      notes: "",
      powers: [],
    });
  });

  it("sends a weapon's fields, and fires nothing for a melee weapon (ADR 0033 step 3)", () => {
    const { body } = draftToBody(
      { ...draftFromItem(sword, "en"), fires: "arrow" },
      "en",
    );
    expect(body).toMatchObject({
      category: "weapon",
      weaponKind: "melee",
      fires: "",
      speed: 5,
      damageTypes: ["s"],
    });
    expect(body).not.toHaveProperty("ammoKind");
    expect(body).not.toHaveProperty("baseAc");
  });

  it("shows each category's fields only", () => {
    expect(hasField("armour", "baseAc")).toBe(true);
    expect(hasField("armour", "speed")).toBe(false);
    expect(hasField("shield", "acMissile")).toBe(true);
    expect(hasField("magic", "magicKind")).toBe(true);
    expect(hasField("gear", "magicKind")).toBe(false);
  });

  it("sends the daily number for daily items only", () => {
    expect(
      draftToBody({ ...emptyDraft(), behaviour: "daily", maxCount: "2" }, "en")
        .body.maxCount,
    ).toBe(2);
    expect(
      draftToBody({ ...emptyDraft(), behaviour: "stock", maxCount: "2" }, "en")
        .body,
    ).not.toHaveProperty("maxCount");
  });

  it("flags text that is not a number instead of sending NaN; a blank bonus is 0; another category's field is not read", () => {
    const { errors } = draftToBody(
      {
        ...emptyDraft("ammunition"),
        count: "six",
        behaviour: "daily",
        maxCount: "",
        baseAc: "x",
      },
      "en",
    );
    expect(errors).toEqual({ count: "notANumber", maxCount: "notANumber" });
    const blank = draftToBody(
      { ...emptyDraft("ammunition"), magicalBonus: " " },
      "en",
    );
    expect(blank.errors).toEqual({});
    expect(blank.body.magicalBonus).toBe(0);
  });

  it("leaves a negative count and bad dice to the server", () => {
    const { body, errors } = draftToBody(
      {
        ...emptyDraft(),
        count: "-1",
        effectDice: "2x6",
      },
      "en",
    );
    expect(errors).toEqual({});
    expect(body.count).toBe(-1);
    expect(body.effectDice).toBe("2x6");
  });
});

describe("changedFields", () => {
  // ADR 0033 step 2: a magic scroll names the spell it holds; the form sends it, sets and clears it.
  it("sends a scroll's spell when it is set or cleared, and none for another piece", () => {
    const scroll = piece({ id: "s", category: "magic", magicKind: "scroll" });
    const named = draftToBody(
      { ...draftFromItem(scroll, "en"), spellId: "burning_hands" },
      "en",
    ).body;
    expect(changedFields(scroll, named, "en")).toEqual({
      spellId: "burning_hands",
    });
    const held = { ...scroll, spellId: "burning_hands" };
    const cleared = draftToBody(
      { ...draftFromItem(held, "en"), spellId: "" },
      "en",
    ).body;
    expect(changedFields(held, cleared, "en")).toEqual({ spellId: "" });
    const potion = draftToBody(
      { ...draftFromItem(held, "en"), magicKind: "potion" },
      "en",
    ).body;
    expect(potion.spellId).toBeUndefined();
  });

  it("sends only what changed: the wand's charges 4 to 3", () => {
    const { body } = draftToBody(
      { ...draftFromItem(wand, "en"), count: "3" },
      "en",
    );
    expect(changedFields(wand, body, "en")).toEqual({ count: 3 });
  });

  it("sends a weapon's damage types when they change", () => {
    const { body } = draftToBody(
      { ...draftFromItem(sword, "en"), damageTypes: ["p", "s"] },
      "en",
    );
    expect(changedFields(sword, body, "en")).toEqual({
      damageTypes: ["p", "s"],
    });
  });

  it("sends the daily number when an item becomes daily, even if equal to its old max", () => {
    const stock: Item = { ...wand, behaviour: "stock", count: 4, maxCount: 4 };
    const { body } = draftToBody(
      {
        ...draftFromItem(stock, "en"),
        behaviour: "daily",
        maxCount: "4",
      },
      "en",
    );
    expect(changedFields(stock, body, "en")).toEqual({
      behaviour: "daily",
      maxCount: 4,
    });
  });

  it("sends the spell component, whatever the category (ADR 0033 step 1.1)", () => {
    const { body } = draftToBody(
      { ...draftFromItem(wand, "en"), component: "gem" },
      "en",
    );
    expect(changedFields(wand, body, "en")).toEqual({ component: "gem" });
    expect(draftFromItem({ ...wand, component: "gem" }, "en").component).toBe(
      "gem",
    );
  });

  it("is empty when nothing changed", () => {
    for (const p of [wand, sword])
      expect(
        changedFields(p, draftToBody(draftFromItem(p, "en"), "en").body, "en"),
      ).toEqual({});
  });
});

// ADR 0023 step 11: the form edits the name in the UI language and sends that language only.
describe("names in the player's language", () => {
  it("shows and sends the French name in French", () => {
    const d = draftFromItem(wand, "fr-CA");
    expect(d.name).toBe("Baguette de foudre");
    const { body } = draftToBody({ ...d, name: "Baguette d'éclairs" }, "fr-CA");
    expect(changedFields(wand, body, "fr-CA")).toEqual({
      name: { fr: "Baguette d'éclairs" },
    });
  });

  it("never writes the English fallback into French when the name is untouched", () => {
    const english: Item = { ...wand, name: { en: "Wand of lightning" } };
    const d = draftFromItem(english, "fr");
    expect(d.name).toBe("Wand of lightning");
    expect(changedFields(english, draftToBody(d, "fr").body, "fr")).toEqual({});
  });

  it("creates an item in the UI language", () => {
    expect(
      draftToBody({ ...emptyDraft(), name: "Corde" }, "fr").body.name,
    ).toEqual({
      fr: "Corde",
    });
  });
});
