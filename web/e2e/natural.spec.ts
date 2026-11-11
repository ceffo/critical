import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  chooseLanguage,
  chooseTheme,
  chooseWeapon,
  openDashboard,
  openWeapons,
} from "./helpers";

// Only a browser proves the treatment's look in a real theme and language: the border round the roll
// row, the chip's words and icon, and that the screenshot has them. Which dice show it is unit-tested
// (attackCard.test.tsx). The dice are on a fixed seed (ADR 0025 step 8.5): the seeds below roll a
// natural 20 and a natural 1 as the first attack's d20.

function rollRow(page: Page) {
  // By test id alone: the region's name changes with the language.
  return page.getByTestId("attack-roll");
}

async function attackOnce(page: Page) {
  await openDashboard(page);
  await openWeapons(page);
  await chooseWeapon(page, "Long Sword +2");
  const form = attackForm(page);
  await form.getByLabel("Target AC").fill("5");
  await form.getByRole("button", { name: "Attack", exact: true }).click();
}

// English in the dark Arcane theme, then French in the light Parchment theme (the OS appearance is
// what the theme list follows, ADR 0026 step 4.6).
async function shots(page: Page, name: string) {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(rollRow(page)).toBeVisible();
  await snap(page, `${name}-en-arcane`);
  await page.emulateMedia({ colorScheme: "light" });
  await chooseTheme(page, "Parchment");
  await chooseLanguage(page, "fr");
  await expect(rollRow(page)).toBeVisible();
  await snap(page, `${name}-fr-parchment`);
}

test.describe("natural 20", () => {
  test.use({ dice: 1 });
  test("shows the gold border and the Natural 20 chip", async ({ page }) => {
    await attackOnce(page);
    const row = rollRow(page);
    await expect(row).toHaveAttribute("data-natural", "20");
    await expect(row.getByTestId("natural-chip")).toHaveText("Natural 20");
    await shots(page, "natural-20");
  });
});

test.describe("natural 1", () => {
  test.use({ dice: 8 });
  test("shows the danger border and the Natural 1 chip", async ({ page }) => {
    await attackOnce(page);
    const row = rollRow(page);
    await expect(row).toHaveAttribute("data-natural", "1");
    await expect(row.getByTestId("natural-chip")).toHaveText("Natural 1");
    await shots(page, "natural-1");
  });
});
