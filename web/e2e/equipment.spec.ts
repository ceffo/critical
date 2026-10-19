import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  chooseWeapon,
  logEntry,
  openDashboard,
  openWeapons,
  openWorkspace,
  undoEntry,
} from "./helpers";

// The one possessions list (ADR 0033 steps 1 to 8 and 27, ADR 0035 items 2, 3, 7 and 12): weapons
// are pieces edited in the Inventaire's one editor, Equip / Unequip is one logged move the journal
// pops, the attack's drop-down equips the weapon it picks, and the review list empties as the
// player settles it. What only a browser proves here: the wiring of those controls, the journal
// lines and the attack panel following the inventory; the rules are the Go tests'.

async function openInventory(page: Page) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  await expect(page.getByTestId("item-bastard_sword")).toBeVisible();
}

function row(page: Page, id: string) {
  return page.getByTestId(`item-${id}`);
}

function form(page: Page) {
  return page.getByTestId("item-form");
}

/** Every weapon not in hand: the previous weapon's button and the drop-down's options. */
async function otherWeapons(page: Page) {
  const group = attackForm(page).getByRole("group", { name: "Other weapons" });
  const previous = await group
    .getByRole("button")
    .evaluateAll((els) => els.map((e) => e.getAttribute("data-weapon") ?? ""));
  const listed = await group
    .locator("option:not([disabled])")
    .evaluateAll((els) => els.map((e) => e.textContent ?? ""));
  return [...previous, ...listed];
}

async function rollAttack(page: Page, weapon: string) {
  await openWorkspace(page, "Act");
  await openWeapons(page);
  await chooseWeapon(page, weapon);
  const f = attackForm(page);
  await f.getByLabel("Target AC").fill("-10");
  await f.getByRole("button", { name: "Attack", exact: true }).click();
  return page
    .getByRole("region", { name: "Detail", exact: true })
    .getByTestId("attack-result");
}

test("edit Bastard Sword +2 to +3 to hit and damage in the one editor: the next attack uses +3; pop restores +2", async ({
  page,
}) => {
  await openInventory(page);
  await row(page, "bastard_sword")
    .getByRole("button", { name: "Edit Bastard Sword +2" })
    .click();
  await expect(form(page).getByLabel("Category")).toHaveValue("weapon");
  await form(page).getByLabel("To-hit bonus").fill("3");
  await form(page).getByLabel("Damage bonus").fill("3");
  await snap(page, "edit-form");
  await form(page).getByRole("button", { name: "Save" }).click();

  await expect(form(page)).toHaveCount(0);
  const r = row(page, "bastard_sword");
  await expect(r.getByTestId("item-stats")).toContainText("+3/+3");

  const card = await rollAttack(page, "Bastard Sword +2");
  await expect(logEntry(page, "Edited Bastard Sword +2")).toContainText(
    "to-hit bonus 2 → 3, damage bonus 2 → 3",
  );
  await expect(card.getByTestId("to-hit-modifiers")).toContainText(
    /Bastard Sword \+2\s*\+3/,
  );

  // The attack is newer than the edit: pop the attack, then the edit.
  await undoEntry(page, "Bastard Sword +2 vs AC");
  await undoEntry(page, "Edited Bastard Sword +2");
  await openWorkspace(page, "Inventory");
  await expect(r.getByTestId("item-stats")).toContainText("+2/+2");
});

test("a new bow is carried: the attack's drop-down lists it under Not equipped, picking it equips it; pop puts it away", async ({
  page,
}) => {
  await openInventory(page);
  await page.getByRole("button", { name: "Add item" }).click();
  const f = form(page);
  await f.getByLabel("Name").fill("Long Bow");
  await f.getByLabel("Category").selectOption("weapon");
  await f.getByLabel("Kind", { exact: true }).selectOption("missile");
  await f.getByLabel("Damage S-M").fill("1d8");
  await f.getByLabel("Damage Large").fill("1d8");
  await f.getByLabel("Two-handed").check();
  await f.getByLabel("Fires").selectOption("arrow");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  const bow = row(page, "long_bow");
  await expect(bow).toContainText("Long Bow");
  await expect(bow.getByTestId("item-equipped")).toHaveCount(0);
  await expect(
    bow.getByRole("button", { name: "Equip Long Bow" }),
  ).toBeVisible();

  await openWorkspace(page, "Act");
  await openWeapons(page);
  const group = attackForm(page)
    .getByRole("combobox", { name: "Other weapon" })
    .locator('optgroup[label="Not equipped"] option');
  await expect(group).toHaveText(["Long Bow"]);
  await snap(page, "drop-down-not-equipped");
  await chooseWeapon(page, "Long Bow");
  await expect(logEntry(page, "Equipped: Long Bow")).toBeVisible();
  await expect(
    attackForm(page)
      .getByRole("radiogroup", { name: "Ammunition" })
      .getByRole("radio", { name: /^Arrows \+1,/ }),
  ).toHaveCount(1);

  // Popping the move puts the bow away: the hand is empty, the bow is offered again.
  await undoEntry(page, "Equipped: Long Bow");
  await expect(attackForm(page).getByTestId("no-weapon")).toBeVisible();
  expect(await otherWeapons(page)).toContain("Long Bow");
  await openWorkspace(page, "Inventory");
  await expect(bow.getByTestId("item-equipped")).toHaveCount(0);
});

test("a new gem marked as a spell component in the editor keeps it after a reload", async ({
  page,
}) => {
  // ADR 0033 step 1.1: the component select is wired to the API and persisted (which piece a spell
  // spends is the Go tests'). What only a browser proves: the select, its labels and the reload.
  await openInventory(page);
  await page.getByRole("button", { name: "Add item" }).click();
  const f = form(page);
  await f.getByLabel("Name").fill("Ruby");
  await expect(f.getByLabel("Spell component")).toHaveValue("");
  await f.getByLabel("Spell component").selectOption("gem");
  await snap(page, "component-form");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  await expect(row(page, "ruby")).toContainText("Ruby");

  await page.reload();
  await openInventory(page);
  await row(page, "ruby").getByRole("button", { name: "Edit Ruby" }).click();
  await expect(form(page).getByLabel("Spell component")).toHaveValue("gem");
});

test("Unequip and Equip in the Inventaire: the Equipped tag follows, each a journal line popped by undo", async ({
  page,
}) => {
  await openInventory(page);
  const gem = row(page, "power_gem_dispel");
  await expect(gem.getByTestId("item-equipped")).toHaveText("Equipped");
  await gem.getByRole("button", { name: "Unequip Power Gem" }).click();
  await expect(gem.getByTestId("item-equipped")).toHaveCount(0);
  await expect(
    gem.getByRole("button", { name: "Equip Power Gem" }),
  ).toBeVisible();
  await snap(page, "gem-unequipped");

  await openWorkspace(page, "Act");
  await expect(logEntry(page, "Unequipped: Power Gem")).toBeVisible();
  await undoEntry(page, "Unequipped: Power Gem");
  await openWorkspace(page, "Inventory");
  await expect(gem.getByTestId("item-equipped")).toHaveText("Equipped");
});

test("the review list: the band counts the pieces to check, Dismiss and a classifying edit empty it", async ({
  page,
}) => {
  await openInventory(page);
  const band = page.getByTestId("review-band");
  await expect(band).toContainText("2 possessions to check");
  await band.getByRole("button", { name: /Check the possessions/ }).click();
  const review = page.getByTestId("inventory-review");
  await expect(review.getByTestId("review-mace")).toContainText(
    "Weapon not recognised",
  );
  await snap(page, "review-open");

  // Dismissing is not a game move: no journal line.
  await review
    .getByRole("button", { name: "Take Mace +1 off the list as it is" })
    .click();
  await expect(review.getByTestId("review-mace")).toHaveCount(0);
  await expect(band).toContainText("1 possession to check");

  // Settling the bastard sword: its speed and damage type.
  await review.getByRole("button", { name: "Edit Bastard Sword +2" }).click();
  await form(page).getByLabel("Speed factor").fill("6");
  await form(page).getByLabel("Slashing").check();
  await form(page).getByRole("button", { name: "Save" }).click();
  await expect(band).toHaveCount(0);
  await expect(page.getByTestId("inventory-review")).toHaveCount(0);
  await expect(
    row(page, "bastard_sword").getByTestId("item-review"),
  ).toHaveCount(0);
  await snap(page, "review-empty");
});

test("a weapon removed behind the Attack panel's back: the attack says so and the picker refreshes", async ({
  page,
}) => {
  await openDashboard(page);
  await openWorkspace(page, "Act");
  await openWeapons(page);
  const f = attackForm(page);
  await chooseWeapon(page, "Darts");
  await f.getByLabel("Target AC").fill("5");
  await expect
    .poll(
      async () =>
        (await (await page.request.get("/api/attack/form")).json()).form
          .targetAc,
    )
    .toBe(5);
  // Removed elsewhere (another device): the panel still has the darts in hand.
  const res = await page.request.delete("/api/items/darts");
  expect(res.ok()).toBe(true);
  await f.getByRole("button", { name: "Attack", exact: true }).click();

  await expect(
    page.getByRole("alert").filter({ hasText: "no longer exists" }),
  ).toBeVisible();
  // No stale weapon in hand: the server's form falls back to an existing weapon.
  await expect(f.getByTestId("weapon-in-hand")).not.toHaveText("Darts");
  expect(await otherWeapons(page)).not.toContain("Darts");
  await snap(page, "attack-weapon-gone");
});
