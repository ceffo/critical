import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import { act } from "react";
import userEvent from "@testing-library/user-event";
import { renderApp } from "@/test/render";
import { useCharacterStore } from "@/stores/character";
import { useSkillsEditor } from "@/stores/skillsEditor";
import { useSkillsStore } from "@/stores/skills";
import { useTableStore } from "@/stores/table";
import type { Character } from "@/types/character";
import type { Skills } from "@/types/skills";
import { SkillsPanel } from "./SkillsPanel";

// The editors themselves are covered by their own tests: here only which ones the panel hosts.
vi.mock("@/components/proficiencies/PickEditor", () => ({
  PickEditor: ({ pool }: { pool: string }) => (
    <form data-testid={`pick-${pool}`} />
  ),
}));
vi.mock("@/components/actions/ThiefPointsEditor", () => ({
  ThiefPointsEditor: ({ set }: { set: string }) => (
    <form data-testid={`points-${set}`} />
  ),
}));

function owner(weapon: boolean): Character {
  return {
    raceCode: "human",
    languages: [],
    classes: [{ class: "thief", level: 3 }],
    derived: {
      levelUp: {
        toSpend: { weaponSlots: 0, nonWeaponSlots: 0, skillPoints: 0 },
      },
      skillSets: [],
      proficiencySlots: {
        nonWeapon: { left: 0 },
        picks: [],
        weapon: weapon ? { left: 0 } : undefined,
        weaponPicks: [],
      },
    },
  } as unknown as Character;
}

const set = (code: string, left: number) => ({
  set: code,
  name: { en: code.toUpperCase() },
  skillArmor: "none",
  budget: { left },
  skills: [{ id: "a", points: 1 }],
});

beforeEach(() => {
  useTableStore.setState({ refresh: () => Promise.resolve() } as never);
  Element.prototype.scrollIntoView = vi.fn();
});
afterEach(() => {
  useCharacterStore.setState({ character: null });
  useSkillsStore.setState({ skills: null });
  useSkillsEditor.getState().close();
});

describe("SkillsPanel", () => {
  it("reads every section at once, with no editor open and no points section without a skill set", async () => {
    useCharacterStore.setState({ character: owner(true) });
    useSkillsStore.setState({ skills: { proficiencies: [], skillSets: [] } });
    await renderApp(<SkillsPanel />);
    expect(screen.getByTestId("skills-read")).toBeInTheDocument();
    expect(
      screen.getByTestId("skills-modify-skills-weapon"),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("skills-modify-skills-nonWeapon"),
    ).toBeInTheDocument();
    expect(screen.queryByTestId(/^skills-modify-skills-points-/)).toBeNull();
    expect(screen.queryByTestId(/^pick-/)).toBeNull();
  });

  it("opens one section's editor from its Modify and leaves the reading", async () => {
    useCharacterStore.setState({ character: owner(true) });
    useSkillsStore.setState({ skills: { proficiencies: [], skillSets: [] } });
    await renderApp(<SkillsPanel />);
    await userEvent.click(screen.getByTestId("skills-modify-skills-weapon"));
    expect(screen.getByTestId("pick-weapon")).toBeInTheDocument();
    expect(screen.queryByTestId("pick-nonWeapon")).toBeNull();
    expect(screen.queryByTestId("skills-read")).toBeNull();
  });

  it("leaves out the weapon section for a character with no weapon pool", async () => {
    useCharacterStore.setState({ character: owner(false) });
    await renderApp(<SkillsPanel />);
    expect(screen.queryByTestId("skills-modify-skills-weapon")).toBeNull();
    expect(
      screen.getByTestId("skills-modify-skills-nonWeapon"),
    ).toBeInTheDocument();
  });

  it("reaches each of two skill sets' editors, named", async () => {
    useCharacterStore.setState({ character: owner(false) });
    useSkillsStore.setState({
      skills: {
        proficiencies: [],
        skillSets: [set("thief", 0), set("bard", 5)],
      } as unknown as Skills,
    });
    await renderApp(<SkillsPanel />);
    await userEvent.click(
      screen.getByTestId("skills-modify-skills-points-bard"),
    );
    expect(screen.getByTestId("points-bard")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "BARD" })).toBeInTheDocument();
  });

  it("a level-up link's request opens that editor, takes focus and is cleared", async () => {
    useCharacterStore.setState({ character: owner(true) });
    useSkillsStore.setState({
      skills: {
        proficiencies: [],
        skillSets: [set("thief", 0), set("bard", 5)],
      } as unknown as Skills,
    });
    await renderApp(<SkillsPanel />);
    act(() => useSkillsEditor.getState().open("skillPoints"));
    // No set named: the first with points left.
    expect(screen.getByTestId("skills-points-bard")).toHaveFocus();
    expect(useSkillsEditor.getState().editor).toBeNull();
    act(() => useSkillsEditor.getState().open("weapon"));
    expect(screen.getByTestId("skills-weapon")).toHaveFocus();
    act(() => useSkillsEditor.getState().open("skillPoints", "thief"));
    expect(screen.getByTestId("skills-points-thief")).toHaveFocus();
  });
});
