import { test, expect } from "./fixtures";
import {
  apiCall,
  expectNoReload,
  expectUndoOn,
  logEntry,
  LOOKUP_RAIL,
  markNoReload,
  openDashboard,
  openWorkspace,
  reloadSettled,
} from "./helpers";
import type { Page } from "@playwright/test";

// ADR 0037 step 15, what only a browser proves: the level-up box's links by keyboard focus the
// Skills activity on their editor, the rail's red pill while something is left, two levels owed one
// after the other, the box following the undo and the redo, all with no reload. The counts themselves
// are Go tests (charactersvc, levelupsvc, composition).

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

test("thief 1 to 3: the level-up box's link by keyboard focuses Skills on the points editor, under a red pill", async ({
  page,
}) => {
  await makeCharacter(page, "thief", "Vex", 2500);
  const prompt = page.getByTestId("level-up");
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 2 reached",
  );
  await expect(page.getByTestId("to-spend-tag")).toHaveCount(0);
  const rail = page.getByRole("navigation", { name: "Activities" });
  await expect(rail.getByTestId("skills-pill")).toBeVisible();
  await roll(page).click();
  const grants = page.getByTestId("level-up-grants");
  await expect(grants).toContainText("Level 2 grants");

  // Keyboard only: Tab to a link in the box, Enter focuses the Skills activity on its editor.
  const link = grants.getByTestId("grant-open-skillPoints");
  await link.focus();
  await page.keyboard.press("Enter");
  await expect(rail.getByRole("button", { name: /^Skills/ })).toHaveAttribute(
    "aria-current",
    "page",
  );
  const block = page.getByTestId("skills-points-thief");
  await expect(block).toBeFocused();
  await expect(block.getByTestId("thief-points-editor")).toBeVisible();
  await expect(block.getByTestId("skill-points-left")).toContainText("90");
});

async function expectPointsLeft(page: Page, left: number) {
  await openWorkspace(page, "Skills");
  await expect(
    page.getByTestId("skills-points-thief").getByTestId("skill-points-left"),
  ).toHaveText(`Spent 0, left ${left}`);
}

test("thief 1 to 3: roll 2 then 3, the panel switches; undo takes it, redo brings it back", async ({
  page,
}) => {
  await makeCharacter(page, "thief", "Vex", 2500);
  const prompt = page.getByTestId("level-up");
  const grants = page.getByTestId("level-up-grants");
  await expectPointsLeft(page, 60);
  await openWorkspace(page, "Act");
  await roll(page).click();
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 3 reached",
  );
  await expect(grants).toContainText("Level 2 grants");
  await expectPointsLeft(page, 90);
  await openWorkspace(page, "Act");
  await roll(page).click();
  await expect(prompt).toHaveCount(0);
  await expect(grants).toContainText("Level 3 grants");
  // The points editor counts what both levels left.
  await expectPointsLeft(page, 120);
  await openWorkspace(page, "Act");

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

// ADR 0030 step 20.8, what only a browser proves: the card's free spell link opens the Spells
// activity's lookup on the wizard spells of that spell level, Ajouter there claims the free spell
// (the journal names it), and the reminder drops it with no reload; a change of the level filter
// drops the claim (Ajouter sends no grantLevel); the claim's undo and redo move the reminder. The rule (which level opens
// which spell level, the Intelligence cap, what is owed, the refusals) is Go's (spellbook,
// charactersvc, learnsvc, composition).
test("mage 1 to 3: the free spell's link opens the lookup on 2nd-level spells, Ajouter claims it", async ({
  page,
}) => {
  await makeCharacter(page, "mage", "Mélia", 5000, 4);
  await roll(page).click();
  await roll(page).click();
  const grants = page.getByTestId("level-up-grants");
  await expect(grants).toContainText("Level 3 grants");
  await expect(grants.getByTestId("grant-freeSpell")).toHaveText(
    /\+1 free level 2 spell/,
  );
  await markNoReload(page);
  const learn = page.getByTestId("learn-spell");
  const rows = page.locator("[id^=lookup-row-]");

  // From the box, then a change of the level filter: the claim is dropped, Ajouter adds a plain
  // spell (no grantLevel) and the free spell stays owed.
  await grants.getByTestId("grant-open-freeSpell-3-2").click();
  await page.locator("#lookup-row-mirror_image").click();
  await expect(learn.getByTestId("free-spell-claim")).toBeVisible();
  await page
    .getByRole("group", { name: "Spell levels" })
    .getByRole("button", { name: "Level 1", exact: true })
    .click();
  await expect(learn.getByTestId("free-spell-claim")).toHaveCount(0);
  await page.locator("#lookup-row-sleep").click();
  await expect(learn.getByTestId("free-spell-claim")).toHaveCount(0);
  const plain = page.waitForRequest("**/api/character/spellbook/add");
  await learn.getByTestId("add-spell").click();
  expect((await plain).postDataJSON()).toEqual({ spellId: "sleep" });
  await expect(learn).toContainText("In the spellbook");

  await openWorkspace(page, "Act");
  await grants.getByTestId("grant-open-freeSpell-3-2").click();
  await expect(
    page
      .getByRole("navigation", { name: "Activities" })
      .getByRole("button", { name: LOOKUP_RAIL }),
  ).toHaveAttribute("aria-current", "page");
  await expect(rows.first()).toBeVisible();
  // Only 2nd-level spells are listed.
  await expect
    .poll(
      async () =>
        new Set(
          await rows.evaluateAll((els) => els.map((e) => e.dataset.level)),
        ),
    )
    .toEqual(new Set(["2"]));
  await page.locator("#lookup-row-mirror_image").click();
  await expect(learn.getByTestId("free-spell-claim")).toHaveText(
    "Level 3's free spell",
  );
  await learn.getByTestId("add-spell").click();
  await expect(learn).toContainText("In the spellbook");
  await expect(learn.getByTestId("free-spell-claim")).toHaveCount(0);
  await openWorkspace(page, "Act");
  await expect(
    logEntry(page, "Level 3's free spell: Mirror Image (no roll)"),
  ).toBeVisible();

  // Undo the claim and redo it: the entry leaves the undo stack and comes back. No reload.
  await expectUndoOn(page, "free spell");
  await page.getByTestId("undo").click();
  await expectUndoOn(page, "free spell", true);
  await page.getByTestId("redo").click();
  await expect(
    logEntry(page, "Level 3's free spell: Mirror Image (no roll)"),
  ).toBeVisible();
  await expectNoReload(page);
});
