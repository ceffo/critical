import { describe, expect, it } from "vitest";
import type { Character } from "@/types/character";
import { sectionPending, skillsPending } from "./skillsPending";

const pool = (left: number) => ({ left });

/** A character with just the figures the pill reads. */
function character(o: {
  toSpend?: Partial<
    Record<"weaponSlots" | "nonWeaponSlots" | "skillPoints", number>
  >;
  weapon?: number;
  nonWeapon?: number;
  sets?: (number | null)[];
}): Character {
  return {
    derived: {
      levelUp: {
        toSpend: {
          weaponSlots: 0,
          nonWeaponSlots: 0,
          skillPoints: 0,
          freeSpells: [],
          ...o.toSpend,
        },
      },
      proficiencySlots:
        o.weapon === undefined && o.nonWeapon === undefined
          ? null
          : {
              weapon: o.weapon === undefined ? undefined : pool(o.weapon),
              nonWeapon: pool(o.nonWeapon ?? 0),
            },
      skillSets: (o.sets ?? []).map((left) => ({
        budget: left === null ? null : { left },
      })),
    },
  } as unknown as Character;
}

// ADR 0037 step 15 (amended): the Skills pill reads the server's figures.
describe("skillsPending", () => {
  it.each([
    ["no character", null, false],
    [
      "every pool settled",
      character({ weapon: 0, nonWeapon: 0, sets: [0] }),
      false,
    ],
    ["no pool at all", character({}), false],
    ["a set before its points start", character({ sets: [null] }), false],
    ["weapon slots granted", character({ toSpend: { weaponSlots: 1 } }), true],
    [
      "non-weapon slots granted",
      character({ toSpend: { nonWeaponSlots: 2 } }),
      true,
    ],
    ["skill points granted", character({ toSpend: { skillPoints: 30 } }), true],
    ["a weapon slot left", character({ weapon: 1, nonWeapon: 0 }), true],
    ["a non-weapon slot left", character({ weapon: 0, nonWeapon: 3 }), true],
    [
      "the weapon pool over budget",
      character({ weapon: -1, nonWeapon: 0 }),
      true,
    ],
    ["the non-weapon pool over budget", character({ nonWeapon: -2 }), true],
    [
      "the second skill set has points left",
      character({ sets: [0, 15] }),
      true,
    ],
    ["a skill set over budget", character({ sets: [-5] }), true],
    [
      "a free spell owed alone (spent in Spells)",
      {
        derived: {
          ...character({}).derived,
          levelUp: {
            toSpend: {
              weaponSlots: 0,
              nonWeaponSlots: 0,
              skillPoints: 0,
              freeSpells: [{ level: 3, spellLevel: 2 }],
            },
          },
        },
      } as unknown as Character,
      false,
    ],
  ])("%s", (_name, c, want) => {
    expect(skillsPending(c)).toBe(want);
  });
});

describe("sectionPending", () => {
  const withSets = (
    left: number,
    sets: { set: string; left: number | null }[],
    toSpend = {},
  ) =>
    ({
      derived: {
        levelUp: {
          toSpend: {
            weaponSlots: 0,
            nonWeaponSlots: 0,
            skillPoints: 0,
            ...toSpend,
          },
        },
        proficiencySlots: { weapon: { left }, nonWeapon: { left: 0 } },
        skillSets: sets.map((x) => ({
          set: x.set,
          budget: x.left === null ? null : { left: x.left },
        })),
      },
    }) as unknown as Character;

  it("reads each pool on its own", () => {
    const c = withSets(2, [{ set: "thief", left: 0 }]);
    expect(sectionPending(c, "weapon")).toBe(true);
    expect(sectionPending(c, "nonWeapon")).toBe(false);
    expect(sectionPending(c, { set: "thief" })).toBe(false);
  });

  it("marks a set by its own budget, an over-budget one too, and not one without a budget", () => {
    const c = withSets(0, [
      { set: "thief", left: -1 },
      { set: "bard", left: 3 },
      { set: "none", left: null },
    ]);
    expect(sectionPending(c, { set: "thief" })).toBe(true);
    expect(sectionPending(c, { set: "bard" })).toBe(true);
    expect(sectionPending(c, { set: "none" })).toBe(false);
    expect(sectionPending(c, { set: "absent" })).toBe(false);
  });

  it("marks a slot pool the level-up grants, and nothing without a character", () => {
    const c = withSets(0, [], { nonWeaponSlots: 1 });
    expect(sectionPending(c, "nonWeapon")).toBe(true);
    expect(sectionPending(c, "weapon")).toBe(false);
    expect(sectionPending(null, "weapon")).toBe(false);
  });
});
