import { test, expect, snap } from "./fixtures";
import {
  ammoChips,
  chooseLanguage,
  chooseSetting,
  chooseWeapon,
  combatLog,
  openDashboard,
  openSettings,
  openWeapons,
  openWorkspace,
} from "./helpers";
import type { Locator, Page } from "@playwright/test";

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
  await expect(
    page.getByRole("navigation", { name: "Workspaces" }),
  ).toBeVisible();
  await expect(page.getByRole("group", { name: "Turn" })).toBeVisible();
  await expect(page.getByRole("group", { name: "Effects" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Actions" })).toBeVisible();
  const fil = page.getByRole("complementary", { name: "Results and journal" });
  await expect(
    fil.getByRole("region", { name: "Detail", exact: true }),
  ).toBeVisible();
  await expect(
    fil.getByRole("region", { name: "Journal", exact: true }),
  ).toBeVisible();
  const moi = page.getByRole("complementary", { name: "Character" });
  await expect(moi.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(moi.getByTestId("hp-value")).toBeVisible();
  await expect(moi.getByRole("group", { name: "HP change" })).toBeVisible();
}

/** `inner` lies entirely within `outer` (bounding boxes, 1px tolerance for subpixel rounding). */
async function expectInside(inner: Locator, outer: Locator, what: string) {
  const i = (await inner.boundingBox())!;
  const o = (await outer.boundingBox())!;
  expect(i.y, `${what}: top`).toBeGreaterThanOrEqual(o.y - 1);
  expect(i.y + i.height, `${what}: bottom`).toBeLessThanOrEqual(
    o.y + o.height + 1,
  );
}

/** The column scrolls no content out of view. */
async function expectNoScroll(column: Locator, what: string) {
  const m = await column.evaluate((el) => ({
    sh: el.scrollHeight,
    ch: el.clientHeight,
  }));
  expect(m.sh, `${what} scrolls`).toBeLessThanOrEqual(m.ch);
}

// ADR 0017 step 7 (content target). 1707x1300 is a third of the user's 5120x1440 table screen.
const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1707, height: 1300 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

test("layout: four regions, full width, no page scroll from 1024 to 2560", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await openDashboard(page);
  await expectRegions(page);
  await expectNoPageScroll(page);

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expectRegions(page);
    await expectNoPageScroll(page);
    // Full width: the Moi column ends at the right edge (minus the layout padding), no centring.
    const moi = await page
      .getByRole("complementary", { name: "Character" })
      .boundingBox();
    expect(vp.width - (moi!.x + moi!.width)).toBeLessThan(32);
    // The workspace keeps a usable width.
    const ws = await page.getByTestId("workspace").boundingBox();
    expect(ws!.width).toBeGreaterThan(vp.width * 0.3);
    // The header is one row of at most 64px.
    const header = await page
      .locator("[data-screen=turn-header]")
      .boundingBox();
    expect(header!.height, "header height").toBeLessThanOrEqual(64);
    if (vp.width === 1280) {
      // Moi shows identity, abilities, HP, HP change and the five saves without scrolling, and the
      // weapon in hand is fully inside the workspace.
      const moi = page.getByTestId("moi");
      await expectNoScroll(moi, "Moi");
      for (const [what, el] of [
        ["identity", moi.getByRole("heading", { level: 1 })],
        ["abilities", moi.locator("dl")],
        ["HP", moi.getByTestId("hp-value")],
        ["HP change", moi.getByRole("group", { name: "HP change" })],
        ["last save", moi.getByTestId("save-row").last()],
      ] as const)
        await expectInside(el, moi, what);
      await expect(moi.getByTestId("save-row")).toHaveCount(5);
      await expectInside(
        page.getByRole("region", { name: "Weapon in hand" }),
        page.getByTestId("workspace"),
        "weapon card",
      );
    }
    await snap(page, `dashboard-${vp.width}x${vp.height}`);
  }
});

test("rail: switches workspaces and each keeps its state", async ({ page }) => {
  await openDashboard(page);
  const rail = page.getByRole("navigation", { name: "Workspaces" });
  await expect(rail.getByRole("button", { name: "Act" })).toHaveAttribute(
    "aria-current",
    "page",
  );

  // Act: pick the Spells tab.
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(page.getByRole("tab", { name: "Spells" })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  // Inventory: items and weapons; open the add-item form.
  await openWorkspace(page, "Inventory");
  await expect(rail.getByRole("button", { name: "Inventory" })).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(page.getByRole("region", { name: "Actions" })).toBeHidden();
  const inventory = page.getByRole("region", { name: "Inventory" });
  await expect(
    inventory.getByRole("button", { name: "Add weapon" }),
  ).toBeVisible();
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
  await expect(page.getByRole("tab", { name: "Spells" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await openWorkspace(page, "Inventory");
  await expect(inventory.getByTestId("item-form")).toBeVisible();
  await openWorkspace(page, "Spells");
  await expect(search).toHaveValue("fire");
  await expect(
    page.getByRole("region", { name: "Journal", exact: true }),
  ).toBeVisible();
  await expect(page.getByTestId("hp-value")).toBeVisible();
});

const isDark = (page: Page) =>
  page.evaluate(() => document.documentElement.classList.contains("dark"));
const rootVar = (page: Page, name: string) =>
  page.evaluate(
    (n) =>
      getComputedStyle(document.documentElement).getPropertyValue(n).trim(),
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
  await expect(
    page.getByRole("button", { name: "Tour suivant" }),
  ).toBeVisible();

  await page.reload();
  expect(await isDark(page)).toBe(true); // painted before the app mounts
  await expect(
    page.getByRole("button", { name: "Tour suivant" }),
  ).toBeVisible();
  expect(await rootVar(page, "--tint")).toBe("oklch(0.68 0.15 22)");
  const popover = await openSettings(page);
  for (const name of ["Français", "Sombre", "Sang"])
    await expect(
      popover.getByRole("radio", { name, exact: true }),
    ).toHaveAttribute("aria-checked", "true");
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
    for (const amb of [
      "Arcane",
      "Grove",
      "Blood",
      "Ocean",
      "Amber",
      "Obsidian",
    ]) {
      await chooseSetting(page, amb);
      await page.waitForTimeout(150); // let colour transitions settle
      await snap(page, `${mode.toLowerCase()}-${amb.toLowerCase()}`);
    }
  }
});

// The Agir workspace (weapon in hand with a bow's ammunition, then the Spells and Items tabs)
// holds at every viewport without a page scroll.
test("agir: weapons, spells and items tabs from 1280 to 2560", async ({
  page,
}) => {
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
  await expect(
    page.getByRole("button", { name: "Prepare spells" }),
  ).toBeVisible();
  await snap(page, "agir-spells");
  await page.getByRole("tab", { name: "Items" }).click();
  await page.getByRole("button", { name: "Manage the inventory" }).click();
  await expect(
    page
      .getByRole("navigation", { name: "Workspaces" })
      .getByRole("button", { name: "Inventory" }),
  ).toHaveAttribute("aria-current", "page");
  await openWorkspace(page, "Act");
  await snap(page, "agir-items");
});

// What only a browser proves: the Inventaire dice chip and the Sorts "To the dice" fill the composer
// (a client store shared across workspaces), and Roll on a spell's dice reaches the dice endpoint
// and lands in the card and the journal. The formula wording is a unit test (spellValues), the
// values at level are Go tests (spellstats). Screens of both workspaces at the four viewports.
test.describe("workspaces to the dice composer", () => {
  test.use({
    extraSpells: {
      rows: [
        {
          name: "Quixotic Stillness",
          class: "Wizard",
          level: 3,
          school: "Alteration",
          sphere: "",
          casting_time: "3",
          range: "Touch",
          duration: "8 rds. + 1d8 rds.",
          area_of_effect: "The caster",
          components: "V, S",
          saving_throw: "None",
          description: "Quixotic Stillness fixture text.",
        },
      ],
    },
  });

  test("Inventaire dice chip and Sorts 'To the dice' fill the composer; Roll rolls on the server", async ({
    page,
  }) => {
    await openDashboard(page);
    const formula = page
      .getByRole("form", { name: "Dice" })
      .getByLabel("Formula");

    await openWorkspace(page, "Inventory");
    await page
      .getByTestId("item-fire_flask")
      .getByRole("button", { name: "Send 5d6 to the dice" })
      .click();
    await expect(formula).toHaveValue("5d6");
    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoPageScroll(page);
      await snap(page, `inventory-${vp.width}x${vp.height}`);
    }
    await page.setViewportSize({ width: 1440, height: 900 });

    await openWorkspace(page, "Spells");
    await page
      .getByRole("combobox", { name: "Search spells..." })
      .fill("quixotic still");
    const detail = page.getByTestId("spell-detail");
    await expect(detail.getByRole("heading")).toHaveText("Quixotic Stillness");
    const dice = detail.getByTestId("spell-dice-duration");
    await expect(dice.getByTestId("spell-dice-formula")).toHaveText("1d8+8");
    await dice.getByRole("button", { name: "Send 1d8+8 to the dice" }).click();
    await expect(formula).toHaveValue("1d8+8");

    await formula.fill("");
    await dice.getByRole("button", { name: "Roll 1d8+8" }).click();
    const result = page
      .getByRole("region", { name: "Detail", exact: true })
      .getByTestId("dice-result");
    // The server answers with its normal form (a single die has no count).
    await expect(result.getByTestId("dice-formula")).toHaveText("d8+8");
    const total = Number(await result.getByTestId("dice-total").textContent());
    expect(total).toBeGreaterThanOrEqual(9);
    expect(total).toBeLessThanOrEqual(16);
    await expect(combatLog(page).locator("[data-selected=true]")).toContainText(
      `Dice d8+8 = ${total}`,
    );
    await expect(formula).toHaveValue("d8+8");

    // The damage row: Fireball at the Bard's level, to the composer and rolled on the server.
    await page
      .getByRole("combobox", { name: "Search spells..." })
      .fill("fireball");
    await expect(detail.getByRole("heading")).toContainText("Fireball");
    const damage = detail.getByTestId("spell-dice-damage");
    await expect(damage.getByTestId("spell-dice-formula")).toHaveText(
      "10d6 fire",
    );
    await damage.getByRole("button", { name: "Send 10d6 to the dice" }).click();
    await expect(formula).toHaveValue("10d6");
    await damage.getByRole("button", { name: "Roll 10d6" }).click();
    await expect(result.getByTestId("dice-formula")).toHaveText("10d6");
    const fire = Number(await result.getByTestId("dice-total").textContent());
    expect(fire).toBeGreaterThanOrEqual(10);
    expect(fire).toBeLessThanOrEqual(60);

    for (const vp of VIEWPORTS) {
      await page.setViewportSize(vp);
      await expectNoPageScroll(page);
      await snap(page, `spells-${vp.width}x${vp.height}`);
    }
  });
});
