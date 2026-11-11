import { test, expect, snap } from "./fixtures";
import {
  currentRound,
  expectNoReload,
  logEntry,
  markNoReload,
  addBuff,
  roundGroup,
  nextRound,
  roundMenu,
  buffBadge,
  openDashboard,
  recordHp,
  undoEntry,
  undoNextRound,
} from "./helpers";

test("Next round advances the counter and starts a new log group", async ({
  page,
}) => {
  await openDashboard(page);
  const r0 = await currentRound(page);
  await markNoReload(page);

  await recordHp(page, "damage", 1, "e2e first round hit");
  await expect(logEntry(page, "e2e first round hit")).toBeVisible();

  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);

  // Next Round is one transaction (ADR 0018): the advance and the initiative it rolls sit together
  // in the round they open, as that turn's own rows, not framed (ADR 0027 step 9.6); the old entries
  // stay in their round.
  const newGroup = roundGroup(page, r0 + 1);
  await expect(newGroup).toContainText("current");
  await expect(newGroup.getByRole("listitem")).toHaveText([
    /Initiative -?\d+ for round \d+ \(d10 \d+/,
    new RegExp(`Round ${r0} → ${r0 + 1}`),
  ]);
  await expect(newGroup.getByTestId("log-transaction")).toHaveCount(0);
  const oldGroup = roundGroup(page, r0);
  await expect(
    oldGroup.getByRole("listitem").filter({ hasText: "e2e first round hit" }),
  ).toBeVisible();

  await recordHp(page, "damage", 1, "e2e second round hit");
  await expect(
    newGroup.getByRole("listitem").filter({ hasText: "e2e second round hit" }),
  ).toBeVisible();

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

  // One round at a time: two clicks in flight race their refreshes (an older one can land last).
  const r0 = await currentRound(page);
  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);
  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 2);
  const before = r0 + 2;

  // Set turn number: a number >= 0 through a small form in the turn menu.
  await roundMenu(page, "Set round number…");
  await page.getByLabel("Round number", { exact: true }).fill("7");
  await snap(page, "set-round-form");
  await page.getByRole("button", { name: "Set", exact: true }).click();
  // The form closes once the write and its refetches are done: opening the menu again before that
  // would be closed by it.
  await expect(page.getByLabel("Round number", { exact: true })).toBeHidden();
  await expect.poll(() => currentRound(page)).toBe(7);
  const setEntry = logEntry(page, `Round set: ${before} → 7`);
  await expect(setEntry).toBeVisible();
  await expect(buffBadge(page, "e2e Haste")).toBeVisible();

  // End combat asks first; cancelling changes nothing.
  await roundMenu(page, "End combat");
  await snap(page, "end-combat-confirm");
  await page.getByRole("button", { name: "Cancel" }).click();
  expect(await currentRound(page)).toBe(7);

  await roundMenu(page, "End combat");
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "Confirm" })
    .click();
  await expect.poll(() => currentRound(page)).toBe(0);
  await expect(logEntry(page, "Combat ended: round 7 → 0")).toBeVisible();
  await expect(buffBadge(page, "e2e Haste")).toBeVisible();
  await snap(page, "end-combat");

  // Next Round then gives 1.
  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(1);
  // The newest "Round 0 → 1" entry (the earlier Next Rounds also went through 0 → 1).
  await undoNextRound(page, logEntry(page, "Round 0 → 1").first());
  await expect.poll(() => currentRound(page)).toBe(0);

  // Voiding End combat brings round 7 back.
  await undoEntry(page, "Combat ended: round 7 → 0");
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
  // From 60 rounds up the pill also shows hours.
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("150 rounds · 2 h 30");
  // One round at a time: two clicks in flight race their refreshes (an older one can land last).
  const r0 = await currentRound(page);
  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);
  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 2);
  const before = r0 + 2;
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("148 rounds · 2 h 28");
  await markNoReload(page);

  const form = page.getByRole("form", { name: "Time passes…" });
  await roundMenu(page, "Time passes…");

  // 1.5 h = 90 rounds: the server's preview says so and names the effects that would end.
  await form.getByLabel("Hours that pass").fill("1.5");
  await expect(form.getByTestId("pass-time-preview")).toHaveText("= 90 rounds");
  await expect(form.getByTestId("pass-time-ending")).toHaveText(
    "Ends: e2e Short",
  );
  await snap(page, "pass-time-form");
  await form.getByRole("button", { name: "Confirm" }).click();
  await expect.poll(() => currentRound(page)).toBe(0);
  await expect(page.getByRole("status")).toContainText(
    "Expired effects: e2e Short",
  );
  await expect(buffBadge(page, "e2e Short")).toHaveCount(0);
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("58 rounds");
  await expect(
    buffBadge(page, "e2e Layers").getByTestId("buff-charges"),
  ).toHaveText("3 / 3 charges · until spent");
  const text = `Time passed: 1.5 h (90 rd), round ${before} → 0`;
  await expect(logEntry(page, text)).toContainText("expired: e2e Short");
  await snap(page, "pass-time-done");

  // Voiding it brings back the buffs and the round.
  await undoEntry(page, text);
  await expect.poll(() => currentRound(page)).toBe(before);
  await expect(
    buffBadge(page, "e2e Short").getByTestId("buff-rounds"),
  ).toHaveText("58 rounds");
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("148 rounds · 2 h 28");
  await expectNoReload(page);
});
