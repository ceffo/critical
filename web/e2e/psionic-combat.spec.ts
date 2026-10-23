import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import {
  apiCall,
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
  test.use({ character: psionicist });

  test("raise a defence, stand mind blank, defend, undo", async ({ page }) => {
    test.setTimeout(90_000);
    await openDashboard(page);
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
    await page
      .getByTestId("rail")
      .getByRole("button", { name: "Powers" })
      .click();

    const current = page.getByTestId("current-defence");
    const shield = page
      .getByTestId("known-power")
      .filter({ hasText: "Thought Shield" });
    const blank = page
      .getByTestId("known-power")
      .filter({ hasText: "Mind Blank" });
    await expect(current).toContainText("None");

    // Out of combat a round-long mode is refused, worded; the standing mind blank is not.
    await shield.getByRole("button", { name: "Raise Thought Shield" }).click();
    await expect(shield.getByTestId("raise-refused")).toContainText(
      "no round is running",
    );
    await expect(current).toContainText("None");
    await blank.getByRole("button", { name: "Raise Mind Blank" }).click();
    await expect(current).toContainText("Mind Blank · standing");
    await expect(blank.getByTestId("power-maintained")).toBeVisible();

    // It stays across Next Round, and is not charged.
    const pool = page.getByTestId("psp-value");
    await expect(pool).toHaveText("40 / 40");
    await nextRound(page);
    await nextRound(page);
    await expect(current).toContainText("Mind Blank · standing");
    await expect(pool).toHaveText("40 / 40");

    // In combat a mode raised for the round comes first, and lapses at the next move of the round.
    await shield.getByRole("button", { name: "Raise Thought Shield" }).click();
    await expect(current).toContainText("Thought Shield · raised this round");
    await expect(pool).toHaveText("38 / 40");
    await expect(shield.getByTestId("raise-refused")).toHaveCount(0);
    await nextRound(page);
    await expect(current).toContainText("Mind Blank · standing");

    // Defend: one roll against the standing mind blank, the check kept for the round.
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
    await page
      .getByTestId("rail")
      .getByRole("button", { name: "Powers" })
      .click();
    await blank
      .getByRole("button", { name: "Stop maintaining Mind Blank" })
      .click();
    await expect(current).toContainText("None");
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
    await page
      .getByTestId("rail")
      .getByRole("button", { name: "Pouvoirs" })
      .click();
    await expect(current).toContainText("Défense actuelle");
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(current, `current defence fr ${vp.width}`);
      await snap(page, `powers-defence-fr-${vp.width}x${vp.height}`);
    }
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
});
