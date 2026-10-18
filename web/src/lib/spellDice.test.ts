import { describe, expect, it } from "vitest";
import i18next from "i18next";
import { resources } from "@/i18n";
import {
  damageFormula,
  damageText,
  diceText,
  rollLabel,
  stepsHeader,
} from "./spellDice";
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
const damage = (
  d: DiceSum,
  type?: string,
  instances = n(1),
  rounds = n(1),
): SpellDamage => ({
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
    [
      damage(dice(1, [1, 4]), "force", n(5)),
      "5 × 1d4+1 force",
      "5 × 1d4+1 force",
    ],
    [
      damage(dice(0, [2, 4]), "acid", n(1), n(6)),
      "2d4 acid per round, 6 rounds",
      "2d4 acide par round, 6 rounds",
    ],
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
    expect(damageText(await makeT("en"), missiles)).toBe(
      "3 × (2d4 [no save] + 15 [save negates]) force",
    );
    expect(damageText(await makeT("fr"), missiles)).toBe(
      "3 × (2d4 [sans sauvegarde] + 15 [annulé par la sauvegarde]) force",
    );
  });
  it("one part, one instance: no parentheses", async () => {
    const rot: SpellDamage = {
      ...damage(dice(0, [2, 6]), "negative"),
      parts: [{ dice: dice(0, [2, 6]), save: "none" }],
    };
    expect(damageText(await makeT("en"), rot)).toBe(
      "2d6 [no save] negative energy",
    );
  });
  it("the composer gets the whole instance", () => {
    expect(damageFormula(missiles)).toBe("2d4+15");
  });
});

describe("dice steps", () => {
  const stepped = (
    per: "round" | "instance",
    type: string | undefined,
    ...steps: DiceSum[]
  ): SpellDamage => ({
    ...damage(
      n(0),
      type,
      per === "instance" ? n(steps.length) : n(1),
      per === "round" ? n(steps.length) : n(1),
    ),
    steps_per: per,
    steps: steps.map((d, i) => ({ step: i + 1, dice: d })),
  });

  it.each([
    [
      stepped(
        "round",
        "fire",
        n(0),
        n(0),
        dice(0, [12, 2]),
        dice(0, [12, 4]),
        dice(0, [12, 2]),
      ),
      "by round: —, —, 12d2, 12d4, 12d2 fire",
      "par round : —, —, 12d2, 12d4, 12d2 feu",
    ],
    [
      stepped(
        "instance",
        "electricity",
        dice(0, [3, 6]),
        dice(0, [2, 6]),
        dice(0, [1, 6]),
      ),
      "in round: 3d6, 2d6, 1d6 electricity",
      "successivement : 3d6, 2d6, 1d6 électricité",
    ],
    [
      stepped("round", undefined, dice(1, [2, 4]), n(3)),
      "by round: 2d4+1, 3",
      "par round : 2d4+1, 3",
    ],
  ])("%j", async (d, en, fr) => {
    expect(damageText(await makeT("en"), d)).toBe(en);
    expect(damageText(await makeT("fr"), d)).toBe(fr);
  });

  it("sends the first step with damage to the dice composer", () => {
    expect(
      damageFormula(
        stepped("round", "fire", n(0), dice(0, [12, 2]), dice(0, [12, 4])),
      ),
    ).toBe("12d2");
  });

  it("labels each roll by its step: a turn for a step per round, else its number", async () => {
    const t = await makeT("en");
    expect(rollLabel(t, "round", 3, 0)).toBe("Round 3");
    expect(rollLabel(t, "instance", 2, 0)).toBe("#2");
    expect(rollLabel(t, undefined, undefined, 4)).toBe("#5");
    expect(rollLabel(await makeT("fr"), "round", 3, 0)).toBe("Tour 3");
  });

  it.each([
    ["round", 5, "5 rounds, one roll each", "5 tours, un jet chacun"],
    ["round", 1, "1 round, one roll", "1 tour, un jet"],
    ["instance", 12, "12 rolls in round", "12 jets successifs"],
    ["instance", 1, "1 roll", "1 jet"],
  ] as const)("header of %s steps, %d", async (per, count, en, fr) => {
    expect(stepsHeader(await makeT("en"), per, count)).toBe(en);
    expect(stepsHeader(await makeT("fr"), per, count)).toBe(fr);
  });
});
