import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { openDashboard, chooseLanguage } from "./helpers";

// The class step's multi-class and dual-class entries (ADR 0042 steps 1, 2 and 4), end to end in
// French: what only a browser proves is the wiring from the entries typed to the body the draft and
// Créer send, the character the sheet then shows, and the layout of the rows. The rules (which
// combinations are allowed, the XP, the refusals) are Go tests (charactersvc, composition).

const scores = [
  ["FOR", "17"],
  ["DEX", "16"],
  ["CON", "15"],
  ["INT", "17"],
  ["SAG", "16"],
  ["CHA", "10"],
] as const;

/** Opens the creation route and picks the kind of character on the class step. */
async function openClassStep(page: Page, kind: string) {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await page.goto("/create");
  const step = page.getByTestId("creation-step");
  await step
    .getByRole("radiogroup", { name: "Type de personnage" })
    .getByRole("radio", { name: kind, exact: true })
    .click();
  return step;
}

/** From the race step to Créer: race, typed scores, 60 HP, the steps' defaults, a name. */
async function finish(page: Page, race: string, name: string, book = false) {
  const step = page.getByTestId("creation-step");
  const next = () =>
    step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  await next();
  await step.getByRole("radio", { name: race, exact: true }).click();
  await next();
  await step.getByRole("radio", { name: "Saisir" }).click();
  for (const [ability, score] of scores)
    await step.getByLabel(ability, { exact: true }).fill(score);
  await next();
  await step.getByLabel("PV max").fill("60");
  await next();
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  if (book) {
    // The spellbook step appears once the draft names the class that carries it.
    await expect(
      page.getByRole("navigation", { name: "Étapes" }),
    ).toContainText("Grimoire");
  }
  await next();
  if (book) {
    await expect(step).toHaveAttribute("data-step", "spellbook");
    await next();
  }
  await step.getByLabel("Nom", { exact: true }).fill(name);
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
  await expect(page.getByTestId("character")).toContainText(name);
}

test("create a half-elf fighter/cleric from its XP per class", async ({
  page,
}) => {
  const step = await openClassStep(page, "Multiclassé");
  const rows = step.getByTestId("class-rows");
  await rows
    .getByTestId("class-row-0")
    .getByLabel("Classe 1")
    .selectOption("fighter");
  await rows.getByTestId("class-row-0").getByLabel("XP").fill("4500");
  // A level typed on the sheet is sent as entered (4,500 XP reaches level 4); the cleric's is blank.
  await rows.getByTestId("class-row-0").getByLabel("Niveau").fill("3");
  await rows
    .getByTestId("class-row-1")
    .getByLabel("Classe 2")
    .selectOption("cleric");
  await rows.getByTestId("class-row-1").getByLabel("XP").fill("4500");
  await snap(page, "multi-class-step");
  await finish(page, "Demi-elfe", "Morrison");
  await expect(page.getByTestId("character")).toContainText("Guerrier");
  await expect(page.getByTestId("character")).toContainText("Clerc");
  await expect(page.getByTestId("character")).toContainText(/niveau 3/i);
  await snap(page, "multi-sheet");
});

test("create an elf fighter who became a mage, with the level it was left at", async ({
  page,
}) => {
  const step = await openClassStep(page, "Double classé");
  const rows = step.getByTestId("class-rows");
  const old = rows.getByTestId("class-row-0");
  await old.getByLabel("Classe quittée").selectOption("fighter");
  await old.getByLabel("Quittée au niveau").fill("9");
  await rows
    .getByTestId("class-row-1")
    .getByLabel("Nouvelle classe")
    .selectOption("mage");
  await rows.getByTestId("class-row-1").getByLabel("XP").fill("20000");
  await snap(page, "dual-class-step");
  await finish(page, "Elfe", "Vael", true);
  await expect(page.getByTestId("character")).toContainText("Guerrier");
  await expect(page.getByTestId("character")).toContainText("Magicien");
  await snap(page, "dual-sheet");
});
