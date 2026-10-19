import { mkdir } from "node:fs/promises";
import path from "node:path";
import type { Locator, Page } from "@playwright/test";
import { test, expect } from "../e2e/fixtures";
import { chooseLanguage, openSettings } from "../e2e/helpers";
import {
  THEME_MODES,
  VIEWPORTS,
  focus,
  journal,
  pickTheme,
  recordHpT,
  seedTable,
  translator,
  webDir,
  type T,
} from "./table";

/**
 * The UI review capture (docs/design/ui-review/README.md): screenshots of the real app, every screen,
 * panel, popover and state that matters, in every colour theme, at the two layout viewports of ADR
 * 0022 (1707x1300, the user's table screen; 1440x900), in English and, for the full set, in French.
 * Output: web/e2e-results/ui-review/<mode>-<theme>[-fr]/<screen>-<WxH>.png (gitignored); the
 * curated set listed in docs/design/ui-review/curated.txt is then copied into
 * docs/design/ui-review/shots/ by ui-review/curate.mjs (`just ui-shots`).
 */

export const outDir = path.join(webDir, "e2e-results", "ui-review");

async function shot(page: Page, theme: string, name: string) {
  const vp = page.viewportSize()!;
  const dir = path.join(outDir, theme);
  await mkdir(dir, { recursive: true });
  await page.waitForTimeout(120); // colour transitions, popover animations
  await page.screenshot({
    path: path.join(dir, `${name}-${vp.width}x${vp.height}.png`),
    animations: "disabled",
  });
}

/** The screens every theme gets: the Act stage after the attack, the lookup, the inventory, XP. */
async function coreScreens(page: Page, t: T, theme: string) {
  await focus(page, t, "act");
  await page.getByRole("tab", { name: t("actions.tabs.weapons") }).click();
  await shot(page, theme, "act-weapons");
  await focus(page, t, "spells");
  await expect(page.getByTestId("spell-row").first()).toBeVisible();
  await shot(page, theme, "lookup");
  await focus(page, t, "inventory");
  await shot(page, theme, "inventory");
  await focus(page, t, "xp");
  await shot(page, theme, "experience");
  await focus(page, t, "act");
  const popover = await openSettings(page);
  await shot(page, theme, "settings");
  await page.keyboard.press("Escape");
  await expect(popover).toBeHidden();
}

async function closePopover(page: Page, popover: Locator) {
  await page.keyboard.press("Escape");
  await expect(popover).toBeHidden();
}

/** Every panel, tab, popover and state of the app, in the current theme. */
async function fullSet(page: Page, theme: string, lang: "en" | "fr") {
  const t = await translator(lang);
  await coreScreens(page, t, theme);
  const actions = page.getByRole("region", { name: t("actions.label") });

  // Act · Weapons: the crit table opened, the target bar with Unknown AC.
  await actions.getByRole("tab", { name: t("actions.tabs.weapons") }).click();
  const critTable = page.getByTestId("crit-table");
  await critTable.locator("summary").click();
  await expect(
    critTable.getByRole("group", { name: t("critTable.row", { n: 1 }) }),
  ).toBeVisible();
  await shot(page, theme, "act-weapons-crit-table");
  await critTable.locator("summary").click();
  // The weapon chips are named in the UI language: switch to the first other weapon, then back
  // (the previous weapon is listed first).
  const form = page.getByRole("form", { name: t("attack.form") });
  const held = form.getByTestId("weapon-in-hand");
  const heldName = await held.textContent();
  const other = form.getByRole("group", {
    name: t("attack.otherWeapons"),
  });
  await other.getByRole("button").first().click();
  await expect(held).not.toHaveText(heldName ?? "");
  await shot(page, theme, "act-weapons-bow");
  await other.getByRole("button").first().click();
  await expect(held).toHaveText(heldName ?? "");

  // Act · Spells tab, then a cast (Fireball: damage, save note) in the result card.
  await actions.getByRole("tab", { name: t("actions.tabs.spells") }).click();
  await shot(page, theme, "act-spells");
  // The Cast button of Fireball (level 3), whatever its name in the UI language.
  await page
    .getByRole("region", { name: t("spellSlots.level", { level: 3 }) })
    .getByRole("button", { name: new RegExp(`^${t("spellSlots.cast")}\\b`) })
    .first()
    .click();
  const result = page.getByRole("region", {
    name: t("journal.card"),
    exact: true,
  });
  await expect(result.getByTestId("cast-damage")).toBeVisible();
  await shot(page, theme, "act-spells-cast");
  // Give the slot back (the cast is the newest transaction), so the next theme can cast again.
  await page.getByTestId("undo").click();
  await expect(journal(page, t).getByRole("listitem").first()).toHaveAttribute(
    "data-status",
    "voided",
  );

  // Act · Items and Skills tabs.
  await actions.getByRole("tab", { name: t("actions.tabs.items") }).click();
  await shot(page, theme, "act-items");
  await actions.getByRole("tab", { name: t("actions.tabs.skills") }).click();
  await shot(page, theme, "act-skills");
  await actions.getByRole("tab", { name: t("actions.tabs.weapons") }).click();

  // A saving throw from the character card: the save card with its adjust form.
  await page
    .getByRole("button", {
      name: t("saves.rollLabel", { name: t("saves.categories.spell") }),
    })
    .click();
  await expect(
    result.getByRole("form", {
      name: t("result.adjust.label"),
    }),
  ).toBeVisible();
  await shot(page, theme, "act-save-result");

  // A journal row selected (an older entry shown in the card), then the journal collapsed.
  const rows = journal(page, t).getByRole("listitem");
  await rows.nth(3).getByTestId("log-select").click();
  await shot(page, theme, "act-journal-selected");
  const handle = page.getByTestId("journal-handle");
  await handle.click();
  await expect(handle).toHaveAttribute(
    "aria-valuetext",
    t("shell.journal.stateCollapsed"),
  );
  await shot(page, theme, "act-journal-collapsed");
  await handle.click();
  await expect(handle).toHaveAttribute(
    "aria-valuetext",
    t("shell.journal.stateExpanded"),
  );

  // Header: the effects expanded past three rows, the add-effect popover, the turn menu.
  const expander = page.getByTestId("effects-expander");
  if (await expander.isVisible()) {
    await expander.click();
    await shot(page, theme, "header-effects-expanded");
    await expander.click();
  }
  await page
    .getByRole("group", { name: t("effects.title") })
    .getByRole("button", { name: t("buffs.add") })
    .click();
  const addPopover = page.getByTestId("add-effect-popover");
  await addPopover.getByRole("tab", { name: t("buffs.tab.new") }).click();
  await shot(page, theme, "header-add-effect");
  await closePopover(page, addPopover);
  await page
    .getByRole("button", {
      name: t("round.menu"),
    })
    .click();
  await expect(page.getByRole("menuitem").first()).toBeVisible();
  await shot(page, theme, "header-turn-menu");
  await page.keyboard.press("Escape");

  // Character details popover (view mode).
  await page.getByTestId("details-trigger").click();
  const details = page.getByTestId("details-popover");
  await expect(details).toBeVisible();
  await shot(page, theme, "character-details");
  await closePopover(page, details);

  // Lookup with a long description (two text columns), and the Priest spheres.
  await focus(page, t, "spells");
  const search = page.getByRole("combobox", {
    name: t("lookup.search"),
  });
  const count = page.getByTestId("lookup-count");
  const before = await count.textContent();
  await search.fill("magic jar");
  await expect(count).not.toHaveText(before ?? "");
  await expect(
    page.getByTestId("spell-detail").getByRole("heading").first(),
  ).toBeVisible();
  await shot(page, theme, "lookup-long");
  await search.fill("");
  await expect(count).toHaveText(before ?? "");

  // Inventory with the add-item form open.
  await focus(page, t, "inventory");
  const inventory = page.getByRole("region", {
    name: t("shell.activity.inventory"),
  });
  await inventory
    .getByRole("button", { name: t("items.form.openAdd") })
    .click();
  await expect(inventory.getByTestId("item-form")).toBeVisible();
  await shot(page, theme, "inventory-item-form");
  await inventory
    .getByTestId("item-form")
    .getByRole("button", { name: t("items.form.cancel") })
    .click();

  // Low HP: the bar in the danger band with its condition.
  await focus(page, t, "act");
  await recordHpT(page, t, "damage", 60, "Dragon breath");
  await expect(journal(page, t).getByRole("listitem").first()).toContainText(
    "Dragon breath",
  );
  await shot(page, theme, "act-hp-low");
  await page.getByTestId("undo").click();
  await expect(journal(page, t).getByRole("listitem").first()).toHaveAttribute(
    "data-status",
    "voided",
  );
}

test("full set EN: every screen and state, Arcane dark and light", async ({
  page,
  stack,
}) => {
  test.setTimeout(20 * 60_000);
  await seedTable(page, stack.apiUrl);
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    for (const mode of ["Dark", "Light"] as const) {
      await pickTheme(page, mode, "Arcane");
      await fullSet(page, `${mode.toLowerCase()}-arcane`, "en");
    }
  }
});

test("full set FR: every screen and state, Arcane dark (longer text)", async ({
  page,
  stack,
}) => {
  test.setTimeout(20 * 60_000);
  await seedTable(page, stack.apiUrl);
  await pickTheme(page, "Dark", "Arcane");
  await chooseLanguage(page, "fr");
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await fullSet(page, "dark-arcane-fr", "fr");
  }
});

test("theme matrix: the core screens in every theme, in each of its modes", async ({
  page,
  stack,
}) => {
  test.setTimeout(20 * 60_000);
  const t = await translator("en");
  await seedTable(page, stack.apiUrl);
  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    for (const { mode, theme } of THEME_MODES) {
      if (theme === "Arcane") continue; // the full set has it
      await pickTheme(page, mode, theme);
      await coreScreens(
        page,
        t,
        `${mode.toLowerCase()}-${theme.toLowerCase()}`,
      );
    }
  }
});
