import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import {
  apiCall,
  openActPowers,
  expectUndoOn,
  chooseLanguage,
  combatLog,
  expandRow,
  rowDetail,
  logEntry,
  nextRound,
  openDashboard,
  reloadSettled,
  undoEntry,
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

// A held mind is picked from the list or typed in any case: it is the same row. Another target ends
// the first's tangents, the journal says so, and Undo brings the row back.
test.describe("a psionicist attacking the same mind again", () => {
  test.use({ character: psionicist });

  test("the picker, the case, another target, undo", async ({ page }) => {
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
    const rail = page.getByTestId("rail");
    await openActPowers(page);

    // One attack with a +20 adjustment (both checks pass), then the bar closed.
    async function attack(target: string) {
      await aim(page, target);
      await adjustToMax(page);
      const attackBar = await openAttack(page);
      await attackBar.getByRole("button", { name: "Attack with" }).click();
      const tangents = attackBar.getByTestId("attack-tangents");
      await expect(tangents).toBeVisible();
      const text = (await tangents.textContent()) ?? "";
      await attackBar.getByRole("button", { name: "Close" }).click();
      return text;
    }

    // A natural 20 fails a check whatever the modifier (the dice are the server's), so the first
    // attack wins one or two tangents.
    expect(await attack("Orc")).toMatch(/Orc: tangents 0 → [12]/);

    // The held mind is on the Target band.
    await expect(
      page.getByTestId("held-mind").filter({ hasText: "Orc" }),
    ).toHaveCount(1);
    await snap(page, "attack-form-picker");

    // Another target ends the first one's tangent row; the journal says so; Undo brings it back.
    expect(await attack("Goblin")).toContain("tangents on Orc ended");
    await rail.getByRole("button", { name: "Act" }).click();
    const rows = (name: string) =>
      page.getByTestId("effect-name").filter({ hasText: name });
    await expect(rows("Orc")).toHaveCount(0);
    const goblinRow = logEntry(page, "on Goblin");
    await expandRow(goblinRow);
    await expect(rowDetail(goblinRow)).toContainText("tangents on Orc ended");
    await expectUndoOn(page, "on Goblin");
    await undoEntry(page, "on Goblin");
    await expect(rows("Orc")).toHaveCount(1);
    await expect(rows("Goblin")).toHaveCount(0);

    // Typed in lower case it is the same row, under the row's own label: the seeded dice make the
    // attack made here the one that completes contact (CRITICAL_DICE_SEED, fixtures.ts); no attack
    // is repeated to get it, and the assertion fails loudly if the sequence changes.
    await openActPowers(page);
    const typed = await attack("orc");
    expect(typed).toContain("contact made");
    await rail.getByRole("button", { name: "Act" }).click();
    await expect(rows("Orc")).toHaveCount(1);
    await expect(rows("Orc")).toContainText("contact");
  });
});

// The result card and the header's last result follow the newest psionic roll: an attack takes them
// from the check before it, and a check takes them back, each as its own structured card (die,
// target, verdict as separate parts). Only a browser proves that the card swaps and fits at the
// layout sizes, in French too; the card's parts are Vitest, which roll is newest is Go.
test.describe("the result card after a psionic attack and a power check", () => {
  test.use({ character: psionicist });

  test("attack, then a check: the newest takes the card", async ({ page }) => {
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
    const card = page.locator("section[data-readonly]");
    const last = page.getByTestId("last-result");
    const sizes = [
      { width: 1280, height: 800 },
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ];
    async function fits(name: string) {
      for (const vp of sizes) {
        await page.setViewportSize(vp);
        await expectNoClippedText(card, `${name} ${vp.width}`);
        if (vp.width > 1280)
          await snap(page, `${name}-${vp.width}x${vp.height}`);
      }
    }

    // Contact, a power paid by the table, on an open mind of level 3.
    async function useContact() {
      await aim(page, "Orc", "3");
      await page
        .getByTestId("target-band")
        .getByRole("radio", { name: "Open" })
        .click();
      await rowOf(page, "Contact").getByRole("button").last().click();
      await bar(page)
        .getByTestId("table-form")
        .getByRole("button", { name: "Use Contact" })
        .click();
    }

    // A power check first: its card.
    await useContact();
    await expect(card.getByTestId("power-check-result")).toBeVisible();
    await expect(card.getByTestId("power-check-tiles")).toContainText("PSPs");
    await expect(last).toContainText("Power check");
    await fits("result-power-check");

    // The attack after it replaces both the card and the last result.
    await page
      .getByTestId("target-band")
      .getByRole("radio", { name: "Closed" })
      .click();
    await adjustToMax(page);
    const attackBar = await openAttack(page);
    await attackBar.getByRole("button", { name: "Attack with" }).click();
    await expect(card.getByTestId("psionic-attack-result")).toBeVisible();
    await expect(card.getByTestId("power-check-result")).toHaveCount(0);
    await expect(card.getByTestId("psionic-attack-check").first())
      .toContainText("Defence roll")
      .catch(() => undefined);
    await expect(last).toContainText("Psionic attack");
    await fits("result-psionic-attack");

    // And a check after the attack takes them back.
    await useContact();
    await expect(card.getByTestId("power-check-result")).toBeVisible();
    await expect(card.getByTestId("psionic-attack-result")).toHaveCount(0);
    await expect(last).toContainText("Power check");

    // The same in French.
    await chooseLanguage(page, "fr");
    await expect(last).toContainText("Jet de pouvoir");
    await expect(card.getByTestId("power-check-tiles")).toContainText("PFP");
    await fits("result-power-check-fr");
    const frBar = await openAttack(page, "Pression mentale", "Attaquer avec");
    await frBar.getByRole("button", { name: /^Attaquer avec/ }).click();
    await expect(card.getByTestId("psionic-attack-result")).toBeVisible();
    await fits("result-psionic-attack-fr");
  });
});
