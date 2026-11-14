import { describe, expect, it } from "vitest";
import type { Item, ItemAbility } from "@/types/items";
import {
  changedFields,
  draftFromItem,
  type ItemDraft,
  draftToBody,
  emptyDraft,
  hasField,
  hasQuantity,
  parseInteger,
  parseStrengthRating,
  ratesStrength,
} from "./itemForm";
import { equipmentRules } from "@/test/equipmentRules";
import { newAbilityDraft, newEffectDraft } from "./abilityEditor";

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
    usable: true,
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
    strengthRating: null,
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
      count: 6,
      magicalBonus: 3,
      ammoKind: "arrow",
      damageSm: "1d6",
      damageL: "1d6",
      damageTypes: [],
      notes: "",
      powers: [],
      abilities: [],
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

  // ADR 0049 steps 13 and 16: the item sends no behaviour, daily number or effect dice any more.
  it("sends none of the old behaviour fields (ADR 0049 step 13)", () => {
    const { body } = draftToBody(emptyDraft(), "en");
    for (const k of ["behaviour", "maxCount", "effectDice", "effectSave"])
      expect(body, k).not.toHaveProperty(k);
  });

  // ADR 0049 step 2: the quantity is shown where the category has one.
  it("has none before the rules are read", () => {
    expect(hasQuantity(null, { category: "gear", magicKind: "" })).toBe(false);
  });

  it.each([
    ["ammunition", "", true],
    ["gear", "", true],
    ["magic", "potion", true],
    ["magic", "scroll", true],
    ["magic", "ring", false],
    ["magic", "wand", false],
    ["weapon", "", false],
    ["armour", "", false],
    ["shield", "", false],
  ] as const)("has a quantity: %s %s is %s", (category, magicKind, want) => {
    expect(hasQuantity(equipmentRules, { category, magicKind })).toBe(want);
  });

  // The server bounds a quantity to 0-999 (ADR 0049 step 4).
  it("flags a quantity above the server's 999", () => {
    expect(
      draftToBody({ ...emptyDraft(), count: "1000" }, "en", 999).errors,
    ).toEqual({ count: "notAQuantity" });
    expect(
      draftToBody({ ...emptyDraft(), count: "999" }, "en", 999).errors,
    ).toEqual({});
  });

  it("flags text that is not a number instead of sending NaN; a blank bonus is 0; another category's field is not read", () => {
    const { errors } = draftToBody(
      {
        ...emptyDraft("ammunition"),
        count: "six",
        baseAc: "x",
      },
      "en",
    );
    expect(errors).toEqual({ count: "notANumber" });
    const blank = draftToBody(
      { ...emptyDraft("ammunition"), magicalBonus: " " },
      "en",
    );
    expect(blank.errors).toEqual({});
    expect(blank.body.magicalBonus).toBe(0);
  });

  it("leaves a negative count to the server", () => {
    const { body, errors } = draftToBody(
      {
        ...emptyDraft(),
        count: "-1",
      },
      "en",
    );
    expect(errors).toEqual({});
    expect(body.count).toBe(-1);
  });
});

describe("changedFields", () => {
  // ADR 0049 step 9: a scroll's spell is the spell effect of its ability; the form sends the abilities
  // (the server mirrors the old spell column) and never a spellId of its own.
  it("sends a scroll's spell with its abilities, never as a spellId", () => {
    const scroll = piece({
      id: "s",
      category: "magic",
      magicKind: "scroll",
      abilities: [
        {
          id: "use",
          limit: { kind: "with_item" },
          effects: [],
          usable: true,
        },
      ],
    });
    const draft = draftFromItem(scroll, "en");
    draft.abilities[0].effects = [
      { ...newEffectDraft("spell"), spellId: "burning_hands", level: "13" },
    ];
    const { body } = draftToBody(draft, "en");
    expect(body.spellId).toBeUndefined();
    expect(changedFields(scroll, body, "en")).toEqual({
      abilities: [
        {
          id: "use",
          limit: { kind: "with_item" },
          effects: [{ kind: "spell", spellId: "burning_hands", level: 13 }],
        },
      ],
    });
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

  // ADR 0049 steps 4 and 16: the abilities go as a whole list; an untouched form sends none, and
  // an unconverted piece (no list) edited with none added stays unconverted.
  describe("abilities", () => {
    const sword2 = piece({
      id: "sw",
      category: "weapon",
      weaponKind: "melee",
    });
    const perDay: ItemAbility = {
      id: "use",
      name: { en: "Turn zombies", fr: "Renvoyer les zombies" },
      note: "holy",
      limit: { kind: "per_day", max: 2, left: 1 },
      effects: [{ kind: "dice", dice: "2d6", save: "spell" }],
      usable: true,
    };
    const sent = (item: Item, mutate: (d: ItemDraft) => ItemDraft) =>
      changedFields(
        item,
        draftToBody(mutate(draftFromItem(item, "en")), "en").body,
        "en",
      );

    it("an untouched form sends no abilities, converted or not", () => {
      expect(sent(sword2, (d) => d)).toEqual({});
      expect(sent({ ...sword2, abilities: [perDay] }, (d) => d)).toEqual({});
    });

    it("adds a per-day ability: the limit numbers, the note, the dice", () => {
      const out = sent(sword2, (d) => ({
        ...d,
        abilities: [
          {
            ...newAbilityDraft("per_day"),
            name: "Turn zombies",
            max: "2",
            left: "2",
            note: "holy",
            effects: [
              { ...newEffectDraft("dice"), dice: "2d6", save: "spell" },
            ],
          },
        ],
      }));
      expect(out.abilities).toEqual([
        {
          id: "",
          name: { en: "Turn zombies" },
          note: "holy",
          limit: { kind: "per_day", max: 2, left: 2 },
          effects: [{ kind: "dice", dice: "2d6", save: "spell" }],
        },
      ]);
    });

    it("adds a potion's heal: dice, or full with no dice; a blank heal is not sent", () => {
      const heal = (patch: Partial<ReturnType<typeof newEffectDraft>>) =>
        sent(sword2, (d) => ({
          ...d,
          abilities: [
            {
              ...newAbilityDraft("with_item"),
              effects: [{ ...newEffectDraft("heal"), ...patch }],
            },
          ],
        })).abilities?.[0].effects;
      expect(heal({ dice: "2d4+2" })).toEqual([
        { kind: "heal", dice: "2d4+2" },
      ]);
      expect(heal({ full: true, dice: "left over" })).toEqual([
        { kind: "heal", full: true },
      ]);
      expect(heal({})).toEqual([]);
    });

    it("an edit keeps the other languages of the name and never sends the served flags", () => {
      const item = { ...sword2, abilities: [perDay] };
      const out = sent(item, (d) => ({
        ...d,
        abilities: d.abilities.map((a) => ({ ...a, name: "Turn undead" })),
      }));
      expect(out.abilities?.[0].name).toEqual({
        en: "Turn undead",
        fr: "Renvoyer les zombies",
      });
      expect(out.abilities?.[0]).not.toHaveProperty("usable");
    });

    it("removing the last ability sends an empty list", () => {
      const item = { ...sword2, abilities: [perDay] };
      expect(sent(item, (d) => ({ ...d, abilities: [] })).abilities).toEqual(
        [],
      );
    });

    it("a charges limit sends its cost; unlimited and with_item send no numbers", () => {
      const out = draftToBody(
        {
          ...emptyDraft(),
          abilities: [
            { ...newAbilityDraft("charges"), max: "10", left: "7", cost: "2" },
            newAbilityDraft("unlimited"),
            newAbilityDraft("with_item"),
          ],
        },
        "en",
      ).body.abilities;
      expect(out?.map((a) => a.limit)).toEqual([
        { kind: "charges", max: 10, left: 7, cost: 2 },
        { kind: "unlimited" },
        { kind: "with_item" },
      ]);
    });

    it("flags a limit number that is not a number", () => {
      const { errors } = draftToBody(
        {
          ...emptyDraft(),
          abilities: [{ ...newAbilityDraft("per_day"), max: "two" }],
        },
        "en",
      );
      expect(errors).toEqual({ abilities: "notANumber" });
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

  // ADR 0033 step 3: a bow's Strength rating is typed as 17, 18/50 or 18/00, set and cleared.
  it("sends a bow's Strength rating when it is set or cleared, and none for another weapon", () => {
    const bow = piece({
      id: "b",
      category: "weapon",
      weaponType: "long_bow",
      weaponKind: "missile",
      fires: "arrow",
    });
    const set = (b: Item, text: string) =>
      changedFields(
        b,
        draftToBody({ ...draftFromItem(b, "en"), strengthRating: text }, "en")
          .body,
        "en",
      );
    expect(set(bow, "18/50")).toEqual({
      strengthRating: { score: 18, exceptional: 50 },
    });
    expect(set(bow, " 17 ")).toEqual({
      strengthRating: { score: 17, exceptional: 0 },
    });
    const rated = { ...bow, strengthRating: { score: 18, exceptional: 100 } };
    expect(draftFromItem(rated, "en").strengthRating).toBe("18/00");
    expect(set(rated, "18/00")).toEqual({});
    expect(set(rated, "")).toEqual({ strengthRating: null });
    // A bow retyped as a crossbow sends no rating any more: the server drops it.
    const crossbow = draftToBody(
      {
        ...draftFromItem(rated, "en"),
        weaponType: "light_crossbow",
      },
      "en",
    ).body;
    expect(crossbow.strengthRating).toBeNull();
    expect(ratesStrength({ category: "weapon", weaponType: "short_bow" })).toBe(
      true,
    );
    expect(
      ratesStrength({ category: "weapon", weaponType: "light_crossbow" }),
    ).toBe(false);
    expect(ratesStrength({ category: "weapon", weaponType: "" })).toBe(false);
  });

  it("flags a rating that is not a Strength instead of sending it", () => {
    const bow = piece({
      category: "weapon",
      weaponType: "short_bow",
      weaponKind: "missile",
    });
    const { errors } = draftToBody(
      { ...draftFromItem(bow, "en"), strengthRating: "eighteen" },
      "en",
    );
    expect(errors).toEqual({ strengthRating: "notAStrength" });
    expect(parseStrengthRating("18/05")).toEqual({ score: 18, exceptional: 5 });
    expect(parseStrengthRating("18/5")).toBeUndefined();
    expect(parseStrengthRating("")).toBeNull();
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
