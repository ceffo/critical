import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  chooseLanguage,
  openDashboard,
  openSettings,
  openWeapons,
  openWorkspace,
} from "./helpers";

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

// The fighter (ADR 0030 steps 9, 11 and 12), what only a browser proves: the 18/xx field appearing
// at Strength 18, the sheet tile reading 18/50 after the reload, the weapon editor's type and
// specialized controls, and the attack card's rate and tag on the server's numbers. The rules
// (refusals, Table 1 at 18/xx, the bonuses, Tables 15/35/45) are Go tests (charactersvc, buffsvc,
// combatsvc, weaponsvc, ruletablesadpt).
test("create a fighter with 18/50, specialize in a long sword, attack", async ({
  page,
}) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const settings = await openSettings(page);
  await settings
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  const form = page
    .getByTestId("character-dialog")
    .getByTestId("character-form");
  await form.getByLabel("Nom").fill("Borin");
  await form.getByLabel("Classe").selectOption({ label: "Guerrier" });
  await form.getByLabel("Race").selectOption({ label: "Humain" });
  for (const [ability, score] of [
    ["FOR", "18"],
    ["DEX", "14"],
    ["CON", "15"],
    ["INT", "10"],
    ["SAG", "10"],
    ["CHA", "9"],
  ])
    await form.getByLabel(ability, { exact: true }).fill(score);
  await form.getByLabel("Force exceptionnelle (18/xx)").fill("50");
  await form.getByLabel("PV max").fill("10");
  await snap(page, "new-fighter-form-fr");
  await form
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();

  const character = page.getByTestId("character");
  await expect(character).toContainText("Borin");
  await expect(page.getByTestId("str-value")).toHaveText("18/50");

  // The weapon editor and the attack, with the English helpers.
  await chooseLanguage(page, "en");
  await openWorkspace(page, "Inventory");
  await page.getByRole("button", { name: "Add weapon" }).click();
  const wf = page.getByTestId("weapon-form");
  await wf.getByLabel("Name").fill("Long Sword");
  await wf.getByLabel("Damage S-M").fill("1d8");
  await wf.getByLabel("Damage Large").fill("1d12");
  await wf.getByLabel("Weapon type").selectOption("long_sword");
  await wf.getByLabel("Specialized").check();
  await wf.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByTestId("weapon-long_sword")).toContainText(
    "specialized",
  );

  await openWorkspace(page, "Act");
  await openWeapons(page);
  const f = attackForm(page);
  // Level 1, specialized melee: PHB Table 35, 3 attacks every 2 rounds.
  await expect(f.getByTestId("tile-specialized")).toBeVisible();
  await expect(f.getByTestId("tile-rate")).toContainText("3/2 att.");
  await f.getByLabel("Target AC").fill("-10");
  await f.getByRole("button", { name: "Attack", exact: true }).click();
  await expect(
    page
      .getByRole("region", { name: "Detail", exact: true })
      .getByTestId("attack-result"),
  ).toContainText("Specialization");

  await chooseLanguage(page, "fr");
  await expect(page.getByTestId("tile-rate")).toContainText("ce round");
  await snap(page, "fighter-attack-fr");
});
