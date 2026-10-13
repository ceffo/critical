import { test, expect, snap } from "./fixtures";
import {
  buffBadge,
  effectiveStr,
  logEntry,
  openDashboard,
  voidEntry,
} from "./helpers";
import type { Page } from "@playwright/test";

// Dice are random on the real stack: assert what the cast did, not the rolled numbers
// (castsvc's Go tests pin the rolls).

async function prepare(apiUrl: string, spells: string[]) {
  const res = await fetch(`${apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells }),
  });
  expect(res.ok).toBe(true);
}

async function openSpells(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(
    page.getByRole("button", { name: "Prepare spells" }),
  ).toBeVisible();
}

function resultCard(page: Page) {
  return page.getByRole("region", { name: "Last Result" });
}

test("casting Stoneskin shows the result card, adds the buff; voiding gives the slot back", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["stoneskin"]);
  await openSpells(page);
  const level4 = page.getByRole("region", { name: "Level 4" });
  await expect(level4.getByTestId("remaining-4")).toHaveText("1 / 1");

  await level4.getByRole("button", { name: "Cast Stoneskin" }).click();
  const card = resultCard(page).getByTestId("cast-result");
  await expect(card).toContainText(
    "Stoneskin (level 4), cast at caster level 15",
  );
  await expect(card.getByTestId("cast-buff")).toContainText("On you");
  await expect(card.getByTestId("cast-charges")).toContainText("1d4+level/2");
  await expect(card.getByTestId("cast-text")).toBeVisible();
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await expect(level4.getByTestId("remaining-4")).toHaveText("0 / 1");
  await expect(logEntry(page, "Cast Stoneskin")).toBeVisible();
  await snap(page, "stoneskin-cast");

  await voidEntry(page, "Cast Stoneskin");
  await expect(buffBadge(page, "Stoneskin")).toHaveCount(0);
  await expect(level4.getByTestId("remaining-4")).toHaveText("1 / 1");
  await snap(page, "stoneskin-voided");
});

test("casting Fireball rolls 10d6 and notes the save for the DM", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["fireball"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Fireball" }).click();

  const damage = resultCard(page).getByTestId("cast-damage");
  await expect(damage).toContainText("1 × (min(level,10))d6");
  const total = Number(
    await damage.getByTestId("cast-damage-total").textContent(),
  );
  expect(total).toBeGreaterThanOrEqual(10);
  expect(total).toBeLessThanOrEqual(60);
  await expect(resultCard(page).getByTestId("cast-save")).toContainText(
    "spell, half damage",
  );
  await expect(
    logEntry(page, `Cast Fireball: ${total} fire damage`),
  ).toBeVisible();
  await snap(page, "fireball");
});

test("a low-confidence spell shows its text only", async ({ page, stack }) => {
  await prepare(stack.apiUrl, ["wall_of_ice"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Wall of Ice" }).click();
  await expect(resultCard(page).getByTestId("cast-text-only")).toContainText(
    "add a buff by hand",
  );
  await expect(page.locator('[data-testid="buff"]')).toHaveCount(0);
  await snap(page, "low-confidence");
});

test("Strength cast on me by a party wizard, found by its French name", async ({
  page,
}) => {
  await openSpells(page);
  const before = await effectiveStr(page);
  const picker = page.getByRole("region", { name: "Spell cast on me" });
  await picker.getByLabel("Search for a spell cast on me").fill("Force");
  await picker.getByLabel("Caster level").fill("9");
  await picker.getByRole("button", { name: "Apply Strength" }).click();

  const card = resultCard(page).getByTestId("cast-result");
  await expect(card).toContainText("Strength cast on you, caster level 9");
  await expect(card.getByTestId("cast-rounds")).toContainText("= 540");
  await expect(card.getByTestId("cast-str-gain")).toContainText("1d6");
  await expect(buffBadge(page, "Strength")).toBeVisible();
  await expect.poll(() => effectiveStr(page)).toBeGreaterThan(before);
  await expect(
    logEntry(page, "Strength cast on me (caster level 9)"),
  ).toBeVisible();
  await snap(page, "strength-received");
});

test("voiding a cast after Next Round is refused with a message naming the later entry", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["stoneskin"]);
  await openSpells(page);
  await page.getByRole("button", { name: "Cast Stoneskin" }).click();
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await page.getByRole("button", { name: "Next Round" }).click();
  await expect(logEntry(page, "Round 0 → 1")).toBeVisible();

  const entry = logEntry(page, "Cast Stoneskin");
  await entry.getByRole("button", { name: "Void" }).click();
  await entry.getByRole("button", { name: "Confirm" }).click();
  const alert = page.getByRole("alert");
  await expect(alert).toContainText(
    "void blocked: a later entry changed the same state",
  );
  await expect(alert).toContainText("void it first");
  await expect(entry).not.toHaveAttribute("data-status", "voided");
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await snap(page, "void-blocked-after-next-round");
});

test("a long rest is logged and blocks voiding an earlier cast", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["haste"]);
  await openSpells(page);
  const level3 = page.getByRole("region", { name: "Level 3" });
  await page.getByRole("button", { name: "Cast Haste" }).click();
  await expect(level3.getByTestId("remaining-3")).toHaveText("0 / 1");
  await page.getByRole("button", { name: "Long Rest (Reset)" }).click();
  await expect(level3.getByTestId("remaining-3")).toHaveText("1 / 1");
  await expect(logEntry(page, "Long rest: 1 spell slots")).toBeVisible();

  const entry = logEntry(page, "Cast Haste");
  await entry.getByRole("button", { name: "Void" }).click();
  await entry.getByRole("button", { name: "Confirm" }).click();
  await expect(page.getByRole("alert")).toContainText("Long rest");
  await expect(level3.getByTestId("remaining-3")).toHaveText("1 / 1");
  await snap(page, "void-blocked-after-long-rest");
});
