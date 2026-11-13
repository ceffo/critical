import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  apiCall,
  chooseLanguage,
  logEntry,
  openDashboard,
  openWorkspace,
  reloadSettled,
  undoEntry,
} from "./helpers";

// Inventory tab: one happy path (use, restock, undo). The behaviour rules (charges, stock, daily,
// long-rest refill, empty refusal) are Go tests in itemsvc and restsvc.
// The journal and the result panel are the Act activity's (ADR 0022 step 2.5): the tests focus Act
// to read them, then come back.

async function openInventory(page: Page) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  await expect(page.getByTestId("item-wand_lightning")).toBeVisible();
}

function row(page: Page, id: string) {
  return page.getByTestId(`item-${id}`);
}

function count(page: Page, id: string) {
  return row(page, id).getByTestId("item-count");
}

function minus(page: Page, name: string) {
  return page.getByRole("button", { name: `Use one ${name}` });
}

function plus(page: Page, name: string) {
  return page.getByRole("button", { name: `Add one ${name}` });
}

function resultCard(page: Page) {
  return page
    .getByRole("region", { name: "Detail", exact: true })
    .getByTestId("item-result");
}

const act = (page: Page) => openWorkspace(page, "Act");
const inventory = (page: Page) => openWorkspace(page, "Inventory");

test("− uses one and + adds one, each logged and popped from the log", async ({
  page,
}) => {
  await openInventory(page);
  await minus(page, "Fire flasks").click();
  await expect(count(page, "fire_flask")).toHaveText("4");
  // The use shows at once in the header's last result, from the Inventory.
  await expect(page.getByTestId("last-result")).toContainText("Fire flasks");
  await act(page);
  // The minus is spend one (ADR 0049 step 2): the flasks hold a dice ability, and the minus rolls none.
  await expect(resultCard(page)).not.toContainText("5d6");
  await expect(resultCard(page)).toContainText("Fire flasks: 4 left");
  await expect(logEntry(page, "Used Fire flasks")).toContainText("(4 left)");

  await inventory(page);
  await plus(page, "Fire flasks").click();
  await expect(count(page, "fire_flask")).toHaveText("5");
  await act(page);
  await expect(
    logEntry(page, "Restocked Fire flasks: +1 (5 now)"),
  ).toContainText("Item");
  await expect(logEntry(page, "Restocked Fire flasks")).not.toContainText(
    "item_restock",
  );
  await snap(page, "flask-used-and-restocked");

  await undoEntry(page, "Restocked Fire flasks");
  await undoEntry(page, "Used Fire flasks");
  await inventory(page);
  await expect(count(page, "fire_flask")).toHaveText("5");
});

// A weapon, an armour and a shield are never used (ADR 0049 step 2): the server says so
// (`usable`), the Inventory rows carry no Use, count or +, and Act · Items lists only what is used.
for (const size of [
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
]) {
  test(`weapons, armour and shields have no use controls at ${size.width}x${size.height}`, async ({
    page,
  }) => {
    await page.setViewportSize(size);
    await openInventory(page);
    const rows = page.locator("[data-testid^='item-'][data-category]");
    const categories = await rows.evaluateAll((els) =>
      els.map((e) => e.getAttribute("data-category")),
    );
    expect(categories).toContain("weapon");
    for (const category of ["weapon", "armour", "shield"]) {
      const ones = page.locator(`[data-category='${category}']`);
      for (const [n] of (await ones.all()).entries()) {
        const one = ones.nth(n);
        await expect(one.getByTestId("item-count")).toHaveCount(0);
        await expect(one.getByTestId("item-behaviour")).toHaveCount(0);
        await expect(
          one.getByRole("button", { name: /^(Use|Add) one / }),
        ).toHaveCount(0);
      }
    }
    await expect(minus(page, "Fire flasks")).toBeVisible();
    await snap(page, `inventory-${size.width}x${size.height}`);

    await act(page);
    await page.getByRole("tab", { name: "Items" }).click();
    const list = page.getByRole("list", { name: "Item abilities" });
    await expect(list).toBeVisible();
    await expect(list.getByTestId("ability-limit")).not.toHaveCount(0);
    await expect(list).toContainText("Fire flasks");
    // Weapons, armour and shields (none holds an ability) are never listed.
    await expect(list).not.toContainText("Long Sword");
    await snap(page, `act-items-${size.width}x${size.height}`);
  });
}

// Act · Items: one row per ability, Use pays its limit and the result card and the journal show it
// (ADR 0049 steps 7 and 14); an exhausted ability stays, disabled, with the reason (the server's
// `usable` and `reason`). The limits and refusals are Go tests; the wiring and the wording are here.
test("Act · Items uses the Fire flasks, the Wand of lightning and the Power Gem", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await openDashboard(page);
  await act(page);
  await page.getByRole("tab", { name: "Items" }).click();
  const flasks = page.getByTestId("ability-fire_flask-use");
  const wand = page.getByTestId("ability-wand_lightning-use");
  const gem = page.getByTestId("ability-power_gem_dispel-use");
  await expect(flasks.getByTestId("ability-limit")).toHaveText("× 5");
  await expect(wand.getByTestId("ability-limit")).toHaveText("4 / 4 charges");
  await expect(gem.getByTestId("ability-limit")).toHaveText("1 / 1 per day");

  await flasks.getByRole("button", { name: "Use Fire flasks" }).click();
  await expect(flasks.getByTestId("ability-limit")).toHaveText("× 4");
  await expect(resultCard(page)).toContainText("Fire flasks: 4 left");
  await expect(resultCard(page).getByTestId("item-rolls")).toBeVisible();

  await wand.getByRole("button", { name: "Use Wand of lightning" }).click();
  await expect(wand.getByTestId("ability-limit")).toHaveText("3 / 4 charges");
  await expect(resultCard(page)).toContainText("Wand of lightning: 3 left");
  await expect(resultCard(page)).toContainText("6d6");
  await expect(logEntry(page, "Used Wand of lightning")).toContainText(
    "(3 left)",
  );

  await gem.getByRole("button", { name: "Use Power Gem" }).click();
  await expect(gem.getByTestId("ability-limit")).toHaveText("0 / 1 per day");
  await expect(gem.getByRole("button", { name: /^Use / })).toBeDisabled();
  await expect(gem.getByTestId("ability-reason")).toHaveText(
    "Used up until the long rest",
  );
  await snap(page, "act-items-used-1920x1080");

  await page.setViewportSize({ width: 2560, height: 1440 });
  await snap(page, "act-items-used-2560x1440");
  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Objets" }).click();
  await expect(page.getByTestId("ability-power_gem_dispel-use")).toContainText(
    "Épuisé jusqu'au repos long",
  );
  await snap(page, "act-items-used-fr-2560x1440");
});

// With nothing to use left (the weapons, armour and shield stay), Act · Items says why and points to
// the Inventory, in English and in French.
test("Act · Items empty state points to the Inventory (en, fr)", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await openDashboard(page);
  for (const id of ["fire_flask", "wand_lightning", "power_gem_dispel"])
    expect(await apiCall(page, "DELETE", `/api/items/${id}`, undefined)).toBe(
      true,
    );
  await reloadSettled(page);
  await act(page);
  await page.getByRole("tab", { name: "Items" }).click();
  await expect(
    page.getByText(/Add abilities to your items in the Inventory/),
  ).toBeVisible();
  await snap(page, "act-items-empty-en");

  await chooseLanguage(page, "fr");
  await page.getByRole("tab", { name: "Objets" }).click();
  await expect(
    page.getByText(/Ajoutez des capacités à vos objets dans l'Inventaire/),
  ).toBeVisible();
  await snap(page, "act-items-empty-fr");
});
