import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { logEntry, openDashboard, voidEntry } from "./helpers";

function attackForm(page: Page) {
  return page.getByRole("form", { name: "Attack" });
}

function resultCard(page: Page) {
  return page.getByRole("region", { name: "Last Result" }).getByTestId("attack-result");
}

/** Remaining count shown in the selected ammunition option ("Arrows +3 (2 left)"). */
async function selectedAmmoCount(page: Page): Promise<number> {
  const text =
    (await attackForm(page).getByLabel("Ammunition").locator("option:checked").textContent()) ?? "";
  const m = /\((\d+) left\)/.exec(text);
  if (!m) throw new Error(`unexpected ammo option: ${JSON.stringify(text)}`);
  return Number(m[1]);
}

async function openAttack(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Attack" }).click();
  await expect(attackForm(page).getByLabel("Weapon")).toBeVisible();
}

test("a melee attack lists every modifier in the result card and the log", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await form.getByLabel("Weapon").selectOption("longsword");
  await expect(form.getByLabel("Ammunition")).toHaveCount(0);
  await form.getByLabel("Target AC").fill("5");
  await form.getByLabel("Target size").selectOption("l");
  await form.getByLabel("Situational").fill("2");
  await form.getByLabel("Reason").fill("Rear attack");
  await form.getByRole("button", { name: "Roll Attack" }).click();

  const card = resultCard(page);
  await expect(card).toContainText("Long Sword +2 vs AC 5");
  await expect(card).toContainText("large target");
  await expect(card).toContainText(/Hit|Miss/);
  const mods = card.getByTestId("to-hit-modifiers");
  await expect(mods.getByRole("listitem")).toHaveCount(3);
  await expect(mods).toContainText(/Strength \d+/);
  await expect(mods.getByRole("listitem").nth(1)).toHaveText(/Long Sword \+2\s*\+2/);
  await expect(mods.getByRole("listitem").nth(2)).toHaveText(/Rear attack\s*\+2/);

  // The attack is in the combat log, in the player's language, rendered from its payload.
  await expect(logEntry(page, "Long Sword +2 vs AC 5")).toBeVisible();
  await snap(page, "melee-modifiers");
});

test("a missile attack spends the chosen arrow", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await form.getByLabel("Weapon").selectOption("shortbow");
  await form.getByLabel("Ammunition").selectOption("arrows_plus3");
  const before = await selectedAmmoCount(page);
  await form.getByLabel("Target AC").fill("2");
  await form.getByRole("button", { name: "Roll Attack" }).click();

  const card = resultCard(page);
  await expect(card).toContainText("Short Bow +2 with Arrows +3 vs AC 2");
  const mods = card.getByTestId("to-hit-modifiers");
  await expect(mods).toContainText(/Dexterity \d+ \(missile\)/);
  await expect(mods).toContainText("Short Bow +2");
  await expect(mods).toContainText("Arrows +3");
  await expect(mods).not.toContainText("Strength");
  await expect(card.getByTestId("ammo-left")).toHaveText(`Arrows +3: ${before - 1} left`);
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);
  await expect(logEntry(page, "Short Bow +2 (Arrows +3) vs AC 2")).toBeVisible();
  await snap(page, "missile-ammo");
});

test("voiding a missile attack gives the arrow back", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await form.getByLabel("Weapon").selectOption("shortbow");
  await form.getByLabel("Ammunition").selectOption("arrows_plus2");
  const before = await selectedAmmoCount(page);
  await form.getByLabel("Target AC").fill("3");
  await form.getByRole("button", { name: "Roll Attack" }).click();
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);

  await voidEntry(page, "Short Bow +2 (Arrows +2) vs AC 3");
  await expect.poll(() => selectedAmmoCount(page)).toBe(before);
  await snap(page, "missile-void");
});

test("spent ammunition stays selected and must be replaced by an explicit pick", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  const roll = form.getByRole("button", { name: "Roll Attack" });
  await form.getByLabel("Weapon").selectOption("shortbow");
  const ammo = form.getByLabel("Ammunition");
  await ammo.selectOption("arrows_plus3");
  const left = await selectedAmmoCount(page);
  for (let i = left; i > 0; i--) {
    await roll.click();
    await expect.poll(() => selectedAmmoCount(page)).toBe(i - 1);
  }

  // The +3 arrows are spent: still the selection, no silent switch to other arrows.
  await expect(ammo).toHaveValue("arrows_plus3");
  await expect(form.getByTestId("ammo-spent")).toHaveText("No Arrows +3 left: pick other ammunition");
  await expect(roll).toBeDisabled();
  await snap(page, "ammo-spent");

  await ammo.selectOption("arrows_plus1");
  await expect(form.getByTestId("ammo-spent")).toHaveCount(0);
  await expect(roll).toBeEnabled();
});

test("a later shot with other arrows does not block voiding an earlier shot", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  const ammo = form.getByLabel("Ammunition");
  await form.getByLabel("Weapon").selectOption("shortbow");
  await form.getByLabel("Target AC").fill("7");

  await ammo.selectOption("arrows_plus3");
  const plus3 = await selectedAmmoCount(page);
  await form.getByRole("button", { name: "Roll Attack" }).click();
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus3 - 1);

  await ammo.selectOption("arrows_plus1");
  const plus1 = await selectedAmmoCount(page);
  await form.getByRole("button", { name: "Roll Attack" }).click();
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus1 - 1);

  await voidEntry(page, "Short Bow +2 (Arrows +3) vs AC 7");
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus1 - 1);
  await ammo.selectOption("arrows_plus3");
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus3);
  await snap(page, "void-earlier-shot");
});
