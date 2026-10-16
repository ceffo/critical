import { describe, expect, it } from "vitest";
import type { DamageTerm, RollResult } from "@/types/combat";
import type { UseResult } from "@/types/items";
import i18next from "i18next";
import { resources } from "@/i18n";
import { damageFormula, itemRollText, modLabel } from "./rollText";

const hit = (over: Partial<RollResult>): RollResult =>
  ({
    baseDamage: 5,
    spellsBuff: 1,
    weaponBuff: 2,
    multipliers: 1,
    diceMultiplier: 1,
    strengthBonus: 0,
    bonusDamage: 0,
    totalDamage: 7,
    unroundedDamage: 7,
    ...over,
  }) as RollResult;

describe("damageFormula", () => {
  it.each([
    [{}, "((5 × 1.0) + 2) × 1.0 + 0 = 7"],
    [
      { multipliers: 1.5, totalDamage: 10, unroundedDamage: 10.5 },
      "((5 × 1.0) + 2) × 1.5 + 0 = 10.5 → 10",
    ],
    [
      {
        spellsBuff: 1.5,
        diceMultiplier: 2,
        multipliers: 2,
        strengthBonus: 2,
        bonusDamage: 1,
        totalDamage: 37,
        unroundedDamage: 37,
      },
      "((5 × 1.5 × 2.0) + 2) × 2.0 + 2 + 1 = 37",
    ],
    [
      {
        multipliers: 1.5,
        bonusDamage: -20,
        totalDamage: 0,
        unroundedDamage: -9.5,
      },
      "((5 × 1.0) + 2) × 1.5 + 0 − 20 = -9.5 → 0",
    ],
    [{ unroundedDamage: undefined }, "((5 × 1.0) + 2) × 1.0 + 0 = 7"],
  ])("%o", (over, want) => expect(damageFormula(hit(over), "en")).toBe(want));

  it("uses the language's decimal separator", () => {
    expect(
      damageFormula(
        hit({ multipliers: 1.5, totalDamage: 10, unroundedDamage: 10.5 }),
        "fr",
      ),
    ).toBe("((5 × 1,0) + 2) × 1,5 + 0 = 10,5 → 10");
  });
});

describe("itemRollText", () => {
  it("lists the dice, the flat modifier and the total", () => {
    expect(
      itemRollText({
        dice: "6d6",
        rolls: [1, 2, 3, 4, 5, 6],
        modifier: 0,
        total: 21,
      } as UseResult),
    ).toBe("6d6: 1 + 2 + 3 + 4 + 5 + 6 = 21");
    expect(
      itemRollText({
        dice: "1d8+2",
        rolls: [5],
        modifier: 2,
        total: 7,
      } as UseResult),
    ).toBe("1d8+2: 5 + 2 = 7");
  });
});

describe("damage formula in French, clamped and floored", () => {
  it.each([
    [
      {
        multipliers: 1.5,
        diceMultiplier: 3,
        totalDamage: 25,
        unroundedDamage: 25.5,
      },
      "((5 × 1,0 × 3,0) + 2) × 1,5 + 0 = 25,5 → 25",
    ],
    [
      {
        multipliers: 1.5,
        bonusDamage: -20,
        totalDamage: 0,
        unroundedDamage: -9.5,
      },
      "((5 × 1,0) + 2) × 1,5 + 0 − 20 = -9,5 → 0",
    ],
  ])("%o", (over, want) => expect(damageFormula(hit(over), "fr")).toBe(want));
});

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

describe("modLabel", () => {
  it("labels the crit term with the roll's range, or the placeholder", async () => {
    for (const lng of ["en", "fr"]) {
      const t = await makeT(lng);
      const crit = { kind: "crit", value: 3 };
      expect(
        modLabel(t, crit, { critPlaceholder: false, critFrom: 51, critTo: 90 }),
      ).toMatch(/\(51–90\)$/);
      expect(
        modLabel(t, crit, { critPlaceholder: true, critFrom: 1, critTo: 100 }),
      ).toBe(t("result.mods.critPlaceholder"));
      expect(modLabel(t, crit)).toBe(t("result.mods.critPlaceholder"));
    }
    const en = await makeT("en");
    expect(
      modLabel(en, { kind: "crit", value: 3 }, { critFrom: 51, critTo: 90 }),
    ).toBe("Critical hit (51–90)");
    expect(
      modLabel(
        await makeT("fr"),
        { kind: "crit", value: 2 },
        { critPlaceholder: true },
      ),
    ).toBe("Coup critique (provisoire)");
  });

  it("labels the other kinds from their detail; an unnamed situational one by its kind", async () => {
    const en = await makeT("en");
    expect(modLabel(en, { kind: "strength", detail: "18", value: 1 })).toBe(
      "Strength 18",
    );
    expect(
      modLabel(en, { kind: "weapon_buff", detail: "Power Gem", value: 5 }),
    ).toBe("Power Gem: weapon +5");
    expect(modLabel(en, { kind: "situational", detail: "", value: -4 })).toBe(
      "Situational",
    );
    expect(
      modLabel(await makeT("fr"), { kind: "situational", value: -4 }),
    ).toBe("Situationnel");
  });

  // ADR 0016 step 14: an item power's contribution is named from its item by id (its current
  // name); without an item id, or when no item has it any more, the stored detail.
  it("names an item power's term from the item, the stored name otherwise", async () => {
    const en = await makeT("en");
    const names: Record<string, string> = {
      power_gem_dispel: "Gemme de puissance",
    };
    const itemName = (id: string) => names[id];
    const gem = {
      kind: "weapon_buff",
      detail: "Power Gem",
      itemId: "power_gem_dispel",
      value: 5,
    };
    expect(modLabel(en, gem, undefined, itemName)).toBe(
      "Gemme de puissance: weapon +5",
    );
    // The dice-multiplier damage term (a DamageTerm is labelled the same way).
    const diceTerm: DamageTerm = {
      term: "dice_multiplier",
      kind: "buff",
      detail: "Power Gem",
      itemId: "power_gem_dispel",
      value: 2,
    };
    expect(modLabel(en, diceTerm, undefined, itemName)).toBe(
      "Gemme de puissance",
    );
    expect(
      modLabel(en, { ...gem, itemId: "sold_gem" }, undefined, itemName),
    ).toBe("Power Gem: weapon +5");
    expect(
      modLabel(
        en,
        { kind: "buff", detail: "power_gem_dispel", value: 1 },
        undefined,
        itemName,
      ),
    ).toBe("power_gem_dispel");
    expect(modLabel(en, gem)).toBe("Power Gem: weapon +5");
  });
});
