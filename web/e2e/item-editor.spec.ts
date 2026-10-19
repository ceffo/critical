import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  ammoChips,
  chooseWeapon,
  logEntry,
  openDashboard,
  openWeapons,
  openWorkspace,
  undoEntry,
} from "./helpers";

// Inventory editor (one editor for every piece, ADR 0033 step 1): add, edit and remove possessions;
// each is one log entry and the newest can be popped. The journal is the Act activity's (ADR 0022
// step 2.5).

async function openInventory(page: Page) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  await expect(page.getByTestId("item-wand_lightning")).toBeVisible();
}

function row(page: Page, id: string) {
  return page.getByTestId(`item-${id}`);
}

function form(page: Page) {
  return page.getByTestId("item-form");
}

function fieldError(page: Page, field: string) {
  return form(page)
    .locator(`[data-field="${field}"]`)
    .getByTestId("field-error");
}

test("add Arrows +3: it shows in the Inventory and in the Short Bow ammo picker; pop removes it", async ({
  page,
}) => {
  await openInventory(page);
  await page.getByRole("button", { name: "Add item" }).click();
  const f = form(page);
  await f.getByLabel("Name").fill("Arrows +3 (blessed)");
  await f.getByLabel("Category").selectOption("ammunition");
  await f.getByLabel("Count").fill("6");
  await f.getByLabel("Magical bonus").fill("3");
  await f.getByLabel("Ammo kind").selectOption("arrow");
  await snap(page, "add-form");
  await f.getByRole("button", { name: "Add", exact: true }).click();

  await expect(form(page)).toHaveCount(0);
  const added = row(page, "arrows_3_blessed");
  await expect(added).toContainText("Arrows +3 (blessed)");
  await expect(added.getByTestId("item-count")).toHaveText("6");
  await expect(added.getByTestId("item-bonus")).toHaveText("+3");
  await snap(page, "item-added");

  await openWorkspace(page, "Act");
  await expect(logEntry(page, "Added Arrows +3 (blessed) (6)")).toBeVisible();
  await openWeapons(page);
  await chooseWeapon(page, "Short Bow +2");
  await expect(
    ammoChips(page).getByRole("radio", { name: "Arrows +3 (blessed), 6 left" }),
  ).toHaveCount(1);
  await snap(page, "ammo-picker-with-new-arrows");

  await undoEntry(page, "Added Arrows +3 (blessed)");
  await expect(
    ammoChips(page).getByRole("radio", { name: /blessed/ }),
  ).toHaveCount(0);
  await openWorkspace(page, "Inventory");
  await expect(added).toHaveCount(0);
});

test("edit the wand's charges 4 to 3: logged; pop restores 4", async ({
  page,
}) => {
  await openInventory(page);
  await row(page, "wand_lightning")
    .getByRole("button", { name: "Edit Wand of lightning" })
    .click();
  await form(page).getByLabel("Count").fill("3");
  await snap(page, "edit-form");
  await form(page).getByRole("button", { name: "Save" }).click();

  await expect(
    row(page, "wand_lightning").getByTestId("item-count"),
  ).toHaveText("3");
  await snap(page, "item-edited");
  await openWorkspace(page, "Act");
  await expect(logEntry(page, "Edited Wand of lightning")).toContainText(
    "count 4 → 3",
  );

  await undoEntry(page, "Edited Wand of lightning");
  await openWorkspace(page, "Inventory");
  await expect(
    row(page, "wand_lightning").getByTestId("item-count"),
  ).toHaveText("4");
});

test("remove an item: gone from the Inventory and the ammo picker; pop brings it back with its count", async ({
  page,
}) => {
  await openInventory(page);
  await row(page, "arrows_plus2")
    .getByRole("button", { name: "Remove Arrows +2" })
    .click();
  await expect(row(page, "arrows_plus2")).toHaveCount(0);
  await snap(page, "item-removed");

  await openWorkspace(page, "Act");
  await expect(logEntry(page, "Removed Arrows +2 (16)")).toBeVisible();
  await openWeapons(page);
  await chooseWeapon(page, "Short Bow +2");
  await expect(
    ammoChips(page).getByRole("radio", { name: /^Arrows \+1,/ }),
  ).toHaveCount(1);
  await expect(
    ammoChips(page).getByRole("radio", { name: /^Arrows \+2,/ }),
  ).toHaveCount(0);

  await undoEntry(page, "Removed Arrows +2");
  await openWorkspace(page, "Inventory");
  await expect(row(page, "arrows_plus2").getByTestId("item-count")).toHaveText(
    "16",
  );
  await snap(page, "item-restored");
});

test("an empty name and invalid dice are rejected with a message under the field", async ({
  page,
}) => {
  await openInventory(page);
  await page.getByRole("button", { name: "Add item" }).click();
  const f = form(page);
  const save = f.getByRole("button", { name: "Add", exact: true });

  await save.click();
  await expect(fieldError(page, "name")).toHaveText("Name is required");

  await f.getByLabel("Name").fill("Bomb");
  await f.getByLabel("Effect dice").fill("2x6");
  await save.click();
  await expect(fieldError(page, "effectDice")).toContainText("2d6");
  await expect(fieldError(page, "name")).toHaveCount(0);

  await snap(page, "field-errors");

  // Nothing was added or logged.
  await expect(row(page, "bomb")).toHaveCount(0);
  await expect(logEntry(page, "Added Bomb")).toHaveCount(0);
});

test("a daily item and an effect with a modifier: add, use rolls the modifier in", async ({
  page,
}) => {
  await openInventory(page);
  await page.getByRole("button", { name: "Add item" }).click();
  const f = form(page);
  await f.getByLabel("Name").fill("Holy water");
  await f.getByLabel("Category").selectOption("magic");
  await f.getByLabel("Magic kind").selectOption("potion");
  await f.getByLabel("Behaviour").selectOption("daily");
  await f.getByLabel("Daily number").fill("2");
  await f.getByLabel("Count").fill("2");
  await f.getByLabel("Effect dice").fill("1d4+3");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(row(page, "holy_water").getByTestId("item-count")).toHaveText(
    "2 / 2",
  );

  await page.getByRole("button", { name: "Use one Holy water" }).click();
  const total = Number(await page.getByTestId("item-total").textContent());
  expect(total).toBeGreaterThanOrEqual(4);
  expect(total).toBeLessThanOrEqual(7);
  await expect(page.getByTestId("item-result")).toContainText("1d4+3");
});
