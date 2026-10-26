import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import {
  apiCall,
  openActPowers,
  openSortsPowers,
  chooseLanguage,
  logEntry,
  nextRound,
  openDashboard,
  reloadSettled,
  undoEntry,
} from "./helpers";
import { expectNoClippedText } from "./noClip";

// Psionic combat from the Powers panel (ADR 0044 steps 4, 5, 8 and 11): a defence mode raised for the
// round in combat and refused out of it, a standing mind blank that stays across Next Round, and a
// defend (the DM's rolls against the current defence) with its result, kept check and Undo. The
// rules, the contest arithmetic and the refusals are Go tests (psionicsvc, composition).
test.describe("a psionicist in psionic combat", () => {
  // The defence and Defend are on Sorts > Pouvoirs, the powers' rows (raise, stop) on Act.
  test.use({ character: psionicist });

  test("raise a defence, stand mind blank, defend, undo", async ({ page }) => {
    test.setTimeout(90_000);
    await openDashboard(page);
    const sorts = () => openSortsPowers(page);
    const act = () => openActPowers(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", {
        picks: [
          { kind: "discipline", id: "telepathy" },
          { kind: "power", id: "mind_blank" },
          { kind: "power", id: "thought_shield" },
        ],
      }),
    ).toBe(true);
    await reloadSettled(page);

    const current = page.getByTestId("current-defence");
    // The defence chips of Act > Powers: one switch per known defence mode.
    const shield = page.getByRole("switch", { name: "Raise Thought Shield" });
    const blank = page.getByRole("switch", { name: "Raise Mind Blank" });
    await sorts();
    await expect(current).toContainText("None");

    // Out of combat a round-long mode cannot be raised (its chip is off, saying why); the standing
    // mind blank can.
    await act();
    await expect(shield).toBeDisabled();
    await expect(shield).toContainText("in combat");
    await sorts();
    await expect(current).toContainText("None");
    await act();
    await blank.click();
    await sorts();
    await expect(current).toContainText("Mind Blank · standing");
    await act();
    await expect(blank).toHaveAttribute("aria-checked", "true");
    await expect(blank).toContainText("standing");

    // It stays across Next Round, and is not charged.
    const pool = page.getByTestId("psp-value");
    await expect(pool).toHaveText("40 / 40");
    await nextRound(page);
    await nextRound(page);
    await sorts();
    await expect(current).toContainText("Mind Blank · standing");
    await expect(pool).toHaveText("40 / 40");

    // In combat a mode raised for the round comes first, and lapses at the next move of the round.
    await act();
    await shield.click();
    await sorts();
    await expect(current).toContainText("Thought Shield · raised this round");
    await expect(pool).toHaveText("39 / 40");
    await act();
    await expect(page.getByTestId("raise-refused")).toHaveCount(0);
    await nextRound(page);
    await sorts();
    await expect(current).toContainText("Mind Blank · standing");

    // Defend: one roll against the standing mind blank, the check kept for the round.
    await sorts();
    await current.getByRole("button", { name: "Defend" }).click();
    const dialog = page.getByRole("dialog");
    const submit = dialog.getByRole("button", { name: "Defend" });
    await expect(submit).toBeDisabled();
    await dialog.getByLabel("Attack mode").selectOption("ego_whip");
    await dialog.getByLabel("Roll 1", { exact: true }).fill("30");
    await expect(submit).toBeDisabled();
    await dialog.getByLabel("Roll 1", { exact: true }).fill("14");
    await dialog.getByLabel("Check 1 succeeded").check();
    await dialog
      .getByLabel("Keep a check for the round")
      .selectOption({ label: "Keep the check against roll 1" });
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(dialog, `defend dialog ${vp.width}`);
      await snap(page, `defend-form-${vp.width}x${vp.height}`);
    }
    await submit.click();
    const result = dialog.getByTestId("defend-result");
    await expect(result).toContainText("Against Mind Blank");
    await expect(result).toContainText("Table 14");
    await expect(result.locator("[data-winner]")).toHaveCount(1);
    await snap(page, "defend-result");
    await page.keyboard.press("Escape");

    // The journal has the contest, and Undo takes it back.
    await page.getByTestId("rail").getByRole("button", { name: "Act" }).click();
    await expect(logEntry(page, "Psychic contest")).toContainText("Mind Blank");
    await snap(page, "defend-journal");
    await undoEntry(page, "Psychic contest");

    // Dismissing the standing mode (the existing Stop) leaves no defence; a defend then names none.
    expect(
      await apiCall(page, "POST", "/api/psionics/powers/mind_blank/stop", {}),
    ).toBe(true);
    await reloadSettled(page);
    await sorts();
    await expect(current).toContainText("None");
    await sorts();
    await current.getByRole("button", { name: "Defend" }).click();
    await dialog.getByLabel("Attack mode").selectOption("ego_whip");
    await dialog.getByLabel("Roll 1", { exact: true }).fill("9");
    await submit.click();
    await expect(dialog.getByTestId("defend-result")).toContainText(
      "No defence",
    );
    await page.keyboard.press("Escape");

    // The same panel in French, at both sizes.
    await chooseLanguage(page, "fr");
    await sorts();
    await expect(current).toContainText("Défense actuelle");
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(current, `current defence fr ${vp.width}`);
      await snap(page, `powers-defence-fr-${vp.width}x${vp.height}`);
    }
    await sorts();
    await current.getByRole("button", { name: "Défendre" }).click();
    await page
      .getByRole("dialog")
      .getByLabel("Mode d’attaque")
      .selectOption("ego_whip");
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(
        page.getByRole("dialog"),
        `defend fr ${vp.width}`,
      );
      await snap(page, `defend-form-fr-${vp.width}x${vp.height}`);
    }
  });
  // The defence's check is worded as it was decided: the kept +1 of a standing mode, the check kept
  // for the round (no die the second time), and Undo / Redo of a Next Round bring a lapsed raise back.
  test("the contest says how it was decided", async ({ page }) => {
    test.setTimeout(40_000);
    await openDashboard(page);
    const sorts = () => openSortsPowers(page);
    const act = () => openActPowers(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", {
        picks: [
          { kind: "discipline", id: "telepathy" },
          { kind: "power", id: "mind_blank" },
          { kind: "power", id: "thought_shield" },
        ],
      }),
    ).toBe(true);
    await reloadSettled(page);
    const current = page.getByTestId("current-defence");
    const chip = (name: string) =>
      page.getByRole("switch", { name: `Raise ${name}` });
    const dialog = page.getByRole("dialog");
    const defend = async (roll: string, keep: boolean) => {
      await sorts();
      await current.getByRole("button", { name: "Defend" }).click();
      await dialog.getByLabel("Attack mode").selectOption("ego_whip");
      await dialog.getByLabel("Roll 1", { exact: true }).fill(roll);
      await dialog.getByLabel("Check 1 succeeded").check();
      if (keep)
        await dialog
          .getByLabel("Keep a check for the round")
          .selectOption({ label: "Keep the check against roll 1" });
      await dialog.getByRole("button", { name: "Defend" }).click();
      return dialog.getByTestId("defend-result");
    };

    // Mind blank raised at round 0, combat started: round 1 shows the kept +1.
    await act();
    await chip("Mind Blank").click();
    await sorts();
    await expect(current).toContainText("Mind Blank · standing");
    await nextRound(page);
    const first = await defend("14", false);
    await expect(first).toContainText("+1 kept");
    await snap(page, "defend-result-kept-plus-one");
    await page.keyboard.press("Escape");

    // The raised row on the panel, in both languages and sizes.
    await act();
    await chip("Thought Shield").click();
    await sorts();
    await expect(current).toContainText("Thought Shield · raised this round");
    await act();
    await expect(chip("Thought Shield")).toContainText("raised this round");
    for (const lang of ["en", "fr"] as const) {
      if (lang === "fr") await chooseLanguage(page, "fr");
      for (const vp of [
        { width: 1920, height: 1080 },
        { width: 2560, height: 1440 },
      ]) {
        await page.setViewportSize(vp);
        await expectNoClippedText(current, `raised ${lang} ${vp.width}`);
        await snap(page, `powers-raised-${lang}-${vp.width}x${vp.height}`);
      }
    }
    await chooseLanguage(page, "en");

    // Two defends in one round, the first keeps its check: the second takes it with no die.
    const kept = await defend("12", true);
    await expect(kept).toContainText("Thought Shield");
    await expect(kept.getByTestId("defend-kept")).toBeVisible();
    await page.keyboard.press("Escape");
    const second = await defend("9", false);
    await expect(second).toContainText("kept for the round, no die");
    await expect(second.locator("[data-winner]")).toHaveCount(1);
    await snap(page, "defend-result-from-kept");
    await page.keyboard.press("Escape");
    await page.getByTestId("rail").getByRole("button", { name: "Act" }).click();
    await expect(logEntry(page, "Psychic contest").first()).toContainText(
      "kept for the round, no die",
    );
    await snap(page, "defend-journal-from-kept");

    // Next Round lowers the raised row; Undo brings it back, Redo lowers it again.
    await nextRound(page);
    await sorts();
    await expect(current).toContainText("Mind Blank · standing");
    await page.getByTestId("rail").getByRole("button", { name: "Act" }).click();
    await page.getByTestId("undo").click();
    await sorts();
    await expect(current).toContainText("Thought Shield · raised this round");
    await page.getByTestId("rail").getByRole("button", { name: "Act" }).click();
    await page.getByTestId("redo").click();
    await sorts();
    await expect(current).toContainText("Mind Blank · standing");
  });
});
