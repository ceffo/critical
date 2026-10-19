import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  combatLog,
  chooseLanguage,
  createInFlow,
  openDashboard,
  openRoster,
  openWeapons,
  openWorkspace,
  undoEntry,
} from "./helpers";

// Creating a character through the creation route (ADR 0034), then switching, renaming and retiring
// from the roster (ADR 0034 step 7): the one flow only a browser proves. The roster opens from the
// character card's header and its Créer opens the route, whose Quitter returns to the roster; the steps keep their values back and forth, Quitter asks first, the server's rolls
// fill the scores and the HP and stay editable, and the page loads again on the character the server
// made current. French first (ADR 0027 step 2). The placement choice, the step rules and a race
// changed after the roll are unit tests (lib/creationFlow.test.ts, components/creation/arrange.test.ts);
// the validation, the rolls, the arrangement's scores and the options are Go tests (composition
// create_test.go, creationrolls_test.go, creationrollsvc); the no-cut audit is in layout.spec.ts.
test("create a bard through the creation route, switch to the first and back, rename and retire it", async ({
  page,
}) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const character = page.getByTestId("character");
  await expect(character).toContainText("Test Bard");

  // ADR 0034 steps 2 and 7: the roster and the creation are full-screen routes, opened and left
  // without a page load (a marker set on the window survives); Quitter with nothing entered goes
  // straight back to the roster, and Retour to the character.
  await page.evaluate(() => {
    (window as unknown as { stayed: boolean }).stayed = true;
  });
  let roster = await openRoster(page);
  await expect(page).toHaveURL(/\/characters$/);
  await roster
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  await expect(page).toHaveURL(/\/create$/);
  await page.getByRole("button", { name: "Quitter la création" }).click();
  await expect(page).toHaveURL(/\/characters$/);
  await page.getByRole("button", { name: "Revenir à la partie" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(character).toContainText("Test Bard");
  roster = await openRoster(page);
  await roster
    .getByRole("button", { name: "Créer un nouveau personnage" })
    .click();
  await expect(page).toHaveURL(/\/create$/);
  expect(
    await page.evaluate(
      () => (window as unknown as { stayed?: boolean }).stayed,
    ),
  ).toBe(true);
  const step = page.getByTestId("creation-step");
  const next = step.getByRole("button", { name: "Passer à l'étape suivante" });
  const back = step.getByRole("button", {
    name: /^Revenir à l'étape précédente/,
  });

  // Step 1: Suivant waits for a class; a level stands for the class's lowest XP, the server's table.
  await expect(next).toBeDisabled();
  await expect(step.getByTestId("next-blocked")).toHaveText(
    "Choisissez une classe.",
  );
  await step.getByRole("radio", { name: "Barde", exact: true }).click();
  await step.getByRole("radio", { name: "Niveau 3", exact: true }).click();
  await expect(step.getByTestId("level-xp")).toContainText(/2\s500 XP/);
  // Leaving with values entered asks first; Rester keeps them.
  await page.getByRole("button", { name: "Quitter la création" }).click();
  const confirm = page.getByTestId("leave-confirm");
  await expect(confirm).toBeVisible();
  await confirm
    .getByRole("button", { name: "Rester et continuer la création" })
    .click();
  await expect(confirm).toBeHidden();
  // A reload with values entered asks the browser's question; staying keeps them.
  const asked = new Promise<string>((resolve) =>
    page.once("dialog", (d) => {
      resolve(d.type());
      void d.dismiss();
    }),
  );
  await page.evaluate(() => location.reload());
  expect(await asked).toBe("beforeunload");
  await expect(
    step.getByRole("radio", { name: "Barde", exact: true }),
  ).toHaveAttribute("aria-checked", "true");
  // The browser's Back stays in the page (no beforeunload): it asks the same question as Quitter, and
  // Rester keeps the route and the values.
  await page.evaluate(() => history.back());
  await expect(confirm).toBeVisible();
  await expect(page).toHaveURL(/\/create$/);
  await confirm
    .getByRole("button", { name: "Rester et continuer la création" })
    .click();
  await expect(confirm).toBeHidden();
  await expect(
    step.getByRole("radio", { name: "Barde", exact: true }),
  ).toHaveAttribute("aria-checked", "true");
  await next.click();

  // Step 2: the races the bard refuses are shown, marked, not pickable.
  await expect(
    step.getByRole("radio", { name: /^Elfe : pas permis/ }),
  ).toHaveAttribute("aria-disabled", "true");
  await step.getByRole("radio", { name: "Demi-elfe", exact: true }).click();
  await next.click();

  // Step 3: method V, rolled by the server (each total's dice, the lowest struck), placed by the
  // player; the scores it gives stay editable, here typed over with the sheet's.
  await step
    .getByRole("radio", { name: "V 4d6 sans le plus bas, répartir" })
    .click();
  await step
    .getByRole("button", { name: "Tirer les dés de la méthode choisie" })
    .click();
  const roll = step.getByTestId("score-roll");
  await expect(roll.locator(".line-through")).toHaveCount(6);
  const abilities = [
    ["str", "Force", "10"],
    ["dex", "Dextérité", "15"],
    ["con", "Constitution", "12"],
    ["int", "Intelligence", "14"],
    ["wis", "Sagesse", "9"],
    ["cha", "Charisme", "16"],
  ] as const;
  for (const [i, [code]] of abilities.entries()) {
    await step.getByTestId(`score-total-${i}`).click();
    await step.getByTestId(`place-${code}`).click();
  }
  for (const [, name] of abilities)
    await expect(step.getByLabel(name, { exact: true })).toHaveValue(/^\d+$/);
  await snap(page, "creation-scores-rolled-fr");
  for (const [, name, score] of abilities)
    await step.getByLabel(name, { exact: true }).fill(score);
  await next.click();

  // Step 4: the HP roll at level 3 shows one row per level and fills the box, which stays editable.
  await step.getByRole("button", { name: /^Tirer les dés de vie/ }).click();
  await expect(step.getByTestId("hp-roll").locator("tbody tr")).toHaveCount(4);
  await expect(step.getByLabel("PV max")).toHaveValue(/^\d+$/);
  await snap(page, "creation-hp-rolled-fr");
  await step.getByLabel("PV max").fill("6");
  // ADR 0030 step 18: a bard enters its points and the armour worn for its skills.
  await step
    .getByLabel("Armure portée")
    .selectOption({ label: "Métal, jusqu’à la maille" });
  await step.getByLabel("Escalade").fill("15");
  // Back and forward keep the values.
  await back.click();
  await expect(step.getByLabel("Force", { exact: true })).toHaveValue("10");
  await next.click();
  await expect(step.getByLabel("PV max")).toHaveValue("6");
  await expect(step.getByLabel("Escalade")).toHaveValue("15");
  await next.click();

  // Step 5, identity (ADR 0034 step 3.5): Suivant waits for a name, an alignment and a sex.
  const details = step.getByTestId("new-character-details");
  await details.getByLabel("Nom", { exact: true }).fill("Lyra");
  await expect(step.getByTestId("next-blocked")).toHaveText(
    "Choisissez un alignement.",
  );
  // ADR 0032 step 2: the nine alignments as a grid, the four a bard refuses marked; picking one
  // shows the refusal and blocks Suivant.
  const alignments = details.getByTestId("alignment-grid");
  await alignments
    .getByRole("radio", { name: "Loyal bon : interdit au barde" })
    .click();
  await expect(details.getByTestId("alignment-refusal")).toBeVisible();
  await expect(next).toBeDisabled();
  await alignments
    .getByRole("radio", { name: "Neutre bon", exact: true })
    .click();
  await expect(details.getByTestId("alignment-refusal")).toHaveCount(0);
  await details
    .getByRole("radiogroup", { name: "Sexe" })
    .getByRole("radio", { name: "Femme" })
    .click();
  // ADR 0032 step 4: the half-elf's own language (Commun) is locked; five more at Intelligence 14
  // (four allowed) bring the server's warning, which never blocks.
  await expect(details.getByTestId("language-native")).toContainText("Commun");
  const chips = details.getByTestId("language-chips");
  for (const name of ["Elfe", "Gnome", "Petites-gens", "Gobelin", "Orque"])
    await chips.getByRole("checkbox", { name, exact: true }).click();
  await expect(details.getByTestId("languages-warning")).toContainText(
    "5 langues en plus de la maternelle, l'Intelligence 14 en permet 4.",
  );
  // ADR 0032 step 5.3: the server rolls the height for a female half-elf; the range and the dice
  // come from the options.
  await expect(details.getByTestId("range-height")).toContainText(
    "Demi-elfe, femme : 152,4 – 177,8 cm · 2d6",
  );
  await details.getByTestId("details-roll-height").click();
  await expect(
    details.getByTestId("details-roll-height-breakdown"),
  ).toContainText("2d6");
  await expect(details.getByRole("textbox", { name: "Taille" })).toHaveValue(
    /^\d+(,\d)?$/,
  );
  const height = await details
    .getByRole("textbox", { name: "Taille" })
    .inputValue();
  await details.getByRole("textbox", { name: "Âge" }).fill("23");
  await expect(next).toBeEnabled();
  await snap(page, "creation-identity-fr");
  await next.click();

  // Step 6, review: every value as it will be sent, the server's figures (THAC0 19 for a level 3
  // bard, the preview's) and its warning on top; an edit button opens its step.
  const review = step.getByTestId("review");
  await expect(review.getByTestId("review-thac0")).toHaveText("19");
  await expect(review.getByTestId("review-class-level")).toContainText(
    /Barde · niveau 3 \(2\s500 XP\)/,
  );
  await expect(review.getByTestId("review-languages")).toHaveText(
    "Commun, Elfe, Gnome, Petites-gens, Gobelin, Orque",
  );
  await expect(review.getByTestId("review-height")).toHaveText(`${height} cm`);
  await expect(review.getByTestId("review-warning")).toHaveCount(1);
  await expect(step.getByTestId("next-blocked")).toHaveText(
    "1 avertissement, rien de bloquant.",
  );
  await snap(page, "creation-review-fr");
  await review.getByRole("button", { name: "Modifier : Identité" }).click();
  await expect(step).toHaveAttribute("data-step", "identity");
  await expect(details.getByLabel("Nom", { exact: true })).toHaveValue("Lyra");
  await next.click();
  await step
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
  await expect(pop).toContainText(
    "Commun, Elfe, Gnome, Petites-gens, Gobelin, Orque",
  );
  await expect(pop).toContainText(height);
  await page.keyboard.press("Escape");

  // 50 + 15 + 0 (half-elf) - 20 (elven chain column) - 5 (metal armour) = 40, the server's.
  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  await expect(page.getByTestId("bard-percent-climb_walls")).toHaveText("40 %");
  await expect(page.getByTestId("thief-armor")).toHaveValue("metal");
  await snap(page, "new-bard-skills-fr");

  // The roster: a card per character, "En jeu" on the current one, Jouer on the others.
  roster = await openRoster(page);
  const cards = roster.getByTestId("roster-card");
  await expect(cards).toHaveCount(2);
  const current = roster.locator("[data-current]");
  await expect(current).toContainText("Lyra");
  await expect(current).toContainText("En jeu");
  await expect(current).toContainText("Barde · niveau 3");
  await expect(current.getByRole("button", { name: /^Jouer/ })).toHaveCount(0);
  await snap(page, "roster-fr");
  await roster.getByRole("button", { name: "Jouer Test Bard" }).click();
  await expect(character).toContainText("Test Bard");

  roster = await openRoster(page);
  await roster.getByRole("button", { name: "Jouer Lyra" }).click();
  await expect(character).toContainText("Lyra");

  // Rename from the roster: the card shows the new name, and the game too.
  roster = await openRoster(page);
  await roster.getByRole("button", { name: "Renommer Lyra" }).click();
  const dialog = page.getByTestId("character-dialog");
  const rename = dialog.getByTestId("rename-form");
  await expect(rename.getByRole("textbox", { name: "Nom" })).toHaveValue(
    "Lyra",
  );
  await rename.getByRole("textbox", { name: "Nom" }).fill("Lyra la Brave");
  await snap(page, "rename-character-fr");
  await rename
    .getByRole("button", { name: "Enregistrer le nouveau nom de Lyra" })
    .click();
  await expect(dialog).toBeHidden();
  await expect(current).toContainText("Lyra la Brave");
  await page.getByRole("button", { name: "Revenir à la partie" }).click();
  await expect(character).toContainText("Lyra la Brave");

  // Retire the one in play from the roster: the server makes the first one left current.
  roster = await openRoster(page);
  await roster.getByRole("button", { name: /^Retirer Lyra la Brave/ }).click();
  await snap(page, "retire-character-fr");
  await dialog
    .getByTestId("retire-confirm")
    .getByRole("button", { name: /^Retirer Lyra la Brave/ })
    .click();
  await expect(cards).toHaveCount(1);
  await expect(current).toContainText("Test Bard");
  await page.getByRole("button", { name: "Revenir à la partie" }).click();
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
  // The 18/xx field appears on the "Points de vie et talents" step at Strength 18.
  await createInFlow(page, {
    name: "Borin",
    className: "Guerrier",
    race: "Humain",
    scores: { FOR: "18", DEX: "14", CON: "15", INT: "10", SAG: "10", CHA: "9" },
    hpMax: "10",
    extras: async (step) => {
      await step.getByLabel("Pourcentage (18/xx)").fill("50");
      await snap(page, "new-fighter-extras-fr");
    },
  });
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
  await createInFlow(page, {
    name: "Vex",
    className: "Voleur",
    race: "Petite-gens",
    scores: { FOR: "16", DEX: "17", CON: "12", INT: "11", SAG: "9", CHA: "10" },
    hpMax: "5",
    extras: async (step) => {
      await step.getByLabel("Armure portée").selectOption({ label: "Aucune" });
      await step.getByLabel("Crochetage").fill("30");
      await step.getByLabel("Vol à la tire").fill("30");
      await snap(page, "new-thief-extras-fr");
    },
  });

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
  await createInFlow(page, {
    name: "Elminster",
    className: "Magicien",
    race: "Humain",
    scores: { FOR: "9", DEX: "12", CON: "11", INT: "19", SAG: "12", CHA: "10" },
    hpMax: "4",
  });

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
  await createInFlow(page, {
    name: "Frère Anselme",
    className: "Clerc",
    race: "Humain",
    scores: {
      FOR: "12",
      DEX: "10",
      CON: "13",
      INT: "10",
      SAG: "15",
      CHA: "11",
    },
    hpMax: "8",
  });
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
