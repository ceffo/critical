import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook, screen, waitFor, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCreationOptionsStore } from "@/stores/creationOptions";
import { useCharacterStore } from "@/stores/character";
import { ApiError } from "@/api/http";
import { fetchSwitchTargets, type SwitchTarget } from "@/api/character";
import { schoolOf, useSwitchableEntries } from "@/lib/classSwitch";
import type { Character, CreationOptions } from "@/types/character";
import { SwitchClassDialog } from "./SwitchClassDialog";

vi.mock("@/api/character", async (orig) => ({
  ...(await orig<typeof import("@/api/character")>()),
  fetchSwitchTargets: vi.fn(),
}));

// ADR 0050 step 15: the school picker on an elf mage (the friend's character). What the entry may
// become comes from the options (the group and the spellbook, for the link); the picker's cards, the
// minimums against the scores, the opposed schools and the race warning are the server's
// (`GET /api/character/class-entry/targets`), and the dialog words them without comparing anything.
const options = {
  classes: [
    { class: "fighter", group: "warrior", spellbook: null },
    { class: "mage", group: "wizard", spellbook: { startingSpells: [] } },
    {
      class: "transmuter",
      group: "wizard",
      schools: ["alteration"],
      spellbook: { startingSpells: [] },
    },
    {
      class: "abjurer",
      group: "wizard",
      schools: ["abjuration"],
      spellbook: { startingSpells: [] },
    },
    { class: "bard", group: "rogue", spellbook: { startingSpells: [] } },
  ],
} as unknown as CreationOptions;

const target = (
  t: Partial<SwitchTarget> & { class: string },
): SwitchTarget => ({
  schools: [],
  opposedSchools: [],
  minimums: [],
  raceWarning: false,
  refusal: "",
  ...t,
});

const targetsOfAMage: SwitchTarget[] = [
  target({
    class: "transmuter",
    schools: ["alteration"],
    opposedSchools: ["abjuration", "necromancy"],
    minimums: [{ ability: "dex", min: 15, score: 17, met: true }],
    raceWarning: true,
  }),
  target({
    class: "abjurer",
    schools: ["abjuration"],
    opposedSchools: ["alteration", "illusion"],
    minimums: [{ ability: "wis", min: 15, score: 8, met: false }],
    refusal: "class_switch_minimums_not_met",
    raceWarning: true,
  }),
];

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

function show(character: Character, lng?: "fr") {
  return renderApp(
    <SwitchClassDialog
      character={character}
      entry={0}
      open
      onOpenChange={() => {}}
    />,
    lng ? { lng } : undefined,
  );
}

beforeEach(() => {
  useCreationOptionsStore.setState({ options });
  switchEntryClass.mockReset().mockResolvedValue(null);
  useCharacterStore.setState({ switchEntryClass });
  vi.mocked(fetchSwitchTargets)
    .mockReset()
    .mockResolvedValue({ entry: 0, from: "mage", targets: targetsOfAMage });
});
afterEach(() => useCreationOptionsStore.setState({ options: null }));

describe("what an entry may switch to", () => {
  it("links exactly the entries the server lists a target for, and compares nothing itself", async () => {
    // The server answers per entry: a mage has targets, a fighter none, a refused list still counts.
    vi.mocked(fetchSwitchTargets).mockImplementation(async (entry) => ({
      entry,
      from: "x",
      targets: entry === 1 ? [] : targetsOfAMage,
    }));
    const character = sheet([{ class: "mage" }, { class: "fighter" }]);
    const { result } = renderHook(() => useSwitchableEntries(character));
    expect(result.current).toEqual([]);
    await waitFor(() => expect(result.current).toEqual([0]));
    expect(schoolOf(options, "transmuter")).toBe("alteration");
    expect(schoolOf(options, "mage")).toBe("");
  });
});

describe("the school picker", () => {
  it("shows a card per class with the server's minimums and warnings, greys the refused one", async () => {
    const { user } = await show(sheet([{ class: "mage" }]));
    const cards = await screen.findByTestId("switch-class-cards");
    const ok = within(cards).getByTestId("school-card-transmuter");
    expect(ok).toHaveTextContent("Alteration");
    expect(ok).toHaveTextContent("Opposed: Abjuration, Necromancy");
    expect(ok).toHaveTextContent("DEX 15");
    expect(ok).toHaveTextContent("Elf not in Table 22: allowed");
    expect(ok).toHaveAttribute("aria-disabled", "false");
    const grey = within(cards).getByTestId("school-card-abjurer");
    expect(grey).toHaveAttribute("aria-disabled", "true");
    expect(grey).toHaveTextContent("WIS 15 needed (8)");
    // A refused card cannot be picked: Next stays off.
    expect(fetchSwitchTargets).toHaveBeenCalledWith(0);
    await user.click(within(cards).getByTestId("school-card-abjurer"));
    expect(screen.getByTestId("switch-class-next")).toBeDisabled();
  });

  it("names what is kept and warns about the race, then sends the entry and the class", async () => {
    const { user } = await show(sheet([{ class: "mage" }]));
    const dialog = screen.getByTestId("switch-class-dialog");
    const next = within(dialog).getByTestId("switch-class-next");
    expect(next).toBeDisabled();
    await user.click(
      await within(dialog).findByTestId("school-card-transmuter"),
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

  it("offers the mage back to a specialist, with no opposed note and no race warning", async () => {
    vi.mocked(fetchSwitchTargets).mockResolvedValue({
      entry: 0,
      from: "transmuter",
      targets: [
        target({
          class: "mage",
          minimums: [{ ability: "int", min: 9, score: 17, met: true }],
        }),
      ],
    });
    const { user } = await show(sheet([{ class: "transmuter" }], "human"));
    const card = await screen.findByTestId("school-card-mage");
    expect(card).toHaveTextContent("Generalist");
    await user.click(card);
    await user.click(screen.getByTestId("switch-class-next"));
    expect(screen.queryByTestId("switch-class-race-warning")).toBeNull();
    expect(screen.queryByTestId("switch-class-opposed")).toBeNull();
  });

  it("words a refusal in the dialog, in French", async () => {
    switchEntryClass.mockResolvedValue(
      new ApiError("Cette classe n'admet pas l'alignement du personnage.", 409),
    );
    const { user } = await show(
      sheet([{ class: "mage" }, { class: "fighter" }]),
      "fr",
    );
    const dialog = screen.getByTestId("switch-class-dialog");
    expect(
      within(dialog).getByText("Choisir une école de magie"),
    ).toBeVisible();
    await user.click(
      await within(dialog).findByTestId("school-card-transmuter"),
    );
    await user.click(within(dialog).getByTestId("switch-class-next"));
    expect(
      within(dialog).getByTestId("switch-class-confirm"),
    ).toHaveTextContent("Magicien devient Transmutateur.");
    await user.click(within(dialog).getByTestId("switch-class-confirm-button"));
    await waitFor(() =>
      expect(
        within(dialog).getByTestId("switch-class-refusal"),
      ).toHaveTextContent("n'admet pas l'alignement"),
    );
  });
});
