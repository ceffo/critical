import { describe, expect, it } from "vitest";
import type { RollResult } from "@/types/combat";
import type { UseResult } from "@/types/items";
import { damageFormula, itemRollText } from "./rollText";

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
    [{ multipliers: 1.5, totalDamage: 10, unroundedDamage: 10.5 }, "((5 × 1.0) + 2) × 1.5 + 0 = 10.5 → 10"],
    [
      { spellsBuff: 1.5, diceMultiplier: 2, multipliers: 2, strengthBonus: 2, bonusDamage: 1, totalDamage: 37, unroundedDamage: 37 },
      "((5 × 1.5 × 2.0) + 2) × 2.0 + 2 + 1 = 37",
    ],
    [{ multipliers: 1.5, bonusDamage: -20, totalDamage: 0, unroundedDamage: -9.5 }, "((5 × 1.0) + 2) × 1.5 + 0 − 20 = -9.5 → 0"],
    [{ unroundedDamage: undefined }, "((5 × 1.0) + 2) × 1.0 + 0 = 7"],
  ])("%o", (over, want) => expect(damageFormula(hit(over), "en")).toBe(want));

  it("uses the language's decimal separator", () => {
    expect(damageFormula(hit({ multipliers: 1.5, totalDamage: 10, unroundedDamage: 10.5 }), "fr")).toBe(
      "((5 × 1,0) + 2) × 1,5 + 0 = 10,5 → 10",
    );
  });
});

describe("itemRollText", () => {
  it("lists the dice, the flat modifier and the total", () => {
    expect(itemRollText({ dice: "6d6", rolls: [1, 2, 3, 4, 5, 6], modifier: 0, total: 21 } as UseResult)).toBe(
      "6d6: 1 + 2 + 3 + 4 + 5 + 6 = 21",
    );
    expect(itemRollText({ dice: "1d8+2", rolls: [5], modifier: 2, total: 7 } as UseResult)).toBe("1d8+2: 5 + 2 = 7");
  });
});
