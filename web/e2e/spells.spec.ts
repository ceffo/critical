import { test, expect, snap } from "./fixtures";
import { expectSlotDots, logEntry, openDashboard } from "./helpers";
import type { Page } from "@playwright/test";

async function openSpells(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(
    page.getByRole("button", { name: "Prepare spells" }),
  ).toBeVisible();
}

async function add(page: Page, ...names: string[]) {
  for (const name of names) {
    await page
      .getByRole("button", { name: `Add ${name}`, exact: true })
      .click();
  }
}

async function put(apiUrl: string, spells: string[]) {
  return fetch(`${apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells }),
  });
}

// A spell prepared several times is one row with a count, rows are sorted by name; a long rest
// refills the used slots and keeps the preparation. The refill and the grouping rules are Go and
// component tests (restsvc, LongRestButton, prepareDraft); the browser proves the wiring.
test("duplicates are prepared, grouped by name and a long rest resets used", async ({
  page,
  stack,
}) => {
  expect(
    (await put(stack.apiUrl, ["magic_missile", "alarm", "magic_missile"])).ok,
  ).toBe(true);
  await openSpells(page);
  const level1 = page.getByRole("region", { name: "Level 1" });
  await expect(level1.getByRole("listitem").nth(0)).toContainText("Alarm");
  await expect(level1.getByRole("listitem").nth(1)).toContainText(
    "Magic Missile ×2",
  );
  await level1.getByRole("button", { name: "Cast Magic Missile" }).click();
  await expectSlotDots(level1, 1, 2, 3);
  await snap(page, "grouped-spells");

  await page.getByRole("button", { name: "Long rest" }).click();
  await expectSlotDots(level1, 1, 3, 3);
  await expect(level1).toContainText("Magic Missile ×2");
  await snap(page, "after-long-rest");
});

test("preparing is logged and voiding it restores the previous preparation", async ({
  page,
  stack,
}) => {
  expect((await put(stack.apiUrl, ["alarm"])).ok).toBe(true);
  await openSpells(page);
  const level1 = page.getByRole("region", { name: "Level 1" });
  await expect(level1).toContainText("Alarm");

  await page.getByRole("button", { name: "Prepare spells" }).click();
  await page
    .getByRole("form", { name: "Prepare today's spells" })
    .getByRole("button", { name: "Remove Alarm" })
    .click();
  await add(page, "Magic Missile", "Identify");
  await page.getByRole("button", { name: "Save preparation" }).click();
  await expect(level1).toContainText("Magic Missile");
  await expect(level1).not.toContainText("Alarm");

  const entry = logEntry(page, "Prepared: Magic Missile, Identify");
  await expect(entry).toBeVisible();
  await snap(page, "prepared-logged");
  await page.getByTestId("undo").click();

  await expect(entry).toHaveAttribute("data-status", "voided");
  await expect(level1).toContainText("Alarm");
  await expect(level1).not.toContainText("Magic Missile");
  await snap(page, "prepare-voided");
});

test("the spell names follow the language: the catalogue's French name, English without one", async ({
  page,
  stack,
}) => {
  expect(
    (await put(stack.apiUrl, ["mirror_image", "melfs_acid_arrow", "fireball"]))
      .ok,
  ).toBe(true);
  await page.addInitScript(() =>
    localStorage.setItem(
      "critical.settings.v1",
      JSON.stringify({ lang: "fr", mode: "system", amb: "arcane" }),
    ),
  );
  await openDashboard(page);
  await page.getByRole("tab", { name: "Sorts", exact: true }).click();
  await expect(page.getByText("Image miroir", { exact: true })).toBeVisible();
  await expect(page.getByText("Boule de feu", { exact: true })).toBeVisible();
  // The longest name in a narrow two-column row: shown whole, no overflow, Cast still visible.
  const acid = page
    .getByRole("region", { name: "Niveau 2" })
    .getByRole("listitem")
    .filter({ hasText: "Melf's Acid Arrow" });
  await expect(acid.getByRole("button", { name: "Lancer" })).toBeInViewport({
    ratio: 1,
  });
  for (const el of [acid, acid.getByText("Melf's Acid Arrow")]) {
    expect(await el.evaluate((e) => e.scrollWidth <= e.clientWidth)).toBe(true);
  }
  await snap(page, "french");

  // A buff from a spell is named from the catalogue too, in the timeline and in the journal.
  await page
    .getByRole("button", { name: "Lancer Image miroir" })
    .first()
    .click();
  const buff = page.locator('[data-testid="buff"][data-buff="Mirror Image"]');
  await expect(buff).toContainText("Image miroir");
  await expect(page.getByTestId("cast-buff")).toContainText("Image miroir");
  await snap(page, "french-buff");
  await page.reload();
  await expect(buff).toContainText("Image miroir");

  // Names are never truncated: a long name wraps in the timeline and on its charge card.
  const long = "Protection contre le mal sur trois mètres de rayon";
  expect(
    (
      await fetch(`${stack.apiUrl}/api/buffs`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: long, charges: 3, effects: {} }),
      })
    ).ok,
  ).toBe(true);
  await page.reload();
  const row = page.locator(`[data-testid="buff"][data-buff="${long}"]`);
  const card = page.locator(`[data-testid="charge-card"][data-buff="${long}"]`);
  for (const width of [1440, 1280]) {
    await page.setViewportSize({ width, height: 900 });
    for (const el of [row, card]) {
      await expect(el.getByText(long, { exact: true })).toBeVisible();
      expect(
        await el.evaluate(
          (root) =>
            [root, ...root.querySelectorAll("*")].filter(
              (e) =>
                getComputedStyle(e).textOverflow === "ellipsis" ||
                (e.textContent?.includes("Protection") &&
                  e.scrollWidth > e.clientWidth),
            ).length,
        ),
      ).toBe(0);
    }
    await snap(page, `french-long-buff-${width}`);
  }
});
