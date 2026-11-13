import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  chooseLanguage,
  createInFlow,
  expectNoReload,
  expectUndoOn,
  markNoReload,
  openDashboard,
  openWeapons,
  openWorkspace,
  chooseWeapon,
  openSpend,
} from "./helpers";

// What only a browser proves for the weapon proficiency UI (ADR 0037 steps 7, 8, 9, 12, 14): the
// editor's save, the journal's Undo and the tags following it with no reload. What the editor lists
// for a class and a kept refused pick are in WeaponPicker.test; the rules are Go tests.

async function openEditor(page: import("@playwright/test").Page) {
  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Skills" }).click();
  const skills = page.getByRole("tabpanel", { name: "Skills" });
  await openSpend(page, "weapon");
  return { skills, editor: page.getByTestId("weapon-editor") };
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
  // Two picks, one specialized: a second weapon of the same family is its own plain pick.
  await editor
    .getByTestId("weapon-row-short_sword")
    .getByTestId("weapon-add")
    .click();
  await expect(editor.getByTestId("weapon-group-melee")).toBeVisible();
  await expect(editor.getByTestId("weapon-pool-spent")).toHaveText(
    "Spent 3, left 1",
  );
  await snap(page, "weapon-editor-dark");
  await editor.getByTestId("weapon-save").click();
  await expect(editor).toBeHidden();
  // A pick is a catalogue weapon (ADR 0037 step 21.6), named from the equipment catalogue.
  const entry =
    "Weapon proficiency slots: Fixture long sword 0 → 2 (specialization)";
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
  await expect(undo).toHaveAttribute("title", /Banc d'essai long sword/);
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
