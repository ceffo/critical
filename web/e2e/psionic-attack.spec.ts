import { test, expect, snap } from "./fixtures";
import { psionicist } from "./psionicist";
import {
  apiCall,
  expectUndoOn,
  chooseLanguage,
  logEntry,
  nextRound,
  openDashboard,
  reloadSettled,
  undoEntry,
} from "./helpers";
import { expectNoClippedText } from "./noClip";

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
    await page
      .getByTestId("rail")
      .getByRole("button", { name: "Powers" })
      .click();

    const row = page
      .getByTestId("known-power")
      .filter({ hasText: "Mind Thrust" });
    await row.getByRole("button", { name: "Attack with Mind Thrust" }).click();
    const dialog = page.getByRole("dialog");
    const submit = dialog.getByRole("button", { name: "Attack with" });
    await expect(submit).toBeDisabled();
    await dialog.getByLabel("Opponent").fill("Orc");
    await dialog.getByLabel("Level").fill("5");
    await dialog.getByLabel("Add a modifier").click();
    await dialog.getByLabel("Modifier 1 value").fill("20");
    await dialog.getByLabel("Modifier 1 reason").fill("test");
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(dialog, `attack dialog ${vp.width}`);
      await snap(page, `attack-form-${vp.width}x${vp.height}`);
    }
    await submit.click();
    const result = dialog.getByTestId("attack-result");
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
      await expectNoClippedText(dialog, `attack result ${vp.width}`);
      await snap(page, `attack-result-${vp.width}x${vp.height}`);
    }
    await page.keyboard.press("Escape");

    // The tangent row names the opponent on the active effects.
    await page.getByTestId("rail").getByRole("button", { name: "Act" }).click();
    const effect = page
      .getByTestId("effect-name")
      .filter({ hasText: "Orc" })
      .first();
    await expect(effect).toContainText(/tangents?|contact/);
    await expect(logEntry(page, "Psionic attack")).toContainText("Orc");
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await snap(page, `attack-effects-journal-${vp.width}x${vp.height}`);
    }

    // The same in French.
    await chooseLanguage(page, "fr");
    await expect(effect).toContainText(/tangentes?|contact/);
    await page
      .getByTestId("rail")
      .getByRole("button", { name: "Pouvoirs" })
      .click();
    await page
      .getByTestId("known-power")
      .filter({ hasText: "Pression mentale" })
      .getByRole("button", { name: "Attaquer avec Pression mentale" })
      .click();
    await expect(
      page.getByRole("dialog").getByLabel("Adversaire"),
    ).toBeVisible();
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(
        page.getByRole("dialog"),
        `attack fr ${vp.width}`,
      );
      await snap(page, `attack-form-fr-${vp.width}x${vp.height}`);
    }
    await page.keyboard.press("Escape");
    await page
      .getByTestId("rail")
      .getByRole("button", { name: "Agir" })
      .click();
    await snap(page, "attack-effects-journal-fr");
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
    await rail.getByRole("button", { name: "Powers" }).click();

    // One attack with a +20 modifier (both checks pass), then the dialog closed.
    async function attack(target: string) {
      await page
        .getByTestId("known-power")
        .filter({ hasText: "Mind Thrust" })
        .getByRole("button", { name: "Attack with Mind Thrust" })
        .click();
      const dialog = page.getByRole("dialog");
      await dialog.getByLabel("Opponent").fill(target);
      await dialog.getByLabel("Add a modifier").click();
      await dialog.getByLabel("Modifier 1 value").fill("20");
      await dialog.getByRole("button", { name: "Attack with" }).click();
      const tangents = dialog.getByTestId("attack-tangents");
      await expect(tangents).toBeVisible();
      const text = (await tangents.textContent()) ?? "";
      await page.keyboard.press("Escape");
      return text;
    }

    // A natural 20 fails a check whatever the modifier (the dice are the server's), so the first
    // attack wins one or two tangents.
    expect(await attack("Orc")).toMatch(/Orc: tangents 0 → [12]/);

    // The held mind is on the list.
    await page
      .getByTestId("known-power")
      .filter({ hasText: "Mind Thrust" })
      .getByRole("button", { name: "Attack with Mind Thrust" })
      .click();
    await expect(
      page.getByRole("dialog").locator('#attack-targets option[value="Orc"]'),
    ).toHaveCount(1);
    await snap(page, "attack-form-picker");
    await page.keyboard.press("Escape");

    // Another target ends the first one's tangent row; the journal says so; Undo brings it back.
    expect(await attack("Goblin")).toContain("tangents on Orc ended");
    await rail.getByRole("button", { name: "Act" }).click();
    const rows = (name: string) =>
      page.getByTestId("effect-name").filter({ hasText: name });
    await expect(rows("Orc")).toHaveCount(0);
    await expect(logEntry(page, "on Goblin")).toContainText(
      "tangents on Orc ended",
    );
    await expectUndoOn(page, "on Goblin");
    await undoEntry(page, "on Goblin");
    await expect(rows("Orc")).toHaveCount(1);
    await expect(rows("Goblin")).toHaveCount(0);

    // Typed in lower case it is the same row, under the row's own label: more tangents until contact
    // (a first attack that won one leaves a second to make).
    await rail.getByRole("button", { name: "Powers" }).click();
    let typed = await attack("orc");
    for (let i = 0; i < 3 && !typed.includes("contact made"); i++) {
      typed = await attack("orc");
    }
    expect(typed).toContain("contact made");
    await rail.getByRole("button", { name: "Act" }).click();
    await expect(rows("Orc")).toHaveCount(1);
    await expect(rows("Orc")).toContainText("contact");
  });
});
