import { test, expect, snap } from "./fixtures";
import {
  chooseLanguage,
  logEntry,
  openDashboard,
  openWorkspace,
  recordHp,
} from "./helpers";
import type { Page } from "@playwright/test";

/** The XP total shown in the progress card, as a number (en formatting: "1,293,418"). */
async function xpTotal(page: Page): Promise<number> {
  const text = (await page.getByTestId("xp-total").textContent()) ?? "";
  const digits = text.replace(/[^\d]/g, "");
  if (!digits) throw new Error(`unexpected XP total: ${JSON.stringify(text)}`);
  return Number(digits);
}

// Layout and screenshots per viewport are in layout.spec (ADR 0022 step 2.5).
// What only a browser proves: the character card's level link focuses the activity, the add form's
// preview, Enter submits, in-row edit (Enter saves) and delete re-render the server's totals, and
// the global undo (Ctrl+Z, from the Experience activity) pops each write and the ledger re-renders;
// the journal (Act activity) shows them undone. The rules (rounding, eligibility,
// Table 25, what each undo restores) and CRUD edge cases are Go tests (xpsvc, xpdal, composition).
test("experience: add with the bonus, edit, delete, then undo each", async ({
  page,
}) => {
  await openDashboard(page);
  const moi = page.getByRole("complementary", { name: "Character" });
  await moi.getByRole("button", { name: /^Level \d+$/ }).click();
  const ws = page.getByRole("region", { name: "Experience", exact: true });
  await expect(ws).toBeVisible();
  await expect(
    page
      .getByRole("navigation", { name: "Activities" })
      .getByRole("button", { name: "Experience" }),
  ).toHaveAttribute("aria-current", "page");
  await expect(ws.getByTestId("xp-total")).toHaveText(/\d/);
  const before = await xpTotal(page);

  // Add 1,205 XP with the +10% (default on for the eligible Bard): the server previews 1,326.
  const add = ws.getByRole("region", { name: "Add XP" });
  await expect(add.getByRole("checkbox")).toBeChecked();
  await add.getByLabel("Amount").fill("1205");
  await add.getByLabel("Reason").fill("Troll bridge");
  await expect(add.getByRole("button", { name: "Add 1,326" })).toBeVisible();
  await add.getByLabel("Reason").press("Enter");
  await expect.poll(() => xpTotal(page)).toBe(before + 1326);
  const ledger = ws.getByRole("region", { name: "Ledger" });
  const row = ledger.getByRole("listitem", { name: "Troll bridge" });
  await expect(row.getByTestId("xp-row-total")).toHaveText("1,326");
  await expect(add.getByLabel("Amount")).toHaveValue("");

  // Edit in the row: 1,000 without the bonus, Enter saves.
  await row.getByRole("button", { name: "Edit Troll bridge" }).click();
  const editing = ledger.getByRole("listitem", { name: "Editing an entry" });
  await editing.getByLabel("Amount").fill("1000");
  await editing.getByRole("checkbox").uncheck();
  await expect(editing.getByTestId("xp-edit-total")).toHaveText("1,000");
  await editing.getByLabel("Amount").press("Enter");
  await expect.poll(() => xpTotal(page)).toBe(before + 1000);
  await expect(row.getByTestId("xp-row-total")).toHaveText("1,000");

  // Delete asks first: Escape keeps the entry, Confirm removes it.
  await row.getByRole("button", { name: "Delete Troll bridge" }).click();
  await expect(
    row.getByRole("button", { name: "Confirm deleting Troll bridge" }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(row.getByTestId("xp-row-total")).toHaveText("1,000");
  expect(await xpTotal(page)).toBe(before + 1000);
  await row.getByRole("button", { name: "Delete Troll bridge" }).click();
  await row
    .getByRole("button", { name: "Confirm deleting Troll bridge" })
    .click();
  await expect.poll(() => xpTotal(page)).toBe(before);
  await expect(row).toHaveCount(0);

  // Each write is a transaction (ADR 0015 step 4): the global undo pops the delete, the edit, then
  // the add, and the ledger follows.
  const undo = async () => {
    await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
    await page.keyboard.press("ControlOrMeta+z");
  };
  await undo();
  await expect(row.getByTestId("xp-row-total")).toHaveText("1,000");
  await expect.poll(() => xpTotal(page)).toBe(before + 1000);
  await undo();
  await expect(row.getByTestId("xp-row-total")).toHaveText("1,326");
  await undo();
  await expect.poll(() => xpTotal(page)).toBe(before);
  await expect(row).toHaveCount(0);

  await openWorkspace(page, "Act");
  for (const text of [
    "XP removed: Troll bridge (1,000 XP)",
    "XP edited: Troll bridge, 1,326 → 1,000 XP",
    "XP +1,326: Troll bridge",
  ])
    await expect(logEntry(page, text)).toHaveAttribute("data-status", "voided");
  await snap(page, "experience-journal");
});

/** The max HP as shown in the Moi column ("current / max"). */
async function maxHp(page: Page): Promise<number> {
  const text = (await page.getByTestId("hp-value").textContent()) ?? "";
  const m = /\/\s*(\d+)/.exec(text);
  if (!m) throw new Error(`unexpected HP text: ${JSON.stringify(text)}`);
  return Number(m[1]);
}

// ADR 0030 step 7, in French (ADR 0027 step 2). What only a browser proves: an XP add that crosses
// a level makes the character card prompt for the hit points without a reload (the refresh wiring),
// the prompt's button raises the max HP the HP panel shows and leaves, the journal words the entry,
// and the journal's Undo brings the prompt back. The rules (the die, Constitution, the fixed gain,
// two levels at once, the stale level) are Go tests (levelupsvc, charactersvc, composition); the
// prompt's wording is a unit test (LevelUpPrompt.test).
test("level up: an XP add crossing a level prompts, the prompt adds the hit points, Undo brings it back", async ({
  page,
}) => {
  await openDashboard(page);
  await chooseLanguage(page, "fr");
  const moi = page.getByRole("complementary", { name: "Personnage" });
  await expect(page.getByTestId("level-up")).toHaveCount(0);
  const before = await maxHp(page);

  // 1,293,418 + 30,000 XP (no bonus) reaches Table 25's level 16 (1,320,000).
  await moi.getByRole("button", { name: /^Niveau 15$/ }).click();
  const add = page.getByRole("region", { name: "Ajouter des XP" });
  await add.getByRole("checkbox").uncheck();
  await add.getByLabel("Montant").fill("30000");
  await add.getByLabel("Motif").fill("Le dragon");
  await add.getByLabel("Motif").press("Enter");

  // Level 16 is above the bard's name level (10): a fixed +2, no die (step 7.2).
  const prompt = page.getByTestId("level-up");
  await expect(prompt).toBeVisible();
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Niveau 16 atteint",
  );
  await expect(prompt).toContainText(
    "Au-delà du niveau de nom : +2 PV max, sans dé",
  );
  await snap(page, "level-up-prompt-fr");

  await prompt
    .getByRole("button", { name: "Ajouter +2 PV max pour le niveau 16" })
    .click();
  await expect(prompt).toHaveCount(0);
  await expect.poll(() => maxHp(page)).toBe(before + 2);
  await page
    .getByRole("navigation", { name: "Activités" })
    .getByRole("button", { name: "Agir", exact: true })
    .click();
  const entry = logEntry(
    page,
    `Niveau 16 : PV max +2, gain fixe (${before} → ${before + 2})`,
  );
  await expect(entry).toBeVisible();

  // The journal's Undo pops the roll: the max HP and the prompt are back.
  await page.getByTestId("undo").click();
  await expect(entry).toHaveAttribute("data-status", "voided");
  await expect(page.getByTestId("level-up")).toBeVisible();
  await expect.poll(() => maxHp(page)).toBe(before);
});

/** "current / max" from the HP panel. */
async function hp(page: Page): Promise<[number, number]> {
  const text = (await page.getByTestId("hp-value").textContent()) ?? "";
  const m = /^\s*(-?\d+)\s*\/\s*(\d+)/.exec(text);
  if (!m) throw new Error(`unexpected HP text: ${JSON.stringify(text)}`);
  return [Number(m[1]), Number(m[2])];
}

// ADR 0030 step 7, the hit die path. What only a browser proves: the card's Roll, the journal's
// Undo and Redo move the max HP and the current HP the HP panel shows together and bring the prompt
// back and away; a heal after a level-up fills to the new max; the manual entry's Confirm stays off
// for a die the group can't roll and the journal words the entered die; an XP delete under the
// rolled level turns the prompt into the step 7.4 warning, in French. The math (the die, CON, the
// heal cap, the snapshot undo restores) is Go tests (levelupsvc, composition, actionlogsvc).
test("level up: roll a d6, undo and redo, heal to the new max, enter a die, then an XP delete warns", async ({
  page,
}) => {
  await openDashboard(page);
  const call = (method: string, url: string, body: unknown) =>
    page.evaluate(
      async ([m, u, b]) =>
        (
          await fetch(u as string, {
            method: m as string,
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(b),
          })
        ).ok,
      [method, url, body] as const,
    );
  // A level 1 bard with 6 max HP (CON 12: no adjustment), then 2,500 XP: levels 2 and 3 owe a d6.
  expect(
    await call("POST", "/api/characters", {
      name: "Lyra",
      class: "bard",
      raceCode: "half_elf",
      details: { alignment: "neutral_good", sex: "male" },
      str: 10,
      dex: 15,
      con: 12,
      int: 14,
      wis: 9,
      cha: 16,
      hpMax: 6,
      xp: 0,
      skillArmor: "leather",
    }),
  ).toBe(true);
  expect(
    await call("POST", "/api/xp", {
      base: 2500,
      bonus: false,
      reason: "Goblins",
    }),
  ).toBe(true);
  await page.reload();
  const prompt = page.getByTestId("level-up");
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 2 reached",
  );

  // The card shows the realized level: the XP gives 3, nothing is taken yet.
  const cardLevel = page
    .getByRole("complementary", { name: "Character" })
    .getByRole("button", { name: /^Level \d+$/ });
  await expect(cardLevel).toHaveText("Level 1");

  // 3 damage first: 3 / 6.
  await recordHp(page, "damage", 3, "e2e rat");
  await expect.poll(() => hp(page)).toEqual([3, 6]);

  // Roll: both rise by the gain (1 to 6), the damage stays; level 3 is next.
  await prompt
    .getByRole("button", { name: "Roll the hit die of level 2 (d6)" })
    .click();
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 3 reached",
  );
  await expect(cardLevel).toHaveText("Level 2");
  const [, rolledMax] = await hp(page);
  const gain = rolledMax - 6;
  expect(gain).toBeGreaterThanOrEqual(1);
  expect(gain).toBeLessThanOrEqual(6);
  await expect.poll(() => hp(page)).toEqual([3 + gain, 6 + gain]);
  const rolled = logEntry(
    page,
    `Level 2: d6 ${gain}, max HP +${gain} (6 → ${6 + gain})`,
  );
  await expect(rolled).toBeVisible();

  // Undo: back to 3 / 6, level 2 owed again. Redo: the same roll is back, level 3 owed.
  await page.getByTestId("undo").click();
  await expect.poll(() => hp(page)).toEqual([3, 6]);
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 2 reached",
  );
  await expect(cardLevel).toHaveText("Level 1");
  await page.getByTestId("redo").click();
  await expect.poll(() => hp(page)).toEqual([3 + gain, 6 + gain]);
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 3 reached",
  );

  // Heal far past the max: full is the new max, never above. Undo the heal and the roll: 3 / 6.
  await recordHp(page, "heal", 50, "e2e potion");
  await expect.poll(() => hp(page)).toEqual([6 + gain, 6 + gain]);
  await page.getByTestId("undo").click();
  await expect.poll(() => hp(page)).toEqual([3 + gain, 6 + gain]);
  await page.getByTestId("undo").click();
  await expect.poll(() => hp(page)).toEqual([3, 6]);

  // Enter the die rolled at the table: 0, 7 and empty keep Confirm off; 4 is taken as entered.
  await prompt
    .getByRole("button", { name: "Enter the die rolled at the table" })
    .click();
  const die = prompt.getByLabel("Die rolled at the table, 1 to 6");
  const confirm = prompt.getByRole("button", { name: "Confirm" });
  await expect(confirm).toBeDisabled();
  for (const bad of ["0", "7"]) {
    await die.fill(bad);
    await expect(confirm).toBeDisabled();
  }
  await die.fill("4");
  await confirm.click();
  await expect(prompt.getByTestId("level-up-title")).toHaveText(
    "Level 3 reached",
  );
  await expect.poll(() => hp(page)).toEqual([7, 10]);
  await expect(
    logEntry(page, "Level 2: d6 4, max HP +4 (6 → 10), die entered"),
  ).toBeVisible();

  // Delete the XP: the XP gives level 1, the max HP is for level 2. Nothing is removed, the card
  // warns in French and prompts for nothing.
  await openWorkspace(page, "Experience");
  const row = page
    .getByRole("region", { name: "Ledger" })
    .getByRole("listitem", { name: "Goblins" });
  await row.getByRole("button", { name: "Delete Goblins" }).click();
  await row.getByRole("button", { name: "Confirm deleting Goblins" }).click();
  await expect(row).toHaveCount(0);
  await chooseLanguage(page, "fr");
  await expect(page.getByTestId("level-up")).toHaveCount(0);
  await expect(page.getByTestId("level-up-warning")).toContainText(
    "Niveau 2 acquis, l'XP donne le niveau 1",
  );
  await expect.poll(() => hp(page)).toEqual([7, 10]);
  await snap(page, "level-up-warning-fr");
});
