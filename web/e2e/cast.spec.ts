import { test, expect, snap } from "./fixtures";
import {
  addBuff,
  chooseLanguage,
  buffBadge,
  currentHp,
  effectiveStr,
  logEntry,
  openAddEffect,
  openDashboard,
  receivePicker,
  recordHp,
  undoEntry,
  expectSlotDots,
  readsAfter,
  combatLog,
} from "./helpers";
import { expectNoClippedText } from "./noClip";
import type { Page } from "@playwright/test";

// Dice are random on the real stack: assert what the cast did, not the rolled numbers
// (castsvc's Go tests pin the rolls). Kept: one cast with its undo, one received spell with a saved
// effect, one heal in both languages. The card's wording, the cast modes, the damage steps, the
// exposure, the undo order and the slots are component and Go tests.

async function prepare(apiUrl: string, spells: string[]) {
  const res = await fetch(`${apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells }),
  });
  expect(res.ok).toBe(true);
}

async function openSpells(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Spells" }).click();
  await expect(
    page.getByRole("button", { name: "Prepare spells" }),
  ).toBeVisible();
}

function resultCard(page: Page) {
  return page.getByRole("region", { name: "Detail", exact: true });
}

test("casting Stoneskin shows the result card, adds the buff; undoing gives the slot back", async ({
  page,
  stack,
}) => {
  await prepare(stack.apiUrl, ["stoneskin"]);
  await openSpells(page);
  const level4 = page.getByRole("region", { name: "Level 4" });
  await expectSlotDots(level4, 4, 1, 1);

  const card = resultCard(page).getByTestId("cast-result");
  const reads = await readsAfter(page, async () => {
    await level4.getByRole("button", { name: "Cast Stoneskin" }).click();
    await expect(card).toContainText(
      "Stoneskin (level 4), cast at caster level 15",
    );
  });
  test.info().annotations.push({ type: "reads", description: reads.join(" ") });
  // The answer carries the slots and names "buffs,log,spell_slots": the journal, the effects and
  // what follows them (STR, AC) are read again, nothing else.
  expect(reads).toEqual([
    "/api/ac",
    "/api/buffs",
    "/api/log",
    "/api/stats/str",
  ]);
  await expect(card.getByTestId("cast-buff")).toContainText("On you");
  await expect(card.getByTestId("cast-charges")).toContainText(/1d4\+\d+/);
  await expect(card.getByTestId("cast-text")).toBeHidden(); // folded by default
  await card.getByText("Spell text").click();
  await expect(card.getByTestId("cast-text")).toBeVisible();
  await expect(buffBadge(page, "Stoneskin")).toBeVisible();
  await expectSlotDots(level4, 4, 0, 1);
  await expect(logEntry(page, "Cast Stoneskin")).toBeVisible();
  await snap(page, "stoneskin-cast");

  await undoEntry(page, "Cast Stoneskin");
  await expect(buffBadge(page, "Stoneskin")).toHaveCount(0);
  await expectSlotDots(level4, 4, 1, 1);
  await snap(page, "stoneskin-undone");
});

test("Strength cast on me by a party wizard, found by its French name; a saved effect joins it", async ({
  page,
}) => {
  await openDashboard(page);
  const before = await effectiveStr(page);
  const picker = await receivePicker(page);
  await picker.getByLabel("Search for a received spell").fill("Force");
  await picker.getByLabel("Caster level").fill("9");
  await picker.getByRole("button", { name: "Apply Strength" }).click();

  const card = resultCard(page).getByTestId("cast-result");
  await expect(card).toContainText("Strength cast on you, caster level 9");
  await expect(card.getByTestId("cast-rounds")).toContainText("= 540");
  await expect(card.getByTestId("cast-str-gain")).toContainText("1d6");
  // One entry point: the popover closes, the buff is on the timeline from the current turn.
  await expect(page.getByTestId("add-effect-popover")).toBeHidden();
  await expect(buffBadge(page, "Strength")).toBeVisible();
  await expect(
    buffBadge(page, "Strength").getByTestId("buff-rounds"),
  ).toContainText("540 rounds");
  await expect.poll(() => effectiveStr(page)).toBeGreaterThan(before);
  await expect(
    logEntry(page, "Strength received (caster level 9)"),
  ).toBeVisible();

  // A saved effect from the same popover.
  await addBuff(page, { name: "e2e Ward", rounds: 5, save: true });
  const saved = await openAddEffect(page, "Saved effect");
  await saved
    .getByRole("button", { name: "Add saved effect e2e Ward" })
    .click();
  await expect(buffBadge(page, "e2e Ward")).toHaveCount(2);

  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await openAddEffect(page, "Received spell");
    await snap(page, `add-effect-${vp.width}x${vp.height}`);
    await page.keyboard.press("Escape");
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await snap(page, "strength-received");
});

// ADR 0008 step 17: a cast heal goes through the HP ledger. Dice are random: the card says what was
// gained, the HP follows it, and one undo gives the slot and the HP back (castsvc's Go tests pin the
// rolls and the cap).
test.describe("a priest healing", () => {
  test.use({
    character: {
      class: "cleric",
      race: "human",
      alignment: "neutral_good",
      wis: 15,
      level: 1,
      xp_total: 0,
      hp_max: 30,
      skill_points: {},
      proficiencies: [],
      weapon_proficiencies: [],
    },
  });

  test("casting Cure Light Wounds on a wounded character raises the HP; one undo restores it, in French at the big viewports", async ({
    page,
    stack,
  }) => {
    await prepare(stack.apiUrl, ["cure_light_wounds"]);
    await openSpells(page);
    const max = await currentHp(page);
    const level1 = page.getByRole("region", { name: "Level 1" });
    await recordHp(page, "damage", 8, "e2e wound");
    await expect.poll(() => currentHp(page)).toBe(max - 8);
    await expectSlotDots(level1, 1, 1, 1);

    // A heal that lands on the bearer by default: the arrow offers the other target.
    await level1
      .getByRole("button", { name: "Choose the target of Cure Light Wounds" })
      .click();
    await expect(page.getByRole("menuitem")).toHaveCount(1);
    await page.keyboard.press("Escape");

    await level1
      .getByRole("button", { name: "Cast Cure Light Wounds" })
      .click();
    const card = resultCard(page).getByTestId("cast-result");
    const heal = card.getByTestId("cast-heal");
    await expect(heal).toBeVisible();
    const gained = Number(
      await heal.getByTestId("cast-heal-total").textContent(),
    );
    expect(gained).toBeGreaterThanOrEqual(1);
    expect(gained).toBeLessThanOrEqual(8);
    await expect(heal.getByTestId("cast-heal-range")).toHaveText(
      `HP ${max - 8} → ${max - 8 + gained}`,
    );
    await expect.poll(() => currentHp(page)).toBe(max - 8 + gained);
    await expectSlotDots(level1, 1, 0, 1);
    await expect(logEntry(page, "Cast Cure Light Wounds")).toBeVisible();
    await snap(page, "cure-light-wounds");

    for (const lng of ["fr", "en"] as const) {
      await chooseLanguage(page, lng);
      for (const vp of [
        { width: 1920, height: 1080 },
        { width: 2560, height: 1440 },
      ]) {
        await page.setViewportSize(vp);
        await expectNoClippedText(
          page.getByRole("region", {
            name: lng === "fr" ? "Détail" : "Detail",
            exact: true,
          }),
          `the heal card (${lng} ${vp.width})`,
        );
        await expectNoClippedText(
          combatLog(page),
          `the heal in the journal (${lng} ${vp.width})`,
        );
        await snap(page, `cure-light-wounds-${lng}-${vp.width}x${vp.height}`);
      }
    }
    await page.setViewportSize({ width: 1440, height: 900 });

    await undoEntry(page, "Cast Cure Light Wounds");
    await expect.poll(() => currentHp(page)).toBe(max - 8);
    await expectSlotDots(level1, 1, 1, 1);
  });
});
