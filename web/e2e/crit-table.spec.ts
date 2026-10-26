import type { Locator, Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  forceAttackAc,
  logEntry,
  openDashboard,
  openSettings,
} from "./helpers";

// Dice are random on the real stack: the tests read the rolled crit d100 from the result card and
// check the multiplier and range against the table they typed in.

const TABLE = [
  { from: 1, to: 50, mult: 2 },
  { from: 51, to: 90, mult: 3 },
  { from: 91, to: 100, mult: 4 },
];

// The crit table lives on the Ruleset screen, reached from the cog menu (an admin's).
async function openEditor(page: Page): Promise<Locator> {
  await openDashboard(page);
  const settings = await openSettings(page);
  await settings.getByRole("link", { name: /^Open the ruleset/ }).click();
  await expect(page).toHaveURL(/\/ruleset$/);
  const editor = page.getByTestId("crit-table");
  await expect(editor.getByRole("group", { name: "Range 1" })).toBeVisible();
  return editor;
}

async function backToWeapons(page: Page) {
  await page.getByRole("link", { name: /^Back/ }).click();
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  await page.getByRole("tab", { name: "Weapons" }).click();
}

function range(editor: Locator, n: number) {
  const row = editor.getByRole("group", { name: `Range ${n}` });
  return {
    from: row.getByLabel("From"),
    to: row.getByLabel("To"),
    mult: row.getByLabel("×"),
  };
}

test("the table saved in the editor is used by the next crit", async ({
  page,
}) => {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Weapons" }).click();
  // The Attack panel no longer carries the editor.
  await expect(page.getByTestId("crit-table")).toHaveCount(0);

  // Before editing, a crit is the x2 placeholder.
  await forceAttackAc(page, 1000); // so low a roll is needed that every attack crits
  await page.getByRole("button", { name: "Attack", exact: true }).click();
  const card = page
    .getByRole("region", { name: "Detail", exact: true })
    .getByTestId("attack-result");
  await expect(card.getByTestId("crit-badge")).toHaveText(
    /^Crit check \d+ · multiplier \d+ → ×2 \(placeholder\)$/,
  );
  await snap(page, "placeholder-crit");

  const settings = await openSettings(page);
  await settings.getByRole("link", { name: /^Open the ruleset/ }).click();
  const editor = page.getByTestId("crit-table");
  await expect(range(editor, 1).to).toBeVisible();
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
  await backToWeapons(page);

  // ADR 0001 step 4: the row is the one of the second (multiplier) d100, not the crit check. The
  // seeded dice (CRITICAL_DICE_SEED, fixtures.ts) put this test's second attack's two d100s in
  // different rows, so the test tells them apart; the assertion fails loudly if the sequence changes.
  const fact = async (label: string) =>
    Number(
      await card
        .locator("dt", { hasText: label })
        .locator("xpath=following-sibling::dd")
        .textContent(),
    );
  const rowOf = (d: number) => TABLE.findIndex((r) => d >= r.from && d <= r.to);
  await page.getByRole("button", { name: "Attack", exact: true }).click();
  await expect
    .poll(async () => await logEntry(page, "vs AC 1000").count())
    .toBe(2);
  const check = await fact("Crit check d100");
  const d100 = await fact("Multiplier d100");
  expect(
    rowOf(check),
    "the seed's second attack no longer puts the two d100s in different rows",
  ).not.toBe(rowOf(d100));
  const row = TABLE.find((r) => d100 >= r.from && d100 <= r.to);
  if (!row) throw new Error(`d100 ${d100} is in no range`);
  await expect(card.getByTestId("crit-badge")).toHaveText(
    `Crit check ${check} · multiplier ${d100} → ×${row.mult} (${row.from}–${row.to})`,
  );
  await expect(card.getByTestId("damage-terms")).toContainText(
    `Critical hit (${row.from}–${row.to})`,
  );
  await snap(page, "table-crit");

  // The earlier placeholder crit was resolved before the edit and keeps its x2.
  await expect(logEntry(page, "→ ×2)").first()).toBeVisible(); // first: the new crit may also be ×2
});

test("a decimal multiplier typed with a comma is saved and the crit shows the floored total", async ({
  page,
}) => {
  const editor = await openEditor(page);

  // 1-100 at x1.5, typed the French way; below 1 and text are refused with the row named.
  await range(editor, 1).mult.fill("0,99");
  await editor.getByRole("button", { name: "Save table" }).click();
  await expect(editor.getByRole("alert")).toContainText(
    "row 1 (1-100) has multiplier 0.99",
  );
  await range(editor, 1).mult.fill("abc");
  await editor.getByRole("button", { name: "Save table" }).click();
  await expect(editor.getByRole("alert")).toContainText("row 1 (1-100)");

  await range(editor, 1).mult.fill("1,5");
  await editor.getByRole("button", { name: "Save table" }).click();
  await expect(editor).toContainText("Saved");
  await expect(range(editor, 1).mult).toHaveValue("1.5");

  await page.reload();
  const again = await openEditor(page);
  await expect(range(again, 1).mult).toHaveValue("1.5");
  await backToWeapons(page);

  await forceAttackAc(page, 1000);
  await page.getByRole("button", { name: "Attack", exact: true }).click();
  const card = page
    .getByRole("region", { name: "Detail", exact: true })
    .getByTestId("attack-result");
  await expect(card.getByTestId("crit-badge")).toHaveText(
    /^Crit check \d+ · multiplier \d+ → ×1\.5 \(1–100\)$/,
  );

  // The breakdown shows the multiplier as entered, and the unrounded → floored step when the
  // damage is not whole; the big total is the floored number.
  const total = Number(await card.getByTestId("total-damage").textContent());
  const breakdown =
    (await card
      .getByText(/ × 1\.5 \+ /)
      .first()
      .textContent()) ?? "";
  const final = Number(breakdown.split(/[=→]/).pop()?.trim());
  expect(final).toBe(total);
  if (breakdown.includes("→")) {
    const unrounded = Number(breakdown.split("=").pop()?.split("→")[0].trim());
    expect(Math.floor(unrounded)).toBe(total);
  }
  await snap(page, "decimal-crit");
});

test("French shows the decimal multiplier with a comma on the card and in the log", async ({
  page,
  stack,
}) => {
  const res = await fetch(`${stack.apiUrl}/api/ruleset/crit-table`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ rows: [{ from: 1, to: 100, multiplier: 1.5 }] }),
  });
  expect(res.ok).toBe(true);
  await page.addInitScript(() =>
    localStorage.setItem(
      "critical.settings.v1",
      JSON.stringify({ lang: "fr", mode: "system", amb: "arcane" }),
    ),
  );
  await openDashboard(page);
  await forceAttackAc(page, 1000);
  await page.getByRole("tab", { name: "Armes" }).click();
  await page.getByRole("button", { name: "Attaquer", exact: true }).click();
  const card = page
    .getByRole("region", { name: "Détail", exact: true })
    .getByTestId("attack-result");
  await expect(card.getByTestId("crit-badge")).toHaveText(
    /^Jet critique \d+ · multiplicateur \d+ → ×1,5 \(1–100\)$/,
  );
  await expect(card.getByTestId("damage-terms")).toContainText("×1,5");
  await expect(
    page
      .getByRole("region", { name: "Journal", exact: true })
      .getByRole("listitem")
      .filter({ hasText: /critique \(jet \d+ · multiplicateur \d+ → ×1,5\)/ })
      .first(),
  ).toBeVisible();
  await snap(page, "decimal-crit-fr");
});
