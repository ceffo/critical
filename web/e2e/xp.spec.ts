import { test, expect, snap } from "./fixtures";
import { openDashboard } from "./helpers";
import type { Page } from "@playwright/test";

const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

/** The XP total shown in the progress card, as a number (en formatting: "1,293,418"). */
async function xpTotal(page: Page): Promise<number> {
  const text = (await page.getByTestId("xp-total").textContent()) ?? "";
  const digits = text.replace(/[^\d]/g, "");
  if (!digits) throw new Error(`unexpected XP total: ${JSON.stringify(text)}`);
  return Number(digits);
}

// What only a browser proves: the Moi level link opens the workspace, the add form's preview,
// Enter submits, in-row edit (Enter saves) and delete re-render the server's totals. The rules
// (rounding, eligibility, Table 25) and CRUD edge cases are Go tests (xpsvc, xpdal, composition).
test("experience: add with the bonus, edit, delete", async ({ page }) => {
  await openDashboard(page);
  const moi = page.getByRole("complementary", { name: "Character" });
  await moi.getByRole("button", { name: /^Level \d+$/ }).click();
  const ws = page.getByRole("region", { name: "Experience", exact: true });
  await expect(ws).toBeVisible();
  await expect(
    page
      .getByRole("navigation", { name: "Workspaces" })
      .getByRole("button", { name: "Experience" }),
  ).toHaveAttribute("aria-current", "page");
  await expect(ws.getByTestId("xp-total")).toHaveText(/\d/);
  const before = await xpTotal(page);

  // Add 1,205 XP with the +10% (default on for the eligible Bard): the server previews 1,326.
  const add = ws.getByRole("region", { name: "Add XP" });
  await expect(add.getByRole("checkbox")).toBeChecked();
  await add.getByLabel("Amount").fill("1205");
  await add.getByLabel("Reason").fill("Troll bridge");
  await expect(add.getByRole("button", { name: "Add 1,326" })).toBeVisible();
  await add.getByLabel("Reason").press("Enter");
  await expect.poll(() => xpTotal(page)).toBe(before + 1326);
  const ledger = ws.getByRole("region", { name: "Ledger" });
  const row = ledger.getByRole("listitem", { name: "Troll bridge" });
  await expect(row.getByTestId("xp-row-total")).toHaveText("1,326");
  await expect(add.getByLabel("Amount")).toHaveValue("");

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(row).toBeVisible();
    const scroll = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return {
        x: el.scrollWidth - el.clientWidth,
        y: el.scrollHeight - el.clientHeight,
      };
    });
    expect(scroll, "no page scroll").toEqual({ x: 0, y: 0 });
    await snap(page, `experience-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });

  // Edit in the row: 1,000 without the bonus, Enter saves.
  await row.getByRole("button", { name: "Edit Troll bridge" }).click();
  const editing = ledger.getByRole("listitem", { name: "Editing an entry" });
  await editing.getByLabel("Amount").fill("1000");
  await editing.getByRole("checkbox").uncheck();
  await expect(editing.getByTestId("xp-edit-total")).toHaveText("1,000");
  await editing.getByLabel("Amount").press("Enter");
  await expect.poll(() => xpTotal(page)).toBe(before + 1000);
  await expect(row.getByTestId("xp-row-total")).toHaveText("1,000");

  // Delete is final, so it asks first: Escape keeps the entry, Confirm removes it.
  await row.getByRole("button", { name: "Delete Troll bridge" }).click();
  await expect(
    row.getByRole("button", { name: "Confirm deleting Troll bridge" }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(row.getByTestId("xp-row-total")).toHaveText("1,000");
  expect(await xpTotal(page)).toBe(before + 1000);
  await row.getByRole("button", { name: "Delete Troll bridge" }).click();
  await row
    .getByRole("button", { name: "Confirm deleting Troll bridge" })
    .click();
  await expect.poll(() => xpTotal(page)).toBe(before);
  await expect(row).toHaveCount(0);
});
