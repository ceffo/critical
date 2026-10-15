import type { Page } from "@playwright/test";
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
  await expect(form, "the form stays open after a refusal").toBeVisible();
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

/** The log entry containing `text` in the French UI. */
function logEntryFr(page: Page, text: string) {
  return page
    .getByRole("region", { name: "Journal de combat" })
    .getByRole("listitem")
    .filter({ hasText: text });
}

/** Void the newest entry whose text contains `text` in the French UI (Annuler, then Confirmer). */
async function voidFr(page: Page, text: string) {
  const entry = logEntryFr(page, text).first();
  await entry.getByRole("button", { name: "Annuler", exact: true }).click();
  await entry.getByRole("button", { name: "Confirmer" }).click();
}

test("Time passes in French: '1,5' previews 90 rounds; a later Next Round must be popped first", async ({
  page,
  stack,
}) => {
  const res = await fetch(`${stack.apiUrl}/api/buffs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: "e2e Long", duration: 100, charges: 0, effects: {} }),
  });
  expect(res.ok).toBe(true);
  await page.addInitScript(() =>
    localStorage.setItem(
      "critical.settings.v1",
      JSON.stringify({ lang: "fr", mode: "system", amb: "arcane" }),
    ),
  );
  await openDashboard(page);
  await page.getByRole("button", { name: "Round suivant" }).click();
  await expect.poll(() => currentRound(page)).toBe(1);
  await expect(buffBadge(page, "e2e Long").getByTestId("buff-rounds")).toHaveText(
    "99 rd. · 1 h 39",
  );

  // Not a positive number: the hint, and Confirm is off.
  const form = page.getByRole("form", { name: "⏩ Le temps passe" });
  await page.getByRole("button", { name: "⏩ Le temps passe" }).click();
  await form.getByLabel("Heures écoulées").fill("0");
  await expect(form.getByTestId("pass-time-preview")).toHaveText(
    "Des heures, par ex. 0,25, 1,5 ou 8",
  );
  await expect(form.getByRole("button", { name: "Confirmer" })).toBeDisabled();

  await form.getByLabel("Heures écoulées").fill("1,5");
  await expect(form.getByTestId("pass-time-preview")).toHaveText("= 90 rounds");
  await snap(page, "pass-time-form-fr");
  await form.getByRole("button", { name: "Confirmer" }).click();
  await expect.poll(() => currentRound(page)).toBe(0);
  const text = "Temps écoulé : 1,5 h (90 rd.), round 1 → 0";
  await expect(logEntryFr(page, text)).toBeVisible();
  await expect(buffBadge(page, "e2e Long").getByTestId("buff-rounds")).toHaveText("9 rd.");

  // The log is a stack: under a later Next Round the fast-forward offers no void.
  await page.getByRole("button", { name: "Round suivant" }).click();
  await expect.poll(() => currentRound(page)).toBe(1);
  await expect(
    logEntryFr(page, text).getByRole("button", { name: "Annuler", exact: true }),
  ).toHaveCount(0);
  await snap(page, "pass-time-void-not-offered-fr");

  // Void Next Round first, then the fast-forward: the buff and round 1 come back.
  await voidFr(page, "Round 0 → 1");
  await expect.poll(() => currentRound(page)).toBe(0);
  await voidFr(page, text);
  await expect(logEntryFr(page, text)).toHaveAttribute("data-status", "voided");
  await expect.poll(() => currentRound(page)).toBe(1);
  await expect(buffBadge(page, "e2e Long").getByTestId("buff-rounds")).toHaveText(
    "99 rd. · 1 h 39",
  );
});
