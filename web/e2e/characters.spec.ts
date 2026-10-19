import type { Locator } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  combatLog,
  chooseLanguage,
  openDashboard,
  openSettings,
  openWeapons,
  openWorkspace,
  undoEntry,
} from "./helpers";

// The first alignment and sex the form offers (both are required, ADR 0032 steps 2 and 3).
async function pickIdentity(form: Locator) {
  await form.getByLabel("Alignement").selectOption({ index: 1 });
  await form.getByLabel("Sexe").selectOption({ index: 1 });
}

// Creating, switching and renaming characters from the cog menu (ADR 0029 step 7.2), the one flow
// only a browser proves: the menu's dialogs, the form's selects and details, and the page loading
// again on the character the server made current, its sheet and details on screen. French first (ADR 0027 step 2). The
// validation (abilities, race and class, a cleric, an entered level), the empty start, ownership
// and retiring are Go tests (composition create_test.go); the no-cut audit is in layout.spec.ts.
test("create a second bard with details, switch to the first and back, rename and retire it", async ({
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
  await form.getByLabel("Nom", { exact: true }).fill("Lyra");
  await expect(form.getByLabel("Classe")).toHaveValue("bard");
  await form
    .getByLabel("Race", { exact: true })
    .selectOption({ label: "Demi-elfe" });
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
  // ADR 0030 step 18: a bard enters its points and the armour worn for its skills.
  await form
    .getByLabel("Armure portée")
    .selectOption({ label: "Métal, jusqu’à la maille" });
  await form.getByLabel("Escalade").fill("15");
  // The details go with it: the card shows them after the reload.
  const details = form.getByTestId("new-character-details");
  // ADR 0032 steps 2 and 3: alignment and sex are required picks; a bard is offered only the
  // alignments with a neutral part.
  await expect(
    details.getByLabel("Alignement").getByRole("option", { name: "Loyal bon" }),
  ).toHaveCount(0);
  await details.getByLabel("Alignement").selectOption({ label: "Neutre bon" });
  await details.getByLabel("Sexe").selectOption({ label: "Femme" });
  // ADR 0032 step 4: the half-elf's own language (Commun) is known without picking it and cannot
  // be unticked; Elfique is picked from the race's list.
  const commun = details.getByRole("checkbox", { name: "Commun", exact: true });
  await expect(commun).toBeChecked();
  await expect(commun).toBeDisabled();
  await details.getByRole("checkbox", { name: "Elfique", exact: true }).check();
  await snap(page, "new-character-form-fr");
  await form
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();

  // The page loads again on the new character: its level is the rules' at 2,500 XP.
  await expect(character).toContainText("Lyra");
  await expect(page.getByTestId("hp-value")).toHaveText("6 / 6");
  await page.getByTestId("details-trigger").click();
  const pop = page.getByTestId("details-popover");
  await expect(pop).toContainText("Neutre bon");
  await expect(pop).toContainText("Femme");
  await expect(pop.getByTestId("details-missing")).toHaveCount(0);
  await expect(pop).toContainText("Commun, Elfique");
  await page.keyboard.press("Escape");

  // 50 + 15 + 0 (half-elf) - 20 (elven chain column) - 5 (metal armour) = 40, the server's.
  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  await expect(page.getByTestId("bard-percent-climb_walls")).toHaveText("40 %");
  await expect(page.getByTestId("thief-armor")).toHaveValue("metal");
  await snap(page, "new-bard-skills-fr");

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

  // Rename from the cog menu: the page loads again with the new name.
  settings = await openSettings(page);
  await settings
    .getByRole("button", { name: "Renommer le personnage joué" })
    .click();
  const rename = dialog.getByTestId("rename-form");
  await expect(rename.getByRole("textbox", { name: "Nom" })).toHaveValue(
    "Lyra",
  );
  await rename.getByRole("textbox", { name: "Nom" }).fill("Lyra la Brave");
  await snap(page, "rename-character-fr");
  await rename
    .getByRole("button", { name: "Enregistrer le nouveau nom de Lyra" })
    .click();
  await expect(character).toContainText("Lyra la Brave");

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
  await form.getByLabel("Nom", { exact: true }).fill("Borin");
  await form.getByLabel("Classe").selectOption({ label: "Guerrier" });
  await form
    .getByLabel("Race", { exact: true })
    .selectOption({ label: "Humain" });
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
  await pickIdentity(form);
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

// The thief (ADR 0030 steps 1, 17 and 19), what only a browser proves: the skill points and armour
// fields of the creation form reaching the server, the Skills tab showing the server's percentages,
// a skill roll's result card, and the attack card's backstab option reaching the roll's breakdown.
// The rules (Tables 26 to 30, the cap, the budget, the refusals, the dice counts) are Go tests
// (ruletablesadpt, charactersvc, skillsvc, combatsvc, composition thief_test.go).
test("create a thief, roll a skill, backstab", async ({ page }) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const settings = await openSettings(page);
  await settings
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  const form = page
    .getByTestId("character-dialog")
    .getByTestId("character-form");
  await form.getByLabel("Nom", { exact: true }).fill("Vex");
  await form.getByLabel("Classe").selectOption({ label: "Voleur" });
  await form
    .getByLabel("Race", { exact: true })
    .selectOption({ label: "Petite-gens" });
  for (const [ability, score] of [
    ["FOR", "16"],
    ["DEX", "17"],
    ["CON", "12"],
    ["INT", "11"],
    ["SAG", "9"],
    ["CHA", "10"],
  ])
    await form.getByLabel(ability, { exact: true }).fill(score);
  await form.getByLabel("Armure portée").selectOption({ label: "Aucune" });
  await form.getByLabel("Crochetage").fill("30");
  await form.getByLabel("Vol à la tire").fill("30");
  await form.getByLabel("PV max").fill("5");
  await snap(page, "new-thief-form-fr");
  await pickIdentity(form);
  await form
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();
  await expect(page.getByTestId("character")).toContainText("Vex");

  // 10 + 30 + 5 (halfling) + 10 (Dexterity 17) + 0 (no armour) = 55, the server's.
  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  await expect(page.getByTestId("thief-percent-open_locks")).toHaveText("55 %");
  await page.getByTestId("skill-open_locks").click();
  await expect(
    page.getByTestId("skill-result").getByTestId("skill-vs"),
  ).toHaveText(/^\d+ contre 55\u00a0%$/);

  // The player spends points after creation (step 17): the server's percentage rises, the budget
  // warns (75 of 60 at 1st) without refusing, and the journal's Undo gives the points back.
  await page.getByTestId("thief-points-edit").click();
  await page.getByTestId("thief-points-open_locks").fill("45");
  await snap(page, "thief-points-editor-fr");
  await page.getByTestId("thief-points-save").click();
  await expect(page.getByTestId("thief-percent-open_locks")).toHaveText("70 %");
  await expect(page.getByTestId("skill-budget")).toBeVisible();
  await expect(page.getByTestId("thief-points-editor")).toHaveCount(0);
  await undoEntry(page, "Points de talent : Crochetage 30 → 45");
  await expect(page.getByTestId("thief-percent-open_locks")).toHaveText("55 %");
  await expect(page.getByTestId("skill-budget")).toHaveCount(0);

  // The armour changes in play (step 1): elven chain takes 5 off Open Locks (Table 29), and the
  // choice survives a reload.
  // A refused request leaves the select enabled and the armour as it was.
  await page.route("**/api/characters/*", (route) =>
    route.request().method() === "PATCH"
      ? route.fulfill({ status: 500, body: "boom" })
      : route.continue(),
  );
  await page
    .getByTestId("thief-armor")
    .selectOption({ label: "Cotte de mailles elfique" });
  await expect(page.getByTestId("thief-armor")).toBeEnabled();
  await expect(page.getByTestId("thief-armor")).toHaveValue("none");
  await page.unroute("**/api/characters/*");
  await page
    .getByTestId("thief-armor")
    .selectOption({ label: "Cotte de mailles elfique" });
  await expect(page.getByTestId("thief-percent-open_locks")).toHaveText("50 %");
  await expect(
    page
      .getByText("Armure des talents : Aucune → Cotte de mailles elfique")
      .first(),
  ).toBeVisible();
  await snap(page, "thief-armor-changed-fr");
  await page.reload();
  await page.getByRole("tab", { name: "Compétences" }).click();
  await expect(page.getByTestId("thief-armor")).toHaveValue("elven_chain");
  await expect(page.getByTestId("thief-percent-open_locks")).toHaveText("50 %");

  await chooseLanguage(page, "en");
  await openWorkspace(page, "Inventory");
  await page.getByRole("button", { name: "Add weapon" }).click();
  const wf = page.getByTestId("weapon-form");
  await wf.getByLabel("Name").fill("Dagger");
  await wf.getByLabel("Damage S-M").fill("1d4");
  await wf.getByLabel("Damage Large").fill("1d3");
  await wf.getByRole("button", { name: "Add", exact: true }).click();
  await openWorkspace(page, "Act");
  await openWeapons(page);
  const f = attackForm(page);
  await f.getByTestId("backstab").check();
  await f.getByLabel("Target AC").fill("-10");
  await f.getByRole("button", { name: "Attack", exact: true }).click();
  const result = page
    .getByRole("region", { name: "Detail", exact: true })
    .getByTestId("attack-result");
  await expect(result).toContainText("Backstab");
  await expect(f.getByTestId("backstab")).not.toBeChecked();
  await chooseLanguage(page, "fr");
  await snap(page, "thief-backstab-fr");
});

// The mage (ADR 0030 steps 8 and 20), what only a browser proves: the class in the creation form,
// the lookup's Learn button reaching the server's roll and its answer on screen, the learned spell
// in the preparation editor, then its Cast button. The rules (Table 4's refusals and chance, the
// failure's block, the undo, Table 21) are Go tests (learnsvc, charactersvc, composition
// learn_test.go). The dice are real here: Intelligence 19 learns at 95%, and a failed spell is
// simply followed by the next one.
test("create a mage, learn a spell, prepare and cast it", async ({ page }) => {
  test.setTimeout(60_000);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const settings = await openSettings(page);
  await settings
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  const form = page
    .getByTestId("character-dialog")
    .getByTestId("character-form");
  await form.getByLabel("Nom", { exact: true }).fill("Elminster");
  await form.getByLabel("Classe").selectOption({ label: "Magicien" });
  await form
    .getByLabel("Race", { exact: true })
    .selectOption({ label: "Humain" });
  for (const [ability, score] of [
    ["FOR", "9"],
    ["DEX", "12"],
    ["CON", "11"],
    ["INT", "19"],
    ["SAG", "12"],
    ["CHA", "10"],
  ])
    await form.getByLabel(ability, { exact: true }).fill(score);
  await form.getByLabel("PV max").fill("4");
  await pickIdentity(form);
  await form
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();
  await expect(page.getByTestId("character")).toContainText("Elminster");

  await chooseLanguage(page, "en");
  await openWorkspace(page, "Spells");
  const searchBox = page.getByRole("combobox", { name: "Search spells..." });
  const detail = page.getByTestId("spell-detail");
  let learned = "";
  for (const name of ["Sleep", "Magic Missile", "Burning Hands"]) {
    await searchBox.fill(name);
    await expect(detail.getByRole("heading")).toHaveText(name);
    await detail.getByRole("button", { name: /^Learn this spell/ }).click();
    const result = detail.getByTestId("learn-result");
    await expect(result).toBeVisible();
    if ((await result.textContent())?.startsWith("Learned")) {
      learned = name;
      break;
    }
  }
  expect(learned, "one of three spells learned at 95%").not.toBe("");
  await expect(detail.getByTestId("learn-spell")).toContainText(
    "In the spellbook",
  );
  await chooseLanguage(page, "fr");
  await snap(page, "mage-learned-fr");

  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Spells" }).click();
  await page.getByRole("button", { name: "Prepare spells" }).click();
  const editor = page.getByRole("form", { name: "Prepare today's spells" });
  await editor
    .getByRole("button", { name: `Add ${learned}`, exact: true })
    .click();
  await editor.getByRole("button", { name: "Save preparation" }).click();
  await expect(editor).toHaveCount(0);
  await page.getByRole("button", { name: `Cast ${learned}` }).click();
  await expect(combatLog(page)).toContainText(`Cast ${learned}`);
  await snap(page, "mage-cast");
});

// The cleric (ADR 0030 steps 13 to 16), what only a browser proves: the class in the creation form,
// the preparation editor offering a sphere spell, its Cast button, then the Undead tab reaching the
// server's turning roll and its breakdown on the result card and in the journal. The rules (Table 24
// with the Wisdom bonus, the sphere filter, the spell failure, Table 61 and its dice) are Go tests
// (ruletablesvc, charactersvc, castsvc, turnsvc, composition cleric_test.go). Wisdom 15: no spell
// failure die; the turning d20 is real, so either verdict passes.
test("create a cleric, prepare and cast a spell, turn undead", async ({
  page,
}) => {
  test.setTimeout(60_000);
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const settings = await openSettings(page);
  await settings
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  const form = page
    .getByTestId("character-dialog")
    .getByTestId("character-form");
  await form.getByLabel("Nom", { exact: true }).fill("Frère Anselme");
  await form.getByLabel("Classe").selectOption({ label: "Clerc" });
  await form
    .getByLabel("Race", { exact: true })
    .selectOption({ label: "Humain" });
  for (const [ability, score] of [
    ["FOR", "12"],
    ["DEX", "10"],
    ["CON", "13"],
    ["INT", "10"],
    ["SAG", "15"],
    ["CHA", "11"],
  ])
    await form.getByLabel(ability, { exact: true }).fill(score);
  await form.getByLabel("PV max").fill("8");
  await pickIdentity(form);
  await form
    .getByRole("button", { name: "Créer ce personnage et le jouer" })
    .click();
  await expect(page.getByTestId("character")).toContainText("Frère Anselme");
  await expect(page.getByTestId("character")).toContainText("Clerc");

  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Sorts" }).click();
  await page.getByRole("button", { name: "Préparer les sorts" }).click();
  const editor = page.getByRole("form", { name: "Préparer les sorts du jour" });
  await editor
    .getByRole("button", { name: "Ajouter Bless", exact: true })
    .click();
  await editor
    .getByRole("button", { name: "Enregistrer la préparation" })
    .click();
  await expect(editor).toHaveCount(0);
  await page.getByRole("button", { name: "Lancer Bless" }).click();
  await expect(combatLog(page)).toContainText("Lancé : Bless");

  await page.getByRole("tab", { name: "Renvoi" }).click();
  const turning = page.getByTestId("turn-undead");
  // Table 61 at level 1, as the server serves it: 10 on the d20 for skeletons, a dash for liches.
  await expect(turning.getByTestId("turn-cell-skeleton")).toHaveText("10+");
  await expect(
    turning.getByRole("button", { name: "Repousser : Liche" }),
  ).toBeDisabled();
  await turning.getByRole("button", { name: "Repousser : Squelette" }).click();
  const card = page.getByTestId("turn-result");
  await expect(card).toContainText("Clerc de niveau 1");
  await expect(card).toContainText("10 ou plus au d20");
  await expect(card.getByTestId("verdict")).toHaveText(/^(Repoussés|Échec)$/);
  await expect(combatLog(page)).toContainText(
    "Renvoi des morts-vivants (Squelette)",
  );
  await snap(page, "cleric-turn-fr");

  // A level-up moves the panel to the next Table 61 column without a reload: 1,500 XP reach the
  // cleric's level 2 (Table 23), the hit die is rolled, and skeletons need 7 instead of 10.
  const moi = page.getByRole("complementary", { name: "Personnage" });
  await moi.getByRole("button", { name: /^Niveau 1$/ }).click();
  const add = page.getByRole("region", { name: "Ajouter des XP" });
  await add.getByLabel("Montant").fill("1500");
  await add.getByLabel("Motif").fill("La crypte");
  await add.getByLabel("Motif").press("Enter");
  const prompt = page.getByTestId("level-up");
  await prompt.getByRole("button", { name: /^Lancer/ }).click();
  await expect(prompt).toHaveCount(0);
  await page
    .getByRole("navigation", { name: "Activités" })
    .getByRole("button", { name: "Agir", exact: true })
    .click();
  await page.getByRole("tab", { name: "Renvoi" }).click();
  await expect(turning.getByTestId("turn-cell-skeleton")).toHaveText("7+");
});
