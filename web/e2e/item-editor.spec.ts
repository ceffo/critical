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
  openTypedAdd,
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

test("add Arrows +3: it shows in the Inventory and in the Short Bow ammo picker; pop removes it", async ({
  page,
}) => {
  await openInventory(page);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Arrows +3 (blessed)");
  await f.getByLabel("Category").selectOption("ammunition");
  await f.getByRole("button", { name: "Next" }).click();
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

// ADR 0033 step 2: a magic scroll names the spell it holds, picked from the catalogue in the
// editor; the edit is logged with the spell's name and popping it gives the scroll back its spell
// as it was (none). Only a browser shows the picker filled from the catalogue's names and the
// round trip through the form; the rules (the casting time it adds) are Go tests.
test("a scroll names its spell in the editor: logged; pop clears it", async ({
  page,
}) => {
  await openInventory(page);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Scroll of flames");
  await f.getByLabel("Category").selectOption("magic");
  await f.getByRole("button", { name: "Next" }).click();
  await f.getByLabel("Magic kind").selectOption("scroll");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  const scroll = row(page, "scroll_of_flames");
  await expect(scroll).toBeVisible();

  await scroll.getByRole("button", { name: "Edit Scroll of flames" }).click();
  await form(page)
    .getByLabel("Spell", { exact: true })
    .selectOption({ label: "Burning Hands" });
  await snap(page, "scroll-spell-form");
  await form(page).getByRole("button", { name: "Save" }).click();
  await expect(form(page)).toHaveCount(0);
  // The row names the spell; clearing it (below, by the pop) takes it away.
  await expect(scroll.getByTestId("item-stats")).toContainText("Burning Hands");

  await openWorkspace(page, "Act");
  await expect(logEntry(page, "Edited Scroll of flames")).toContainText(
    "spell none → Burning Hands",
  );
  await undoEntry(page, "Edited Scroll of flames");
  await openWorkspace(page, "Inventory");
  await scroll.getByRole("button", { name: "Edit Scroll of flames" }).click();
  await expect(form(page).getByLabel("Spell", { exact: true })).toHaveValue("");
  await form(page).getByRole("button", { name: "Cancel" }).click();
  await expect(scroll.getByTestId("item-stats")).not.toContainText(
    "Burning Hands",
  );
});
