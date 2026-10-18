import { test, expect, snap } from "./fixtures";
import {
  chooseLanguage,
  logEntry,
  openDashboard,
  openWorkspace,
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
