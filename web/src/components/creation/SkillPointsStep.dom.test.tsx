import { describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { CreationDraft, CreationOptions } from "@/types/character";
import { EMPTY_DRAFT, type StepProps } from "./flow";
import { SkillPointsStep } from "./skillPoints";

// No fixture class grants two skill sets, so the draft answer is built here (ADR 0040 step 3.5).
const set = (code: string, skills: string[]) => ({
  set: code,
  name: { en: `${code} skills` },
  skills,
  budget: { level: 1, available: 60, spent: 0, left: 60, warnings: [] },
});
const answer = {
  skillSets: [
    set("thief", ["open_locks", "pick_pockets"]),
    set("bard", ["pick_pockets", "climb_walls"]),
  ],
} as unknown as CreationDraft;

function props(over: Partial<StepProps> = {}): StepProps {
  return {
    options: {
      skillPoints: { min: 0, max: 95 },
    } as unknown as CreationOptions,
    draft: { ...EMPTY_DRAFT, cls: "thief" },
    update: vi.fn(),
    error: null,
    errorFor: () => null,
    xp: "0",
    watch: () => () => true,
    answer: { current: answer, latest: answer, failed: null },
    refusals: [],
    body: null,
    wording: (e) => `worded ${e.field}`,
    language: "en",
    ...over,
  };
}

describe("SkillPointsStep with two skill sets", () => {
  it("shows one board per set, the shared skill in each under its own slider", async () => {
    await renderApp(<SkillPointsStep {...props()} />);
    const thief = screen.getByTestId("thief-skills-entry");
    const bard = screen.getByTestId("bard-skills-entry");
    // One slider and one typed box per skill.
    expect(within(thief).getAllByRole("slider")).toHaveLength(2);
    expect(within(bard).getAllByRole("textbox")).toHaveLength(2);
    expect(thief.querySelector("#thief-points-pick_pockets")).not.toBeNull();
    expect(bard.querySelector("#bard-points-pick_pockets")).not.toBeNull();
  });

  it("types into the set it belongs to and shows a refusal beside its skill only", async () => {
    const update = vi.fn();
    const { user } = await renderApp(
      <SkillPointsStep
        {...props({
          update,
          errorFor: (f) =>
            f === "skillPoints.bard.pick_pockets" ? "over the limit" : null,
        })}
      />,
    );
    const bard = screen.getByTestId("bard-skills-entry");
    expect(within(bard).getByRole("alert")).toHaveTextContent("over the limit");
    expect(
      within(screen.getByTestId("thief-skills-entry")).queryByRole("alert"),
    ).toBeNull();
    await user.type(within(bard).getAllByRole("textbox")[0]!, "5");
    expect(update).toHaveBeenLastCalledWith({
      skillPoints: { bard: { pick_pockets: "5" } },
    });
  });

  it("shows a refusal of the set itself under its board", async () => {
    await renderApp(
      <SkillPointsStep
        {...props({
          errorFor: (f) =>
            f === "skillPoints.bard" ? "over the bard budget" : null,
        })}
      />,
    );
    expect(
      within(screen.getByTestId("bard-skills-entry")).getByText(
        "over the bard budget",
      ),
    ).toBeInTheDocument();
  });
});
