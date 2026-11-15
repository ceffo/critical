import { describe, expect, it, vi } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { CreationDraft, CreationOptions } from "@/types/character";
import { EMPTY_DRAFT, type StepProps } from "./flow";
import { ExtrasStep } from "./steps";

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
      hpMax: { min: 1, max: 100 },
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

describe("ExtrasStep with two skill sets", () => {
  it("shows one block per set, the shared skill in each under its own field", async () => {
    await renderApp(<ExtrasStep {...props()} />);
    const thief = screen.getByTestId("thief-skills-entry");
    const bard = screen.getByTestId("bard-skills-entry");
    expect(within(thief).getAllByRole("textbox")).toHaveLength(2);
    expect(within(bard).getAllByRole("textbox")).toHaveLength(2);
    expect(
      thief.querySelector("#new-character-skillPoints\\.thief\\.pick_pockets"),
    ).not.toBeNull();
    expect(
      bard.querySelector("#new-character-skillPoints\\.bard\\.pick_pockets"),
    ).not.toBeNull();
  });

  it("types into the set it belongs to and shows a refusal in that set's block only", async () => {
    const update = vi.fn();
    const { user } = await renderApp(
      <ExtrasStep
        {...props({
          update,
          errorFor: (f) =>
            f === "skillPoints.bard" ? "over the bard budget" : null,
        })}
      />,
    );
    const bard = screen.getByTestId("bard-skills-entry");
    expect(within(bard).getByRole("alert")).toHaveTextContent(
      "over the bard budget",
    );
    expect(
      within(screen.getByTestId("thief-skills-entry")).queryByRole("alert"),
    ).toBeNull();
    await user.type(
      bard.querySelector<HTMLInputElement>(
        "#new-character-skillPoints\\.bard\\.pick_pockets",
      )!,
      "5",
    );
    expect(update).toHaveBeenLastCalledWith({
      skillPoints: { bard: { pick_pockets: "5" } },
    });
  });

  it("shows a refusal on a set the draft does not list on the step", async () => {
    await renderApp(
      <ExtrasStep
        {...props({
          refusals: [{ field: "skillPoints.fence", code: "not_allowed" }],
        })}
      />,
    );
    expect(screen.getByText("worded skillPoints.fence")).toBeInTheDocument();
  });
});
