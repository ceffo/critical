import { test, expect, snap } from "./fixtures";
import {
  addBuff,
  buffBadge,
  currentRound,
  effectiveStr,
  expectNoReload,
  logEntry,
  markNoReload,
  nextTurn,
  openAddEffect,
  openDashboard,
  undoEntry,
  undoNextTurn,
  turnGroup,
} from "./helpers";

test("Next turn then void restores the round, the durations and the expired buff", async ({
  page,
}) => {
  await openDashboard(page);
  const r0 = await currentRound(page);
  await markNoReload(page);

  await addBuff(page, { name: "e2e Haste", rounds: 1 });
  await addBuff(page, { name: "e2e Bless", rounds: 3 });
  await expect(logEntry(page, "Activated e2e Haste (1 rd)")).toBeVisible();

  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);
  await expect(page.getByRole("status")).toContainText(
    "Expired buffs: e2e Haste",
  );
  await expect(buffBadge(page, "e2e Haste")).toHaveCount(0);
  await expect(
    buffBadge(page, "e2e Bless").getByTestId("buff-rounds"),
  ).toHaveText("2 turns");
  const advance = `Turn ${r0} → ${r0 + 1} — expired: e2e Haste`;
  // Logged with its initiative as one transaction, in the round it opens.
  await expect(
    turnGroup(page, r0 + 1)
      .getByRole("listitem")
      .filter({ hasText: advance }),
  ).toBeVisible();
  await snap(page, "next-round-expired");

  await undoNextTurn(page, logEntry(page, advance));
  await expect.poll(() => currentRound(page)).toBe(r0);
  await expect(page.getByRole("status")).toHaveCount(0);
  await expect(
    buffBadge(page, "e2e Haste").getByTestId("buff-rounds"),
  ).toHaveText("1 turn");
  await expect(
    buffBadge(page, "e2e Bless").getByTestId("buff-rounds"),
  ).toHaveText("3 turns");

  await expectNoReload(page);
  await snap(page, "next-round-voided");
});

test("editing a buff's remaining rounds is logged and voidable", async ({
  page,
}) => {
  await openDashboard(page);
  await markNoReload(page);

  await addBuff(page, { name: "e2e Strength", rounds: 10 });
  await buffBadge(page, "e2e Strength")
    .getByRole("button", { name: "Edit e2e Strength" })
    .click();
  const form = page.getByRole("form", { name: "Edit e2e Strength" });
  await form.getByLabel("Turns").fill("4");
  await form.getByRole("button", { name: "Save" }).click();

  await expect(
    buffBadge(page, "e2e Strength").getByTestId("buff-rounds"),
  ).toHaveText("4 turns");
  const edit = logEntry(page, "e2e Strength: 10 rd → 4 rd");
  await expect(edit).toBeVisible();
  await snap(page, "edit-rounds");

  await undoEntry(page, "e2e Strength: 10 rd → 4 rd");
  await expect(
    buffBadge(page, "e2e Strength").getByTestId("buff-rounds"),
  ).toHaveText("10 turns");

  await expectNoReload(page);
  await snap(page, "edit-rounds-voided");
});

test("Stoneskin layers absorb hits from the Moi card, the buff ends at 0, and the absorb is voidable", async ({
  page,
}) => {
  await openDashboard(page);
  await markNoReload(page);

  await addBuff(page, { name: "e2e Stoneskin", charges: 2 });
  const badge = buffBadge(page, "e2e Stoneskin");
  await expect(badge.getByTestId("buff-charges")).toHaveText(
    "2 / 2 charges · until spent",
  );
  // The Moi column shows a card for it, above the damage entry.
  const card = page
    .getByRole("complementary", { name: "Character" })
    .getByRole("group", { name: "e2e Stoneskin" });
  const absorb = card.getByRole("button", {
    name: "Absorb a hit with e2e Stoneskin",
  });
  await expect(card.getByTestId("charge-count")).toHaveText("2 / 2 charges");
  await expect(badge.getByTestId("pip-left")).toHaveCount(2);

  await absorb.click();
  await expect(card.getByTestId("charge-count")).toHaveText("1 / 2 charges");
  await expect(badge.getByTestId("buff-charges")).toHaveText(
    "1 / 2 charges · until spent",
  );
  await expect(badge.getByTestId("pip-left")).toHaveCount(1);
  await expect(badge.getByTestId("pip-used")).toHaveCount(1);
  await expect(
    logEntry(page, "e2e Stoneskin absorbed a hit (1 left)"),
  ).toBeVisible();
  await snap(page, "stoneskin-absorbed");

  // The timeline row's absorb does the same.
  await badge
    .getByRole("button", { name: "e2e Stoneskin absorbed a hit" })
    .click();
  await expect(badge).toHaveCount(0);
  await expect(card).toHaveCount(0);
  await expect(
    logEntry(page, "e2e Stoneskin absorbed a hit, ended"),
  ).toBeVisible();
  await snap(page, "stoneskin-ended");

  await undoEntry(page, "e2e Stoneskin absorbed a hit, ended");
  await expect(badge.getByTestId("buff-charges")).toHaveText(
    "1 / 2 charges · until spent",
  );
  await expect(card.getByTestId("charge-count")).toHaveText("1 / 2 charges");

  await expectNoReload(page);
  await snap(page, "stoneskin-voided");
});

test.describe("with the sheet's STR 12", () => {
  test.use({ character: { str: 12 } });

  test("a Strength buff reaching 18 turns the Power Gem's hammer on, and it drops on expiry", async ({
    page,
  }) => {
    await openDashboard(page);
    await markNoReload(page);
    await expect.poll(() => effectiveStr(page)).toBe(12);
    // The gem's thunder hammer is an item power: below its STR 18 trigger it shows nowhere.
    const hammer = page.locator(
      '[data-testid="item-power"][data-item="power_gem_dispel"]',
    );
    await expect(hammer).toHaveCount(0);

    await addBuff(page, {
      name: "e2e Strength",
      rounds: 1,
      effects: [
        ["str_bonus", 6],
        ["str_max", 18],
      ],
    });
    await expect.poll(() => effectiveStr(page)).toBe(18);
    // ADR 0027 step 5.4: only the changed value shows; the sheet's is in a tooltip on keyboard
    // focus (and hover), not under it.
    const str = page.getByTestId("str-cell");
    await expect(str).not.toContainText("12");
    // Keyboard focus (a programmatic focus is not focus-visible): from the cell, back and forth.
    await str.focus();
    await page.keyboard.press("Shift+Tab");
    await page.keyboard.press("Tab");
    await expect(str).toBeFocused();
    await expect(page.locator("[data-slot=tooltip-content]")).toHaveText(
      "STR 18 (sheet 12)",
    );
    await snap(page, "strength-18-tooltip");
    await str.blur();
    // On by itself, under the item's name, with nothing to edit or dismiss.
    await expect(hammer).toContainText("Power Gem");
    await expect(hammer.getByTestId("power-effects")).toHaveText(
      "weapon +5 · dice ×2",
    );
    await expect(hammer.getByTestId("effect-bar")).toHaveText("while STR ≥ 18");
    await expect(hammer.getByRole("button")).toHaveCount(0);
    await snap(page, "strength-18-gem-on");

    await nextTurn(page);
    await expect(buffBadge(page, "e2e Strength")).toHaveCount(0);
    await expect.poll(() => effectiveStr(page)).toBe(12);
    await expect(hammer).toHaveCount(0);

    await expectNoReload(page);
    await snap(page, "strength-expired-gem-off");
  });
});

test("a buff note shows on its row, edits are logged, void restores it", async ({
  page,
}) => {
  await openDashboard(page);
  await markNoReload(page);

  await addBuff(page, {
    name: "e2e Ward",
    rounds: 5,
    note: "+4 saves vs fire",
  });
  const badge = buffBadge(page, "e2e Ward");
  await expect(badge.getByTestId("buff-note")).toHaveText("+4 saves vs fire");

  await badge.getByRole("button", { name: "Edit e2e Ward" }).click();
  const form = page.getByRole("form", { name: "Edit e2e Ward" });
  await form.getByLabel("Note").fill("fire damage −50%");
  await form.getByRole("button", { name: "Save" }).click();
  await expect(badge.getByTestId("buff-note")).toHaveText("fire damage −50%");
  await expect(
    logEntry(page, "e2e Ward: note “fire damage −50%”"),
  ).toBeVisible();
  await snap(page, "note-edited");

  await undoEntry(page, "e2e Ward: note “fire damage −50%”");
  await expect(badge.getByTestId("buff-note")).toHaveText("+4 saves vs fire");

  await expectNoReload(page);
  await snap(page, "note-voided");
});

test("a saved buff is added from the Saved effect tab, deleting it leaves active buffs alone and is voidable", async ({
  page,
}) => {
  await openDashboard(page);
  await markNoReload(page);
  await addBuff(page, {
    name: "e2e Hammer form",
    rounds: 10,
    effects: [
      ["weapon_buff", 5],
      ["min_str", 18],
      ["spells_buff", 2],
    ],
    note: "thunderclap stuns 30 m",
    save: true,
  });
  await expect(
    logEntry(page, "Saved buff e2e Hammer form (10 rd)"),
  ).toBeVisible();

  // Add it again from the Saved effect tab, as it was saved.
  const saved = await openAddEffect(page, "Saved effect");
  await expect(saved.getByTestId("saved-buff")).toHaveCount(1);
  await snap(page, "saved-tab");
  await saved
    .getByRole("button", { name: "Add saved effect e2e Hammer form" })
    .click();
  await expect(buffBadge(page, "e2e Hammer form")).toHaveCount(2);
  await expect(
    buffBadge(page, "e2e Hammer form").last().getByTestId("buff-rounds"),
  ).toHaveText("10 turns");
  await expect(saved).toBeHidden();

  // Delete the template: the active buffs stay.
  const again = await openAddEffect(page, "Saved effect");
  await again
    .getByRole("button", { name: "Delete saved buff e2e Hammer form" })
    .click();
  await expect(again.getByTestId("saved-buff")).toHaveCount(0);
  await expect(buffBadge(page, "e2e Hammer form")).toHaveCount(2);
  await expect(
    logEntry(page, "Deleted saved buff e2e Hammer form"),
  ).toBeVisible();
  await snap(page, "saved-deleted");

  await undoEntry(page, "Deleted saved buff e2e Hammer form");
  await openAddEffect(page, "Saved effect");
  await expect(again.getByTestId("saved-buff")).toHaveCount(1);

  await expectNoReload(page);
});
