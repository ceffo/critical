import { test, expect } from "./fixtures";
import {
  apiCall,
  expectNoReload,
  markNoReload,
  openDashboard,
  openWorkspace,
  reloadSettled,
} from "./helpers";
import type { Page } from "@playwright/test";

// ADR 0037 step 15, what only a browser proves: the "to spend" tag and its links by keyboard, two
// levels owed one after the other, the panel following the undo and the redo, an editor opened
// from another tab or activity, and the tag going when the last slot or point is spent, all with
// no reload. The counts themselves are Go tests (charactersvc, levelupsvc, composition).

async function makeCharacter(
  page: Page,
  cls: string,
  name: string,
  xp: number,
  hpMax = 6,
) {
  await openDashboard(page);
  expect(
    await apiCall(page, "POST", "/api/characters", {
      name,
      class: cls,
      raceCode: "human",
      details: { alignment: "neutral_good", sex: "male" },
      str: 12,
      dex: 15,
      con: 12,
      int: 14,
      wis: 9,
      cha: 10,
      hpMax,
      xp: 0,
    }),
  ).toBe(true);
  if (xp > 0)
    expect(
      await apiCall(page, "POST", "/api/xp", {
        base: xp,
        bonus: false,
        reason: "Goblins",
      }),
    ).toBe(true);
  await reloadSettled(page);
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
}

const roll = (page: Page) => page.getByTestId("level-up-roll");

test("thief 1 to 3: the tag by keyboard, the panel follows each level, undo and redo", async ({
  page,
}) => {
  await makeCharacter(page, "thief", "Vex", 2500);
  const prompt = page.getByTestId("level-up");
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 2 reached",
  );
  const tag = page.getByTestId("to-spend-tag");
  await expect(tag).toBeVisible();

  // Keyboard only: focus the tag, Enter opens it, Tab reaches a link, Enter opens the editor.
  await tag.focus();
  await page.keyboard.press("Enter");
  const pop = page.getByTestId("level-up-to-spend");
  await expect(pop).toBeVisible();
  // The popover takes focus onto its first link.
  const firstLink = pop.getByRole("button").first();
  await expect(firstLink).toBeFocused();
  // Every link is reachable by Tab, in order.
  const links = await pop.getByRole("button").count();
  expect(links).toBeGreaterThan(0);
  for (let i = 1; i < links; i++) {
    await page.keyboard.press("Tab");
    await expect(pop.getByRole("button").nth(i)).toBeFocused();
  }
  // Escape returns focus to the tag.
  await page.keyboard.press("Escape");
  await expect(pop).toBeHidden();
  await expect(tag).toBeFocused();
  // Enter on a link: the editor opens in the Skills tab and the popover closes.
  await page.keyboard.press("Enter");
  await expect(firstLink).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(pop).toBeHidden();
  await expect(page.getByRole("tab", { name: "Skills" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(
    page
      .getByTestId("weapon-editor")
      .or(page.getByTestId("proficiency-editor"))
      .or(page.getByTestId("thief-points-editor")),
  ).toBeVisible();
});

async function toSpendText(page: Page): Promise<string> {
  await page.getByTestId("to-spend-tag").click();
  const pop = page.getByTestId("level-up-to-spend");
  const text = (await pop.textContent()) ?? "";
  await page.keyboard.press("Escape");
  await expect(pop).toBeHidden();
  return text;
}

test("thief 1 to 3: roll 2 then 3, the panel switches; undo takes it, redo brings it back", async ({
  page,
}) => {
  await makeCharacter(page, "thief", "Vex", 2500);
  const prompt = page.getByTestId("level-up");
  const grants = page.getByTestId("level-up-grants");
  expect(await toSpendText(page)).toContain("60 skill points");
  await roll(page).click();
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 3 reached",
  );
  await expect(grants).toContainText("Level 2 grants");
  expect(await toSpendText(page)).toContain("90 skill points");
  await roll(page).click();
  await expect(prompt).toHaveCount(0);
  await expect(grants).toContainText("Level 3 grants");
  // The tag counts what both levels left.
  expect(await toSpendText(page)).toContain("120 skill points");

  // Undo the level 3 roll: its panel goes; redo: it is back.
  await page.getByTestId("undo").click();
  await expect(grants).toHaveCount(0);
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 3 reached",
  );
  await page.getByTestId("redo").click();
  await expect(grants).toContainText("Level 3 grants");

  // Closed with "Fermer", it stays closed.
  await grants.getByTestId("level-up-grants-dismiss").click();
  await expect(grants).toHaveCount(0);
});

test("spend every slot and point in the editors: the tag goes with no reload", async ({
  page,
}) => {
  await makeCharacter(page, "thief", "Vex", 0);
  await markNoReload(page);
  const tag = page.getByTestId("to-spend-tag");
  await expect(tag).toBeVisible();
  const left = async () => {
    await tag.click();
    const text =
      (await page.getByTestId("level-up-to-spend").textContent()) ?? "";
    await page.keyboard.press("Escape");
    return text;
  };
  await page.getByRole("tab", { name: "Skills" }).click();
  const skills = page.getByRole("tabpanel", { name: "Skills" });

  // Weapons: both slots on one type.
  await skills.getByTestId("weapon-edit").click();
  const weapon = skills.getByTestId("weapon-editor");
  const add = weapon.getByTestId("weapon-add").first();
  await add.click();
  await add.click();
  await expect(weapon.getByTestId("weapon-pool-spent")).toHaveText(
    "Spent 2, left 0",
  );
  await weapon.getByTestId("weapon-save").click();
  await expect(weapon).toBeHidden();
  expect(await left()).not.toContain("weapon proficiency");

  // Non-weapon: one cost-1 proficiency with all seven slots.
  await skills.getByTestId("proficiency-edit").click();
  const prof = skills.getByTestId("proficiency-editor");
  await prof
    .locator("[data-testid^=proficiency-add-]:not([disabled])")
    .first()
    .click();
  for (let i = 0; i < 6; i++)
    await prof.getByTestId("proficiency-more").first().click();
  await expect(prof.getByTestId("proficiency-pool-spent")).toHaveText(
    "Spent 7, left 0",
  );
  await prof.getByTestId("proficiency-save").click();
  await expect(prof).toBeHidden();
  const afterProf = await left();
  expect(afterProf).not.toContain("proficiency slots");
  expect(afterProf).toContain("60 skill points");

  // Points: 30 on each of two skills; the tag goes with the last point.
  await skills.getByTestId("thief-points-edit").click();
  const pts = skills.getByTestId("thief-points-editor");
  const inputs = pts.locator("input");
  await inputs.nth(0).fill("30");
  await inputs.nth(1).fill("30");
  await expect(pts.getByTestId("skill-points-preview")).toContainText(
    "Spent 60, left 0",
  );
  await pts.getByTestId("thief-points-save").click();
  await expect(pts).toBeHidden();
  await expect(tag).toHaveCount(0);
  await expectNoReload(page);
});

test("an editor opened from the card, from another tab and another activity", async ({
  page,
}) => {
  await makeCharacter(page, "thief", "Vex", 0);
  const tag = page.getByTestId("to-spend-tag");
  const open = async (link: string) => {
    await tag.click();
    await page.getByTestId(link).click();
  };
  const skillsTab = page.getByRole("tab", { name: "Skills" });

  // From the Weapons tab.
  await page.getByRole("tab", { name: "Weapons" }).click();
  await open("grant-open-weapon");
  await expect(skillsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByTestId("weapon-editor")).toBeVisible();
  // Leaving the tab closes the editor.
  await page.getByRole("tab", { name: "Weapons" }).click();
  await expect(page.getByTestId("weapon-editor")).toHaveCount(0);

  // From the Experience activity: Act is focused, the tab and the editor are right.
  await openWorkspace(page, "Experience");
  await open("grant-open-skillPoints");
  await expect(
    page
      .getByRole("navigation", { name: "Activities" })
      .getByRole("button", { name: "Act", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  await expect(skillsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByTestId("thief-points-editor")).toBeVisible();
  await page.getByRole("tab", { name: "Weapons" }).click();
  await expect(page.getByTestId("thief-points-editor")).toHaveCount(0);
});

test("a fighter at 4 has no grants and no panel; a mage at 3 shows only the non-weapon row", async ({
  page,
}) => {
  // Table 34: a warrior gains a weapon and a non-weapon slot every 3 levels, so level 3 grants and
  // level 4 does not: the panel of 3 gives way to nothing.
  await makeCharacter(page, "fighter", "Orla", 8000, 10);
  const grants = page.getByTestId("level-up-grants");
  await roll(page).click();
  await expect(grants).toHaveCount(0); // level 2 grants nothing
  await roll(page).click();
  await expect(grants).toContainText("Level 3 grants");
  await roll(page).click();
  await expect(page.getByTestId("level-up")).toHaveCount(0);
  await expect(grants).toHaveCount(0);

  // A mage gains a non-weapon slot every 3 levels but a weapon one only every 6.
  await makeCharacter(page, "mage", "Mélia", 5000, 4);
  await roll(page).click();
  await roll(page).click();
  await expect(grants).toContainText("Level 3 grants");
  await expect(grants.getByTestId("grant-nonWeaponSlots")).toBeVisible();
  await expect(grants.getByTestId("grant-weaponSlots")).toHaveCount(0);
  await expect(grants.getByTestId("grant-skillPoints")).toHaveCount(0);
});
