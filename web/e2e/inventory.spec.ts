import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { logEntry, openDashboard, openWorkspace, undoEntry } from "./helpers";

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
  await expect(resultCard(page)).toContainText("5d6");
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
    const list = page.getByRole("list", { name: "Combat items" });
    await expect(list).toBeVisible();
    await expect(list.getByTestId("item-count")).not.toHaveCount(0);
    await expect(list).toContainText("Fire flasks");
    await snap(page, `act-items-${size.width}x${size.height}`);
  });
}
