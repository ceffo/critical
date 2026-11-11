import { test, expect, snap } from "./fixtures";
import { combatLog, openDashboard, openSpend } from "./helpers";

const VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

// What only a browser proves: the name opens the popover, Edit switches it to a form whose first
// field is the name (Rename lives in the details too, ADR 0034 step 7.2), the server's
// field error shows under its field (the rule itself is a Go test in detailssvc), Save shows the
// stored values, and they survive a reload. The card shows the languages read-only: they are
// picked in the Skills tab's editor (ADR 0037 step 18), from the catalogue with the race's own
// language ticked and fixed; a save is one journal entry the card then shows, after a reload too. The
// slot budget, the units, the identity picks and the rolls are Go and unit tests.
test("character details: view, edit with a field error, save, reload", async ({
  page,
}) => {
  // The creation options are read once per page load: the card, its edit form and the Skills tab's
  // editor share the one read, the journal's language names (read at start) included.
  let optionReads = 0;
  page.on("request", (r) => {
    if (new URL(r.url()).pathname === "/api/characters/options") optionReads++;
  });
  await openDashboard(page);
  await page.getByTestId("details-trigger").click();
  const pop = page.getByTestId("details-popover");
  await expect(pop).toBeVisible();
  await expect(pop.getByText("Bard", { exact: true })).toBeVisible();
  // The PHB race the rules read (ADR 0030 step 1); there is no free-text race detail any more.
  await expect(pop.getByText("Half-elf", { exact: true })).toBeVisible();
  await expect(pop.getByText("Not set").first()).toBeVisible();

  for (const vp of VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(pop).toBeVisible();
    await snap(page, `details-view-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });

  await pop.getByRole("button", { name: "Edit" }).click();
  await pop.getByLabel("Hair", { exact: true }).fill("Chestnut");
  await pop.getByLabel("Age", { exact: true }).fill("10001");
  await pop.getByRole("button", { name: "Save" }).click();
  await expect(pop.getByRole("alert")).toHaveText(
    "Enter a whole age between 1 and 10,000.",
  );
  await expect(pop.getByLabel("Hair", { exact: true })).toHaveValue("Chestnut");
  await snap(page, "details-error-1440x900");

  await pop.getByLabel("Age", { exact: true }).fill("112");
  await pop.getByLabel("Eyes", { exact: true }).fill("green");
  await expect(pop.getByLabel("Name", { exact: true })).toHaveValue(
    "Test Bard",
  );
  await pop.getByLabel("Name", { exact: true }).fill("Test Bard the Bold");
  // ADR 0037 step 18: no language to pick on the card any more.
  await expect(pop.getByTestId("language-picker")).toHaveCount(0);
  await pop.getByRole("button", { name: "Save" }).click();
  await expect(pop.getByRole("button", { name: "Edit" })).toBeVisible();
  await expect(pop.getByText("Chestnut")).toBeVisible();
  await expect(pop.getByText("112")).toBeVisible();
  await expect(page.getByTestId("details-trigger")).toHaveText(
    "Test Bard the Bold",
  );
  await expect(pop.getByText("Common", { exact: true })).toBeVisible();
  await page.keyboard.press("Escape");

  // The languages in the Skills tab's editor.
  await openSpend(page, "nonWeapon");
  const editor = page.getByTestId("proficiency-editor");
  const pool = editor.getByTestId("proficiency-languages");
  const common = pool.getByRole("checkbox", { name: "Common", exact: true });
  await expect(common).toBeChecked();
  await expect(common).toBeDisabled();
  for (const name of ["Elvish", "Gnomish"])
    await pool.getByRole("checkbox", { name, exact: true }).check();
  await editor.getByTestId("proficiency-save").click();
  await expect(editor).toHaveCount(0);
  await expect(combatLog(page)).toContainText("Languages: + Elvish, + Gnomish");
  await page.getByTestId("details-trigger").click();
  await expect(pop.getByText("Common, Elvish, Gnomish")).toBeVisible();
  expect(optionReads).toBe(1);
  await expect(pop.getByTestId("details-language-slots")).toHaveText(
    "2 proficiency slots",
  );

  await page.reload();
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  await expect(page.getByTestId("details-trigger")).toHaveText(
    "Test Bard the Bold",
  );
  await page.getByTestId("details-trigger").click();
  await expect(pop.getByText("green")).toBeVisible();
  await expect(pop.getByText("Common, Elvish, Gnomish")).toBeVisible();
  await expect(pop.getByTestId("details-language-slots")).toBeVisible();
  await page.keyboard.press("Escape");
});
