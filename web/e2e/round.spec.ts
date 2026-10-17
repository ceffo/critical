import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  combatLog,
  currentRound,
  expectNoReload,
  logEntry,
  markNoReload,
  addBuff,
  turnGroup,
  chooseSetting,
  nextTurn,
  turnMenu,
  buffBadge,
  openDashboard,
  recordHp,
  undoEntry,
  undoNextTurn,
  expectUndoOn,
} from "./helpers";

test("Next turn advances the counter and starts a new log group", async ({
  page,
}) => {
  await openDashboard(page);
  const r0 = await currentRound(page);
  await markNoReload(page);

  await recordHp(page, "damage", 1, "e2e first round hit");
  await expect(logEntry(page, "e2e first round hit")).toBeVisible();

  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);

  // Next Round is one transaction (ADR 0018): the advance and the initiative it rolls sit together
  // in the round they open, as that turn's own rows, not framed (ADR 0027 step 9.6); the old entries
  // stay in their round.
  const newGroup = turnGroup(page, r0 + 1);
  await expect(newGroup).toContainText("current");
  await expect(newGroup.getByRole("listitem")).toHaveText([
    /Initiative \d+ \(d10\) for turn/,
    new RegExp(`Turn ${r0} → ${r0 + 1}`),
  ]);
  await expect(newGroup.getByTestId("log-transaction")).toHaveCount(0);
  const oldGroup = turnGroup(page, r0);
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
  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);
  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 2);
  const before = r0 + 2;

  // Set turn number: a number >= 0 through a small form in the turn menu.
  await turnMenu(page, "Set turn number…");
  await page.getByLabel("Turn number", { exact: true }).fill("7");
  await snap(page, "set-round-form");
  await page.getByRole("button", { name: "Set", exact: true }).click();
  await expect.poll(() => currentRound(page)).toBe(7);
  const setEntry = logEntry(page, `Turn set: ${before} → 7`);
  await expect(setEntry).toBeVisible();
  await expect(buffBadge(page, "e2e Haste")).toBeVisible();

  // End combat asks first; cancelling changes nothing.
  await turnMenu(page, "End combat");
  await snap(page, "end-combat-confirm");
  await page.getByRole("button", { name: "Cancel" }).click();
  expect(await currentRound(page)).toBe(7);

  await turnMenu(page, "End combat");
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "Confirm" })
    .click();
  await expect.poll(() => currentRound(page)).toBe(0);
  await expect(logEntry(page, "Combat ended: turn 7 → 0")).toBeVisible();
  await expect(buffBadge(page, "e2e Haste")).toBeVisible();
  await snap(page, "end-combat");

  // Next Round then gives 1.
  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(1);
  // The newest "Turn 0 → 1" entry (the earlier Next Rounds also went through 0 → 1).
  await undoNextTurn(page, logEntry(page, "Turn 0 → 1").first());
  await expect.poll(() => currentRound(page)).toBe(0);

  // Voiding End combat brings round 7 back.
  await undoEntry(page, "Combat ended: turn 7 → 0");
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
  // From 60 turns up the pill also shows hours.
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("150 turns · 2 h 30");
  // One round at a time: two clicks in flight race their refreshes (an older one can land last).
  const r0 = await currentRound(page);
  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 1);
  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(r0 + 2);
  const before = r0 + 2;
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("148 turns · 2 h 28");
  await markNoReload(page);

  const form = page.getByRole("form", { name: "Time passes…" });
  await turnMenu(page, "Time passes…");

  // 1.5 h = 90 turns: the server's preview says so and names the effects that would end.
  await form.getByLabel("Hours that pass").fill("1.5");
  await expect(form.getByTestId("pass-time-preview")).toHaveText("= 90 turns");
  await expect(form.getByTestId("pass-time-ending")).toHaveText(
    "Ends: e2e Short",
  );
  await snap(page, "pass-time-form");
  await form.getByRole("button", { name: "Confirm" }).click();
  await expect.poll(() => currentRound(page)).toBe(0);
  await expect(page.getByRole("status")).toContainText(
    "Expired buffs: e2e Short",
  );
  await expect(buffBadge(page, "e2e Short")).toHaveCount(0);
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("58 turns");
  await expect(
    buffBadge(page, "e2e Layers").getByTestId("buff-charges"),
  ).toHaveText("3 / 3 charges · until spent");
  const text = `Time passed: 1.5 h (90 rd), turn ${before} → 0`;
  await expect(logEntry(page, text)).toContainText("expired: e2e Short");
  await snap(page, "pass-time-done");

  // Voiding it brings back the buffs and the round.
  await undoEntry(page, text);
  await expect.poll(() => currentRound(page)).toBe(before);
  await expect(
    buffBadge(page, "e2e Short").getByTestId("buff-rounds"),
  ).toHaveText("58 turns");
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("148 turns · 2 h 28");
  await expectNoReload(page);
});

/** The log entry containing `text` in the French UI. */
function logEntryFrench(page: Page, text: string) {
  return page
    .getByRole("region", { name: "Journal", exact: true })
    .getByRole("listitem")
    .filter({ hasText: text });
}

/** Undo the newest entry (the journal's one button, "Annuler" in the French UI). */
async function voidFrench(page: Page, text: string) {
  const entry = logEntryFrench(page, text).first();
  await page.getByTestId("undo").click();
  await expect(entry).toHaveAttribute("data-status", "voided");
}

test("Time passes in French: '1,5' previews 90 turns; a later Next Round must be popped first", async ({
  page,
  stack,
}) => {
  const res = await fetch(`${stack.apiUrl}/api/buffs`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      name: "e2e Long",
      duration: 100,
      charges: 0,
      effects: {},
    }),
  });
  expect(res.ok).toBe(true);
  await page.addInitScript(() =>
    localStorage.setItem(
      "critical.settings.v1",
      JSON.stringify({ lang: "fr", mode: "system", amb: "arcane" }),
    ),
  );
  await openDashboard(page);
  await page.getByRole("button", { name: "Tour suivant" }).click();
  await expect.poll(() => currentRound(page)).toBe(1);
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("99 tours · 1 h 39");

  // Not a positive number: the hint, and Confirm is off.
  const form = page.getByRole("form", { name: "Le temps passe…" });
  await page.getByRole("button", { name: "Autres actions du tour" }).click();
  await page.getByRole("menuitem", { name: "Le temps passe…" }).click();
  await form.getByLabel("Heures écoulées").fill("0");
  await expect(form.getByTestId("pass-time-preview")).toHaveText(
    "Des heures, par ex. 0,25, 1,5 ou 8",
  );
  await expect(form.getByRole("button", { name: "Confirmer" })).toBeDisabled();

  await form.getByLabel("Heures écoulées").fill("1,5");
  await expect(form.getByTestId("pass-time-preview")).toHaveText("= 90 tours");
  await snap(page, "pass-time-form-fr");
  await form.getByRole("button", { name: "Confirmer" }).click();
  await expect.poll(() => currentRound(page)).toBe(0);
  const text = "Temps écoulé : 1,5 h (90 rd.), tour 1 → 0";
  await expect(logEntryFrench(page, text)).toBeVisible();
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("9 tours");

  // The log is a stack: under a later Next Round the fast-forward offers no void.
  await page.getByRole("button", { name: "Tour suivant" }).click();
  await expect.poll(() => currentRound(page)).toBe(1);
  await expect(
    logEntryFrench(page, text).getByRole("button", {
      name: "Annuler",
      exact: true,
    }),
  ).toHaveCount(0);
  await snap(page, "pass-time-void-not-offered-fr");

  // Undo Next Round first (one Undo: the advance and its initiative), then the fast-forward: the
  // buff and round 1 come back.
  await undoNextTurn(page, logEntryFrench(page, "Tour 0 → 1").first());
  await expect.poll(() => currentRound(page)).toBe(0);
  await voidFrench(page, text);
  await expect(logEntryFrench(page, text)).toHaveAttribute(
    "data-status",
    "voided",
  );
  await expect.poll(() => currentRound(page)).toBe(1);
  await expect(
    buffBadge(page, "e2e Long").getByTestId("buff-rounds"),
  ).toHaveText("99 tours · 1 h 39");
});

const HEADER_VIEWPORTS = [
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
  { width: 2560, height: 1440 },
];

test("Next turn moves the timeline window; a 1-turn effect shows the warning, then is gone", async ({
  page,
}) => {
  await openDashboard(page);
  await addBuff(page, { name: "e2e Long", rounds: 40, note: "+2 to hit" });
  await addBuff(page, { name: "e2e Layers", charges: 6 });
  await addBuff(page, { name: "e2e Gem", effects: [["min_str", 25]] });
  const track = page.getByTestId("effects-track");
  const current = track.locator('[aria-current="step"]');

  // Five turns: the current column follows the turn, the window keeps two turns of history.
  for (let i = 0; i < 5; i++) {
    const turn = await currentRound(page);
    await expect(current).toHaveText(`T${turn}`);
    await expect(track.getByText(/^T\d+$/).first()).toHaveText(
      `T${turn === 0 ? 0 : Math.max(1, turn - 2)}`,
    );
    await nextTurn(page);
    await expect.poll(() => currentRound(page)).toBe(turn + 1);
  }

  // A 2-turn effect: not yet on its last turn; one Next turn later it is (warning), then gone.
  await addBuff(page, { name: "e2e Blink", rounds: 2 });
  const blink = buffBadge(page, "e2e Blink");
  await expect(blink).toHaveAttribute("data-last-turn", "false");
  await nextTurn(page);
  await expect(blink).toHaveAttribute("data-last-turn", "true");
  await expect(blink.getByTestId("buff-rounds")).toHaveText("1 turn");

  // The header at every desktop size: more turn columns on wider screens, never fewer.
  let columns = 0;
  for (const vp of HEADER_VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect
      .poll(async () => Number(await track.getAttribute("data-columns")))
      .toBeGreaterThanOrEqual(columns);
    // Wait for the resize to settle: the count stops moving before it is recorded.
    let last = -1;
    await expect
      .poll(async () => {
        const now = Number(await track.getAttribute("data-columns"));
        const settled = now === last;
        last = now;
        return settled;
      })
      .toBe(true);
    columns = last;
    await snap(page, `turn-header-${vp.width}x${vp.height}`);
  }
  expect(columns, "16 turns at 2560").toBe(16);
  await page.setViewportSize({ width: 1440, height: 900 });
  await chooseSetting(page, "Dark");
  await snap(page, "turn-header-dark-1440x900");

  await nextTurn(page);
  await expect(blink).toHaveCount(0);
  await expect(page.getByRole("status")).toContainText(
    "Expired buffs: e2e Blink",
  );
});

test("Next turn rolls the initiative; one Undo pops a turn with its initiative; reroll replaces it", async ({
  page,
}) => {
  await openDashboard(page);
  const row = page.getByTestId("initiative-row");
  await expect(row).toHaveText("Out of combat · initiative will be rolled");

  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(1);
  const value = page.getByTestId("initiative-value");
  await expect(value).toHaveText(/^([1-9]|10)$/);
  const firstValue = await value.textContent();
  const firstId = await value.getAttribute("data-entry-id");
  const first = combatLog(page).locator(`[data-entry-id="${firstId}"]`);
  await expect(first).toContainText(
    `Initiative ${firstValue} (d10) for turn 1`,
  );

  // Next turn and the initiative it rolls are one transaction (ADR 0018): the Undo names the
  // turn, and one click brings back turn 1 with its initiative.
  await nextTurn(page);
  await expect.poll(() => currentRound(page)).toBe(2);
  await expect(value).not.toHaveAttribute("data-entry-id", firstId ?? "");
  await expectUndoOn(page, "Turn 1 → 2");
  await snap(page, "undo-names-next-turn");
  await undoNextTurn(page, logEntry(page, "Turn 1 → 2"));
  await expect.poll(() => currentRound(page)).toBe(1);
  // The popped turn opening: each of its rows reads as undone.
  for (const text of ["Turn 1 → 2", "for turn 2"]) {
    const popped = logEntry(page, text);
    await expect(popped).toHaveAttribute("data-status", "voided");
    await expect(popped).toContainText("Undone");
  }
  await snap(page, "next-round-undone");
  await expect(value).toHaveAttribute("data-entry-id", firstId ?? "");
  await expect(value).toHaveText(firstValue ?? "");

  // The reroll logs a new roll as its own step; the roll it replaces stays (an Undo gives it back).
  await page.getByRole("button", { name: "Reroll initiative" }).click();
  await expect(value).not.toHaveAttribute("data-entry-id", firstId ?? "");
  await expect(first).toHaveAttribute("data-status", "active");
  const rerolled = combatLog(page).locator(
    `[data-entry-id="${await value.getAttribute("data-entry-id")}"]`,
  );
  await expect(rerolled).toHaveAttribute("data-status", "active");
  await expect(rerolled).toContainText(
    `Initiative ${await value.textContent()} (d10) for turn 1`,
  );

  for (const vp of HEADER_VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(value).toBeVisible();
    await snap(page, `initiative-${vp.width}x${vp.height}`);
  }
});
