import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import type { ItemPower } from "@/types/buffs";
import type { Item } from "@/types/items";
import {
  itemNamer,
  namedDetail,
  powerEffectsText,
  powerRows,
} from "./itemPowers";

async function makeT(lng: string) {
  const inst = i18next.createInstance();
  await inst.init({
    lng,
    fallbackLng: "en",
    defaultNS: "common",
    resources,
    interpolation: { escapeValue: false },
  });
  return inst.t;
}

const hammerEffects = {
  weapon_buff: 5,
  multiplier: 2,
  weapon_kind: "melee",
} as const;

const gem: Item = {
  id: "power_gem_dispel",
  name: { en: "Power Gem", fr: "Gemme de pouvoir" },
  category: "magic",
  magicKind: "misc",
  catalogueCode: "",
  component: "gem",
  state: "worn",
  equipState: "worn",
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
  baseAc: 0,
  skillColumn: "",
  armorRow: "",
  shieldKind: "",
  acMelee: 0,
  acMissile: 0,
  review: [],
  warnings: [],
  type: "gem",
  behaviour: "daily",
  magicalBonus: 0,
  ammoKind: "",
  count: 0,
  maxCount: 1,
  effectDice: "",
  effectSave: "",
  notes: "",
  powers: [{ id: "thunder_hammer", minStr: 18, effects: hammerEffects }],
};

const hammer: ItemPower = {
  itemId: "power_gem_dispel",
  powerId: "thunder_hammer",
  effects: hammerEffects,
};

// ADR 0016 step 13: the timeline lists the powers the server serves as on (at STR 18), each
// under its item's current name, with nothing to edit or dismiss; at 17 the server serves none and
// the timeline has no row.
describe("powerRows", () => {
  it("lists the hammer at STR 18 under the item's name, with its trigger", () => {
    expect(powerRows([hammer], [gem], "en")).toEqual([
      {
        key: "power_gem_dispel/thunder_hammer",
        itemId: "power_gem_dispel",
        name: "Power Gem",
        minStr: 18,
        effects: hammerEffects,
      },
    ]);
  });

  it("follows the item's name, in the player's language (ADR 0023 step 12)", () => {
    expect(powerRows([hammer], [gem], "fr")[0].name).toBe("Gemme de pouvoir");
    const renamed = {
      ...gem,
      name: { en: "Power Gem", fr: "Gemme de puissance" },
    };
    expect(powerRows([hammer], [renamed], "fr")[0].name).toBe(
      "Gemme de puissance",
    );
  });

  it("has no row at STR 17 (the server lists no power)", () => {
    expect(powerRows([], [gem], "en")).toEqual([]);
  });

  it("offers nothing to edit or dismiss: no buff id, no counters", () => {
    const [row] = powerRows([hammer], [gem], "en");
    for (const key of ["id", "duration", "charges", "applies"])
      expect(row).not.toHaveProperty(key);
  });
});

describe("powerEffectsText", () => {
  it("words the hammer in each language", async () => {
    expect(powerEffectsText(await makeT("en"), hammerEffects)).toBe(
      "weapon +5 · dice ×2",
    );
    expect(powerEffectsText(await makeT("fr"), hammerEffects)).toBe(
      "arme +5 · dés ×2",
    );
  });

  it("words the other effects with their sign", async () => {
    expect(
      powerEffectsText(await makeT("en"), {
        to_hit: 1,
        ac: -2,
        weapon_kind: "missile",
      }),
    ).toBe("To hit +1 · AC −2");
  });
});

describe("namedDetail", () => {
  it("names from the item by id, else the stored detail", () => {
    const name = itemNamer([gem], "en");
    expect(
      namedDetail({ detail: "Old", itemId: "power_gem_dispel" }, name),
    ).toBe("Power Gem");
    expect(namedDetail({ detail: "Old", itemId: "gone" }, name)).toBe("Old");
    expect(namedDetail({ detail: "Bless" }, name)).toBe("Bless");
  });
});
