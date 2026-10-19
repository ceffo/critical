import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { combatLog, logEntry, openDashboard, recordHp } from "./helpers";

// Only a browser proves the wiring between the journal and the card: a click on a row swaps the
// card, the selection highlight, "Latest" and the filter. What each kind renders is unit-tested
// (src/journal, one test file per kind).

function card(page: Page) {
  return page.getByRole("region", { name: "Detail", exact: true });
}

async function rollSpellSave(page: Page) {
  const rolled = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/roll/save") && r.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Roll save vs Spells" }).click();
  expect((await rolled).ok()).toBe(true);
}

test("a journal entry opens read-only in the card, Latest returns, Rolls filters", async ({
  page,
}) => {
  await openDashboard(page);
  await rollSpellSave(page);
  await recordHp(page, "damage", 3, "e2e journal orc");
  await expect(logEntry(page, "e2e journal orc")).toBeVisible();
  await rollSpellSave(page);
  const saves = logEntry(page, "Save vs Spells");
  await expect(saves).toHaveCount(2);

  // By default the card is the latest result, the newest save, with its adjustment line.
  const adjust = card(page).getByTestId("adjust-check");
  await expect(adjust).toBeVisible();
  await expect(saves.first()).toHaveAttribute("data-selected", "true");
  const latest = card(page).getByRole("button", { name: "Latest" });
  await expect(latest).toHaveCount(0);

  // The older save: read-only, no correction.
  await saves.last().getByTestId("log-select").click();
  await expect(saves.last()).toHaveAttribute("data-selected", "true");
  await expect(card(page)).toHaveAttribute("data-readonly", "true");
  await expect(card(page).getByTestId("save-result")).toBeVisible();
  await expect(adjust).toHaveCount(0);
  await expect(latest).toBeVisible();

  // An entry without a roll: its text and the damage value.
  await logEntry(page, "e2e journal orc").getByTestId("log-select").click();
  const info = card(page).getByTestId("info-result");
  await expect(info).toContainText("e2e journal orc");
  await expect(info.getByTestId("info-value")).toHaveText("−3");
  await snap(page, "older-entry");

  // Latest brings back the newest save and its adjustment.
  await latest.click();
  await expect(card(page)).toHaveAttribute("data-readonly", "false");
  await expect(adjust).toBeVisible();

  // Rolls: the HP change is filtered out, the saves stay. Select the older save for the shots.
  await combatLog(page).getByRole("radio", { name: "Rolls" }).click();
  await expect(logEntry(page, "e2e journal orc")).toHaveCount(0);
  await expect(saves).toHaveCount(2);
  await saves.last().getByTestId("log-select").click();
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expect(card(page).getByTestId("save-result")).toBeVisible();
    await snap(page, `result-card-${vp.width}x${vp.height}`);
  }
});
