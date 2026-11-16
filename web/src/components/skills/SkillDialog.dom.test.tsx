import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { RogueSkill } from "@/types/skills";
import { SkillNameButton } from "./SkillDialog";

const getSkillDescription = vi.fn();
vi.mock("@/api/skills", () => ({
  getSkillDescription: (set: string, skill: string) =>
    getSkillDescription(set, skill),
}));

const pockets: RogueSkill = {
  id: "pick_pockets",
  base: 15,
  points: 20,
  race: 5,
  dexterity: -5,
  armor: -20,
  percent: 15,
  rawPercent: 15,
  maxPercent: 95,
  maxPoints: 60,
  usable: true,
  last: null,
};

const TEXT = {
  thief: {
    en: "Invented thief text.\n\n- a list item\n- another item",
    fr: "Texte inventé du voleur.",
  },
  bard: { en: "Invented bard text.", fr: "Texte inventé du barde." },
};

beforeEach(() => {
  getSkillDescription.mockImplementation((set: string, skill: string) =>
    Promise.resolve({
      set,
      skill,
      description: TEXT[set as keyof typeof TEXT],
    }),
  );
});
afterEach(() => getSkillDescription.mockReset());

// ADR 0037 step 19.6: the skill's name opens the pop-up; it shows the name, the description of
// that set for that skill, then the terms of the percentage, the server's figures.
describe("SkillPopup_Step19_6_ShowsDescriptionThenTerms", () => {
  it("shows the name, the description, then base, points, race, Dexterity, armour and the total", async () => {
    const { user } = await renderApp(
      <SkillNameButton set="thief" skill={pockets} />,
    );
    await user.click(
      screen.getByRole("button", { name: "Description of Pick pockets" }),
    );
    const dialog = await screen.findByTestId("skill-details");
    expect(within(dialog).getByRole("heading")).toHaveTextContent(
      "Pick pockets",
    );
    const description = await within(dialog).findByTestId("skill-description");
    expect(description).toHaveTextContent("Invented thief text.");
    expect(within(description).getAllByRole("listitem")).toHaveLength(2);
    expect(getSkillDescription).toHaveBeenCalledWith("thief", "pick_pockets");

    const figures = within(dialog).getByTestId("skill-figures");
    const value = (key: string) =>
      within(within(figures).getByTestId(`skill-figure-${key}`)).getByText(
        /./,
        { selector: "dd" },
      ).textContent;
    expect([
      value("base"),
      value("points"),
      value("race"),
      value("dexterity"),
      value("armor"),
      value("total"),
    ]).toEqual(["15", "20", "+5", "−5", "−20", "15%"]);
    // The description comes first, the terms after it.
    expect(
      description.compareDocumentPosition(figures) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("asks for the set's own text: the bard's Pick Pockets is not the thief's", async () => {
    const { user } = await renderApp(
      <SkillNameButton set="bard" skill={pockets} />,
    );
    await user.click(
      screen.getByRole("button", { name: "Description of Pick pockets" }),
    );
    const description = await screen.findByTestId("skill-description");
    expect(description).toHaveTextContent("Invented bard text.");
    expect(description).not.toHaveTextContent("thief");
    expect(getSkillDescription).toHaveBeenCalledWith("bard", "pick_pockets");
  });

  it("reads the text and the terms in French", async () => {
    const { user } = await renderApp(
      <SkillNameButton set="thief" skill={pockets} />,
      { lng: "fr" },
    );
    await user.click(screen.getByRole("button", { name: /^Description de / }));
    const description = await screen.findByTestId("skill-description");
    expect(description).toHaveTextContent("Texte inventé du voleur.");
    expect(screen.getByTestId("skill-figure-dexterity")).toHaveTextContent(
      "Dextérité",
    );
    expect(screen.getByTestId("skill-figure-armor")).toHaveTextContent(
      "Armure",
    );
  });

  it("says so when the description could not be read, and still shows the terms", async () => {
    getSkillDescription.mockRejectedValue(new Error("boom"));
    const { user } = await renderApp(
      <SkillNameButton set="thief" skill={pockets} />,
    );
    await user.click(
      screen.getByRole("button", { name: "Description of Pick pockets" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "The description could not be read.",
    );
    expect(screen.getByTestId("skill-figure-total")).toBeInTheDocument();
  });
});
