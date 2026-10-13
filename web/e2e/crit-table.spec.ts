import type { Locator, Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { logEntry, openDashboard } from "./helpers";

// Dice are random on the real stack: the tests read the rolled crit d100 from the result card and
// check the multiplier and range against the table they typed in.

const TABLE = [
  { from: 1, to: 50, mult: 2 },
  { from: 51, to: 90, mult: 3 },
  { from: 91, to: 100, mult: 4 },
];

async function openEditor(page: Page): Promise<Locator> {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Attack" }).click();
  const editor = page.getByTestId("crit-table");
  await editor.getByText("Crit table (house rule)").click();
  await expect(editor.getByRole("group", { name: "Range 1" })).toBeVisible();
  return editor;
}

function range(editor: Locator, n: number) {
  const row = editor.getByRole("group", { name: `Range ${n}` });
  return {
    from: row.getByLabel("From"),
    to: row.getByLabel("To"),
    mult: row.getByLabel("×"),
  };
}

test("an invalid table is refused with the bad row named and the stored table stays", async ({
  page,
}) => {
  const editor = await openEditor(page);
  await expect(editor).toContainText("Placeholder");
  await expect(range(editor, 1).to).toHaveValue("100");

  // 1-40 then 50-100: 41-49 is a gap.
  await range(editor, 1).to.fill("40");
  await editor.getByRole("button", { name: "Add range" }).click();
  await range(editor, 2).from.fill("50");
  await range(editor, 2).to.fill("100");
  await editor.getByRole("button", { name: "Save table" }).click();
  await expect(editor.getByRole("alert")).toContainText(
    "gap before row 2 (50-100)",
  );
  await snap(page, "invalid-table");

  await page.reload();
  const again = await openEditor(page);
  await expect(again).toContainText("Placeholder");
  await expect(range(again, 1).to).toHaveValue("100");
  await expect(again.getByRole("group", { name: "Range 2" })).toHaveCount(0);
});

test("the table saved in the editor is used by the next crit", async ({
  page,
}) => {
  const editor = await openEditor(page);

  // Before editing, a crit is the x2 placeholder.
  await page.getByLabel("Target AC").fill("1000"); // so low a roll is needed that every attack crits
  await page.getByRole("button", { name: "Roll Attack" }).click();
  const card = page
    .getByRole("region", { name: "Last Result" })
    .getByTestId("attack-result");
  await expect(card).toContainText("Critical ×2 (placeholder)");
  await snap(page, "placeholder-crit");

  await range(editor, 1).to.fill("50");
  await editor.getByRole("button", { name: "Add range" }).click();
  await expect(range(editor, 2).from).toHaveValue("51"); // starts where the last range stopped
  await range(editor, 2).to.fill("90");
  await range(editor, 2).mult.fill("3");
  await editor.getByRole("button", { name: "Add range" }).click();
  await range(editor, 3).to.fill("100");
  await range(editor, 3).mult.fill("4");
  await editor.getByRole("button", { name: "Save table" }).click();
  await expect(editor).toContainText("Saved");
  await expect(editor).not.toContainText("Placeholder");
  await snap(page, "table-saved");

  // The saved table survives a reload.
  await page.reload();
  const again = await openEditor(page);
  await expect(range(again, 3).from).toHaveValue("91");
  await expect(range(again, 3).mult).toHaveValue("4");

  await page.getByLabel("Target AC").fill("1000");
  await page.getByRole("button", { name: "Roll Attack" }).click();
  await expect(card).toBeVisible();
  const d100 = Number(
    await card
      .locator("dt", { hasText: "Crit d100" })
      .locator("xpath=following-sibling::dd")
      .textContent(),
  );
  const row = TABLE.find((r) => d100 >= r.from && d100 <= r.to);
  if (!row) throw new Error(`d100 ${d100} is in no range`);
  await expect(card).toContainText(
    `Critical ×${row.mult} (d100 in ${row.from}–${row.to})`,
  );
  await expect(card.getByTestId("damage-terms")).toContainText(
    `Critical hit (${row.from}-${row.to})`,
  );
  await snap(page, "table-crit");

  // The earlier placeholder crit was resolved before the edit and keeps its x2.
  await expect(logEntry(page, "critical ×2").first()).toBeVisible(); // first: the new crit may also be ×2
});
