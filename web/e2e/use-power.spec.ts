import { test, expect, snap } from "./fixtures";
import {
  openDashboard,
  chooseLanguage,
  openActPowers,
  undoEntry,
} from "./helpers";
import { expectNoClippedText } from "./noClip";

// Using a psionic power, end to end in French (ADR 0043 steps 10 to 16): a psionicist built through
// the creation form uses a known power from the Powers panel (the dialog, the breakdown in the
// journal, the PSPs spent), keeps it up across Next Round (the upkeep charged), and stops it. The
// check's arithmetic, the maintenance order and the undo are Go tests; the d20 is the server's, so
// a natural 20 (always a failure) is rolled again.

const scores = [
  ["FOR", "12"],
  ["DEX", "16"],
  ["CON", "14"],
  ["INT", "15"],
  ["SAG", "17"],
  ["CHA", "10"],
] as const;

test("use a known power, keep it up across Next Round, stop it", async ({
  page,
}) => {
  test.setTimeout(90_000);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.goto("/create");
  const step = page.getByTestId("creation-step");
  const next = () =>
    step.getByRole("button", { name: "Passer à l'étape suivante" }).click();

  await step
    .getByRole("radiogroup", { name: "Type de personnage" })
    .getByRole("radio", { name: "Multiclassé", exact: true })
    .click();
  const rows = step.getByTestId("class-rows");
  await rows
    .getByTestId("class-row-0")
    .getByLabel("Classe 1")
    .selectOption("thief");
  await rows.getByTestId("class-row-0").getByLabel("XP").fill("1250");
  await rows
    .getByTestId("class-row-1")
    .getByLabel("Classe 2")
    .selectOption("psionicist");
  await rows.getByTestId("class-row-1").getByLabel("XP").fill("1250");
  await next();
  await step.getByRole("radio", { name: "Humain", exact: true }).click();
  await next();
  await step.getByRole("radio", { name: "Saisir" }).click();
  for (const [ability, score] of scores)
    await step.getByLabel(ability, { exact: true }).fill(score);
  await next();
  await step.getByLabel("PV max").fill("2");
  await next();
  await next();
  await expect(step).toHaveAttribute("data-step", "psionics");
  const panel = step.getByTestId("known-powers");
  await panel.getByRole("button", { name: /^Ajouter Métapsionisme/ }).click();
  await panel.getByRole("searchbox").fill("intensification");
  // By name, not the first row: until the search answers, the first row is the unfiltered list's.
  await panel.getByRole("button", { name: /^Ajouter Intensification/ }).click();
  await expect(panel.getByTestId("known-power")).toHaveCount(1);
  await next();
  await step.getByLabel("Nom", { exact: true }).fill("Aria");
  await step
    .getByTestId("alignment-grid")
    .getByRole("radio", { name: /^[^:]+$/ })
    .first()
    .click();
  await step
    .getByRole("radiogroup", { name: "Sexe" })
    .getByRole("radio")
    .first()
    .click();
  await next();
  await step
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();
  await expect(page.getByTestId("character")).toContainText("Aria");

  await openActPowers(page);
  const row = page
    .getByTestId("power-row")
    .filter({ hasText: "Intensification" });
  await expect(row).toBeVisible();
  await snap(page, "powers-panel");

  const journal = page.getByRole("region", { name: "Journal", exact: true });
  const pool = page.getByTestId("psp-value");
  const before = Number((await pool.innerText()).split("/")[0]);
  const dialog = page.getByRole("dialog");
  let resultText = "";
  for (let attempt = 0; attempt < 6; attempt++) {
    await row.getByRole("button", { name: "Utiliser Intensification" }).click();
    await dialog.getByLabel("Coût initial (PFP)").fill("3");
    // A power with no score of its own takes the player's, set high so the check succeeds.
    await dialog.getByLabel("Score du pouvoir").fill("25");
    await dialog
      .getByRole("button", { name: "Ajouter un modificateur" })
      .click();
    await dialog.getByLabel("Valeur du modificateur 1").fill("2");
    await dialog.getByLabel("Raison du modificateur 1").fill("calme");
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(dialog, `use dialog ${vp.width}`);
      if (attempt === 0)
        await snap(page, `use-dialog-${vp.width}x${vp.height}`);
    }
    await dialog
      .getByRole("button", { name: "Utiliser Intensification" })
      .click();
    const result = dialog.getByTestId("use-result");
    await expect(result).toBeVisible();
    resultText = await result.innerText();
    await snap(page, `use-result-${attempt}`);
    await page.keyboard.press("Escape");
    if (!resultText.includes("20 naturel")) break;
  }
  expect(resultText).toContain("PFP");
  await expect(row.getByTestId("power-maintained")).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 800 });
  await expectNoClippedText(row, "maintained power row 1280");
  await snap(page, "powers-row-maintained-1280x800");
  await page.setViewportSize({ width: 1920, height: 1080 });
  const afterUse = Number((await pool.innerText()).split("/")[0]);
  // The power is kept up, so the check succeeded: the full cost (3), exactly.
  expect(afterUse).toBe(before - 3);

  await page.getByTestId("rail").getByRole("button", { name: "Agir" }).click();
  // The journal words the breakdown: the ability, the labelled modifier.
  const check = journal
    .getByRole("listitem")
    .filter({ hasText: "Intensification" })
    .first();
  await expect(check).toContainText("calme");
  await snap(page, "after-use-1920");

  // Next Round charges the upkeep, named in French.
  // The activation round's own Next Round charges nothing; the one after does.
  await page.getByRole("button", { name: "Tour suivant" }).click();
  await expect(journal.getByRole("region", { name: "Tour 1" })).toBeVisible();
  await page.getByRole("button", { name: "Tour suivant" }).click();
  await expect(
    journal.getByRole("listitem").filter({ hasText: "maintien" }).first(),
  ).toContainText("Intensification 1");
  await expect(row.getByTestId("power-maintained")).toBeAttached();
  const afterRound = Number((await pool.innerText()).split("/")[0]);
  expect(afterRound).toBe(afterUse - 1);

  // Stop: free, the tag goes.
  await openActPowers(page);
  await row
    .getByRole("button", {
      name: "Arrêter le maintien de Intensification",
    })
    .click();
  await expect(row.getByTestId("power-maintained")).toHaveCount(0);
  await page.getByTestId("rail").getByRole("button", { name: "Agir" }).click();
  await expect(
    journal
      .getByRole("listitem")
      .filter({ hasText: "Maintien arrêté : Intensification" }),
  ).toBeVisible();
  expect(Number((await pool.innerText()).split("/")[0])).toBe(afterRound);
  await page.setViewportSize({ width: 2560, height: 1440 });
  await snap(page, "after-stop-2560");

  // Undo the stop from the journal: the tag is back, the PSPs unchanged, the stop line struck.
  await undoEntry(page, "Maintien arrêté : Intensification");
  expect(Number((await pool.innerText()).split("/")[0])).toBe(afterRound);
  await openActPowers(page);
  await expect(row.getByTestId("power-maintained")).toBeVisible();
});
