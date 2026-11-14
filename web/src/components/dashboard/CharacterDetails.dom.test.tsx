import { beforeEach, describe, expect, it } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { useCreationOptionsStore } from "@/stores/creationOptions";
import type { Character, CreationOptions } from "@/types/character";
import { CharacterDetails } from "./CharacterDetails";
import { useCharacterStore } from "@/stores/character";
import { CardDetails } from "./CardDetails";
import { DetailRows } from "./DetailRows";

/** A character with the details the test names set, every other optional one left unset. */
function character(set: Partial<Character> = {}): Character {
  return {
    name: "Aldéric",
    classes: [{ class: "bard", level: 3 }],
    raceCode: "human",
    alignment: "true_neutral",
    sex: "male",
    age: null,
    height_cm: null,
    weight_kg: null,
    hair: "",
    eyes: "",
    languages: [],
    warnings: [],
    derived: { realizedLevel: 3, proficiencySlots: undefined },
    ...set,
  } as unknown as Character;
}

beforeEach(() => {
  // The language catalogue is read once and held: no request in the test.
  useCreationOptionsStore.setState({
    options: {
      languages: [{ code: "elvish", name: { en: "Elvish" } }],
    } as unknown as CreationOptions,
  });
});

describe("details popover shows only what is set", () => {
  it("hides the unset fields and offers one link to set them", async () => {
    const { user } = await renderApp(
      <CharacterDetails character={character({ hair: "Chestnut" })} />,
    );
    await user.click(screen.getByTestId("details-trigger"));
    const pop = screen.getByTestId("details-popover");
    expect(within(pop).getByText("Chestnut")).toBeInTheDocument();
    for (const label of ["Age", "Height", "Weight", "Eyes", "Languages"])
      expect(within(pop).queryByText(label)).not.toBeInTheDocument();
    expect(within(pop).queryByText("Not set")).not.toBeInTheDocument();
    expect(within(pop).getAllByTestId("details-set")).toHaveLength(1);

    await user.click(within(pop).getByTestId("details-set"));
    expect(within(pop).getByLabelText("Eyes")).toBeInTheDocument();
  });

  it("drops the link when everything is set, and still asks for a missing alignment", async () => {
    const full = character({
      age: 30,
      height_cm: 180,
      weight_kg: 80,
      hair: "Black",
      eyes: "Green",
      languages: ["elvish"],
      alignment: "",
    });
    const { user } = await renderApp(<CharacterDetails character={full} />);
    await user.click(screen.getByTestId("details-trigger"));
    const pop = screen.getByTestId("details-popover");
    expect(within(pop).queryByTestId("details-set")).not.toBeInTheDocument();
    expect(within(pop).getByText("Elvish")).toBeInTheDocument();
    expect(within(pop).getByTestId("details-missing")).toBeInTheDocument();
  });
});

describe("card details", () => {
  it("words the specialist's race warning under the race, and only when there is one", async () => {
    const warned = character({
      raceCode: "elf",
      warnings: [
        {
          code: "specialist_race",
          field: "race",
          class: "abjurer",
          race: "elf",
        },
      ] as Character["warnings"],
    });
    const { unmount } = await renderApp(<DetailRows character={warned} />);
    expect(screen.getByTestId("details-race-warning")).toHaveTextContent(
      "The race (Elf) is not one the book lists for this specialist",
    );
    unmount();
    await renderApp(<DetailRows character={character({ raceCode: "elf" })} />);
    expect(
      screen.queryByTestId("details-race-warning"),
    ).not.toBeInTheDocument();
  });

  it("shows the set details inline, without the name, class and level", async () => {
    await renderApp(
      <DetailRows
        character={character({ eyes: "Green", languages: ["elvish"] })}
      />,
    );
    const rows = screen.getByTestId("detail-rows");
    expect(within(rows).getByText("Green")).toBeInTheDocument();
    expect(within(rows).getByText("Elvish")).toBeInTheDocument();
    expect(within(rows).queryByText("Class")).not.toBeInTheDocument();
    expect(within(rows).queryByText("Age")).not.toBeInTheDocument();
  });

  it("lists the race's save bonuses the server derives, and only the ones there are", async () => {
    const c = character();
    c.derived = {
      ...c.derived,
      raceSaveBonus: {
        paraPoison: 3,
        rodStaffWand: 3,
        petriPoly: 0,
        breath: 0,
        spell: 3,
      },
    } as Character["derived"];
    useCharacterStore.setState({ character: c });
    await renderApp(<CardDetails />);
    const list = within(screen.getByTestId("save-bonuses"));
    expect(list.getAllByRole("listitem")).toHaveLength(3);
    expect(list.getByText("Spells +3")).toBeInTheDocument();
    expect(list.queryByText(/Breath/)).not.toBeInTheDocument();
  });
});
