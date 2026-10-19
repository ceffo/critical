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
} from "./helpers";

// Inventory tab: one item model, the count behaves as stock (− and +, stays
// at 0), charges (− only, gone at 0) or daily (− only, refilled by a long rest).
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

async function longRest(page: Page) {
  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Spells" }).click();
  await page.getByRole("button", { name: "Long rest" }).click();
  await expect(logEntry(page, "Long rest")).toBeVisible();
  await openWorkspace(page, "Inventory");
}

test("the Inventory tab lists every item with its behaviour and count; + only on stock", async ({
  page,
}) => {
  await openInventory(page);
  const rows: [string, string, string, string][] = [
    ["arrows_plus1", "Arrows +1", "Stock", "18"],
    ["arrows_plus2", "Arrows +2", "Stock", "16"],
    ["arrows_plus3", "Arrows +3", "Stock", "2"],
    ["fire_flask", "Fire flasks", "Stock", "5"],
    ["wand_lightning", "Wand of lightning", "Charges", "4"],
    ["power_gem_dispel", "Power Gem", "Daily", "1 / 1"],
  ];
  for (const [id, name, behaviour, n] of rows) {
    await expect(row(page, id)).toContainText(name);
    // The meta line is the behaviour in the UI language; the free-text type stays in the form.
    await expect(row(page, id).getByTestId("item-type")).toHaveCount(0);
    await expect(row(page, id).getByTestId("item-behaviour")).toHaveText(
      behaviour,
    );
    await expect(count(page, id)).toHaveText(n);
    await expect(minus(page, name)).toBeEnabled();
    await expect(plus(page, name)).toHaveCount(behaviour === "Stock" ? 1 : 0);
  }
  await expect(row(page, "arrows_plus2").getByTestId("item-bonus")).toHaveText(
    "+2",
  );
  await expect(
    row(page, "arrows_plus2").getByLabel("Magical bonus +2"),
  ).toBeVisible();
  await expect(
    row(page, "wand_lightning").getByTestId("item-bonus"),
  ).toHaveCount(0);
  await snap(page, "inventory-tab");
});

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

test("the wand rolls 6d6 with a breakdown and the save it allows", async ({
  page,
}) => {
  await openInventory(page);
  await minus(page, "Wand of lightning").click();
  await expect(count(page, "wand_lightning")).toHaveText("3");

  await act(page);
  const card = resultCard(page);
  await expect(card).toContainText("Used Wand of lightning");
  await expect(
    card.getByTestId("item-rolls").getByRole("listitem"),
  ).toHaveCount(6);
  await expect(card.getByTestId("item-save")).toHaveText(
    "Save: Rod/Staff/Wand",
  );
  await expect(card.getByTestId("item-left")).toHaveText(
    "Wand of lightning: 3 left",
  );
  const rolls = (
    await card.getByTestId("item-rolls").getByRole("listitem").allTextContents()
  ).map(Number);
  const total = Number(await card.getByTestId("item-total").textContent());
  expect(total).toBe(rolls.reduce((a, b) => a + b, 0));
  await expect(logEntry(page, "Used Wand of lightning")).toContainText(
    `6d6 = ${total} (3 left)`,
  );
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
  await act(page);
  await expect(resultCard(page).getByTestId("item-left")).toHaveText(
    "Wand of lightning: last charge used, removed from the inventory",
  );
  const last = logEntry(page, "(last charge, removed)");
  await expect(last).toContainText("Used Wand of lightning: 6d6");
  await snap(page, "wand-removed");

  await undoEntry(page, "(last charge, removed)");
  await inventory(page);
  await expect(count(page, "wand_lightning")).toHaveText("1");
  await expect(row(page, "power_gem_dispel")).toBeVisible();
  // Back in its place, between the flasks and the gem.
  const ids = await page
    .getByRole("list", { name: "Inventory", exact: true })
    .getByRole("listitem")
    .evaluateAll((els) => els.map((e) => e.getAttribute("data-testid")));
  expect(ids.slice(3, 6)).toEqual([
    "item-fire_flask",
    "item-wand_lightning",
    "item-power_gem_dispel",
  ]);
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

  await openWorkspace(page, "Act");
  await openWeapons(page);
  await chooseWeapon(page, "Short Bow +2");
  const ammo = ammoChips(page);
  // The spent +3 arrows were never chosen here: not offered.
  await expect(ammo.getByRole("radio")).toHaveCount(2);
  await expect(
    ammo.getByRole("radio", { name: "Arrows +1, 18 left" }),
  ).toBeVisible();
  await expect(
    ammo.getByRole("radio", { name: "Arrows +2, 16 left" }),
  ).toBeVisible();

  await openWorkspace(page, "Inventory");
  await plus(page, "Arrows +3").click();
  await expect(count(page, "arrows_plus3")).toHaveText("1");

  await act(page);
  await expect(logEntry(page, "Restocked Arrows +3: +1 (1 now)")).toBeVisible();
  await expect(
    ammo.getByRole("radio", { name: "Arrows +3, 1 left" }),
  ).toBeVisible();
  await snap(page, "arrows-restocked-in-picker");
});

test("a long rest refills the gem (daily) and nothing else", async ({
  page,
}) => {
  await openInventory(page);
  await minus(page, "Power Gem").click();
  await expect(count(page, "power_gem_dispel")).toHaveText("0 / 1");
  await expect(minus(page, "Power Gem")).toBeDisabled();
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
  await snap(page, "after-long-rest");
  await act(page);
  await expect(logEntry(page, "Long rest")).toContainText("1 items refilled");
});

// --- API-level rules: restock refusals and empty items ---

async function post(base: string, path: string, body: unknown = {}) {
  const res = await fetch(`${base}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return {
    status: res.status,
    body: (await res.json().catch(() => ({}))) as Record<string, unknown>,
  };
}

test("an empty item is refused by the API, and a stale − shows the error", async ({
  page,
  stack,
}) => {
  await openInventory(page);
  // The gem is emptied behind the UI's back; the stale row still offers −.
  expect(
    (await post(stack.apiUrl, "/api/items/power_gem_dispel/use")).status,
  ).toBe(200);
  const refused = await post(stack.apiUrl, "/api/items/power_gem_dispel/use");
  expect(refused.status).toBe(409);
  expect(refused.body).toEqual({
    code: "item_empty",
    error: expect.stringContaining("empty"),
  });

  await expect(count(page, "power_gem_dispel")).toHaveText("1 / 1");
  await minus(page, "Power Gem").click();
  await expect(page.getByRole("alert")).toContainText("empty");
  await expect(count(page, "power_gem_dispel")).toHaveText("0 / 1");
  await snap(page, "use-at-zero-error");
});
