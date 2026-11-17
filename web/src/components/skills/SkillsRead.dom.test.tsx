import { afterEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { ProficiencySlots } from "@/types/character";
import type { Proficiency, SkillSet } from "@/types/skills";
import { ProficienciesRead, SkillSetRead, WeaponsRead } from "./SkillsRead";
import { classTag } from "@/lib/classTag";

const slots = {
  nonWeapon: { available: 7, spent: 6, left: 1, languages: 0 },
  picks: [
    {
      id: "swimming",
      ability: "str",
      slots: 1,
      extraSlots: 0,
      free: false,
      cost: 1,
      modifier: -2,
      target: 10,
    },
  ],
  weapon: { available: 4, spent: 5, left: -1 },
  weaponPicks: [
    { code: "long_sword", slots: 2, allowed: true },
    { code: "dagger", slots: 1, allowed: true },
  ],
} as unknown as ProficiencySlots;

const proficiencies = [
  {
    id: "swimming",
    ability: "str",
    abilityScore: 12,
    modifier: -2,
    target: 10,
    last: null,
  },
] as unknown as Proficiency[];

const noop = vi.fn();
afterEach(() => noop.mockReset());

describe("ProficienciesRead", () => {
  it("reads the budget and each pick with the ability, the modifier and the target", async () => {
    await renderApp(
      <ProficienciesRead
        slots={slots}
        proficiencies={proficiencies}
        languages={[]}
        race="human"
        pending={false}
        onModify={noop}
      />,
    );
    expect(screen.getByTestId("skills-nonWeapon-budget")).toHaveTextContent(
      "6 / 7 slots",
    );
    expect(screen.getByTestId("skill-target-swimming")).toHaveTextContent(
      /12 −2 → 10/,
    );
    expect(screen.queryByTestId("skills-nonWeapon-tag")).toBeNull();
  });

  it("shows the class tag it is given and marks Edit pending", async () => {
    await renderApp(
      <ProficienciesRead
        slots={slots}
        proficiencies={proficiencies}
        languages={[]}
        race="human"
        tag="Fighter 4 + Thief 5"
        pending
        onModify={noop}
      />,
    );
    expect(screen.getByTestId("skills-nonWeapon-tag")).toHaveTextContent(
      "Fighter 4 + Thief 5",
    );
    expect(
      screen.getByTestId("skills-modify-skills-nonWeapon"),
    ).toHaveAttribute("data-pending", "true");
  });
});

describe("WeaponsRead", () => {
  it("marks a specialization with a chip and flags an over-budget pool", async () => {
    await renderApp(
      <WeaponsRead slots={slots} pending={false} onModify={noop} />,
    );
    expect(screen.getByTestId("weapon-specialized-long_sword")).toBeVisible();
    expect(screen.queryByTestId("weapon-specialized-dagger")).toBeNull();
    expect(screen.getByTestId("skills-weapon-budget")).toHaveAttribute(
      "data-over",
      "true",
    );
  });
});

describe("SkillSetRead", () => {
  it("reads the budget and the terms of each skill", async () => {
    const set = {
      set: "thief",
      name: { en: "Thief skills" },
      skillArmor: "",
      budget: {
        available: 60,
        spent: 40,
        left: 20,
        first: 60,
        perLevel: 30,
        level: 2,
        perSkillCap: 0,
        perSkillFirst: 0,
        perSkillPerLevel: 0,
      },
      skills: [
        {
          id: "climb_walls",
          base: 60,
          points: 10,
          race: 0,
          dexterity: 5,
          armor: 0,
          percent: 75,
          rawPercent: 75,
          maxPercent: 99,
          maxPoints: 40,
          usable: true,
          last: null,
        },
      ],
    } as unknown as SkillSet;
    await renderApp(<SkillSetRead set={set} pending={false} onModify={noop} />);
    expect(screen.getByTestId("skills-points-thief-budget")).toHaveTextContent(
      "40 / 60 points",
    );
    expect(
      screen.getByTestId("thief-read-percent-climb_walls"),
    ).toHaveTextContent("75%");
  });
});

describe("classTag", () => {
  const t = ((_k: string, o: { class: string; level: number }) =>
    `${o.class} ${o.level}`) as never;
  const name = (c: string) => c.toUpperCase();
  it("is a multi-class character's active classes, joined", () => {
    expect(
      classTag(
        [
          { class: "fighter", level: 4, state: "active" },
          { class: "thief", level: 5, state: "active" },
          { class: "mage", level: 2, state: "inactive" },
        ] as never,
        name,
        t,
      ),
    ).toBe("FIGHTER 4 + THIEF 5");
  });
  it("is none for one class", () => {
    expect(
      classTag(
        [{ class: "fighter", level: 4, state: "active" }] as never,
        name,
        t,
      ),
    ).toBeUndefined();
  });
});
