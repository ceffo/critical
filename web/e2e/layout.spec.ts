import { test, expect, snap } from "./fixtures";
import {
  ammoChips,
  chooseLanguage,
  chooseSetting,
  chooseWeapon,
  openDashboard,
  openSettings,
  openWeapons,
  openWorkspace,
} from "./helpers";
import type { Page } from "@playwright/test";

/** The page itself never scrolls: each column scrolls on its own. */
async function expectNoPageScroll(page: Page) {
  const m = await page.evaluate(() => {
    const el = document.scrollingElement ?? document.documentElement;
    return {
      sw: el.scrollWidth,
      cw: el.clientWidth,
      sh: el.scrollHeight,
      ch: el.clientHeight,
    };
  });
  expect(m.sw, "horizontal page scroll").toBeLessThanOrEqual(m.cw);
  expect(m.sh, "vertical page scroll").toBeLessThanOrEqual(m.ch);
}

async function expectRegions(page: Page) {
  await expect(page.getByRole("navigation", { name: "Workspaces" })).toBeVisible();
  await expect(page.getByRole("group", { name: "Turn" })).toBeVisible();
  await expect(page.getByRole("group", { name: "Effects" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Actions" })).toBeVisible();
  const fil = page.getByRole("complementary", { name: "Results and journal" });
  await expect(fil.getByRole("region", { name: "Last Result" })).toBeVisible();
  await expect(fil.getByRole("region", { name: "Combat Log" })).toBeVisible();
  const moi = page.getByRole("complementary", { name: "Character" });
  await expect(moi.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(moi.getByTestId("hp-value")).toBeVisible();
  await expect(moi.getByRole("group", { name: "HP change" })).toBeVisible();
}

const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

test("layout: four regions, full width, no page scroll from 1024 to 2560", async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await openDashboard(page);
  await expectRegions(page);
  await expectNoPageScroll(page);

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expectRegions(page);
    await expectNoPageScroll(page);
    // Full width: the Moi column ends at the right edge (minus the layout padding), no centring.
    const moi = await page.getByRole("complementary", { name: "Character" }).boundingBox();
    expect(vp.width - (moi!.x + moi!.width)).toBeLessThan(32);
    // The workspace keeps a usable width.
    const ws = await page.getByTestId("workspace").boundingBox();
    expect(ws!.width).toBeGreaterThan(vp.width * 0.3);
    await snap(page, `dashboard-${vp.width}x${vp.height}`);
  }
});

test("rail: switches workspaces and each keeps its state", async ({ page }) => {
  await openDashboard(page);
  const rail = page.getByRole("navigation", { name: "Workspaces" });
  await expect(rail.getByRole("button", { name: "Act" })).toHaveAttribute("aria-current", "page");

  // Act: pick the Spells tab.
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(page.getByRole("tab", { name: "Spells" })).toHaveAttribute("aria-selected", "true");

  // Inventory: items and weapons; open the add-item form.
  await openWorkspace(page, "Inventory");
  await expect(rail.getByRole("button", { name: "Inventory" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(page.getByRole("region", { name: "Actions" })).toBeHidden();
  const inventory = page.getByRole("region", { name: "Inventory" });
  await expect(inventory.getByRole("button", { name: "Add weapon" })).toBeVisible();
  await inventory.getByRole("button", { name: "Add item" }).click();
  await expect(inventory.getByTestId("item-form")).toBeVisible();
  await snap(page, "workspace-inventory");

  // Spells: the spell lookup; type a search.
  await openWorkspace(page, "Spells");
  const search = page.getByRole("combobox", { name: "Search spells..." });
  await search.fill("fire");
  await expect(page.getByTestId("spell-row").first()).toBeVisible();
  await snap(page, "workspace-spells");

  // Back: every workspace is as it was left. The Fil and Moi columns never moved.
  await openWorkspace(page, "Act");
  await expect(page.getByRole("tab", { name: "Spells" })).toHaveAttribute("aria-selected", "true");
  await openWorkspace(page, "Inventory");
  await expect(inventory.getByTestId("item-form")).toBeVisible();
  await openWorkspace(page, "Spells");
  await expect(search).toHaveValue("fire");
  await expect(page.getByRole("region", { name: "Combat Log" })).toBeVisible();
  await expect(page.getByTestId("hp-value")).toBeVisible();
});

const isDark = (page: Page) =>
  page.evaluate(() => document.documentElement.classList.contains("dark"));
const rootVar = (page: Page, name: string) =>
  page.evaluate(
    (n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim(),
    name,
  );

test("settings: language, appearance and ambiance apply live and persist across reload", async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: "light" });
  await openDashboard(page);
  expect(await isDark(page)).toBe(false); // System by default, the OS is light

  await chooseSetting(page, "Dark");
  expect(await isDark(page)).toBe(true);
  await chooseSetting(page, "Blood");
  await expect.poll(() => rootVar(page, "--tint")).toBe("oklch(0.68 0.15 22)");
  await chooseLanguage(page, "fr");
  await expect(page.getByRole("button", { name: "Tour suivant" })).toBeVisible();

  await page.reload();
  expect(await isDark(page)).toBe(true); // painted before the app mounts
  await expect(page.getByRole("button", { name: "Tour suivant" })).toBeVisible();
  expect(await rootVar(page, "--tint")).toBe("oklch(0.68 0.15 22)");
  const popover = await openSettings(page);
  for (const name of ["Français", "Sombre", "Sang"])
    await expect(popover.getByRole("radio", { name, exact: true })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  await snap(page, "settings-popover-fr");

  // Système follows the OS live.
  await popover.getByRole("radio", { name: "Système", exact: true }).click();
  expect(await isDark(page)).toBe(false);
  await page.emulateMedia({ colorScheme: "dark" });
  await expect.poll(() => isDark(page)).toBe(true);
});

test("theme: every ambiance in dark and light", async ({ page }) => {
  test.setTimeout(60_000);
  await openDashboard(page);
  for (const mode of ["Dark", "Light"]) {
    await chooseSetting(page, mode);
    for (const amb of ["Arcane", "Grove", "Blood", "Ocean", "Amber", "Obsidian"]) {
      await chooseSetting(page, amb);
      await page.waitForTimeout(150); // let colour transitions settle
      await snap(page, `${mode.toLowerCase()}-${amb.toLowerCase()}`);
    }
  }
});

// The Agir workspace (weapon in hand with a bow's ammunition, then the Spells and Items tabs)
// holds at every viewport without a page scroll.
test("agir: weapons, spells and items tabs from 1280 to 2560", async ({ page }) => {
  await openDashboard(page);
  await openWeapons(page);
  await chooseWeapon(page, "Short Bow +2");
  await expect(ammoChips(page)).toBeVisible();
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(page.getByTestId("tile-thac0")).toBeVisible();
    await expectNoPageScroll(page);
    await snap(page, `agir-weapons-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(page.getByRole("button", { name: "Prepare spells" })).toBeVisible();
  await snap(page, "agir-spells");
  await page.getByRole("tab", { name: "Items" }).click();
  await page.getByRole("button", { name: "Manage the inventory" }).click();
  await expect(page.getByRole("navigation", { name: "Workspaces" }).getByRole("button", { name: "Inventory" })).toHaveAttribute("aria-current", "page");
  await openWorkspace(page, "Act");
  await snap(page, "agir-items");
});
