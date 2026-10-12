import { test, expect, snap } from "./fixtures";
import {
  currentRound,
  expectNoReload,
  logEntry,
  markNoReload,
  openDashboard,
  recordHp,
} from "./helpers";

test("Next Round advances the counter and starts a new log group", async ({
  page,
}) => {
  await openDashboard(page);
  const r0 = await currentRound(page);
  await markNoReload(page);

  await recordHp(page, "damage", 1, "e2e first round hit");
  await expect(logEntry(page, "e2e first round hit")).toBeVisible();

  await page.getByRole("button", { name: "Next Round" }).click();
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);

  // The new round is the current group and is empty; the old entry stays in its round.
  const newGroup = page.getByRole("region", { name: `Round ${r0 + 1}` });
  await expect(newGroup).toContainText("current");
  await expect(newGroup).toContainText("Nothing recorded this round yet.");
  const oldGroup = page.getByRole("region", { name: `Round ${r0}` });
  await expect(oldGroup.getByRole("listitem")).toContainText("e2e first round hit");

  await recordHp(page, "damage", 1, "e2e second round hit");
  await expect(newGroup.getByRole("listitem")).toContainText("e2e second round hit");

  await expectNoReload(page);
  await snap(page, "next-round");
});
