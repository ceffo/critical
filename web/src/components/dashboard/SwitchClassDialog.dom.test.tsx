import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, waitFor, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCreationOptionsStore } from "@/stores/creationOptions";
import { useCharacterStore } from "@/stores/character";
import { ApiError } from "@/api/http";
import { switchableEntries, switchTargets } from "@/lib/classSwitch";
import type { Character, CreationOptions } from "@/types/character";
import { SwitchClassDialog } from "./SwitchClassDialog";

// ADR 0050 step 15: the dialog on a mage of a race that is no specialist's (the friend's elf
// mage). What the entry may become is the options' (the group and the spellbook), the warning is
// Table 22's races for the class; the server decides the rest.
const options = {
  classes: [
    { class: "fighter", group: "warrior", spellbook: null },
    { class: "mage", group: "wizard", spellbook: { startingSpells: [] } },
    { class: "transmuter", group: "wizard", spellbook: { startingSpells: [] } },
    { class: "abjurer", group: "wizard", spellbook: { startingSpells: [] } },
    { class: "bard", group: "rogue", spellbook: { startingSpells: [] } },
  ],
  specialistRaces: {
    transmuter: ["human", "half_elf"],
    abjurer: ["human"],
  },
} as unknown as CreationOptions;

function sheet(
  classes: { class: string; state?: "active" | "frozen" }[],
  raceCode = "elf",
): Character {
  return {
    name: "Ilsevel",
    raceCode,
    classes: classes.map((c) => ({ level: 14, state: "active", ...c })),
    warnings: [],
    derived: { realizedLevel: 14 },
  } as unknown as Character;
}

const switchEntryClass = vi.fn();

beforeEach(() => {
  useCreationOptionsStore.setState({ options });
  switchEntryClass.mockReset().mockResolvedValue(null);
  useCharacterStore.setState({ switchEntryClass });
});
afterEach(() => useCreationOptionsStore.setState({ options: null }));

describe("what an entry may switch to", () => {
  it("offers the other classes of its group that keep a spellbook, none a character holds", () => {
    const mage = sheet([{ class: "mage" }]);
    expect(switchTargets(mage, options, 0)).toEqual(["transmuter", "abjurer"]);
    // A bard keeps a spellbook but is alone of its group: nothing to switch to.
    expect(switchableEntries(sheet([{ class: "bard" }]), options)).toEqual([]);
    expect(switchableEntries(sheet([{ class: "fighter" }]), options)).toEqual(
      [],
    );
    // A wizard held by another entry is not offered.
    const two = sheet([{ class: "mage" }, { class: "transmuter" }]);
    expect(switchTargets(two, options, 0)).toEqual(["abjurer"]);
  });
});

describe("the switch dialog", () => {
  it("names what is kept and warns about the race, then sends the entry and the class", async () => {
    const { user } = await renderApp(
      <SwitchClassDialog
        character={sheet([{ class: "mage" }])}
        open
        onOpenChange={() => {}}
      />,
    );
    const dialog = screen.getByTestId("switch-class-dialog");
    // One entry can switch: no entry picker.
    expect(within(dialog).queryByTestId("switch-class-entry")).toBeNull();
    const next = within(dialog).getByTestId("switch-class-next");
    expect(next).toBeDisabled();
    await user.selectOptions(
      within(dialog).getByTestId("switch-class-pick"),
      "transmuter",
    );
    await user.click(next);
    expect(
      within(dialog).getByTestId("switch-class-confirm"),
    ).toHaveTextContent(
      "Mage becomes Transmuter. Kept as they are: level, XP, hit points, proficiencies and the whole spellbook.",
    );
    expect(
      within(dialog).getByTestId("switch-class-opposed"),
    ).toHaveTextContent("stay in the book but can no longer be prepared");
    expect(
      within(dialog).getByTestId("switch-class-race-warning"),
    ).toHaveTextContent("A Elf is not among the races");
    await user.click(within(dialog).getByTestId("switch-class-confirm-button"));
    expect(switchEntryClass).toHaveBeenCalledExactlyOnceWith(0, "transmuter");
  });

  it("shows no race warning for a race Table 22 lists, and none about opposition for a mage", async () => {
    const { user } = await renderApp(
      <SwitchClassDialog
        character={sheet([{ class: "transmuter" }], "human")}
        open
        onOpenChange={() => {}}
      />,
    );
    const dialog = screen.getByTestId("switch-class-dialog");
    await user.selectOptions(
      within(dialog).getByTestId("switch-class-pick"),
      "mage",
    );
    await user.click(within(dialog).getByTestId("switch-class-next"));
    expect(
      within(dialog).queryByTestId("switch-class-race-warning"),
    ).toBeNull();
    expect(within(dialog).queryByTestId("switch-class-opposed")).toBeNull();
  });

  it("asks which entry when several can switch, in French, and words a refusal in the dialog", async () => {
    switchEntryClass.mockResolvedValue(
      new ApiError("Cette classe n'admet pas l'alignement du personnage.", 409),
    );
    const { user } = await renderApp(
      <SwitchClassDialog
        character={sheet([{ class: "mage" }, { class: "fighter" }])}
        open
        onOpenChange={() => {}}
      />,
      { lng: "fr" },
    );
    const dialog = screen.getByTestId("switch-class-dialog");
    // Only the mage entry can switch: the fighter's group has no spellbook class.
    expect(within(dialog).queryByTestId("switch-class-entry")).toBeNull();
    await user.selectOptions(
      within(dialog).getByTestId("switch-class-pick"),
      "abjurer",
    );
    await user.click(within(dialog).getByTestId("switch-class-next"));
    expect(
      within(dialog).getByTestId("switch-class-confirm"),
    ).toHaveTextContent("Magicien devient Abjurateur.");
    await user.click(within(dialog).getByTestId("switch-class-confirm-button"));
    await waitFor(() =>
      expect(
        within(dialog).getByTestId("switch-class-refusal"),
      ).toHaveTextContent("n'admet pas l'alignement"),
    );
  });
});
