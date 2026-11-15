import { afterEach, describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCharacterStore } from "@/stores/character";
import type { Character } from "@/types/character";
import { Rail } from "./Rail";

function owing(skillPoints: number, left = 0): Character {
  return {
    classes: [],
    derived: {
      levelUp: {
        toSpend: {
          weaponSlots: 0,
          nonWeaponSlots: 0,
          skillPoints,
          freeSpells: [],
        },
      },
      spellSlots: [],
      knownPowers: null,
      psp: null,
      proficiencySlots: { weapon: { left }, nonWeapon: { left: 0 } },
      skillSets: [],
    },
  } as unknown as Character;
}

afterEach(() => useCharacterStore.setState({ character: null }));

describe("Rail's Skills entry", () => {
  it.each([
    ["en", ["Act", "Reference", "Inventory", "Skills", "Experience"]],
    ["fr", ["Agir", "Référence", "Inventaire", "Compétences", "Expérience"]],
  ] as const)(
    "%s: after Inventory, for a character owing nothing",
    async (lng, names) => {
      useCharacterStore.setState({ character: owing(0) });
      await renderApp(<Rail active="act" onSelect={() => {}} />, { lng });
      const buttons = screen.getAllByRole("button");
      expect(buttons.map((b) => b.textContent)).toEqual(names);
      expect(screen.queryByTestId("skills-pill")).toBeNull();
    },
  );

  it.each([
    ["en", "Something is left to spend in Skills"],
    ["fr", "Il reste à dépenser dans Compétences"],
  ] as const)(
    "%s: a red pill with a text alternative while points are left",
    async (lng, text) => {
      useCharacterStore.setState({ character: owing(30) });
      await renderApp(<Rail active="act" onSelect={() => {}} />, { lng });
      expect(screen.getByTestId("skills-pill")).toBeInTheDocument();
      expect(screen.getByRole("img", { name: text })).toBeInTheDocument();
    },
  );

  it("keeps the pill while a pool is over budget and drops it once settled", async () => {
    useCharacterStore.setState({ character: owing(0, -1) });
    await renderApp(<Rail active="act" onSelect={() => {}} />);
    expect(screen.getByTestId("skills-pill")).toBeInTheDocument();
    useCharacterStore.setState({ character: owing(0, 0) });
    expect(await screen.findByRole("button", { name: "Skills" })).toBeVisible();
    expect(screen.queryByTestId("skills-pill")).toBeNull();
  });
});
