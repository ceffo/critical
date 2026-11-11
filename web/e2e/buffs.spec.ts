import { test, expect, snap } from "./fixtures";
import {
  addBuff,
  buffBadge,
  expectNoReload,
  logEntry,
  markNoReload,
  openAddEffect,
  openDashboard,
  undoEntry,
} from "./helpers";

test("Stoneskin layers absorb hits from the character card, the buff ends at 0, and the absorb is voidable", async ({
  page,
}) => {
  await openDashboard(page);
  await markNoReload(page);

  await addBuff(page, { name: "e2e Stoneskin", charges: 2 });
  const badge = buffBadge(page, "e2e Stoneskin");
  await expect(badge.getByTestId("buff-charges")).toHaveText(
    "2 / 2 charges · until spent",
  );
  // The character card shows a card for it, above the damage entry.
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
    logEntry(page, "Saved effect e2e Hammer form (10 rd)"),
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
  ).toHaveText("10 rounds");
  await expect(saved).toBeHidden();

  // Delete the template: the active buffs stay.
  const again = await openAddEffect(page, "Saved effect");
  await again
    .getByRole("button", { name: "Delete saved effect e2e Hammer form" })
    .click();
  await expect(again.getByTestId("saved-buff")).toHaveCount(0);
  await expect(buffBadge(page, "e2e Hammer form")).toHaveCount(2);
  await expect(
    logEntry(page, "Deleted saved effect e2e Hammer form"),
  ).toBeVisible();
  await snap(page, "saved-deleted");

  await undoEntry(page, "Deleted saved effect e2e Hammer form");
  await openAddEffect(page, "Saved effect");
  await expect(again.getByTestId("saved-buff")).toHaveCount(1);

  await expectNoReload(page);
});
