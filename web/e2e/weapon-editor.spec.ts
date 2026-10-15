import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  chooseWeapon,
  logEntry,
  openDashboard,
  openWeapons,
  openWorkspace,
  voidEntry,
} from "./helpers";

// Weapon editor: add, edit and remove weapons in the Inventory tab; each is one
// log entry and the newest can be popped.

async function openInventory(page: Page) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  await expect(page.getByTestId("weapon-bastard_sword")).toBeVisible();
}

function row(page: Page, id: string) {
  return page.getByTestId(`weapon-${id}`);
}

function form(page: Page) {
  return page.getByTestId("weapon-form");
}

/** The weapons "Other weapon" offers (every weapon not in hand). */
async function otherWeapons(page: Page) {
  await attackForm(page).getByRole("button", { name: "Other weapon" }).click();
  const list = page.getByRole("list", { name: "Other weapon" });
  await expect(list).toBeVisible();
  const names = await list.getByRole("button").evaluateAll((els) =>
    els.map((e) => e.querySelector("span")?.textContent ?? ""),
  );
  await page.keyboard.press("Escape");
  return names;
}

/** Every weapon the Weapons tab can put in hand: the one held and the others. */
async function pickableWeapons(page: Page) {
  await openWorkspace(page, "Act");
  await openWeapons(page);
  const held = (await attackForm(page).getByTestId("weapon-in-hand").textContent()) ?? "";
  return [held, ...(await otherWeapons(page))];
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

  const card = await rollAttack(page, "Bastard Sword +2");
  await expect(card.getByTestId("to-hit-modifiers")).toContainText(
    /Bastard Sword \+2\s*\+3/,
  );
  await snap(page, "attack-uses-plus-3");

  // The attack is newer than the edit: pop the attack, then the edit.
  await voidEntry(page, "Bastard Sword +2 vs AC");
  await openWorkspace(page, "Inventory");
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

  await openWorkspace(page, "Act");
  await openWeapons(page);
  await chooseWeapon(page, "Long Bow");
  await expect(
    attackForm(page).getByRole("radiogroup", { name: "Ammunition" }).getByRole("radio", { name: /^Arrows \+1,/ }),
  ).toHaveCount(1);
  await snap(page, "bow-offered-arrows");

  await openWorkspace(page, "Inventory");
  await voidEntry(page, "Added weapon Long Bow");
  await expect(row(page, "long_bow")).toHaveCount(0);
  // The bow was in hand: the Weapons tab falls back to the first weapon.
  expect(await pickableWeapons(page)).not.toContain("Long Bow");
});

test("remove the darts: gone from the picker, an earlier attack line still reads Darts; pop restores them", async ({
  page,
}) => {
  await openInventory(page);
  await rollAttack(page, "Darts");
  await expect(logEntry(page, "Darts vs AC -10")).toBeVisible();

  await openWorkspace(page, "Inventory");
  await row(page, "darts")
    .getByRole("button", { name: "Remove Darts" })
    .click();
  await expect(row(page, "darts")).toHaveCount(0);
  await expect(logEntry(page, "Removed weapon Darts")).toBeVisible();
  await expect(logEntry(page, "Darts vs AC -10")).toBeVisible();
  await snap(page, "darts-removed");

  expect(await pickableWeapons(page)).not.toContain("Darts");

  await openWorkspace(page, "Inventory");
  await voidEntry(page, "Removed weapon Darts");
  await expect(row(page, "darts")).toBeVisible();
  expect(await pickableWeapons(page)).toContain("Darts");
  await snap(page, "darts-restored");
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
    .poll(async () => (await (await page.request.get("/api/attack/form")).json()).form.targetAc)
    .toBe(5);
  // Removed elsewhere (another device): the panel still has the darts in hand.
  const res = await page.request.delete("/api/weapons/darts");
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
