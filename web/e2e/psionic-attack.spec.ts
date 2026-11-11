import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import {
  apiCall,
  openActPowers,
  chooseLanguage,
  combatLog,
  logEntry,
  nextRound,
  openDashboard,
  reloadSettled,
} from "./helpers";
import { expectNoClippedText } from "./noClip";
import type { Page } from "@playwright/test";

// The Act > Powers tab: the target and the adjustment are set on the tab, the attack is sent from the
// action bar that opens under the rows.
const rowOf = (page: Page, name: string) =>
  page.getByTestId("power-row").filter({ hasText: name });
const bar = (page: Page) => page.getByTestId("action-bar");

/** Names the target on the Target band (and its level when given). */
async function aim(page: Page, target: string, level?: string) {
  const band = page.getByTestId("target-band");
  await band.getByTestId("target-name").fill(target);
  if (level) await band.getByLabel("Level", { exact: true }).fill(level);
}

/** Raises the tab's adjustment to +20 (both checks pass whatever the die, bar a natural 20). */
async function adjustToMax(page: Page) {
  const value = page.getByTestId("power-adjustment");
  const more = page.getByRole("button", { name: "Raise the adjustment" });
  for (let i = 0; i < 20 && (await value.innerText()) !== "+20"; i++)
    await more.click();
}

/** Opens the bar of Mind Thrust and returns it. */
async function openAttack(
  page: Page,
  name = "Mind Thrust",
  verb = "Attack with",
) {
  // The row's button toggles the bar: leave it alone when this power's is already open.
  if ((await bar(page).getByTestId("attack-form").count()) === 0)
    await rowOf(page, name)
      .getByRole("button", { name: `${verb} ${name}` })
      .click();
  await expect(bar(page).getByTestId("attack-form")).toBeVisible();
  return bar(page);
}

// An attack mode used on a closed mind from the Powers panel (ADR 0044 step 7): the form, the answer
// with its checks and the tangents, the opponent's row on the active effects and the journal line.
// The dice are the server's; a +20 modifier makes a check pass whatever the die. The rules, the
// contests and the refusals are Go tests (psionicsvc, composition).
test.describe("a psionicist attacking a closed mind", () => {
  test.use({ character: psionicist });

  test("attack, tangents on the opponent, journal", async ({ page }) => {
    test.setTimeout(90_000);
    await openDashboard(page);
    expect(
      await apiCall(page, "PUT", "/api/psionics/picks", {
        picks: [
          { kind: "discipline", id: "telepathy" },
          { kind: "power", id: "mindlink" },
          { kind: "power", id: "contact" },
          { kind: "power", id: "mind_thrust" },
        ],
      }),
    ).toBe(true);
    await reloadSettled(page);
    await nextRound(page);
    await openActPowers(page);

    const attackBar = await openAttack(page);
    const submit = attackBar.getByRole("button", { name: "Attack with" });
    // No target named: nothing to send.
    await expect(submit).toBeDisabled();
    await aim(page, "Orc", "5");
    await adjustToMax(page);
    await expect(submit).toBeEnabled();
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(attackBar, `attack bar ${vp.width}`);
      await snap(page, `attack-form-${vp.width}x${vp.height}`);
    }
    await submit.click();
    const result = attackBar.getByTestId("attack-result");
    await expect(result).toContainText("Check 1");
    await expect(
      result.locator("[data-winner=attacker]").first(),
    ).toBeVisible();
    await expect(result.getByTestId("attack-tangents")).toContainText("Orc");
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(attackBar, `attack result ${vp.width}`);
      await snap(page, `attack-result-${vp.width}x${vp.height}`);
    }

    // The tangent row names the opponent on the active effects.
    await page.getByTestId("rail").getByRole("button", { name: "Act" }).click();
    const effect = page
      .getByTestId("effect-name")
      .filter({ hasText: "Orc" })
      .first();
    await expect(effect).toContainText(/tangents?|contact/);
    // The journal row is the outcome first; the dice are on its detail, opened by keyboard.
    const attackRow = logEntry(page, "on Orc");
    await expect(attackRow).toHaveAttribute("data-kind", "psionic_attack");
    await expect(attackRow.getByTestId("log-detail")).toHaveCount(0);
    const expand = attackRow.getByTestId("log-expand");
    await expect(expand).toHaveAttribute("aria-expanded", "false");
    await expand.focus();
    await page.keyboard.press("Enter");
    await expect(expand).toHaveAttribute("aria-expanded", "true");
    await expect(attackRow.getByTestId("log-detail")).toContainText("d20");
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(combatLog(page), `attack journal ${vp.width}`);
      await snap(page, `attack-effects-journal-${vp.width}x${vp.height}`);
    }

    // The same in French.
    await chooseLanguage(page, "fr");
    await expect(effect).toContainText(/tangentes?|contact/);
    await openActPowers(page);
    const frBar = await openAttack(page, "Pression mentale", "Attaquer avec");
    await expect(
      frBar.getByRole("button", { name: /^Attaquer avec/ }),
    ).toBeVisible();
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(frBar, `attack fr ${vp.width}`);
      await snap(page, `attack-form-fr-${vp.width}x${vp.height}`);
    }
    await page
      .getByTestId("rail")
      .getByRole("button", { name: "Agir" })
      .click();
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(combatLog(page), `journal fr ${vp.width}`);
      await snap(page, `attack-effects-journal-fr-${vp.width}x${vp.height}`);
    }
  });
});
