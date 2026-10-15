import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import { damageFormula, damageText, diceText } from "./spellDice";
import type { DiceSum, SpellDamage } from "@/types/spells";

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

const n = (modifier: number): DiceSum => ({ groups: [], modifier });
const dice = (modifier: number, ...groups: [number, number][]): DiceSum => ({
  groups: groups.map(([count, sides]) => ({ count, sides })),
  modifier,
});
const damage = (d: DiceSum, type?: string, instances = n(1), rounds = n(1)): SpellDamage => ({
  caster_level: 15,
  dice: d,
  instances,
  rounds,
  type,
});

describe("diceText", () => {
  it.each([
    [dice(0, [10, 6]), "10d6"],
    [dice(1, [1, 4]), "1d4+1"],
    [dice(-1, [2, 4]), "2d4-1"],
    [dice(0, [1, 6], [4, 6]), "1d6+4d6"],
    [n(10), "10"],
  ])("%j", (sum, want) => {
    expect(diceText(sum)).toBe(want);
  });
});

describe("damageText", () => {
  it.each([
    [damage(dice(0, [10, 6]), "fire"), "10d6 fire", "10d6 feu"],
    [damage(dice(1, [1, 4]), "force", n(5)), "5 × 1d4+1 force", "5 × 1d4+1 force"],
    [damage(dice(0, [2, 4]), "acid", n(1), n(6)), "2d4 acid per round, 6 rounds", "2d4 acide par round, 6 rounds"],
    [damage(dice(15, [15, 4])), "15d4+15", "15d4+15"],
    [damage(dice(0, [1, 6]), "radiant"), "1d6 radiant", "1d6 radiant"],
  ])("%j", async (d, en, fr) => {
    expect(damageText(await makeT("en"), d)).toBe(en);
    expect(damageText(await makeT("fr"), d)).toBe(fr);
  });

  it("sends one instance to the dice composer", () => {
    expect(damageFormula(damage(dice(1, [1, 4]), "force", n(5)))).toBe("1d4+1");
  });
});

describe("damageText with parts", () => {
  const missiles: SpellDamage = {
    ...damage(dice(15, [2, 4]), "force", n(3)),
    parts: [
      { dice: dice(0, [2, 4]), save: "none" },
      { dice: n(15), save: "negates" },
    ],
  };
  it("words each part and what a save does to it, in en and fr", async () => {
    expect(damageText(await makeT("en"), missiles)).toBe("3 × (2d4 [no save] + 15 [save negates]) force");
    expect(damageText(await makeT("fr"), missiles)).toBe(
      "3 × (2d4 [sans sauvegarde] + 15 [annulé par la sauvegarde]) force",
    );
  });
  it("one part, one instance: no parentheses", async () => {
    const rot: SpellDamage = { ...damage(dice(0, [2, 6]), "negative"), parts: [{ dice: dice(0, [2, 6]), save: "none" }] };
    expect(damageText(await makeT("en"), rot)).toBe("2d6 [no save] negative energy");
  });
  it("the composer gets the whole instance", () => {
    expect(damageFormula(missiles)).toBe("2d4+15");
  });
});
