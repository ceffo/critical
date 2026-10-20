import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  chooseLanguage,
  createInFlow,
  expectNoReload,
  expectUndoOn,
  markNoReload,
  openDashboard,
  openRoster,
  openWeapons,
  openWorkspace,
  chooseWeapon,
} from "./helpers";

// What only a browser proves for the weapon proficiency UI (ADR 0037 steps 7, 8, 9, 12, 14): the
// editor's save, the journal's Undo and the tags following it with no reload, what the editor lists
// for a class, a refused pick kept marked, and the creation flow's picks. The rules are Go tests.

async function openEditor(page: import("@playwright/test").Page) {
  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Skills" }).click();
  const skills = page.getByRole("tabpanel", { name: "Skills" });
  await skills.getByTestId("weapon-edit").click();
  return { skills, editor: skills.getByTestId("weapon-editor") };
}

test("fighter: pick and specialize in the editor, Undo from the journal, tags follow with no reload", async ({
  page,
}) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await createInFlow(page, {
    name: "Orla",
    className: "Guerrier",
    race: "Humain",
    scores: { FOR: "16", DEX: "14", CON: "15", INT: "10", SAG: "10", CHA: "9" },
    hpMax: "10",
  });
  await chooseLanguage(page, "en");
  const res = await page.request.post("/api/items", {
    data: { catalogueCode: "long_sword", equip: true },
    headers: { Origin: new URL(page.url()).origin },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  await page.reload();
  await markNoReload(page);

  // Not proficient: the piece's tag and the attack form's tag, in the dark too.
  await openWorkspace(page, "Inventory");
  const tag = page
    .getByTestId("item-fixture_long_sword")
    .getByTestId("nonproficient");
  await expect(tag).toHaveText("Not proficient: −2 to hit");
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveClass(/dark/);
  await snap(page, "nonproficient-inventory-dark");
  await openWorkspace(page, "Act");
  await openWeapons(page);
  const f = attackForm(page);
  await chooseWeapon(page, "Fixture long sword");
  await expect(f.getByTestId("nonproficient")).toBeVisible();
  await snap(page, "nonproficient-attack-dark");

  const { editor } = await openEditor(page);
  const row = editor.getByTestId("weapon-row-long_sword");
  await row.getByTestId("weapon-add").click();
  await row.getByTestId("weapon-specialize").click();
  await expect(editor.getByTestId("weapon-pool-spent")).toHaveText(
    "Spent 2, left 2",
  );
  await snap(page, "weapon-editor-dark");
  await editor.getByTestId("weapon-save").click();
  await expect(editor).toBeHidden();
  const entry = "Weapon proficiency slots: Long sword 0 → 2 (specialization)";
  await expectUndoOn(page, entry);

  // The tags followed the save with no reload.
  await openWorkspace(page, "Act");
  await openWeapons(page);
  await expect(f.getByTestId("tile-specialized")).toBeVisible();
  await expect(f.getByTestId("nonproficient")).toHaveCount(0);
  await openWorkspace(page, "Inventory");
  await expect(
    page.getByTestId("item-fixture_long_sword").getByTestId("item-stats"),
  ).toContainText("specialized");

  // The journal words the entry in French.
  await chooseLanguage(page, "fr");
  const undo = page.getByTestId("undo");
  await expect(undo).toHaveAttribute("title", /Long sword|Épée longue/);
  await expect(undo).toHaveAttribute("title", /spécialisation/);
  await snap(page, "journal-fr-specialization");
  await chooseLanguage(page, "en");

  // Undo: the specialization goes, the piece is non-proficient again, still with no reload.
  await openWorkspace(page, "Act");
  await undo.click();
  await openWeapons(page);
  await expect(f.getByTestId("tile-specialized")).toHaveCount(0);
  await expect(f.getByTestId("nonproficient")).toBeVisible();
  await openWorkspace(page, "Inventory");
  await expect(
    page.getByTestId("item-fixture_long_sword").getByTestId("item-stats"),
  ).not.toContainText("specialized");
  await expect(tag).toBeVisible();
  await expectNoReload(page);
});

test("mage: only allowed types, no Spécialiser; a kept refused pick is marked, removable, not re-addable", async ({
  page,
}) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  await createInFlow(page, {
    name: "Mélia",
    className: "Magicien",
    race: "Humain",
    scores: { FOR: "10", DEX: "14", CON: "12", INT: "16", SAG: "10", CHA: "9" },
    hpMax: "4",
  });
  await chooseLanguage(page, "en");
  // A refused pick kept on the sheet (a class change leaves one, and the server never lets a
  // save create one): the stack cannot hold it, so the character and the preview carry it here.
  const kept = { type: "long_sword", slots: 1, allowed: false };
  await page.route("**/api/character", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = (await res.json()) as {
      derived: { proficiencySlots: { weaponPicks?: unknown[] } };
    };
    body.derived.proficiencySlots.weaponPicks = [kept];
    await route.fulfill({ response: res, json: body });
  });
  await page.route("**/api/proficiencies/preview", async (route) => {
    const sent = route.request().postDataJSON() as {
      weapon: { type: string; slots: number }[];
    };
    const res = await route.fetch({
      postData: JSON.stringify({ weapon: [] }),
    });
    const body = (await res.json()) as {
      proficiencySlots: { weaponPicks?: unknown[] };
    };
    body.proficiencySlots.weaponPicks = sent.weapon.map((w) => ({
      ...w,
      allowed: w.type !== "long_sword",
    }));
    await route.fulfill({ response: res, json: body });
  });
  await page.reload();
  const { editor } = await openEditor(page);
  const rows = editor.locator('[data-testid^="weapon-row-"]');
  const bad = editor.getByTestId("weapon-row-long_sword");
  await expect(bad.getByTestId("weapon-row-not-allowed")).toBeVisible();
  await expect(bad.getByTestId("weapon-add")).toHaveCount(0);
  await expect(editor.getByTestId("weapon-specialize")).toHaveCount(0);
  // Only the mage's types plus the kept one.
  const types = (
    await rows.evaluateAll((els) =>
      els.map((e) => e.getAttribute("data-testid")),
    )
  ).filter((t) => t && t !== "weapon-row-status");
  expect(types).toContain("weapon-row-dagger");
  expect(types).not.toContain("weapon-row-short_sword");
  await snap(page, "weapon-editor-mage");
  // A pick of an allowed type still offers no specialization.
  await editor
    .getByTestId("weapon-row-dagger")
    .getByTestId("weapon-add")
    .click();
  await expect(editor.getByTestId("weapon-specialize")).toHaveCount(0);
  // Remove the refused one: its row leaves the list, and no control re-adds it.
  await bad.getByTestId("weapon-remove").click();
  await expect(bad).toHaveCount(0);
  await expect(editor.getByTestId("weapon-row-dagger")).toBeVisible();
});

test("creation: a fighter with a specialization through Créer; changing class clears the picks", async ({
  page,
}) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const r = await openRoster(page);
  await r.getByRole("button", { name: "Créer un nouveau personnage" }).click();
  const step = page.getByTestId("creation-step");
  const next = () =>
    step.getByRole("button", { name: "Passer à l'étape suivante" }).click();
  const back = () =>
    step.getByRole("button", { name: /^Revenir à l'étape précédente/ }).click();
  await step.getByRole("radio", { name: "Guerrier", exact: true }).click();
  await next();
  await step.getByRole("radio", { name: "Humain", exact: true }).click();
  await next();
  await step.getByRole("radio", { name: "Saisir" }).click();
  for (const [a, s] of Object.entries({
    FOR: "16",
    DEX: "14",
    CON: "15",
    INT: "10",
    SAG: "10",
    CHA: "9",
  }))
    await step.getByLabel(a, { exact: true }).fill(s);
  await next();
  await step.getByLabel("PV max").fill("10");
  await next();
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  const row = step.getByTestId("weapon-row-long_sword");
  await row.getByTestId("weapon-add").click();
  await row.getByTestId("weapon-specialize").click();
  await expect(row).toHaveAttribute("data-specialized", "true");
  await snap(page, "creation-fighter-specialized");

  // Changing the class clears the picks.
  for (let i = 0; i < 4; i++) await back();
  await step.getByRole("radio", { name: "Magicien", exact: true }).click();
  for (let i = 0; i < 4; i++) await next();
  await expect(step).toHaveAttribute("data-step", "proficiencies");
  await expect(step.locator('[data-picked="true"]')).toHaveCount(0);
  await snap(page, "creation-mage-cleared");

  // Back to the fighter: the pool starts empty, pick and specialize again, then Créer.
  for (let i = 0; i < 4; i++) await back();
  await step.getByRole("radio", { name: "Guerrier", exact: true }).click();
  for (let i = 0; i < 4; i++) await next();
  await expect(step.locator('[data-picked="true"]')).toHaveCount(0);
  const row2 = step.getByTestId("weapon-row-long_sword");
  await row2.getByTestId("weapon-add").click();
  await row2.getByTestId("weapon-specialize").click();
  await next();
  await step.getByLabel("Nom", { exact: true }).fill("Torvald");
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
  await expect(page.getByTestId("character")).toContainText("Torvald");
  await chooseLanguage(page, "en");
  await openEditor(page);
  await expect(
    page.getByTestId("weapon-editor").getByTestId("weapon-row-long_sword"),
  ).toHaveAttribute("data-specialized", "true");
});
