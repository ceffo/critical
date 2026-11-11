import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  logEntry,
  openDashboard,
  openWorkspace,
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
