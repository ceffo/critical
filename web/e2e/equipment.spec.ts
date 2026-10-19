import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  apiCall,
  attackForm,
  chooseLanguage,
  receivePicker,
  chooseWeapon,
  logEntry,
  openDashboard,
  openWeapons,
  openWorkspace,
  reloadSettled,
  undoEntry,
  openTypedAdd,
} from "./helpers";

// The one possessions list (ADR 0033 steps 1 to 8 and 27, ADR 0035 items 2, 3, 7 and 12): weapons
// are pieces edited in the Inventaire's one editor, Equip / Unequip is one logged move the journal
// pops, the attack's drop-down equips the weapon it picks, and the review list empties as the
// player settles it. What only a browser proves here: the wiring of those controls, the journal
// lines and the attack panel following the inventory; the rules are the Go tests'.

async function openInventory(page: Page) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  await expect(page.getByTestId("item-bastard_sword")).toBeVisible();
}

function row(page: Page, id: string) {
  return page.getByTestId(`item-${id}`);
}

function form(page: Page) {
  return page.getByTestId("item-form");
}

/** Every weapon not in hand: the previous weapon's button and the drop-down's options. */
async function otherWeapons(page: Page) {
  const group = attackForm(page).getByRole("group", { name: "Other weapons" });
  const previous = await group
    .getByRole("button")
    .evaluateAll((els) => els.map((e) => e.getAttribute("data-weapon") ?? ""));
  const listed = await group
    .locator("option:not([disabled])")
    .evaluateAll((els) => els.map((e) => e.textContent ?? ""));
  return [...previous, ...listed];
}

async function rollAttack(page: Page, weapon: string) {
  await openWorkspace(page, "Act");
  await openWeapons(page);
  await chooseWeapon(page, weapon);
  const f = attackForm(page);
  await f.getByLabel("Target AC").fill("-10");
  await f.getByRole("button", { name: "Attack", exact: true }).click();
  return page
    .getByRole("region", { name: "Detail", exact: true })
    .getByTestId("attack-result");
}

test("edit Bastard Sword +2 to +3 to hit and damage in the one editor: the next attack uses +3; pop restores +2", async ({
  page,
}) => {
  await openInventory(page);
  await row(page, "bastard_sword")
    .getByRole("button", { name: "Edit Bastard Sword +2" })
    .click();
  await expect(form(page).getByLabel("Category")).toHaveValue("weapon");
  await form(page).getByLabel("To-hit bonus").fill("3");
  await form(page).getByLabel("Damage bonus").fill("3");
  await snap(page, "edit-form");
  await form(page).getByRole("button", { name: "Save" }).click();

  await expect(form(page)).toHaveCount(0);
  const r = row(page, "bastard_sword");
  await expect(r.getByTestId("item-stats")).toContainText("+3/+3");

  const card = await rollAttack(page, "Bastard Sword +2");
  await expect(logEntry(page, "Edited Bastard Sword +2")).toContainText(
    "to-hit bonus 2 → 3, damage bonus 2 → 3",
  );
  await expect(card.getByTestId("to-hit-modifiers")).toContainText(
    /Bastard Sword \+2\s*\+3/,
  );

  // The attack is newer than the edit: pop the attack, then the edit.
  await undoEntry(page, "Bastard Sword +2 vs AC");
  await undoEntry(page, "Edited Bastard Sword +2");
  await openWorkspace(page, "Inventory");
  await expect(r.getByTestId("item-stats")).toContainText("+2/+2");
});

test("a new bow is carried: the attack's drop-down lists it under Not equipped, picking it equips it; pop puts it away", async ({
  page,
}) => {
  await openInventory(page);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Long Bow");
  await f.getByLabel("Category").selectOption("weapon");
  await f
    .getByLabel("Melee or missile", { exact: true })
    .selectOption("missile");
  await f.getByLabel("Damage S-M").fill("1d8");
  await f.getByLabel("Damage Large").fill("1d8");
  await f.getByLabel("Two-handed").check();
  await f.getByLabel("Fires").selectOption("arrow");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  const bow = row(page, "long_bow");
  await expect(bow).toContainText("Long Bow");
  await expect(bow.getByTestId("item-equipped")).toHaveCount(0);
  await expect(
    bow.getByRole("button", { name: "Equip Long Bow" }),
  ).toBeVisible();

  await openWorkspace(page, "Act");
  await openWeapons(page);
  const group = attackForm(page)
    .getByRole("combobox", { name: "Other weapon" })
    .locator('optgroup[label="Not equipped"] option');
  await expect(group).toHaveText(["Long Bow"]);
  await snap(page, "drop-down-not-equipped");
  await chooseWeapon(page, "Long Bow");
  await expect(logEntry(page, "Equipped: Long Bow")).toBeVisible();
  await expect(
    attackForm(page)
      .getByRole("radiogroup", { name: "Ammunition" })
      .getByRole("radio", { name: /^Arrows \+1,/ }),
  ).toHaveCount(1);

  // Popping the move puts the bow away: the hand is empty, the bow is offered again.
  await undoEntry(page, "Equipped: Long Bow");
  await expect(attackForm(page).getByTestId("no-weapon")).toBeVisible();
  expect(await otherWeapons(page)).toContain("Long Bow");
  await openWorkspace(page, "Inventory");
  await expect(bow.getByTestId("item-equipped")).toHaveCount(0);
});

// ADR 0033 step 24, ADR 0035 item 10: Ajouter > Catalogue in French, searched by the French name,
// renamed, made +1 and equipped at once; one undo takes it away; picked again it survives a reload.
// What only a browser proves: the tabs, the search, the copy panel and the wiring of one add with
// its equip (what the server copies, and the one undo, are the Go tests').
test("pick a long sword from the catalogue in French: renamed, +1, equipped at once; one undo takes it away", async ({
  page,
}) => {
  await openInventory(page);
  await chooseLanguage(page, "fr");
  await page.getByRole("button", { name: "Ajouter un objet" }).click();
  const add = page.getByTestId("add-piece");
  await expect(add.getByRole("tab", { name: "Catalogue" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(
    add.getByRole("button", { name: "Armes", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await add.getByTestId("add-search").fill("essai long SWORD");
  await expect(add.getByTestId("add-rows").getByRole("button")).toHaveCount(1);
  await add.getByTestId("add-row-long_sword").click();
  const copy = add.getByTestId("add-copy");
  await expect(copy.getByTestId("add-name")).toHaveValue(
    "Banc d'essai long sword",
  );
  await copy.getByTestId("add-name").fill("Lame de Kelsen");
  await copy.getByRole("button", { name: "Un de magie en plus" }).click();
  await expect(copy.getByTestId("add-magic")).toHaveText("+1");
  await expect(copy.getByTestId("add-equip")).toBeChecked();
  await snap(page, "add-catalogue-fr");
  await copy.getByTestId("add-submit").click();

  await expect(add).toHaveCount(0);
  // The id is made from the name the piece has (the row's English one stays beside the French).
  const sword = row(page, "fixture_long_sword");
  await expect(sword).toContainText("Lame de Kelsen");
  await expect(sword.getByTestId("item-equipped")).toHaveText("Équipé");
  await expect(sword.getByTestId("item-stats")).toContainText("+1/+1");

  // One undo takes the add and its equip away together (one transaction).
  await chooseLanguage(page, "en");
  await openWorkspace(page, "Act");
  // In English the copy keeps the row's English name (only French was renamed).
  await undoEntry(page, "Equipped: Fixture long sword");
  await expect(logEntry(page, "Added Fixture long sword")).toHaveAttribute(
    "data-status",
    "voided",
  );
  await openWorkspace(page, "Inventory");
  await expect(row(page, "fixture_long_sword")).toHaveCount(0);

  // Picked again (English name, kept carried): it survives a reload.
  await page.getByRole("button", { name: "Add item" }).click();
  await page.getByTestId("add-row-long_sword").click();
  await page.getByTestId("add-equip").uncheck();
  await page.getByTestId("add-submit").click();
  await page.reload();
  await openInventory(page);
  const again = row(page, "fixture_long_sword");
  await expect(again).toContainText("Fixture long sword");
  await expect(again.getByTestId("item-equipped")).toHaveCount(0);

  // A catalogue launcher has no dice of its own (ADR 0033 step 4): its row shows none, and in hand
  // the attack offers its arrows.
  await page.getByRole("button", { name: "Add item" }).click();
  await page.getByTestId("add-search").fill("long bow");
  await page.getByTestId("add-row-long_bow").click();
  await page.getByTestId("add-submit").click();
  const bow = row(page, "fixture_long_bow");
  await expect(bow.getByTestId("item-stats")).toHaveText(
    "Missile · speed 8 · two-handed · ammo: Arrow",
  );
  await snap(page, "picked-launcher-row");
  await openWorkspace(page, "Act");
  await openWeapons(page);
  await chooseWeapon(page, "Fixture long bow");
  await expect(
    attackForm(page)
      .getByRole("radiogroup", { name: "Ammunition" })
      .getByRole("radio", { name: /^Arrows \+1,/ }),
  ).toHaveCount(1);
  // The dice shown are the arrow's, the ones the attack rolls.
  await expect(attackForm(page).getByTestId("tile-damage-sm")).toHaveText(
    "1d6",
  );
  await snap(page, "picked-launcher-attack");
});

test("a new gem marked as a spell component in the editor keeps it after a reload", async ({
  page,
}) => {
  // ADR 0033 step 1.1: the component select is wired to the API and persisted (which piece a spell
  // spends is the Go tests'). What only a browser proves: the select, its labels and the reload.
  await openInventory(page);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Ruby");
  await expect(f.getByLabel("Spell component")).toHaveValue("");
  await f.getByLabel("Spell component").selectOption("gem");
  await snap(page, "component-form");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  await expect(row(page, "ruby")).toContainText("Ruby");

  await page.reload();
  await openInventory(page);
  await row(page, "ruby").getByRole("button", { name: "Edit Ruby" }).click();
  await expect(form(page).getByLabel("Spell component")).toHaveValue("gem");
});

test("Unequip and Equip in the Inventaire: the Equipped tag follows, each a journal line popped by undo", async ({
  page,
}) => {
  await openInventory(page);
  const gem = row(page, "power_gem_dispel");
  await expect(gem.getByTestId("item-equipped")).toHaveText("Equipped");
  await gem.getByRole("button", { name: "Unequip Power Gem" }).click();
  await expect(gem.getByTestId("item-equipped")).toHaveCount(0);
  await expect(
    gem.getByRole("button", { name: "Equip Power Gem" }),
  ).toBeVisible();
  await snap(page, "gem-unequipped");

  await openWorkspace(page, "Act");
  await expect(logEntry(page, "Unequipped: Power Gem")).toBeVisible();
  await undoEntry(page, "Unequipped: Power Gem");
  await openWorkspace(page, "Inventory");
  await expect(gem.getByTestId("item-equipped")).toHaveText("Equipped");
});

test("the review list: the band counts the pieces to check, Dismiss and a classifying edit empty it", async ({
  page,
}) => {
  await openInventory(page);
  const band = page.getByTestId("review-band");
  await expect(band).toContainText("2 items to check");
  await band.getByRole("button", { name: /Check the items/ }).click();
  const review = page.getByTestId("inventory-review");
  await expect(review.getByTestId("review-mace")).toContainText(
    "Weapon not recognised",
  );
  await snap(page, "review-open");

  // Dismissing is not a game move: no journal line.
  await review
    .getByRole("button", { name: "Take Mace +1 off the list as it is" })
    .click();
  await expect(review.getByTestId("review-mace")).toHaveCount(0);
  await expect(band).toContainText("1 item to check");

  // Settling the bastard sword: its speed and damage type.
  await review.getByRole("button", { name: "Edit Bastard Sword +2" }).click();
  // The form opens in the list below the panel: it is brought into view.
  await expect(form(page)).toBeInViewport();
  await form(page).getByLabel("Speed factor").fill("6");
  await form(page).getByLabel("Slashing").check();
  await form(page).getByRole("button", { name: "Save" }).click();
  await expect(band).toHaveCount(0);
  await expect(page.getByTestId("inventory-review")).toHaveCount(0);
  await expect(
    row(page, "bastard_sword").getByTestId("item-review"),
  ).toHaveCount(0);
  await snap(page, "review-empty");
});

test("a weapon removed behind the Attack panel's back: the attack says so and the picker refreshes", async ({
  page,
}) => {
  await openDashboard(page);
  await openWorkspace(page, "Act");
  await openWeapons(page);
  const f = attackForm(page);
  await chooseWeapon(page, "Darts");
  await f.getByLabel("Target AC").fill("5");
  await expect
    .poll(
      async () =>
        (await (await page.request.get("/api/attack/form")).json()).form
          .targetAc,
    )
    .toBe(5);
  // Removed elsewhere (another device): the panel still has the darts in hand.
  const res = await page.request.delete("/api/items/darts");
  expect(res.ok()).toBe(true);
  await f.getByRole("button", { name: "Attack", exact: true }).click();

  await expect(
    page.getByRole("alert").filter({ hasText: "no longer exists" }),
  ).toBeVisible();
  // No stale weapon in hand: the server's form falls back to an existing weapon.
  await expect(f.getByTestId("weapon-in-hand")).not.toHaveText("Darts");
  expect(await otherWeapons(page)).not.toContain("Darts");
  await snap(page, "attack-weapon-gone");
});

test("a weapon with a daily behaviour counts in the list: its row has the use button, a plain weapon has none", async ({
  page,
}) => {
  await openInventory(page);
  await expect(
    row(page, "bastard_sword").getByTestId("item-count"),
  ).toHaveCount(0);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Flame Tongue");
  await f.getByLabel("Category").selectOption("weapon");
  await f.getByLabel("Damage S-M").fill("1d8");
  await f.getByLabel("Damage Large").fill("1d8");
  await f.getByLabel("Behaviour").selectOption("daily");
  await f.getByLabel("Daily number").fill("2");
  await f.getByLabel("Count").fill("2");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  const tongue = row(page, "flame_tongue");
  await expect(tongue.getByTestId("item-count")).toHaveText("2 / 2");
  await tongue.getByRole("button", { name: "Use one Flame Tongue" }).click();
  await expect(tongue.getByTestId("item-count")).toHaveText("1 / 2");
});

test("the review section is bounded and its list scrolls, so the rows below stay reachable", async ({
  page,
}) => {
  await openInventory(page);
  await page
    .getByTestId("review-band")
    .getByRole("button", { name: /Check the items/ })
    .click();
  const section = page.getByTestId("inventory-review");
  await expect(section).toBeVisible();
  const style = await section.evaluate((el) => getComputedStyle(el).maxHeight);
  expect(style).not.toBe("none");
  const overflow = await page
    .getByTestId("inventory-review-list")
    .evaluate((el) => getComputedStyle(el).overflowY);
  expect(overflow).toBe("auto");
});

// The character's AC (ADR 0033 steps 10, 11 and 13, ADR 0035 items 4, 7 and 8), what only a
// browser proves: the shield beside the HP and the Inventaire's AC card follow a spell received and
// an Equip with no entry of their own; the breakdown mutes the Armor spell once armour is worn; the
// override set from the popover is refused with a worded error, then set, shown in amber with the
// derived AC beside it, worded in the journal (en, fr), undone, redone and cleared as one entry
// each. The derivation itself is the Go tests' (acsvc).
test("AC: follows the Armor spell and an Equip with no entry of its own; the override is refused, set, popped, redone and cleared", async ({
  page,
}) => {
  const entries = async () =>
    (
      (await (await page.request.get("/api/log")).json()) as {
        entries: unknown[];
      }
    ).entries.length;
  await openDashboard(page);
  // The seed bard wears the leather armour he entered (ADR 0033 step 14): off, for an unarmoured start.
  expect(
    await apiCall(page, "POST", "/api/items/leather/state", {
      state: "carried",
    }),
  ).toBe(true);
  await reloadSettled(page);
  const shield = page.getByTestId("ac-shield");
  const value = shield.getByTestId("ac-value");
  await expect(value).toHaveText("7"); // 10, Dexterity 17 -3

  const picker = await receivePicker(page);
  await picker.getByLabel("Search for a received spell").fill("Armor");
  await picker.getByLabel("Caster level").fill("1");
  await picker.getByRole("button", { name: "Apply Armor" }).click();
  await expect(value).toHaveText("3"); // Armor's base 6, Dexterity -3

  await openWorkspace(page, "Inventory");
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Chain mail");
  await f.getByLabel("Category").selectOption("armour");
  await f.getByLabel("Base AC").fill("5");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  const before = await entries();
  await row(page, "chain_mail")
    .getByRole("button", { name: "Equip Chain mail" })
    .click();
  await expect(page.getByTestId("ac-card-value")).toHaveText("2");
  await expect(value).toHaveText("2");
  expect(await entries(), "the Equip is the only entry").toBe(before + 1);

  await shield.click();
  const pop = page.getByTestId("ac-popover");
  const base = pop.locator('[data-testid="ac-row"][data-term="base"]');
  await expect(base.first()).toContainText("Chain mail");
  await expect(base.nth(1)).toHaveAttribute("data-off", "armour_worn");
  await expect(base.nth(1)).toContainText("armour worn");

  // On a short screen the panel is capped to the room the positioner gives it and scrolls, so the
  // buttons at its foot stay reachable.
  const tall = page.viewportSize();
  await page.keyboard.press("Escape");
  await page.setViewportSize({ width: 1920, height: 200 });
  await shield.click();
  await expect
    .poll(
      async () => {
        const box = await pop.boundingBox();
        return box !== null && box.y >= 0 && box.y + box.height <= 200;
      },
      { message: "the panel fits the screen" },
    )
    .toBe(true);
  expect(
    await pop.evaluate((el) => el.scrollHeight > el.clientHeight),
    "the panel scrolls",
  ).toBe(true);
  await pop.getByTestId("ac-override-open").scrollIntoViewIfNeeded();
  await expect(pop.getByTestId("ac-override-open")).toBeInViewport();
  await snap(page, "ac-popover-short-screen");
  if (tall) await page.setViewportSize(tall);

  // Refused: 11 is out of range, a 61-character label too long; worded, nothing logged.
  const logged = await entries();
  await pop.getByTestId("ac-override-open").click();
  await pop.getByTestId("ac-override-value").fill("11");
  await pop.getByTestId("ac-override-save").click();
  await expect(pop.getByTestId("ac-override-error")).toHaveText(
    "Between −10 and 10",
  );
  await pop.getByTestId("ac-override-value").fill("4");
  await pop.getByTestId("ac-override-label").fill("x".repeat(61));
  await pop.getByTestId("ac-override-save").click();
  await expect(pop.getByTestId("ac-override-error")).toHaveText(
    "60 characters at most",
  );
  expect(await entries(), "a refused override logs nothing").toBe(logged);

  await pop.getByTestId("ac-override-label").fill("Sort Bouclier");
  await pop.getByTestId("ac-override-save").click();
  await expect(value).toHaveText("4");
  await expect(shield).toHaveAttribute("data-overridden", "true");
  await expect(pop.getByTestId("ac-override")).toContainText("Derived");
  await expect(pop.getByTestId("ac-derived")).toHaveText("2");
  await page.keyboard.press("Escape");

  await openWorkspace(page, "Act");
  await expect(
    logEntry(page, "AC overridden: 4 (Sort Bouclier)"),
  ).toBeVisible();
  await chooseLanguage(page, "fr");
  await expect(logEntry(page, "CA forcée : 4 (Sort Bouclier)")).toBeVisible();
  await chooseLanguage(page, "en");
  await undoEntry(page, "AC overridden: 4 (Sort Bouclier)");
  await expect(value).toHaveText("2");
  await expect(shield).not.toHaveAttribute("data-overridden");
  await page.getByTestId("redo").click();
  await expect(value).toHaveText("4");

  const set = await entries();
  await openWorkspace(page, "Inventory");
  await page.getByTestId("ac-card").getByTestId("ac-override-clear").click();
  await expect(value).toHaveText("2");
  expect(await entries(), "Clear is one entry").toBe(set + 1);
  await openWorkspace(page, "Act");
  await expect(logEntry(page, "AC override cleared")).toBeVisible();
});
