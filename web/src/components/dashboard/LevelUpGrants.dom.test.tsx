import { afterEach, describe, expect, it } from "vitest";
import { fireEvent, screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCharacterStore } from "@/stores/character";
import type { Character } from "@/types/character";
import { GrantRows } from "./LevelUpGrants";

// the spend link of a set's points names the set, so the editor opens on it.
describe("GrantRows' skill points link", () => {
  afterEach(() => useCharacterStore.setState({ character: null }));

  const sets = (lefts: number[]) =>
    ({
      derived: {
        skillSets: lefts.map((left, i) => ({
          set: i === 0 ? "thief" : "bard",
          name:
            i === 0
              ? { en: "Thief", fr: "Voleur" }
              : { en: "Bard", fr: "Barde" },
          budget: { left },
        })),
      },
    }) as unknown as Character;
  const grants = {
    weaponSlots: 0,
    nonWeaponSlots: 0,
    skillPoints: 30,
    freeSpells: [],
  };
  const rows = () => (
    <GrantRows
      grants={grants}
      onOpenEditor={() => {}}
      onOpenFreeSpell={() => {}}
    />
  );

  it("lists one row for each set with points left, by the set's own points", async () => {
    useCharacterStore.setState({ character: sets([30, 20]) });
    await renderApp(rows(), { lng: "fr" });
    expect(screen.getAllByTestId("grant-skillPoints")).toHaveLength(2);
    expect(
      screen.getByText("+30 points de talent · Voleur"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("+20 points de talent · Barde"),
    ).toBeInTheDocument();
  });

  it("keeps the level's one row when one set has points left", async () => {
    useCharacterStore.setState({ character: sets([0, 20]) });
    await renderApp(rows(), { lng: "fr" });
    expect(screen.getAllByTestId("grant-skillPoints")).toHaveLength(1);
    expect(screen.getByText("+30 points de talent")).toBeInTheDocument();
  });

  it("passes the clicked row's set to the editor", async () => {
    useCharacterStore.setState({
      character: {
        derived: {
          skillSets: [
            {
              set: "thief",
              name: { en: "Thief", fr: "Voleur" },
              budget: { left: 30 },
            },
            {
              set: "bard",
              name: { en: "Bard", fr: "Barde" },
              budget: { left: 20 },
            },
          ],
        },
      } as unknown as Character,
    });
    const calls: unknown[][] = [];
    await renderApp(
      <GrantRows
        grants={{
          weaponSlots: 0,
          nonWeaponSlots: 0,
          skillPoints: 30,
          freeSpells: [],
        }}
        onOpenEditor={(...a) => calls.push(a)}
        onOpenFreeSpell={() => {}}
      />,
    );
    const links = screen.getAllByTestId("grant-open-skillPoints");
    expect(links).toHaveLength(2);
    fireEvent.click(links[1]);
    expect(calls).toEqual([["skillPoints", "bard"]]);
  });
});
