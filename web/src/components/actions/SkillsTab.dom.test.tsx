import { afterEach, describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderApp } from "@/test/render";
import { useSkillsStore } from "@/stores/skills";
import { useRollStore } from "@/stores/roll";
import { useTableStore } from "@/stores/table";
import type { RogueSkill, Skills } from "@/types/skills";
import { SkillsTab } from "./SkillsTab";

const climb: RogueSkill = {
  id: "climb_walls",
  base: 60,
  points: 10,
  race: 0,
  dexterity: 0,
  armor: 0,
  percent: 70,
  rawPercent: 70,
  maxPercent: 99,
  maxPoints: 40,
  usable: true,
  last: null,
};

const skills: Skills = {
  proficiencies: [
    {
      id: "swimming",
      ability: "str",
      abilityScore: 12,
      modifier: 0,
      target: 12,
      last: null,
    },
  ],
  skillSets: [
    {
      set: "thief",
      name: { en: "Thief skills" },
      skillArmor: "",
      budget: null,
      skills: [climb],
    },
  ],
};

const rollSkill = vi.fn();

function setup() {
  rollSkill.mockResolvedValue(true);
  useSkillsStore.setState({ skills });
  useTableStore.setState({ refresh: vi.fn().mockResolvedValue(undefined) });
  useRollStore.setState({ rollSkill, rolling: false });
}

afterEach(() => {
  rollSkill.mockReset();
  useSkillsStore.setState({ skills: null });
});

describe("SkillsTab situational modifier", () => {
  it("rolls in one click with no modifier set", async () => {
    setup();
    await renderApp(<SkillsTab />);
    await userEvent.click(screen.getByTestId("skill-swimming"));
    expect(rollSkill).toHaveBeenCalledWith("swimming", 0, undefined, "");
  });

  it("steps a proficiency by 1 up to 20 and a percentage by 5 up to 100, and sends the reason", async () => {
    setup();
    await renderApp(<SkillsTab />);
    const [profToggle, thiefToggle] = screen.getAllByTestId(
      "skill-modifier-toggle",
    );
    await userEvent.click(profToggle);
    const prof = screen.getAllByTestId("skill-modifier-panel")[0];
    await userEvent.click(
      within(prof).getByRole("button", { name: "Raise the modifier" }),
    );
    await userEvent.click(
      within(prof).getByRole("button", { name: "Raise the modifier" }),
    );
    await userEvent.type(
      within(prof).getByPlaceholderText("e.g. rear attack"),
      "torchlight",
    );
    await userEvent.click(screen.getByTestId("skill-swimming"));
    expect(rollSkill).toHaveBeenLastCalledWith(
      "swimming",
      2,
      undefined,
      "torchlight",
    );
    // Reset after the roll: no tag, and the next roll carries nothing.
    expect(screen.queryByTestId("skill-situational-tag")).toBeNull();
    await userEvent.click(screen.getByTestId("skill-swimming"));
    expect(rollSkill).toHaveBeenLastCalledWith("swimming", 0, undefined, "");

    await userEvent.click(thiefToggle);
    const thief = screen.getAllByTestId("skill-modifier-panel").at(-1)!;
    const raise = within(thief).getByRole("button", {
      name: "Raise the modifier",
    });
    for (let i = 0; i < 25; i++) await userEvent.click(raise);
    expect(within(thief).getByLabelText("Situational")).toHaveValue("+100");
    expect(raise).toBeDisabled();
    await userEvent.click(screen.getByTestId("skill-climb_walls"));
    expect(rollSkill).toHaveBeenLastCalledWith("climb_walls", 100, "thief", "");
  });
});
