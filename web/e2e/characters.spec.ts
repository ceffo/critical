import { test, expect, snap } from "./fixtures";
import {
  combatLog,
  chooseLanguage,
  createInFlow,
  openDashboard,
  openRoster,
  openWorkspace,
  showWizardSpells,
} from "./helpers";

// Creating a character through the creation route (ADR 0034), then switching, renaming and retiring
// from the roster (ADR 0034 step 7): the one flow only a browser proves. The roster opens from the
// character card's header and its Créer opens the route, whose Quitter returns to the roster; the steps keep their values back and forth, Quitter asks first, the server's rolls
// fill the scores and the HP and stay editable, and the page loads again on the character the server
// made current. French first (ADR 0027 step 2). The placement choice, the step rules and a race
// changed after the roll are unit tests (components/creation/flow.test.ts, components/creation/arrange.test.ts);
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
  // ADR 0030 step 18: a bard enters its points; its skills read the armour it wears (ADR 0033
  // step 14), none at creation.
  await expect(step.getByLabel("Armure portée")).toHaveCount(0);
  await step.getByLabel("Escalade").fill("15");
  // Back and forward keep the values.
  await back.click();
  await expect(step.getByLabel("Force", { exact: true })).toHaveValue("10");
  await next.click();
  await expect(step.getByLabel("PV max")).toHaveValue("6");
  await expect(step.getByLabel("Escalade")).toHaveValue("15");
  await next.click();
  // The proficiency step (ADR 0037 step 14): no pick, the slots wait to be spent. The languages
  // are picked here (ADR 0034 step 3.5 as amended by ADR 0037 step 18): ADR 0032 step 4, the
  // half-elf's own language (Commun) is locked; ADR 0037 step 6, five more spend five of the level 3
  // bard's 7 non-weapon slots (Intelligence 14), as the server counts.
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  await expect(step.getByTestId("language-native")).toContainText("Commun");
  const chips = step.getByTestId("language-chips");
  for (const name of ["Elfe", "Gnome", "Petites-gens", "Gobelin", "Orque"])
    await chips.getByRole("checkbox", { name, exact: true }).click();
  await expect(step.getByTestId("languages-slots")).toHaveText(
    "5 emplacements · 5 sur 7 au total",
  );
  await snap(page, "creation-proficiencies-languages-fr");
  await next.click();
  // The bard's starting spellbook (ADR 0030 step 20.7): left empty, it never blocks Suivant.
  await expect(step).toHaveAttribute("data-step", "spellbook");
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
  await expect(details.getByTestId("language-chips")).toHaveCount(0);
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
  // An age outside the half-elf's range: the server's warning, which never blocks.
  await details.getByRole("textbox", { name: "Âge" }).fill("500");
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

  // 50 + 15 + 0 (half-elf) + 10 (Table 29's none column: nothing worn) = 75, the server's.
  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Compétences" }).click();
  await expect(page.getByTestId("bard-percent-climb_walls")).toHaveText("75 %");
  await expect(page.getByTestId("skill-armor")).toHaveText(
    "Armure des talents : Aucune",
  );
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
  const retireButton = roster.getByRole("button", {
    name: /^Retirer Lyra la Brave/,
  });
  // Cancel and Escape change nothing and give the focus back to the button that opened the dialog.
  await retireButton.click();
  await expect(dialog.getByTestId("retire-confirm")).toContainText(
    "Lyra la Brave",
  );
  await dialog.getByRole("button", { name: "Annuler" }).click();
  await expect(dialog).toBeHidden();
  await expect(retireButton).toBeFocused();
  await retireButton.click();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(retireButton).toBeFocused();
  await expect(cards).toHaveCount(2);
  await retireButton.click();
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
    // ADR 0030 step 20.7: the starting spellbook, no roll. Detect Magic comes pre-ticked (the
    // fixture has no Read Magic); Light is ticked from the level 1 list, its count against Table
    // 4's maximum (Intelligence 19: none).
    spellbook: async (step) => {
      const picks = step.getByTestId("spellbook-picks");
      await expect(picks).toContainText("Détection de la magie");
      const level1 = step.getByTestId("spellbook-level-1");
      await expect(level1.getByTestId("spellbook-level-count")).toHaveText(
        "1 (sans maximum)",
      );
      await step.getByTestId("spellbook-search").fill("lumi");
      await level1.getByRole("button", { name: "Inscrire Lumière" }).click();
      await expect(level1.getByTestId("spellbook-level-count")).toHaveText(
        "2 (sans maximum)",
      );
      await expect(page.getByTestId("building-spellbook")).toHaveText(
        "Détection de la magie, Lumière",
      );
      await snap(page, "mage-spellbook-fr");
    },
  });

  await chooseLanguage(page, "en");
  await openWorkspace(page, "Spells");
  await showWizardSpells(page);
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
  // The starting spells are in the created mage's book.
  const book = editor.getByRole("region", { name: "Spellbook" });
  for (const name of ["Detect Magic", "Light"])
    await expect(
      book.getByRole("button", { name: `Add ${name}`, exact: true }),
    ).toBeVisible();
  await editor
    .getByRole("button", { name: `Add ${learned}`, exact: true })
    .click();
  await editor.getByRole("button", { name: "Save preparation" }).click();
  await expect(editor).toHaveCount(0);
  await page.getByRole("button", { name: `Cast ${learned}` }).click();
  await expect(combatLog(page)).toContainText(`Cast ${learned}`);
  await snap(page, "mage-cast");
});
