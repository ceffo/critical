import { afterEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCharacterStore } from "@/stores/character";
import type {
  Character,
  ProficiencySlots,
  WeaponOption,
} from "@/types/character";
import { PickEditor } from "./PickEditor";

// The editor reads the creation options and asks the server's preview: neither is under test here.
vi.mock("@/lib/useCreationOptions", () => ({
  useCreationOptions: () => ({ options: null, failed: false, retry: () => {} }),
}));
vi.mock("./pickEditor", async (orig) => ({
  ...(await orig<typeof import("./pickEditor")>()),
  schedulePreview: () => () => {},
}));

const LEAD = "A fighter may also specialize in one weapon,";

function slots(weapons: WeaponOption[]): ProficiencySlots {
  return {
    nonWeapon: {} as ProficiencySlots["nonWeapon"],
    picks: [],
    weapon: {
      initial: 4,
      perLevels: 3,
      levelGained: 0,
      available: 4,
      spent: 2,
      left: 2,
    },
    weaponPicks: [{ code: "long_sword", slots: 2, allowed: true }],
    weapons,
  };
}

// ADR 0037 step 9 and ADR 0042 step 12: the weapon editor offers a new specialization (its lead)
// when the server gives a type a specialization cost (an active class that specializes), never
// from the character's `specializes`: a dual-class keeps its old fighter's specialization past
// the crossover, but its mage offers no new one.
describe("PickEditor's specialization lead", () => {
  afterEach(() => useCharacterStore.setState({ character: null }));

  it.each([
    // [case, the character's specializes, the server's cost per type, lead shown]
    ["a single-class fighter", true, 2, true],
    ["a mage who became a fighter (the active fighter)", true, 2, true],
    [
      "the elf past the crossover: a kept fighter specialization, a mage now",
      true,
      0,
      false,
    ],
    ["the elf before the crossover", false, 0, false],
    ["a fighter/thief (multi-class never specializes)", false, 0, false],
  ])("%s", async (_name, specializes, cost, shown) => {
    useCharacterStore.setState({
      character: { derived: { specializes } } as unknown as Character,
    });
    await renderApp(
      <PickEditor
        pool="weapon"
        cls="fighter"
        race="human"
        languages={[]}
        slots={slots([
          { code: "long_sword", specializationSlots: cost },
          { code: "dagger", specializationSlots: cost },
        ])}
        onDone={() => {}}
      />,
    );
    if (shown) {
      expect(screen.getByText(new RegExp(LEAD))).toBeInTheDocument();
    } else {
      expect(screen.queryByText(new RegExp(LEAD))).toBeNull();
    }
  });
});
