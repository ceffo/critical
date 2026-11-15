import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCreationOptionsStore } from "@/stores/creationOptions";
import type { Character, CreationOptions } from "@/types/character";
import { ClassEntries } from "./ClassEntries";

// ADR 0050 step 15.7: a fighter who took Mage. Only the mage entry's name is a link to the school
// picker; the rest of the line still opens the class entries' popover, a separate click target.
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
  ],
} as unknown as CreationOptions;

function sheet(second: string): Character {
  const classes = [
    { class: "fighter", level: 6, state: "frozen" },
    { class: second, level: 1, state: "active" },
  ];
  return {
    classes,
    warnings: [],
    derived: {
      classes: classes.map((c) => ({ ...c, xp: 0, nextLevelXp: 100 })),
    },
  } as unknown as Character;
}

beforeEach(() => useCreationOptionsStore.setState({ options }));
afterEach(() => useCreationOptionsStore.setState({ options: null }));

describe("the class entries line", () => {
  it("links only the mage entry to the school picker, apart from the popover", async () => {
    const onOpenSchool = vi.fn();
    const { user } = await renderApp(
      <ClassEntries
        character={sheet("mage")}
        onOpenXp={() => {}}
        onOpenSchool={onOpenSchool}
      />,
    );
    expect(screen.queryByTestId("class-school-link-fighter")).toBeNull();
    await user.click(screen.getByTestId("class-school-link-mage"));
    expect(onOpenSchool).toHaveBeenCalledExactlyOnceWith(1);
    expect(screen.queryByTestId("class-entries")).toBeNull();

    await user.click(screen.getByTestId("class-summary"));
    expect(await screen.findByTestId("class-entries")).toBeInTheDocument();
    expect(onOpenSchool).toHaveBeenCalledTimes(1);
  });

  it("reads the specialist and its school once switched, and still links", async () => {
    await renderApp(
      <ClassEntries
        character={sheet("transmuter")}
        onOpenXp={() => {}}
        onOpenSchool={() => {}}
      />,
    );
    expect(
      screen.getByTestId("class-school-link-transmuter"),
    ).toHaveTextContent("Transmuter · Alteration");
  });
});
