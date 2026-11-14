import { describe, expect, it } from "vitest";
import { screen, within } from "@testing-library/react";
import { renderApp } from "@/test/render";
import type { CastResult, FormulaRoll } from "@/types/spells";
import { CastResultView, ExposureResultView } from "./CastResultView";

// The card words what the server answered: arithmetic and dice are castsvc's Go tests.

const base: CastResult = {
  logEntryId: 1,
  action: "cast",
  spellId: "fireball",
  spell: "Fireball",
  level: 3,
  casterLevel: 15,
  kind: "damage",
  untracked: [],
};

const roll = (formula: string, total: number, step?: number): FormulaRoll => {
  const [count, sides] = formula.split("+")[0].split("d").map(Number);
  return {
    formula,
    total,
    step,
    dice: Array.from({ length: count }, () => ({ sides, value: 1 })),
  };
};

describe("CastResultView", () => {
  it("shows a damage cast's roll and total, and the save the DM rules on", async () => {
    await renderApp(
      <CastResultView
        result={{
          ...base,
          damage: {
            total: 35,
            type: "fire",
            dice: "10d6",
            instances: 1,
            rolls: [roll("10d6", 35)],
          },
          save: { type: "spell", effect: "half" },
        }}
      />,
    );
    const damage = screen.getByTestId("cast-damage");
    expect(damage).toHaveTextContent("1 × 10d6");
    expect(within(damage).getByTestId("cast-damage-total")).toHaveTextContent(
      "35",
    );
    expect(screen.getByTestId("cast-save")).toHaveTextContent(
      "spell, half damage",
    );
  });

  // ADR 0050 step 13: a specialist's own cast of a spell of his school tells the DM the targets'
  // −1 beside the save, worded from the server's figure; none without it.
  it("words the targets' save modifier of a specialist's school spell", async () => {
    const fireball = (save: {
      type: string;
      effect: string;
      targetSaveModifier?: number;
    }) => <CastResultView result={{ ...base, save }} />;
    const first = await renderApp(
      fireball({ type: "spell", effect: "half", targetSaveModifier: -1 }),
    );
    expect(screen.getByTestId("cast-save")).toHaveTextContent(
      "spell, half damage (targets -1: specialist caster)",
    );
    first.unmount();
    await renderApp(fireball({ type: "spell", effect: "half" }));
    expect(screen.queryByTestId("cast-save-modifier")).toBeNull();
  });

  it("names the cast-time option; a cast with no damage shows neither a damage block nor the text-only banner", async () => {
    await renderApp(
      <CastResultView
        result={{
          ...base,
          spellId: "ice_storm",
          spell: "Ice Storm",
          action: "received",
          mode: "sleet",
        }}
      />,
    );
    expect(screen.getByTestId("cast-mode")).toHaveTextContent("Option: Sleet");
    expect(screen.queryByTestId("cast-damage")).toBeNull();
    expect(screen.queryByTestId("cast-text-only")).toBeNull();
  });

  it("lists a damage made of dice steps one row per round, the smoke rounds without dice", async () => {
    await renderApp(
      <CastResultView
        result={{
          ...base,
          spellId: "incendiary_cloud",
          spell: "Incendiary Cloud",
          action: "received",
          casterLevel: 12,
          damage: {
            total: 20,
            type: "fire",
            stepsPer: "round",
            rolls: [
              { formula: "", dice: [], total: 0, step: 1 },
              { formula: "", dice: [], total: 0, step: 2 },
              roll("12d2", 10, 3),
              roll("12d4", 6, 4),
              roll("12d2", 4, 5),
            ],
          },
        }}
      />,
    );
    const damage = screen.getByTestId("cast-damage");
    expect(damage).toHaveTextContent("5 rounds, one roll each");
    const rows = within(damage).getAllByTestId("cast-damage-roll");
    expect(rows).toHaveLength(5);
    expect(rows[0]).toHaveTextContent("Round 1no damage");
    expect(rows[1]).toHaveTextContent("Round 2no damage");
    expect(rows[2]).toHaveTextContent("Round 3");
    expect(rows[2]).toHaveTextContent("12d2");
    expect(rows[3]).toHaveTextContent("12d4");
    expect(rows[4]).toHaveTextContent("12d2");
  });

  it("lists one strike per instance for a spell that strikes several times", async () => {
    await renderApp(
      <CastResultView
        result={{
          ...base,
          spellId: "chain_lightning",
          spell: "Chain Lightning",
          action: "received",
          damage: {
            total: 60,
            type: "electricity",
            stepsPer: "instance",
            rolls: Array.from({ length: 12 }, (_, i) =>
              roll(`${12 - i}d6`, 5, i + 1),
            ),
          },
        }}
      />,
    );
    const strikes = screen.getAllByTestId("cast-damage-roll");
    expect(strikes).toHaveLength(12);
    expect(strikes[0]).toHaveTextContent("#1");
    expect(strikes[0]).toHaveTextContent("12d6");
    expect(strikes[11]).toHaveTextContent("#12");
    expect(strikes[11]).toHaveTextContent("1d6");
  });

  it("shows a heal on the bearer with his HP before and after", async () => {
    await renderApp(
      <CastResultView
        result={{
          ...base,
          spellId: "cure_light_wounds",
          spell: "Cure Light Wounds",
          level: 1,
          kind: "heal",
          heal: {
            dice: "1d8",
            rolls: [{ sides: 8, value: 5 }],
            constant: 0,
            total: 5,
            hpBefore: 22,
            hpAfter: 27,
          },
        }}
      />,
    );
    expect(screen.getByTestId("cast-heal-total")).toHaveTextContent("5");
    expect(screen.getByTestId("cast-heal-range")).toHaveTextContent(
      "HP 22 → 27",
    );
  });

  it("says a low-confidence spell is text only, and the effect it has is added by hand", async () => {
    await renderApp(
      <CastResultView
        result={{ ...base, kind: "utility", textOnly: "low_confidence" }}
      />,
    );
    expect(screen.getByTestId("cast-text-only")).toHaveTextContent(
      "add an effect by hand",
    );
  });
});

describe("ExposureResultView", () => {
  const exposure = {
    logEntryId: 2,
    spellId: "wall_of_fire",
    spell: "Wall of Fire",
    casterLevel: 11,
    mode: "passing_through",
    round: 1,
    damage: {
      total: 9,
      type: "fire",
      rolls: [roll("2d6", 9)],
    },
    hpDelta: 0,
  };

  it("names the option and tells the DM when the damage is not on the Bard", async () => {
    await renderApp(
      <ExposureResultView
        interactive={false}
        result={{ ...exposure, onSelf: false }}
      />,
    );
    expect(screen.getByTestId("exposure-mode")).toHaveTextContent(
      "Option: Passing through",
    );
    expect(screen.getByTestId("exposure-roll")).toHaveTextContent("2d6");
    expect(screen.getByTestId("exposure-target")).toHaveTextContent(
      "For the DM: your HP is unchanged",
    );
  });

  it("shows what came off the Bard's HP", async () => {
    await renderApp(
      <ExposureResultView
        interactive={false}
        result={{ ...exposure, onSelf: true, hpDelta: -9 }}
      />,
    );
    expect(screen.getByTestId("exposure-target")).toHaveTextContent(
      "Taken off your HP: −9",
    );
    expect(screen.queryByTestId("exposure-floored")).toBeNull();
  });
});
