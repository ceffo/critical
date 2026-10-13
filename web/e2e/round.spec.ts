import { test, expect, snap } from "./fixtures";
import {
  currentRound,
  expectNoReload,
  logEntry,
  markNoReload,
  addBuff,
  buffBadge,
  openDashboard,
  recordHp,
  voidEntry,
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

  // The new round is the current group and is empty; the old entries stay in their round.
  const newGroup = page.getByRole("region", { name: `Round ${r0 + 1}` });
  await expect(newGroup).toContainText("current");
  await expect(newGroup).toContainText("Nothing recorded this round yet.");
  const oldGroup = page.getByRole("region", { name: `Round ${r0}` });
  await expect(
    oldGroup.getByRole("listitem").filter({ hasText: "e2e first round hit" }),
  ).toBeVisible();
  // Next Round itself is logged in the round it ends (voidable, see buffs.spec.ts).
  await expect(
    oldGroup
      .getByRole("listitem")
      .filter({ hasText: `Round ${r0} → ${r0 + 1}` }),
  ).toBeVisible();

  await recordHp(page, "damage", 1, "e2e second round hit");
  await expect(newGroup.getByRole("listitem")).toContainText(
    "e2e second round hit",
  );

  await expectNoReload(page);
  await snap(page, "next-round");
});

test("End combat resets to round 0 and Set round corrects it; both are voidable and leave buffs", async ({
  page,
}) => {
  await openDashboard(page);
  await addBuff(page, { name: "e2e Haste", rounds: 5 });
  await expect(buffBadge(page, "e2e Haste")).toBeVisible();
  await markNoReload(page);

  await page.getByRole("button", { name: "Next Round" }).click();
  await page.getByRole("button", { name: "Next Round" }).click();
  await expect.poll(() => currentRound(page)).toBeGreaterThanOrEqual(2);
  const before = await currentRound(page);

  // Set round: a number >= 0 through a small form.
  await page.getByRole("button", { name: "Set round" }).click();
  await page.getByLabel("Round number").fill("7");
  await snap(page, "set-round-form");
  await page.getByRole("button", { name: "Set", exact: true }).click();
  await expect.poll(() => currentRound(page)).toBe(7);
  const setEntry = logEntry(page, `Round set: ${before} → 7`);
  await expect(setEntry).toBeVisible();
  await expect(buffBadge(page, "e2e Haste")).toBeVisible();

  // End combat asks first; cancelling changes nothing.
  await page.getByRole("button", { name: "End combat" }).click();
  await snap(page, "end-combat-confirm");
  await page.getByRole("button", { name: "Cancel" }).click();
  expect(await currentRound(page)).toBe(7);

  await page.getByRole("button", { name: "End combat" }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "Confirm" })
    .click();
  await expect.poll(() => currentRound(page)).toBe(0);
  await expect(logEntry(page, "Combat ended: round 7 → 0")).toBeVisible();
  await expect(buffBadge(page, "e2e Haste")).toBeVisible();
  await snap(page, "end-combat");

  // Next Round then gives 1.
  await page.getByRole("button", { name: "Next Round" }).click();
  await expect.poll(() => currentRound(page)).toBe(1);
  // The newest "Round 0 → 1" entry (the earlier Next Rounds also went through 0 → 1).
  const advance = logEntry(page, "Round 0 → 1").first();
  await advance.getByRole("button", { name: "Void" }).click();
  await advance.getByRole("button", { name: "Confirm" }).click();
  await expect.poll(() => currentRound(page)).toBe(0);

  // Voiding End combat brings round 7 back.
  await voidEntry(page, "Combat ended: round 7 → 0");
  await expect.poll(() => currentRound(page)).toBe(7);
  await expectNoReload(page);
});

test("Time passes fast-forwards hours: timed buffs count down, round to 0, voidable", async ({
  page,
}) => {
  await openDashboard(page);
  await addBuff(page, { name: "e2e Long", rounds: 150 });
  await addBuff(page, { name: "e2e Short", rounds: 60 });
  await addBuff(page, { name: "e2e Layers", charges: 3 });
  // From 60 rounds up the badge also shows hours.
  await expect(buffBadge(page, "e2e Long").getByTestId("buff-rounds")).toHaveText(
    "150 rd · 2 h 30",
  );
  await page.getByRole("button", { name: "Next Round" }).click();
  await page.getByRole("button", { name: "Next Round" }).click();
  await expect.poll(() => currentRound(page)).toBeGreaterThanOrEqual(2);
  const before = await currentRound(page);
  await expect(buffBadge(page, "e2e Long").getByTestId("buff-rounds")).toHaveText(
    "148 rd · 2 h 28",
  );
  await markNoReload(page);

  const form = page.getByRole("form", { name: "⏩ Time passes" });
  await page.getByRole("button", { name: "⏩ Time passes" }).click();

  // 0.01 h is less than a round: the server refuses it with a clear message.
  await form.getByLabel("Hours that pass").fill("0.01");
  await expect(form.getByTestId("pass-time-preview")).toHaveText("= 0 rounds");
  await form.getByRole("button", { name: "Confirm" }).click();
  await expect(page.getByRole("alert")).toContainText("less than one round");
  expect(await currentRound(page)).toBe(before);

  // 1.5 h = 90 rounds, previewed before confirming.
  await form.getByLabel("Hours that pass").fill("1.5");
  await expect(form.getByTestId("pass-time-preview")).toHaveText("= 90 rounds");
  await snap(page, "pass-time-form");
  await form.getByRole("button", { name: "Confirm" }).click();
  await expect.poll(() => currentRound(page)).toBe(0);
  await expect(page.getByRole("status")).toContainText("Expired buffs: e2e Short");
  await expect(buffBadge(page, "e2e Short")).toHaveCount(0);
  await expect(buffBadge(page, "e2e Long").getByTestId("buff-rounds")).toHaveText(
    "58 rd",
  );
  await expect(buffBadge(page, "e2e Layers").getByTestId("buff-charges")).toHaveText(
    "3 ch",
  );
  const text = `Time passed: 1.5 h (90 rd), round ${before} → 0`;
  await expect(logEntry(page, text)).toContainText("expired: e2e Short");
  await snap(page, "pass-time-done");

  // Voiding it brings back the buffs and the round.
  await voidEntry(page, text);
  await expect.poll(() => currentRound(page)).toBe(before);
  await expect(buffBadge(page, "e2e Short").getByTestId("buff-rounds")).toHaveText(
    "58 rd",
  );
  await expect(buffBadge(page, "e2e Long").getByTestId("buff-rounds")).toHaveText(
    "148 rd · 2 h 28",
  );
  await expectNoReload(page);
});
