import { test, expect, snap } from "./fixtures";
import {
  addBuff,
  buffBadge,
  currentRound,
  effectiveStr,
  expectNoReload,
  logEntry,
  markNoReload,
  openDashboard,
  voidEntry,
} from "./helpers";

test("Next Round then void restores the round, the durations and the expired buff", async ({
  page,
}) => {
  await openDashboard(page);
  const r0 = await currentRound(page);
  await markNoReload(page);

  await addBuff(page, { name: "e2e Haste", rounds: 1 });
  await addBuff(page, { name: "e2e Bless", rounds: 3 });
  await expect(logEntry(page, "Activated e2e Haste (1 rd)")).toBeVisible();

  await page.getByRole("button", { name: "Next Round" }).click();
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);
  await expect(page.getByRole("status")).toContainText(
    "Expired buffs: e2e Haste",
  );
  await expect(buffBadge(page, "e2e Haste")).toHaveCount(0);
  await expect(
    buffBadge(page, "e2e Bless").getByTestId("buff-rounds"),
  ).toHaveText("2 rd");
  const advance = `Round ${r0} → ${r0 + 1} — expired: e2e Haste`;
  // Logged in the round it ends.
  await expect(
    page
      .getByRole("region", { name: `Round ${r0}` })
      .getByRole("listitem")
      .filter({ hasText: advance }),
  ).toBeVisible();
  await snap(page, "next-round-expired");

  await voidEntry(page, advance);
  await expect.poll(() => currentRound(page)).toBe(r0);
  await expect(page.getByRole("status")).toHaveCount(0);
  await expect(
    buffBadge(page, "e2e Haste").getByTestId("buff-rounds"),
  ).toHaveText("1 rd");
  await expect(
    buffBadge(page, "e2e Bless").getByTestId("buff-rounds"),
  ).toHaveText("3 rd");

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
  await form.getByLabel("Rounds").fill("4");
  await form.getByRole("button", { name: "Save" }).click();

  await expect(
    buffBadge(page, "e2e Strength").getByTestId("buff-rounds"),
  ).toHaveText("4 rd");
  const edit = logEntry(page, "e2e Strength: 10 rd → 4 rd");
  await expect(edit).toBeVisible();
  await snap(page, "edit-rounds");

  await voidEntry(page, "e2e Strength: 10 rd → 4 rd");
  await expect(
    buffBadge(page, "e2e Strength").getByTestId("buff-rounds"),
  ).toHaveText("10 rd");

  await expectNoReload(page);
  await snap(page, "edit-rounds-voided");
});

test("Stoneskin layers absorb hits, the buff ends at 0, and the absorb is voidable", async ({
  page,
}) => {
  await openDashboard(page);
  await markNoReload(page);

  await addBuff(page, { name: "e2e Stoneskin", charges: 2 });
  const badge = buffBadge(page, "e2e Stoneskin");
  await expect(badge.getByTestId("buff-charges")).toHaveText("2 ch");

  await badge
    .getByRole("button", { name: "e2e Stoneskin absorbed a hit" })
    .click();
  await expect(badge.getByTestId("buff-charges")).toHaveText("1 ch");
  await expect(
    logEntry(page, "e2e Stoneskin absorbed a hit (1 left)"),
  ).toBeVisible();
  await snap(page, "stoneskin-absorbed");

  await badge
    .getByRole("button", { name: "e2e Stoneskin absorbed a hit" })
    .click();
  await expect(badge).toHaveCount(0);
  await expect(
    logEntry(page, "e2e Stoneskin absorbed a hit, ended"),
  ).toBeVisible();
  await snap(page, "stoneskin-ended");

  await voidEntry(page, "e2e Stoneskin absorbed a hit, ended");
  await expect(badge.getByTestId("buff-charges")).toHaveText("1 ch");

  await expectNoReload(page);
  await snap(page, "stoneskin-voided");
});

test.describe("with the sheet's STR 12", () => {
  test.use({ character: { str: 12 } });

  test("a Strength buff reaching 18 turns the Power Gem on, and it drops on expiry", async ({
    page,
  }) => {
    await openDashboard(page);
    await markNoReload(page);
    await expect.poll(() => effectiveStr(page)).toBe(12);

    await addBuff(page, {
      name: "e2e Power Gem",
      effects: [
        ["min_str", 18],
        ["weapon_buff", 5],
        ["multiplier", 2],
      ],
    });
    const gem = buffBadge(page, "e2e Power Gem");
    await expect(gem).toHaveAttribute("data-applies", "false");
    await expect(gem).toContainText("needs STR 18");

    await addBuff(page, {
      name: "e2e Strength",
      rounds: 1,
      effects: [
        ["str_bonus", 6],
        ["str_max", 18],
      ],
    });
    await expect.poll(() => effectiveStr(page)).toBe(18);
    await expect(gem).toHaveAttribute("data-applies", "true");
    await expect(gem).not.toContainText("needs STR");
    await expect(gem.getByTestId("buff-effects")).toHaveText("weapon +5 · dice ×2");
    await snap(page, "strength-18-gem-on");

    await page.getByRole("button", { name: "Next Round" }).click();
    await expect(buffBadge(page, "e2e Strength")).toHaveCount(0);
    await expect.poll(() => effectiveStr(page)).toBe(12);
    await expect(gem).toHaveAttribute("data-applies", "false");

    await expectNoReload(page);
    await snap(page, "strength-expired-gem-off");
  });

  test("a hand-made Strength with no max stacks with a capped one up to 18; Improved replaces both", async ({
    page,
  }) => {
    await openDashboard(page);
    await markNoReload(page);
    await expect.poll(() => effectiveStr(page)).toBe(12);
    await addBuff(page, {
      name: "e2e Gem",
      effects: [
        ["min_str", 18],
        ["weapon_buff", 5],
      ],
    });
    const gem = buffBadge(page, "e2e Gem");

    // ADR 0004 step 7: +4 (no max) and +3 (max 18) on 12: 19, capped at 18.
    await addBuff(page, {
      name: "e2e Strength hand",
      rounds: 20,
      effects: [["str_bonus", 4]],
    });
    await addBuff(page, {
      name: "e2e Strength cast",
      rounds: 20,
      effects: [
        ["str_bonus", 3],
        ["str_max", 18],
      ],
    });
    await expect.poll(() => effectiveStr(page)).toBe(18);
    await expect(gem).toHaveAttribute("data-applies", "true");

    // ADR 0004 step 8: a low Improved Strength (1 + 4) replaces both: 17, the gem goes off and the
    // Strength badges say they are replaced.
    await addBuff(page, {
      name: "e2e Improved",
      rounds: 5,
      effects: [
        ["str_bonus", 5],
        ["str_max", 23],
      ],
    });
    await expect.poll(() => effectiveStr(page)).toBe(17);
    await expect(gem).toHaveAttribute("data-applies", "false");
    for (const name of ["e2e Strength hand", "e2e Strength cast"]) {
      await expect(buffBadge(page, name)).toHaveAttribute(
        "data-applies",
        "false",
      );
      await expect(buffBadge(page, name)).toContainText(
        "replaced by Improved Strength",
      );
    }
    await expect(buffBadge(page, "e2e Improved")).toHaveAttribute(
      "data-applies",
      "true",
    );
    await expectNoReload(page);
    await snap(page, "improved-replaces-strength");
  });
});

test("a buff note shows on the badge, edits are logged, void restores it", async ({
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

  await voidEntry(page, "e2e Ward: note “fire damage −50%”");
  await expect(badge.getByTestId("buff-note")).toHaveText("+4 saves vs fire");

  await expectNoReload(page);
  await snap(page, "note-voided");
});

test("a saved buff fills the form from its chip, deleting it leaves active buffs alone and is voidable", async ({
  page,
}) => {
  await openDashboard(page);
  await markNoReload(page);
  const strip = page.getByRole("group", { name: "Buffs" });

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

  // Re-add from the chip, changing the rounds first.
  await strip.getByRole("button", { name: "Add buff" }).click();
  const form = strip.getByRole("form", { name: "Add buff" });
  await expect(form.getByTestId("saved-buff")).toHaveCount(1);
  await form
    .getByRole("button", { name: "Fill the form with e2e Hammer form" })
    .click();
  await expect(form.getByLabel("Buff name")).toHaveValue("e2e Hammer form");
  await expect(form.getByLabel("Rounds")).toHaveValue("10");
  await expect(form.getByLabel("Note")).toHaveValue("thunderclap stuns 30 m");
  await expect(form.getByTestId("effect-row")).toHaveCount(3);
  await snap(page, "saved-chip-filled");
  await form.getByLabel("Rounds").fill("4");
  await form.getByRole("button", { name: "Activate" }).click();
  await expect(
    buffBadge(page, "e2e Hammer form").last().getByTestId("buff-rounds"),
  ).toHaveText("4 rd");

  // Delete the template: the active buff stays.
  await strip.getByRole("button", { name: "Add buff" }).click();
  const form2 = strip.getByRole("form", { name: "Add buff" });
  await form2
    .getByRole("button", { name: "Delete saved buff e2e Hammer form" })
    .click();
  await expect(form2.getByTestId("saved-buff")).toHaveCount(0);
  await expect(buffBadge(page, "e2e Hammer form")).toHaveCount(2);
  await expect(
    logEntry(page, "Deleted saved buff e2e Hammer form"),
  ).toBeVisible();
  await snap(page, "saved-deleted");

  await voidEntry(page, "Deleted saved buff e2e Hammer form");
  await expect(form2.getByTestId("saved-buff")).toHaveCount(1);

  await expectNoReload(page);
});
