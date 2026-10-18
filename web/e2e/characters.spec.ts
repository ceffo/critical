import { test, expect, snap } from "./fixtures";
import { chooseLanguage, openDashboard, openSettings } from "./helpers";

// Creating and switching characters from the cog menu (ADR 0029 step 7.2), the one flow only a
// browser proves: the menu's dialogs, the form's selects, and the page loading again on the
// character the server made current, its sheet on screen. French first (ADR 0027 step 2). The
// validation (abilities, race and class, a cleric, an entered level), the empty start, ownership
// and retiring are Go tests (composition create_test.go); the no-cut audit is in layout.spec.ts.
test("create a second bard, switch to the first and back, retire it", async ({
  page,
}) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const character = page.getByTestId("character");
  await expect(character).toContainText("Test Bard");

  let settings = await openSettings(page);
  await settings
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  const dialog = page.getByTestId("character-dialog");
  const form = dialog.getByTestId("character-form");
  await form.getByLabel("Nom").fill("Lyra");
  await expect(form.getByLabel("Classe")).toHaveValue("bard");
  await form.getByLabel("Race").selectOption({ label: "Demi-elfe" });
  for (const [ability, score] of [
    ["FOR", "10"],
    ["DEX", "15"],
    ["CON", "12"],
    ["INT", "14"],
    ["SAG", "9"],
    ["CHA", "16"],
  ])
    await form.getByLabel(ability, { exact: true }).fill(score);
  await form.getByLabel("PV max").fill("6");
  await form.getByLabel("XP de départ").fill("2500");
  await snap(page, "new-character-form-fr");
  await form
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();

  // The page loads again on the new character: its level is the rules' at 2,500 XP.
  await expect(character).toContainText("Lyra");
  await expect(page.getByTestId("hp-value")).toHaveText("6 / 6");

  settings = await openSettings(page);
  await settings
    .getByRole("button", { name: /^Changer de personnage/ })
    .click();
  const list = dialog.getByTestId("character-list");
  await expect(list.getByRole("listitem")).toHaveCount(2);
  await expect(list.locator("[data-current]")).toContainText("Lyra");
  await expect(list.locator("[data-current]")).toContainText(
    "Barde · niveau 3",
  );
  await snap(page, "switch-character-fr");
  await list.getByRole("button", { name: "Jouer Test Bard" }).click();
  await expect(character).toContainText("Test Bard");

  settings = await openSettings(page);
  await settings
    .getByRole("button", { name: /^Changer de personnage/ })
    .click();
  await list.getByRole("button", { name: "Jouer Lyra" }).click();
  await expect(character).toContainText("Lyra");

  settings = await openSettings(page);
  await settings
    .getByRole("button", { name: /^Retirer le personnage joué/ })
    .click();
  await snap(page, "retire-character-fr");
  await dialog
    .getByTestId("retire-confirm")
    .getByRole("button", {
      name: /^Retirer Lyra/,
    })
    .click();
  await expect(character).toContainText("Test Bard");
});
