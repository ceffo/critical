import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { logEntry, openDashboard, voidEntry } from "./helpers";

// Weapon editor (critical-qki.32): add, edit and remove weapons in the Inventory tab; each is one
// log entry and the newest can be popped.

async function openInventory(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Inventory" }).click();
  await expect(page.getByTestId("weapon-bastard_sword")).toBeVisible();
}

function row(page: Page, id: string) {
  return page.getByTestId(`weapon-${id}`);
}

function form(page: Page) {
  return page.getByTestId("weapon-form");
}

function fieldError(page: Page, field: string) {
  return form(page)
    .locator(`[data-field="${field}"]`)
    .getByTestId("field-error");
}

function attackForm(page: Page) {
  return page.getByRole("form", { name: "Attack" });
}

async function rollAttack(page: Page, weapon: string, ammo?: string) {
  await page.getByRole("tab", { name: "Attack" }).click();
  const f = attackForm(page);
  await f.getByLabel("Weapon").selectOption(weapon);
  if (ammo) await f.getByLabel("Ammunition").selectOption(ammo);
  await f.getByLabel("Target AC").fill("-10");
  await f.getByRole("button", { name: "Roll Attack" }).click();
  return page
    .getByRole("region", { name: "Last Result" })
    .getByTestId("attack-result");
}

test("edit Bastard Sword +2 to +3 to hit and damage: the next attack uses +3; pop restores +2", async ({
  page,
}) => {
  await openInventory(page);
  await row(page, "bastard_sword")
    .getByRole("button", { name: "Edit Bastard Sword +2" })
    .click();
  await form(page).getByLabel("To-hit bonus").fill("3");
  await form(page).getByLabel("Damage bonus").fill("3");
  await snap(page, "edit-form");
  await form(page).getByRole("button", { name: "Save" }).click();

  await expect(form(page)).toHaveCount(0);
  const r = row(page, "bastard_sword");
  await expect(r.getByTestId("weapon-to-hit")).toHaveText("+3");
  await expect(r.getByTestId("weapon-damage")).toHaveText("+3");
  await expect(logEntry(page, "Edited weapon Bastard Sword +2")).toContainText(
    "to-hit bonus 2 → 3, damage bonus 2 → 3",
  );
  await snap(page, "weapon-edited");

  const card = await rollAttack(page, "bastard_sword");
  await expect(card.getByTestId("to-hit-modifiers")).toContainText(
    /Bastard Sword \+2\s*\+3/,
  );
  await snap(page, "attack-uses-plus-3");

  // The attack is newer than the edit: pop the attack, then the edit.
  await voidEntry(page, "Bastard Sword +2 vs AC");
  await page.getByRole("tab", { name: "Inventory" }).click();
  await voidEntry(page, "Edited weapon Bastard Sword +2");
  await expect(r.getByTestId("weapon-to-hit")).toHaveText("+2");
  await expect(r.getByTestId("weapon-damage")).toHaveText("+2");
  await snap(page, "edit-popped");
});

test("add a Long Bow (missile, arrow): it is in the attack picker and offered arrows; pop removes it", async ({
  page,
}) => {
  await openInventory(page);
  await page.getByRole("button", { name: "Add weapon" }).click();
  const f = form(page);
  await f.getByLabel("Name").fill("Long Bow");
  await f.getByLabel("Damage S-M").fill("1d8");
  await f.getByLabel("Damage Large").fill("1d8");
  await f.getByLabel("Kind", { exact: true }).selectOption("missile");
  await f.getByLabel("Two-handed").check();
  await f.getByLabel("Ammo kind").fill("arrow");
  await snap(page, "add-form");
  await f.getByRole("button", { name: "Add", exact: true }).click();

  await expect(form(page)).toHaveCount(0);
  await expect(row(page, "long_bow")).toContainText("Long Bow");
  await expect(row(page, "long_bow").getByTestId("weapon-kind")).toHaveText(
    "Missile",
  );
  await expect(logEntry(page, "Added weapon Long Bow")).toBeVisible();
  await snap(page, "weapon-added");

  await page.getByRole("tab", { name: "Attack" }).click();
  await attackForm(page).getByLabel("Weapon").selectOption("long_bow");
  await expect(
    attackForm(page).getByLabel("Ammunition").locator("option", {
      hasText: "Arrows +1",
    }),
  ).toHaveCount(1);
  await snap(page, "bow-offered-arrows");

  await page.getByRole("tab", { name: "Inventory" }).click();
  await voidEntry(page, "Added weapon Long Bow");
  await expect(row(page, "long_bow")).toHaveCount(0);
  await page.getByRole("tab", { name: "Attack" }).click();
  await expect(
    attackForm(page).getByLabel("Weapon").locator("option", {
      hasText: "Long Bow",
    }),
  ).toHaveCount(0);
});

test("remove the darts: gone from the picker, an earlier attack line still reads Darts; pop restores them", async ({
  page,
}) => {
  await openInventory(page);
  await rollAttack(page, "darts");
  await expect(logEntry(page, "Darts vs AC -10")).toBeVisible();

  await page.getByRole("tab", { name: "Inventory" }).click();
  await row(page, "darts")
    .getByRole("button", { name: "Remove Darts" })
    .click();
  await expect(row(page, "darts")).toHaveCount(0);
  await expect(logEntry(page, "Removed weapon Darts")).toBeVisible();
  await expect(logEntry(page, "Darts vs AC -10")).toBeVisible();
  await snap(page, "darts-removed");

  await page.getByRole("tab", { name: "Attack" }).click();
  await expect(
    attackForm(page).getByLabel("Weapon").locator("option", {
      hasText: "Darts",
    }),
  ).toHaveCount(0);

  await page.getByRole("tab", { name: "Inventory" }).click();
  await voidEntry(page, "Removed weapon Darts");
  await expect(row(page, "darts")).toBeVisible();
  await page.getByRole("tab", { name: "Attack" }).click();
  await expect(
    attackForm(page).getByLabel("Weapon").locator("option", {
      hasText: "Darts",
    }),
  ).toHaveCount(1);
  await snap(page, "darts-restored");
});

test("invalid dice are rejected with the server's message under the field", async ({
  page,
}) => {
  await openInventory(page);
  await page.getByRole("button", { name: "Add weapon" }).click();
  const f = form(page);
  await f.getByLabel("Name").fill("Club");
  await f.getByLabel("Damage S-M").fill("2x6");
  await f.getByLabel("Damage Large").fill("1d6");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(fieldError(page, "damage_sm")).toContainText(
    "Damage must look like 1d8",
  );
  await snap(page, "invalid-dice");

  await f.getByLabel("Damage S-M").fill("1d6+1");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(fieldError(page, "damage_sm")).toBeVisible();

  await f.getByLabel("Damage S-M").fill("1d6");
  await f.getByLabel("To-hit bonus").fill("1.5");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(fieldError(page, "magic_to_hit")).toContainText("whole number");
  await f.getByRole("button", { name: "Cancel" }).click();
  await expect(form(page)).toHaveCount(0);
  await expect(row(page, "club")).toHaveCount(0);
});

test("a weapon removed behind the Attack panel's back: the attack says so and the picker refreshes", async ({
  page,
}) => {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Attack" }).click();
  const f = attackForm(page);
  await f.getByLabel("Weapon").selectOption("darts");
  // Removed elsewhere (another device): the panel still has the darts selected.
  const res = await page.request.delete("/api/weapons/darts");
  expect(res.ok()).toBe(true);
  await f.getByLabel("Target AC").fill("5");
  await f.getByRole("button", { name: "Roll Attack" }).click();

  await expect(
    page.getByRole("alert").filter({ hasText: "weapon not found" }),
  ).toBeVisible();
  await expect(
    f.getByLabel("Weapon").locator("option", { hasText: "Darts" }),
  ).toHaveCount(0);
  // No stale selection: the picker shows an existing weapon.
  await expect(f.getByLabel("Weapon").locator("option:checked")).not.toHaveText(
    /Darts/,
  );
  await snap(page, "attack-weapon-gone");
});

test("a missile weapon switched to melee loses its ammo kind, server-side and in the UI", async ({
  page,
}) => {
  await openInventory(page);
  await row(page, "shortbow")
    .getByRole("button", { name: "Edit Short Bow +2" })
    .click();
  await form(page).getByLabel("Kind", { exact: true }).selectOption("melee");
  await expect(form(page).getByLabel("Ammo kind")).toHaveCount(0);
  await form(page).getByRole("button", { name: "Save" }).click();
  await expect(form(page)).toHaveCount(0);
  await expect(row(page, "shortbow").getByTestId("weapon-kind")).toHaveText(
    "Melee",
  );
  await expect(
    row(page, "shortbow").getByTestId("weapon-detail"),
  ).not.toContainText("arrow");
  await expect(logEntry(page, "Edited weapon Short Bow +2")).toContainText(
    "ammo kind arrow →",
  );
  // Reopening and going back to missile starts with no ammo kind.
  await row(page, "shortbow")
    .getByRole("button", { name: "Edit Short Bow +2" })
    .click();
  await form(page).getByLabel("Kind", { exact: true }).selectOption("missile");
  await expect(form(page).getByLabel("Ammo kind")).toHaveValue("");
  await snap(page, "bow-to-melee");
  await form(page).getByRole("button", { name: "Cancel" }).click();

  // Server-side: a PATCH that only changes the kind clears it too.
  const bow = await page.request.post("/api/weapons", {
    data: {
      name: "Bow B",
      damage_sm: "1d6",
      damage_l: "1d6",
      kind: "missile",
      ammo_kind: "arrow",
    },
  });
  const id = ((await bow.json()) as { weapon: { id: string } }).weapon.id;
  const patched = await page.request.patch(`/api/weapons/${id}`, {
    data: { kind: "melee" },
  });
  expect(
    ((await patched.json()) as { weapon: { ammo_kind: string } }).weapon
      .ammo_kind,
  ).toBe("");
});

test("two weapons with the same name get slug and slug_2; pop the second, add again", async ({
  page,
}) => {
  await openInventory(page);
  async function add() {
    await page.getByRole("button", { name: "Add weapon" }).click();
    const f = form(page);
    await f.getByLabel("Name").fill("Spear");
    await f.getByLabel("Damage S-M").fill("1d6");
    await f.getByLabel("Damage Large").fill("1d8");
    await f.getByRole("button", { name: "Add", exact: true }).click();
    await expect(form(page)).toHaveCount(0);
  }
  await add();
  await expect(row(page, "spear")).toBeVisible();
  await add();
  await expect(row(page, "spear_2")).toBeVisible();
  await snap(page, "two-spears");

  // The newest entry is the first listed: pop it.
  const newest = logEntry(page, "Added weapon Spear").first();
  await newest.getByRole("button", { name: "Void" }).click();
  await newest.getByRole("button", { name: "Confirm" }).click();
  await expect(row(page, "spear_2")).toHaveCount(0);
  await expect(row(page, "spear")).toBeVisible();
  await add();
  await expect(row(page, "spear_2")).toBeVisible();
  await snap(page, "spear-readded");
});
