import { test, expect, snap } from "./fixtures";
import { chooseLanguage, openDashboard, openWorkspace } from "./helpers";

// What only a browser proves for the multi-class weapon lift (ADR 0041 step 2.5): the admin's switch
// on the Ruleset screen reaches the weapon picker of a real multi-class cleric/fighter. Whether a
// pick is allowed is the server's rule (Go tests: skills, skillsvc, charactersvc); here the wiring.

const scores = [
  ["FOR", "17"],
  ["DEX", "16"],
  ["CON", "15"],
  ["INT", "17"],
  ["SAG", "16"],
  ["CHA", "10"],
] as const;

test("French: the switch on the Ruleset screen lets a cleric/fighter pick a long sword", async ({
  page,
}) => {
  test.setTimeout(90_000);
  await openDashboard(page);
  await chooseLanguage(page, "fr");

  // A half-elf fighter/cleric, made through the creation flow.
  await page.goto("/create");
  const step = page.getByTestId("creation-step");
  await step
    .getByRole("radiogroup", { name: "Type de personnage" })
    .getByRole("radio", { name: "Multiclassé", exact: true })
    .click();
  const rows = step.getByTestId("class-rows");
  await rows
    .getByTestId("class-row-0")
    .getByLabel("Classe 1")
    .selectOption("fighter");
  await rows.getByTestId("class-row-0").getByLabel("XP").fill("4500");
  await rows
    .getByTestId("class-row-1")
    .getByLabel("Classe 2")
    .selectOption("cleric");
  await rows.getByTestId("class-row-1").getByLabel("XP").fill("4500");
  const next = () =>
    step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  await next();
  await step.getByRole("radio", { name: "Demi-elfe", exact: true }).click();
  await next();
  await step.getByRole("radio", { name: "Saisir" }).click();
  for (const [ability, score] of scores)
    await step.getByLabel(ability, { exact: true }).fill(score);
  await next();
  await step.getByLabel("PV max").fill("60");
  await next();
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  await next();
  await step.getByLabel("Nom", { exact: true }).fill("Morrison");
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
  await expect(page.getByTestId("character")).toContainText("Morrison");

  // The rail's labels are English in the helpers; the Ruleset screen is checked in French.
  await chooseLanguage(page, "en");

  // Switch off (the default): the cleric's limit holds and the server refuses the long sword, with its words in the row.
  const openEditor = async () => {
    await openWorkspace(page, "Act");
    await page.getByRole("tab", { name: "Skills" }).click();
    const skills = page.getByRole("tabpanel", { name: "Skills" });
    await skills.getByTestId("weapon-edit").click();
    return skills.getByTestId("weapon-editor");
  };
  let editor = await openEditor();
  let row = editor.getByTestId("weapon-row-long_sword");
  await row.getByTestId("weapon-add").click();
  await expect(row.getByRole("alert")).toBeVisible();
  await page.setViewportSize({ width: 1920, height: 1080 });
  await snap(page, "picker-refused-1920x1080");

  // The admin turns the switch on from the Ruleset screen; it is saved and survives a reload.
  await page.goto("/");
  await chooseLanguage(page, "fr");
  await page.goto("/ruleset");
  const lift = page.getByTestId("ruleset-weapon-lift");
  await expect(lift).not.toBeChecked();
  await lift.click();
  await expect(page.getByTestId("ruleset-house-rules")).toContainText(
    "Enregistré.",
  );
  await expect(lift).toBeChecked();
  await page.reload();
  await expect(page.getByTestId("ruleset-weapon-lift")).toBeChecked();
  for (const vp of [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await snap(page, `ruleset-weapon-lift-${vp.width}x${vp.height}`);
  }

  // Back on the sheet, the same picker lets the cleric/fighter take the long sword.
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.goto("/");
  await chooseLanguage(page, "en");
  editor = await openEditor();
  row = editor.getByTestId("weapon-row-long_sword");
  await expect(row).toBeVisible();
  await row.getByTestId("weapon-add").click();
  await expect(row).toHaveAttribute("data-picked", "true");
  await expect(row.getByRole("alert")).toHaveCount(0);
  for (const vp of [
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await snap(page, `picker-long-sword-${vp.width}x${vp.height}`);
  }
});
