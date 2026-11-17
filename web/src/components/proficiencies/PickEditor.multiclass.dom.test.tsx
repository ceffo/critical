import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCharacterStore } from "@/stores/character";
import type { Character, ProficiencySlots } from "@/types/character";
import { PickEditor } from "./PickEditor";

// The creation options carry the fighter's (first class) figures; the server's figures for the
// character, a fighter/mage, differ from them on purpose: the editor must show the server's.
vi.mock("@/lib/useCreationOptions", () => ({
  useCreationOptions: () => ({
    options: {
      proficiencies: [
        {
          id: "bowyer",
          ability: "dex",
          modifier: 0,
          slots: 1,
          groups: ["warrior"],
        },
        {
          id: "spellcraft",
          ability: "int",
          modifier: -2,
          slots: 1,
          groups: ["wizard"],
        },
        {
          id: "cooking",
          ability: "int",
          modifier: 0,
          slots: 1,
          groups: ["general"],
        },
      ],
      proficiencyGroups: { fighter: ["warrior", "general"] },
      proficiencyCosts: {
        fighter: { bowyer: 1, spellcraft: 2, cooking: 1 },
      },
      languages: [],
      races: [],
    },
    failed: false,
    retry: () => {},
  }),
}));
vi.mock("./pickEditor", async (orig) => ({
  ...(await orig<typeof import("./pickEditor")>()),
  schedulePreview: () => () => {},
}));

// CRI-416 point 2: a multi-class character's groups and costs are the server's, read as served.
const served = {
  nonWeapon: {
    initial: 3,
    perLevels: 3,
    levelGained: 0,
    intelligence: 0,
    available: 6,
    spent: 0,
    left: 6,
    languages: 0,
  },
  picks: [],
  groups: ["warrior", "wizard", "general"],
  catalogue: [
    { id: "bowyer", cost: 7 },
    { id: "spellcraft", cost: 6 },
    { id: "cooking", cost: 5 },
  ],
} as unknown as ProficiencySlots;

describe("PickEditor for a multi-class character", () => {
  afterEach(() => useCharacterStore.setState({ character: null }));

  it("lists the groups of every class as the character's own, at the served costs", async () => {
    useCharacterStore.setState({
      character: {
        str: 10,
        dex: 10,
        con: 10,
        int: 10,
        wis: 10,
        cha: 10,
      } as unknown as Character,
    });
    await renderApp(
      <PickEditor
        pool="nonWeapon"
        cls="fighter"
        race="human"
        languages={[]}
        slots={served}
        classes="Fighter / Mage"
        onDone={() => {}}
      />,
    );
    // Both classes' groups are the character's own, the wizard's too (the first-class table has no
    // wizard group, and charges Spellcraft 2).
    for (const g of ["warrior", "wizard", "general"]) {
      expect(screen.getByTestId(`proficiency-group-${g}`)).toHaveTextContent(
        "your class",
      );
    }
    const wizard = screen.getByTestId("proficiency-group-wizard");
    expect(within(wizard).queryByTestId("proficiency-other-wizard")).toBeNull();
    // The first band is open: the served cost, not the fighter table's.
    const own = screen.getByTestId("proficiency-own-warrior");
    expect(within(own).getByTestId("proficiency-cost")).toHaveTextContent(
      "7 slots",
    );
  });
});
