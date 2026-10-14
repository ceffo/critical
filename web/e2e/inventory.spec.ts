import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { logEntry, openDashboard, voidEntry } from "./helpers";

// Inventory tab (critical-qki.13): one item model, the count behaves as stock (− and +, stays
// at 0), charges (− only, gone at 0) or daily (− only, refilled by a long rest).

async function openInventory(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Inventory" }).click();
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
  return page.getByRole("region", { name: "Last Result" }).getByTestId("item-result");
}

async function longRest(page: Page) {
  await page.getByRole("tab", { name: "Spells" }).click();
  await page.getByRole("button", { name: "Long Rest (Reset)" }).click();
  await expect(logEntry(page, "Long rest")).toBeVisible();
  await page.getByRole("tab", { name: "Inventory" }).click();
}

test("the Inventory tab lists every item with its type, behaviour and count; + only on stock", async ({
  page,
}) => {
  await openInventory(page);
  const rows: [string, string, string, string, string][] = [
    ["arrows_plus1", "Arrows +1", "arrow", "Stock", "18"],
    ["arrows_plus2", "Arrows +2", "arrow", "Stock", "16"],
    ["arrows_plus3", "Arrows +3", "arrow", "Stock", "2"],
    ["fire_flask", "Fire flasks", "flask", "Stock", "5"],
    ["wand_lightning", "Wand of lightning", "wand", "Charges", "4"],
    ["power_gem_dispel", "Power Gem: Dispel Magic", "gem", "Daily", "1 / 1"],
  ];
  for (const [id, name, type, behaviour, n] of rows) {
    await expect(row(page, id)).toContainText(name);
    await expect(row(page, id).getByTestId("item-type")).toHaveText(type);
    await expect(row(page, id).getByTestId("item-behaviour")).toHaveText(behaviour);
    await expect(count(page, id)).toHaveText(n);
    await expect(minus(page, name)).toBeEnabled();
    await expect(plus(page, name)).toHaveCount(behaviour === "Stock" ? 1 : 0);
  }
  await expect(row(page, "arrows_plus2").getByTestId("item-bonus")).toHaveText("+2");
  await expect(row(page, "arrows_plus2").getByLabel("Magical bonus +2")).toBeVisible();
  await expect(row(page, "wand_lightning").getByTestId("item-bonus")).toHaveCount(0);
  await snap(page, "inventory-tab");
});

test("− uses one and + adds one, each logged and popped from the log", async ({ page }) => {
  await openInventory(page);
  await minus(page, "Fire flasks").click();
  await expect(count(page, "fire_flask")).toHaveText("4");
  await expect(resultCard(page)).toContainText("5d6");
  await expect(logEntry(page, "Used Fire flasks")).toContainText("(4 left)");

  await plus(page, "Fire flasks").click();
  await expect(count(page, "fire_flask")).toHaveText("5");
  await expect(logEntry(page, "Restocked Fire flasks: +1 (5 now)")).toContainText("Item");
  await expect(logEntry(page, "Restocked Fire flasks")).not.toContainText("item_restock");
  await snap(page, "flask-used-and-restocked");

  await voidEntry(page, "Restocked Fire flasks");
  await expect(count(page, "fire_flask")).toHaveText("4");
  await voidEntry(page, "Used Fire flasks");
  await expect(count(page, "fire_flask")).toHaveText("5");
});

test("the wand rolls 6d6 with a breakdown and the save it allows", async ({ page }) => {
  await openInventory(page);
  await minus(page, "Wand of lightning").click();

  const card = resultCard(page);
  await expect(card).toContainText("Used Wand of lightning");
  await expect(card.getByTestId("item-rolls").getByRole("listitem")).toHaveCount(6);
  await expect(card.getByTestId("item-save")).toHaveText("Save: Rod/Staff/Wand");
  await expect(card.getByTestId("item-left")).toHaveText("Wand of lightning: 3 left");
  const rolls = (await card.getByTestId("item-rolls").getByRole("listitem").allTextContents()).map(Number);
  const total = Number(await card.getByTestId("item-total").textContent());
  expect(total).toBe(rolls.reduce((a, b) => a + b, 0));
  await expect(count(page, "wand_lightning")).toHaveText("3");
  await expect(logEntry(page, "Used Wand of lightning")).toContainText(`6d6 = ${total} (3 left)`);
  await snap(page, "wand-used");
});

test("a wand used down to 0 disappears; popping that use brings it back with 1 charge", async ({
  page,
}) => {
  await openInventory(page);
  for (const left of ["3", "2", "1"]) {
    await minus(page, "Wand of lightning").click();
    await expect(count(page, "wand_lightning")).toHaveText(left);
  }
  await minus(page, "Wand of lightning").click();
  await expect(row(page, "wand_lightning")).toHaveCount(0);
  await expect(resultCard(page).getByTestId("item-left")).toHaveText(
    "Wand of lightning: last charge used, removed from the inventory",
  );
  const last = logEntry(page, "(last charge, removed)");
  await expect(last).toContainText("Used Wand of lightning: 6d6");
  await snap(page, "wand-removed");

  await voidEntry(page, "(last charge, removed)");
  await expect(count(page, "wand_lightning")).toHaveText("1");
  await expect(row(page, "power_gem_dispel")).toBeVisible();
  // Back in its place, between the flasks and the gem.
  const ids = await page
    .getByRole("tabpanel")
    .getByRole("listitem")
    .evaluateAll((els) => els.map((e) => e.getAttribute("data-testid")));
  expect(ids.slice(3)).toEqual(["item-fire_flask", "item-wand_lightning", "item-power_gem_dispel"]);
  await snap(page, "wand-back");
});

test("arrows at 0 stay listed and can be restocked; the attack picker offers only arrows left", async ({
  page,
}) => {
  await openInventory(page);
  for (const left of ["1", "0"]) {
    await minus(page, "Arrows +3").click();
    await expect(count(page, "arrows_plus3")).toHaveText(left);
  }
  await expect(row(page, "arrows_plus3")).toBeVisible();
  await expect(minus(page, "Arrows +3")).toBeDisabled();
  await snap(page, "arrows-at-zero");

  await page.getByRole("tab", { name: "Attack" }).click();
  const form = page.getByRole("form", { name: "Attack" });
  await form.getByLabel("Weapon").selectOption("shortbow");
  const ammo = form.getByLabel("Ammunition");
  await expect(ammo.locator("option")).toHaveText(["Arrows +1 (18 left)", "Arrows +2 (16 left)"]);

  await page.getByRole("tab", { name: "Inventory" }).click();
  await plus(page, "Arrows +3").click();
  await expect(count(page, "arrows_plus3")).toHaveText("1");
  await expect(logEntry(page, "Restocked Arrows +3: +1 (1 now)")).toBeVisible();

  await page.getByRole("tab", { name: "Attack" }).click();
  await form.getByLabel("Weapon").selectOption("shortbow");
  await expect(ammo.locator('option[value="arrows_plus3"]')).toHaveText("Arrows +3 (1 left)");
  await snap(page, "arrows-restocked-in-picker");
});

test("a long rest refills the gem (daily) and nothing else", async ({ page }) => {
  await openInventory(page);
  await minus(page, "Power Gem: Dispel Magic").click();
  await expect(count(page, "power_gem_dispel")).toHaveText("0 / 1");
  await expect(minus(page, "Power Gem: Dispel Magic")).toBeDisabled();
  await expect(row(page, "power_gem_dispel")).toBeVisible();
  for (const [name, id, left] of [
    ["Wand of lightning", "wand_lightning", "3"],
    ["Fire flasks", "fire_flask", "4"],
    ["Arrows +1", "arrows_plus1", "17"],
  ]) {
    await minus(page, name).click();
    await expect(count(page, id)).toHaveText(left);
  }
  await snap(page, "before-long-rest");

  await longRest(page);
  await expect(count(page, "power_gem_dispel")).toHaveText("1 / 1");
  await expect(count(page, "wand_lightning")).toHaveText("3");
  await expect(count(page, "fire_flask")).toHaveText("4");
  await expect(count(page, "arrows_plus1")).toHaveText("17");
  await expect(logEntry(page, "Long rest")).toContainText("1 items refilled");
  await snap(page, "after-long-rest");
});

// --- API-level rules: restock refusals and empty items ---

async function post(base: string, path: string, body: unknown = {}) {
  const res = await fetch(`${base}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return { status: res.status, body: (await res.json().catch(() => ({}))) as Record<string, unknown> };
}

async function apiCount(base: string, id: string): Promise<number | undefined> {
  const items = (await (await fetch(`${base}/api/items`)).json()) as { id: string; count: number }[];
  return items.find((i) => i.id === id)?.count;
}

test("charges and daily items cannot be restocked; a bad count is refused", async ({ stack }) => {
  for (const id of ["wand_lightning", "power_gem_dispel"]) {
    const res = await post(stack.apiUrl, `/api/items/${id}/restock`, { count: 1 });
    expect(res.status).toBe(409);
    expect(res.body).toEqual({ error: expect.stringContaining("cannot be restocked") });
  }
  expect(await apiCount(stack.apiUrl, "wand_lightning")).toBe(4);
  expect((await post(stack.apiUrl, "/api/items/arrows_plus1/restock", { count: 0 })).status).toBe(400);
  expect((await post(stack.apiUrl, "/api/items/arrows_plus1/restock", {})).status).toBe(400);
  expect((await post(stack.apiUrl, "/api/items/nope/restock", { count: 1 })).status).toBe(404);
  expect(await apiCount(stack.apiUrl, "arrows_plus1")).toBe(18);
});

test("an empty item is refused by the API, and a stale − shows the error", async ({ page, stack }) => {
  await openInventory(page);
  // The gem is emptied behind the UI's back; the stale row still offers −.
  expect((await post(stack.apiUrl, "/api/items/power_gem_dispel/use")).status).toBe(200);
  const refused = await post(stack.apiUrl, "/api/items/power_gem_dispel/use");
  expect(refused.status).toBe(409);
  expect(refused.body).toEqual({ error: expect.stringContaining("empty") });

  await expect(count(page, "power_gem_dispel")).toHaveText("1 / 1");
  await minus(page, "Power Gem: Dispel Magic").click();
  await expect(page.getByRole("alert")).toContainText("empty");
  await expect(count(page, "power_gem_dispel")).toHaveText("0 / 1");
  await snap(page, "use-at-zero-error");
});
