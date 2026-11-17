import { describe, expect, it } from "vitest";
import { screen } from "@testing-library/react";
import { renderApp } from "@/test/render";
import { SkillPointsBoard } from "./SkillPointsBoard";
import type { SkillBudget } from "@/types/skills";

const budget: SkillBudget = {
  level: 1,
  first: 60,
  perLevel: 30,
  available: 60,
  spent: 20,
  left: 40,
  perSkillFirst: 30,
  perSkillPerLevel: 15,
  perSkillCap: 30,
};

const rows = [
  {
    id: "pick_pockets",
    base: 15,
    points: 20,
    race: 0,
    dexterity: 0,
    armor: 0,
    rawPercent: 35,
    percent: 35,
  },
];

const show = (errorFor: (id: string) => string | null) =>
  renderApp(
    <SkillPointsBoard
      setId="thief"
      skillIds={["pick_pockets", "open_locks"]}
      rows={rows}
      budget={budget}
      maxPercent={95}
      values={{ pick_pockets: "20", open_locks: "" }}
      onChange={() => {}}
      errorFor={errorFor}
      name={(id) => <span>{id}</span>}
    />,
  );

describe("SkillPointsBoard", () => {
  it("draws a slider and a box per skill, the unanswered one from zero", async () => {
    await show(() => null);
    expect(screen.getByTestId("thief-points-pick_pockets")).toHaveValue("20");
    expect(screen.getByTestId("thief-points-open_locks")).toHaveValue("");
    expect(
      screen.getByTestId("thief-points-slider-pick_pockets"),
    ).toHaveAttribute("max", "30");
    expect(screen.getByTestId("skill-points-over")).toHaveTextContent("");
  });

  it("words a refusal beside its skill and marks only that box invalid", async () => {
    await show((id) => (id === "pick_pockets" ? "Too many points" : null));
    const bad = screen.getByTestId("thief-points-pick_pockets");
    expect(bad).toHaveAttribute("aria-invalid", "true");
    expect(bad).toHaveAttribute(
      "aria-describedby",
      "skill-error-thief-pick_pockets",
    );
    expect(screen.getByTestId("thief-points-open_locks")).not.toHaveAttribute(
      "aria-invalid",
    );
  });
});
