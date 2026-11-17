import { afterEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
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

describe("SkillsTab", () => {
  it("rolls in one click with no modifier set", async () => {
    setup();
    await renderApp(<SkillsTab />);
    await userEvent.click(screen.getByTestId("skill-swimming"));
    expect(rollSkill).toHaveBeenCalledWith("swimming", 0, undefined, "");
  });

  it("carries no pre-roll situational controls (the modifier is adjusted on the result)", async () => {
    setup();
    await renderApp(<SkillsTab />);
    expect(screen.queryByTestId("skill-modifier-toggle")).toBeNull();
    expect(screen.queryByTestId("skill-modifier-panel")).toBeNull();
    await userEvent.click(screen.getByTestId("skill-climb_walls"));
    expect(rollSkill).toHaveBeenLastCalledWith("climb_walls", 0, "thief", "");
  });
});
