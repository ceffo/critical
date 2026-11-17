import { test, expect } from "./fixtures";
import {
  combatLog,
  expectUndoOn,
  openDashboard,
  openSpend,
  openWorkspace,
} from "./helpers";

// ADR 0037 step 15 (amended 2026-11-15), what only a browser proves: the Skills activity is open at
// any time. Spend the pool's slots, then, with none left, swap a
// proficiency (one removed, another added), save, and the journal's Undo restores the picks in the
// editor. The pools, the costs and the refusals are Go tests; the pill's cases and the panel's
// blocks are component tests (skillsPending, Rail, SkillsPanel).
test("skills activity: with nothing left to spend, swap a proficiency, save, undo", async ({
  page,
}) => {
  await openDashboard(page);
  // Something is left (the bard's slots and points): the rail's pill shows.
  await expect(page.getByTestId("skills-pill")).toBeVisible();

  // The bard's four free proficiencies spend nothing (ADR 0037 step 22): six slots left. Spend
  // them on Tumbling (an extra slot each) and save.
  const editor = await openSpend(page, "nonWeapon");
  const spent = editor.getByTestId("proficiency-pool-spent");
  await expect(spent).toHaveText("Spent 4, left 6");
  await expect(
    editor
      .getByTestId("proficiency-pick-singing")
      .getByTestId("proficiency-pick-terms"),
  ).toContainText("free (class)");
  const tumbling = editor.getByTestId("proficiency-pick-tumbling");
  for (let i = 0; i < 6; i++)
    await tumbling.getByTestId("proficiency-more").click();
  await expect(spent).toHaveText("Spent 10, left 0");
  await editor.getByTestId("proficiency-save").click();
  // A save returns to the reading, on the stored picks.
  await expect(page.getByTestId("skills-nonWeapon-budget")).toContainText(
    "10 / 10 slots",
  );
  await expect(page.getByTestId("skill-read-tumbling")).toBeVisible();
  await expectUndoOn(page, "Tumbling");

  // With the pool spent, the editor still opens: remove Tumbling and add Agriculture, then save.
  await page.getByTestId("skills-modify-skills-nonWeapon").click();
  await tumbling.getByTestId("proficiency-remove").click();
  await editor.getByTestId("proficiency-search").fill("agri");
  await editor.getByTestId("proficiency-add-agriculture").click();
  await expect(
    editor.getByTestId("proficiency-pick-agriculture"),
  ).toBeVisible();
  await editor.getByTestId("proficiency-save").click();
  await expectUndoOn(page, "Agriculture");
  await openWorkspace(page, "Act");
  await expect(combatLog(page)).toContainText("Agriculture");

  // Undo takes the swap back: Tumbling is a pick again in the editor, Agriculture is not.
  await page.getByTestId("undo").click();
  await openWorkspace(page, "Skills");
  await page.getByTestId("skills-modify-skills-nonWeapon").click();
  await expect(
    page
      .getByTestId("proficiency-editor")
      .getByTestId("proficiency-pick-tumbling"),
  ).toBeVisible();
  await expect(
    page
      .getByTestId("proficiency-editor")
      .getByTestId("proficiency-pick-agriculture"),
  ).toHaveCount(0);
});
