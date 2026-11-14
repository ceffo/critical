import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { expectNoClippedText } from "./noClip";
import {
  ammoChips,
  buffBadge,
  chooseLanguage,
  chooseWeapon,
  combatLog,
  currentHp,
  logEntry,
  openDashboard,
  openWeapons,
  recordHp,
  openWorkspace,
  reloadSettled,
  undoEntry,
  openTypedAdd,
} from "./helpers";

// Inventory editor (one editor for every piece, ADR 0033 step 1): add, edit and remove possessions;
// each is one log entry and the newest can be popped. The journal is the Act activity's (ADR 0022
// step 2.5).

async function openInventory(page: Page) {
  await openDashboard(page);
  await openWorkspace(page, "Inventory");
  await expect(page.getByTestId("item-wand_lightning")).toBeVisible();
}

function row(page: Page, id: string) {
  return page.getByTestId(`item-${id}`);
}

function form(page: Page) {
  return page.getByTestId("item-form");
}

test("add Arrows +3: it shows in the Inventory and in the Short Bow ammo picker; pop removes it", async ({
  page,
}) => {
  await openInventory(page);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Arrows +3 (blessed)");
  await f.getByLabel("Category").selectOption("ammunition");
  await f.getByRole("button", { name: "Next" }).click();
  await f.getByLabel("Quantity").fill("6");
  await f.getByLabel("Magical bonus").fill("3");
  await f.getByLabel("Ammo kind").selectOption("arrow");
  await snap(page, "add-form");
  await f.getByRole("button", { name: "Add", exact: true }).click();

  await expect(form(page)).toHaveCount(0);
  const added = row(page, "arrows_3_blessed");
  await expect(added).toContainText("Arrows +3 (blessed)");
  await expect(added.getByTestId("item-count")).toHaveText("6");
  await expect(added.getByTestId("item-bonus")).toHaveText("+3");
  await snap(page, "item-added");

  await openWorkspace(page, "Act");
  await expect(logEntry(page, "Added Arrows +3 (blessed) (6)")).toBeVisible();
  await openWeapons(page);
  await chooseWeapon(page, "Short Bow +2");
  await expect(
    ammoChips(page).getByRole("radio", { name: "Arrows +3 (blessed), 6 left" }),
  ).toHaveCount(1);
  await snap(page, "ammo-picker-with-new-arrows");

  await undoEntry(page, "Added Arrows +3 (blessed)");
  await expect(
    ammoChips(page).getByRole("radio", { name: /blessed/ }),
  ).toHaveCount(0);
  await openWorkspace(page, "Inventory");
  await expect(added).toHaveCount(0);
});

// ADR 0033 step 2 and ADR 0049 step 9: a magic scroll names the spell it holds (a spell effect of its
// ability), picked from the catalogue in the editor; the edit is logged with the spell's name and popping it gives the scroll back its spell
// as it was (none). Only a browser shows the picker filled from the catalogue's names and the
// round trip through the form; the rules (the casting time it adds) are Go tests.
test("a scroll names its spell in the editor: logged; pop clears it", async ({
  page,
}) => {
  await openInventory(page);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Scroll of flames");
  await f.getByLabel("Category").selectOption("magic");
  await f.getByRole("button", { name: "Next" }).click();
  await f.getByLabel("Magic kind").selectOption("scroll");
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  const scroll = row(page, "scroll_of_flames");
  await expect(scroll).toBeVisible();

  await scroll.getByRole("button", { name: "Edit Scroll of flames" }).click();
  // The spell is the spell effect of the scroll's ability (ADR 0049 step 9): add the ability and its
  // Spell effect, pick the spell; the level is left to the server (the book's).
  await form(page).getByTestId("ability-add").click();
  await form(page).getByTestId("ability-add-spell").click();
  await form(page)
    .getByTestId("ability-spell")
    .selectOption({ label: "Burning Hands" });
  await snap(page, "scroll-spell-form");
  await form(page).getByRole("button", { name: "Save" }).click();
  await expect(form(page)).toHaveCount(0);
  // The row names the spell; clearing it (below, by the pop) takes it away.
  await expect(scroll.getByTestId("item-stats")).toContainText("Burning Hands");

  await openWorkspace(page, "Act");
  await expect(logEntry(page, "Edited Scroll of flames")).toContainText(
    "spell none → Burning Hands",
  );
  await undoEntry(page, "Edited Scroll of flames");
  await openWorkspace(page, "Inventory");
  await scroll.getByRole("button", { name: "Edit Scroll of flames" }).click();
  await expect(form(page).getByTestId("ability-spell")).toHaveCount(0);
  await form(page).getByRole("button", { name: "Cancel" }).click();
  await expect(scroll.getByTestId("item-stats")).not.toContainText(
    "Burning Hands",
  );
});

// ADR 0049 step 16 (and 13): the editor's Abilities section adds a per-day ability to a sword, with
// its note and a dice effect. It shows as a chip on the row with its limit, survives a reload and is
// listed in Act · Items, where Use pays it. Only a browser proves the wiring from the form to the
// chip, the persistence, and the Act tab reading the same abilities; the limit rules are Go tests
// and the form's parsing is a unit test.
test("add a per-day ability to a sword: chip on the row, kept after a reload, used from Act · Items", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await openInventory(page);
  const sword = row(page, "bastard_sword");
  await sword.getByRole("button", { name: "Edit Bastard Sword +2" }).click();
  const f = form(page);
  await f.getByRole("button", { name: "Add an ability to this item" }).click();
  await f.getByTestId("ability-name").fill("Turn zombies");
  await f.getByLabel("Limit").selectOption("per_day");
  await f.getByTestId("ability-max").fill("2");
  await f.getByTestId("ability-note").fill("Holy light");
  await f
    .getByRole("button", { name: "Roll dice when this ability is used" })
    .click();
  await f.getByTestId("ability-dice").fill("1d6");
  await snap(page, "sword-ability-form");
  await f.getByRole("button", { name: "Save" }).click();
  await expect(form(page)).toHaveCount(0);

  const chip = sword.getByTestId("ability-chip-ability_1");
  await expect(chip).toContainText("Turn zombies");
  await expect(chip.getByTestId("ability-chip-limit")).toHaveText(
    "2 / 2 per day",
  );
  await snap(page, "sword-ability-chip");

  await reloadSettled(page);
  await openWorkspace(page, "Inventory");
  await expect(
    row(page, "bastard_sword").getByTestId("ability-chip-limit"),
  ).toHaveText("2 / 2 per day");

  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Items" }).click();
  const line = page.getByTestId("ability-bastard_sword-ability_1");
  await expect(line).toContainText("Bastard Sword +2 · Turn zombies");
  await line.getByRole("button", { name: /^Use / }).click();
  await expect(line.getByTestId("ability-limit")).toHaveText("1 / 2 per day");
  await snap(page, "sword-ability-used");
});

// ADR 0049 steps 9 and 12 (slice 7): a potion of healing is built in the editor with a heal effect
// (full: the gap to max HP, so the outcome does not depend on a roll), used from Act · Items: the
// wounded character's HP goes back to its max, the result card says what the ledger took and the
// journal words it, and one undo gives the HP and the dose back. Only a browser proves the form to
// the use to the dashboard's HP (the server names `hp` as changed) and the card and form at the big
// viewports in both languages; the ledger, cap, rollback and undo rules are Go tests.
test("a potion of healing with a full heal: built in the editor, used from Act · Items, HP back, undone", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await openInventory(page);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Potion of healing");
  await f.getByLabel("Category").selectOption("magic");
  await f.getByRole("button", { name: "Next" }).click();
  await f.getByLabel("Magic kind").selectOption("potion");
  await f.getByRole("button", { name: "Add an ability to this item" }).click();
  await f
    .getByRole("button", {
      name: "Regain hit points when this ability is used",
    })
    .click();
  await f.getByLabel("Fill to max HP").check();
  await expect(f.getByTestId("ability-dice")).toBeDisabled();
  for (const lng of ["fr", "en"] as const) {
    await chooseLanguage(page, lng);
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(
        form(page),
        `the heal in the editor (${lng} ${vp.width})`,
      );
      await snap(page, `potion-heal-form-${lng}-${vp.width}x${vp.height}`);
    }
  }
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  const potion = row(page, "potion_of_healing");
  await expect(potion.getByTestId("ability-chip-ability_1")).toBeVisible();

  await openWorkspace(page, "Act");
  const max = await currentHp(page);
  await recordHp(page, "damage", 9, "e2e wound");
  await expect.poll(() => currentHp(page)).toBe(max - 9);
  await page.getByRole("tab", { name: "Items" }).click();
  const line = page.getByTestId("ability-potion_of_healing-ability_1");
  await line.getByRole("button", { name: /^Use / }).click();
  await expect.poll(() => currentHp(page)).toBe(max);
  const card = page.getByTestId("item-result");
  await expect(card.getByTestId("item-heal-total")).toHaveText("9");
  await expect(card).toContainText(`HP ${max - 9} → ${max}`);
  await expect(logEntry(page, "Used Potion of healing")).toContainText(
    "heals 9 HP",
  );
  for (const lng of ["fr", "en"] as const) {
    await chooseLanguage(page, lng);
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(card, `the heal card (${lng} ${vp.width})`);
      await expectNoClippedText(
        combatLog(page),
        `the heal in the journal (${lng} ${vp.width})`,
      );
      await snap(page, `potion-heal-used-${lng}-${vp.width}x${vp.height}`);
    }
  }

  await undoEntry(page, "Used Potion of healing");
  await expect.poll(() => currentHp(page)).toBe(max - 9);
});

// ADR 0049 steps 9, 10 and 12 (slice 8): a potion of heroism is built in the editor with a buff
// effect (a name, rounds, a to-hit and a saves modifier), used from Act · Items: the buff is on the
// list the dashboard shows (the server names `buffs` as changed), the result card and the journal
// word it, and one undo takes the buff off. Only a browser proves the form to the use to the
// dashboard's buff list and the form and card at the big viewports in both languages; applying,
// renewing, rollback and undo are Go tests.
test("a potion of heroism with a buff: built in the editor, used from Act · Items, buff on the list, undone", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await openInventory(page);
  await openTypedAdd(page);
  const f = form(page);
  await f.getByLabel("Name").fill("Potion of heroism");
  await f.getByLabel("Category").selectOption("magic");
  await f.getByRole("button", { name: "Next" }).click();
  await f.getByLabel("Magic kind").selectOption("potion");
  await f.getByRole("button", { name: "Add an ability to this item" }).click();
  await f
    .getByRole("button", {
      name: "Put a timed effect on the list when this ability is used",
    })
    .click();
  await f.getByLabel("Effect name").fill("Heroism");
  await f.getByLabel("Rounds", { exact: true }).fill("60");
  await f.getByTestId("ability-buff-mod-key").selectOption("to_hit");
  await f.getByTestId("ability-buff-add-mod").click();
  for (const lng of ["fr", "en"] as const) {
    await chooseLanguage(page, lng);
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(
        form(page),
        `the buff in the editor (${lng} ${vp.width})`,
      );
      await snap(page, `potion-buff-form-${lng}-${vp.width}x${vp.height}`);
    }
  }
  await f.getByRole("button", { name: "Add", exact: true }).click();
  await expect(form(page)).toHaveCount(0);
  await expect(
    row(page, "potion_of_heroism").getByTestId("ability-chip-ability_1"),
  ).toBeVisible();

  await openWorkspace(page, "Act");
  await page.getByRole("tab", { name: "Items" }).click();
  const line = page.getByTestId("ability-potion_of_heroism-ability_1");
  await line.getByRole("button", { name: /^Use / }).click();
  const card = page.getByTestId("item-result");
  await expect(card.getByTestId("item-buff-name")).toContainText("Heroism");
  await expect(buffBadge(page, "Heroism")).toBeVisible();
  await expect(logEntry(page, "Used Potion of heroism")).toContainText(
    "effect Heroism (60 rd)",
  );
  for (const lng of ["fr", "en"] as const) {
    await chooseLanguage(page, lng);
    for (const vp of [
      { width: 1920, height: 1080 },
      { width: 2560, height: 1440 },
    ]) {
      await page.setViewportSize(vp);
      await expectNoClippedText(card, `the buff card (${lng} ${vp.width})`);
      await expectNoClippedText(
        combatLog(page),
        `the buff in the journal (${lng} ${vp.width})`,
      );
      await snap(page, `potion-buff-used-${lng}-${vp.width}x${vp.height}`);
    }
  }

  await undoEntry(page, "Used Potion of heroism");
  await expect(buffBadge(page, "Heroism")).toHaveCount(0);
});
