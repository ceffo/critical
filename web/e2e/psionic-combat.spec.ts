import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import {
  apiCall,
  openActPowers,
  chooseLanguage,
  logEntry,
  nextRound,
  openDashboard,
  reloadSettled,
  undoEntry,
} from "./helpers";

// Psionic combat from the Powers panel (ADR 0044 steps 4, 5, 8 and 11): a defence mode raised for the
// round in combat and refused out of it, a standing mind blank that stays across Next Round, and a
// defend (the DM's rolls against the current defence) with its result, kept check and Undo. The
// rules, the contest arithmetic and the refusals are Go tests (psionicsvc, composition).
test.describe("a psionicist in psionic combat", () => {
  // The defence band, Incoming attack and the powers' rows (raise, stop) are all on Act > Pouvoirs.
  test.use({ character: psionicist });

  const PICKS = [
    { kind: "discipline", id: "telepathy" },
    { kind: "power", id: "mind_blank" },
    { kind: "power", id: "thought_shield" },
  ];
  const chip = (page: Page, name: string) =>
    page.getByRole("switch", { name: `Raise ${name}` });
  // One incoming attack from the bar: the DM's mode and roll, then Resolve; the result stays in the bar.
  async function defend(page: Page, roll: string, keep: boolean) {
    const tab = page.getByTestId("powers-tab");
    const bar = page.getByTestId("action-bar");
    if (!(await bar.isVisible()))
      await tab.getByTestId("incoming-attack").click();
    await bar.getByLabel("Attack mode").selectOption("ego_whip");
    await bar.getByLabel("Roll 1", { exact: true }).fill(roll);
    await bar.getByLabel("Check 1 succeeded").check();
    if (keep)
      await bar
        .getByLabel("Keep a check for the round")
        .selectOption({ label: "Keep the check against roll 1" });
    await bar.getByRole("button", { name: "Resolve" }).click();
    return bar.getByTestId("defend-result");
  }

  test("raise a defence, stand mind blank, defend, undo", async ({ page }) => {
    test.setTimeout(90_000);
    await openDashboard(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", { picks: PICKS }),
    ).toBe(true);
    await reloadSettled(page);
    const tab = await openActPowers(page);
    const shield = chip(page, "Thought Shield");
    const blank = chip(page, "Mind Blank");
    await expect(blank).toHaveAttribute("data-defence", "down");

    // Out of combat a round-long mode cannot be raised (its chip is off, saying why); the standing
    // mind blank can, and then shows a Stop.
    await expect(shield).toBeDisabled();
    await expect(shield).toContainText("in combat");
    await blank.click();
    await expect(blank).toHaveAttribute("aria-checked", "true");
    await expect(blank).toContainText("standing");
    await expect(blank).toHaveAttribute("data-defence", "standing");

    // It stays across Next Round, and is not charged.
    const pool = page.getByTestId("psp-value");
    await expect(pool).toHaveText("40 / 40");
    await nextRound(page);
    await nextRound(page);
    await expect(blank).toHaveAttribute("data-defence", "standing");
    await expect(pool).toHaveText("40 / 40");

    // In combat a mode raised for the round comes first, and lapses at the next move of the round.
    await shield.click();
    await expect(shield).toHaveAttribute("data-defence", "raised");
    await expect(shield).toContainText("raised this round");
    await expect(pool).toHaveText("39 / 40");
    await expect(page.getByTestId("raise-refused")).toHaveCount(0);
    await nextRound(page);
    await expect(shield).toHaveAttribute("data-defence", "down");
    await expect(blank).toHaveAttribute("data-defence", "standing");

    // Defend: one roll against the standing mind blank, the check kept for the round.
    await tab.getByTestId("incoming-attack").click();
    const bar = page.getByTestId("action-bar");
    const submit = bar.getByRole("button", { name: "Resolve" });
    await expect(submit).toBeDisabled();
    await bar.getByLabel("Attack mode").selectOption("ego_whip");
    await bar.getByLabel("Roll 1", { exact: true }).fill("30");
    await expect(submit).toBeDisabled();
    await bar.getByLabel("Roll 1", { exact: true }).fill("14");
    await bar.getByLabel("Check 1 succeeded").check();
    await bar
      .getByLabel("Keep a check for the round")
      .selectOption({ label: "Keep the check against roll 1" });
    await submit.click();
    const result = bar.getByTestId("defend-result");
    await expect(result).toContainText("Against Mind Blank");
    await expect(result).toContainText("Table 14");
    await expect(result.locator("[data-winner]")).toHaveCount(1);
    await snap(page, "defend-result");

    // The journal has the contest, and Undo takes it back.
    await expect(logEntry(page, "Psychic contest")).toContainText("Mind Blank");
    await snap(page, "defend-journal");
    await undoEntry(page, "Psychic contest");

    // Stop ends the standing mode: no defence is raised, and a defend then names none.
    await expect(blank).toHaveAttribute("data-defence", "standing");
    await tab
      .getByRole("button", { name: "Stop maintaining Mind Blank" })
      .click();
    await expect(blank).toHaveAttribute("data-defence", "down");
    await expect(
      tab.getByRole("button", { name: "Stop maintaining Mind Blank" }),
    ).toHaveCount(0);
    await defend(page, "9", false);
    await expect(page.getByTestId("defend-refused")).toContainText(
      "no defence",
    );
  });

  // The defence's check is worded as it was decided: the kept +1 of a standing mode, the check kept
  // for the round (no die the second time), and Undo / Redo of a Next Round bring a lapsed raise back.
  test("the contest says how it was decided", async ({ page }) => {
    test.setTimeout(60_000);
    await openDashboard(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", { picks: PICKS }),
    ).toBe(true);
    await reloadSettled(page);
    await openActPowers(page);
    const bar = page.getByTestId("action-bar");

    // Mind blank raised at round 0, combat started: round 1 shows the kept +1.
    await chip(page, "Mind Blank").click();
    await nextRound(page);
    const first = await defend(page, "14", false);
    await expect(first).toContainText("+1 kept");
    await snap(page, "defend-result-kept-plus-one");
    await bar.getByRole("button", { name: "Close" }).click();

    // The raised row, in both languages and sizes.
    await chip(page, "Thought Shield").click();
    await expect(chip(page, "Thought Shield")).toContainText(
      "raised this round",
    );
    await chooseLanguage(page, "en");
    await snap(page, "powers-raised-en");

    // Two defends in one round, the first keeps its check: the second takes it with no die.
    const kept = await defend(page, "12", true);
    await expect(kept).toContainText("Thought Shield");
    await expect(kept.getByTestId("defend-kept")).toBeVisible();
    await bar.getByRole("button", { name: "Close" }).click();
    const second = await defend(page, "9", false);
    await expect(second).toContainText("kept for the round, no die");
    await expect(second.locator("[data-winner]")).toHaveCount(1);
    await snap(page, "defend-result-from-kept");
    await expect(logEntry(page, "Psychic contest").first()).toContainText(
      "kept for the round, no die",
    );
    await snap(page, "defend-journal-from-kept");

    // Next Round lowers the raised row; Undo brings it back, Redo lowers it again.
    await nextRound(page);
    await expect(chip(page, "Thought Shield")).toHaveAttribute(
      "data-defence",
      "down",
    );
    await page.getByTestId("undo").click();
    await expect(chip(page, "Thought Shield")).toHaveAttribute(
      "data-defence",
      "raised",
    );
    await page.getByTestId("redo").click();
    await expect(chip(page, "Thought Shield")).toHaveAttribute(
      "data-defence",
      "down",
    );
  });
});
