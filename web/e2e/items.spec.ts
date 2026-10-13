import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { logEntry, openDashboard, voidEntry } from "./helpers";

async function openItems(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Items" }).click();
  await expect(page.getByTestId("item-wand_lightning")).toBeVisible();
}

function row(page: Page, id: string) {
  return page.getByTestId(`item-${id}`);
}

function resultCard(page: Page) {
  return page.getByRole("region", { name: "Last Result" }).getByTestId("item-result");
}

test("the Items tab counts every item and arrow type", async ({ page }) => {
  await openItems(page);
  await expect(row(page, "arrows_plus1").getByTestId("item-count")).toHaveText("18 left");
  await expect(row(page, "arrows_plus2").getByTestId("item-count")).toHaveText("16 left");
  await expect(row(page, "arrows_plus3").getByTestId("item-count")).toHaveText("2 left");
  await expect(row(page, "wand_lightning").getByTestId("item-count")).toHaveText("4 / 4");
  await expect(row(page, "fire_flask").getByTestId("item-count")).toHaveText("5 / 5");
  await expect(row(page, "power_gem_dispel").getByTestId("item-count")).toHaveText("1 / 1");
  await snap(page, "items-tab");
});

test("the wand rolls 6d6 with a breakdown, logs it, and voiding gives the charge back", async ({
  page,
}) => {
  await openItems(page);
  await row(page, "wand_lightning").getByRole("button", { name: /^Use/ }).click();

  const card = resultCard(page);
  await expect(card).toContainText("Used Wand of lightning");
  await expect(card).toContainText("6d6");
  await expect(card.getByTestId("item-rolls").getByRole("listitem")).toHaveCount(6);
  await expect(card.getByTestId("item-save")).toHaveText("Save: Rod/Staff/Wand");
  await expect(card.getByTestId("item-left")).toHaveText("Wand of lightning: 3 left");

  // The total is the sum of the dice shown, and a 6d6 total is between 6 and 36.
  const rolls = (await card.getByTestId("item-rolls").getByRole("listitem").allTextContents()).map(Number);
  const total = Number(await card.getByTestId("item-total").textContent());
  expect(total).toBe(rolls.reduce((a, b) => a + b, 0));
  expect(total).toBeGreaterThanOrEqual(6);
  expect(total).toBeLessThanOrEqual(36);

  await expect(row(page, "wand_lightning").getByTestId("item-count")).toHaveText("3 / 4");
  const entry = logEntry(page, "Used Wand of lightning");
  await expect(entry).toContainText(`6d6 = ${total} (3 left)`);
  await snap(page, "wand-used");

  await voidEntry(page, "Used Wand of lightning");
  await expect(row(page, "wand_lightning").getByTestId("item-count")).toHaveText("4 / 4");
  await snap(page, "wand-voided");
});

test("fire flasks roll 5d6 and run out", async ({ page }) => {
  await openItems(page);
  const use = row(page, "fire_flask").getByRole("button", { name: /^Use/ });
  await use.click();
  const card = resultCard(page);
  await expect(card).toContainText("5d6");
  await expect(card.getByTestId("item-rolls").getByRole("listitem")).toHaveCount(5);
  await expect(card.getByTestId("item-save")).toHaveCount(0);
  await expect(row(page, "fire_flask").getByTestId("item-count")).toHaveText("4 / 5");

  for (let left = 3; left >= 0; left--) {
    await use.click();
    await expect(row(page, "fire_flask").getByTestId("item-count")).toHaveText(`${left} / 5`);
  }
  await expect(row(page, "fire_flask").getByRole("button", { name: /^Empty/ })).toBeDisabled();
  await snap(page, "flasks-empty");
});

test("an empty item is refused by the API with an error", async ({ page, stack }) => {
  await openItems(page);
  const url = `${stack.apiUrl}/api/items/power_gem_dispel/use`;
  expect((await page.request.post(url, { data: {} })).status()).toBe(200);
  const refused = await page.request.post(url, { data: {} });
  expect(refused.status()).toBe(409);
  expect(await refused.json()).toEqual({ error: expect.stringContaining("empty") });
});

test("the Power Gem's Dispel is usable once, until a long rest", async ({ page }) => {
  await openItems(page);
  const gem = row(page, "power_gem_dispel");
  await gem.getByRole("button", { name: /^Use/ }).click();

  const card = resultCard(page);
  await expect(card).toContainText("Used Power Gem: Dispel Magic");
  await expect(card.getByTestId("item-rolls")).toHaveCount(0);
  await expect(card.getByTestId("item-left")).toHaveText("Power Gem: Dispel Magic: 0 left");
  await expect(gem.getByTestId("item-count")).toHaveText("0 / 1");
  await expect(gem.getByRole("button", { name: /^Empty/ })).toBeDisabled();
  await expect(logEntry(page, "Used Power Gem: Dispel Magic")).toBeVisible();
  await snap(page, "gem-used");

  await page.getByRole("tab", { name: "Spells" }).click();
  await page.getByRole("button", { name: "Long Rest (Reset)" }).click();
  await page.getByRole("tab", { name: "Items" }).click();
  await expect(gem.getByTestId("item-count")).toHaveText("1 / 1");
  await expect(gem.getByRole("button", { name: /^Use/ })).toBeEnabled();
  await snap(page, "gem-after-long-rest");
});
