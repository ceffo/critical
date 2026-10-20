import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  combatLog,
  currentRound,
  expectNoReload,
  logEntry,
  markNoReload,
  addBuff,
  roundGroup,
  chooseSetting,
  nextRound,
  roundMenu,
  buffBadge,
  openDashboard,
  recordHp,
  undoEntry,
  undoNextRound,
  expectUndoOn,
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

test("Out of combat the timeline has no round numbers and the rows do not move; Next round and End combat switch (French)", async ({
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
  const track = page.getByTestId("effects-track");
  const numbers = track.getByText(/^T\d+$/);
  const row = buffBadge(page, "e2e Long");
  await expect(numbers).toHaveCount(0);
  await expect(track.locator('[aria-current="step"]')).toHaveCount(0);
  const rest = await row.boundingBox();
  await snap(page, "timeline-out-of-combat-fr");

  await page.getByRole("button", { name: "Tour suivant" }).click();
  await expect(numbers.first()).toBeVisible();
  await expect(track.locator('[aria-current="step"]')).toHaveText("T1");
  expect(await row.boundingBox()).toEqual(rest);
  await snap(page, "timeline-in-combat-fr");

  await page.getByRole("button", { name: "Autres actions du tour" }).click();
  await page.getByRole("menuitem", { name: "Fin du combat" }).click();
  await page
    .getByRole("alertdialog")
    .getByRole("button", { name: "Confirmer" })
    .click();
  await expect(numbers).toHaveCount(0);
  expect(await row.boundingBox()).toEqual(rest);
});

test("Time passes in French: '1,5' previews 90 rounds; a later Next Round must be popped first", async ({
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
  const text = "Temps écoulé : 1,5 h (90 tr.), tour 1 → 0";
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
  await undoNextRound(page, logEntryFrench(page, "Tour 0 → 1").first());
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

test("Next round moves the timeline window; a 1-turn effect shows the warning, then is gone", async ({
  page,
}) => {
  await openDashboard(page);
  await addBuff(page, { name: "e2e Long", rounds: 40, note: "+2 to hit" });
  await addBuff(page, { name: "e2e Layers", charges: 6 });
  await addBuff(page, { name: "e2e Gem", effects: [["min_str", 25]] });
  const track = page.getByTestId("effects-track");
  const current = track.locator('[aria-current="step"]');

  // Five rounds: the current column follows the turn, the window keeps two rounds of history.
  for (let i = 0; i < 5; i++) {
    const turn = await currentRound(page);
    if (turn === 0) {
      // Out of combat no round is counted: no numbers, no current column (ADR 0027 step 4.3).
      await expect(track.getByText(/^R\d+$/)).toHaveCount(0);
      await expect(current).toHaveCount(0);
    } else {
      await expect(current).toHaveText(`R${turn}`);
      await expect(track.getByText(/^R\d+$/).first()).toHaveText(
        `R${Math.max(1, turn - 2)}`,
      );
    }
    await nextRound(page);
    await expect.poll(() => currentRound(page)).toBe(turn + 1);
  }

  // A 2-turn effect: not yet on its last turn; one Next turn later it is (warning), then gone.
  await addBuff(page, { name: "e2e Blink", rounds: 2 });
  const blink = buffBadge(page, "e2e Blink");
  await expect(blink).toHaveAttribute("data-last-round", "false");
  await nextRound(page);
  await expect(blink).toHaveAttribute("data-last-round", "true");
  await expect(blink.getByTestId("buff-rounds")).toHaveText("1 round");

  // The header at every desktop size: five turn columns (ADR 0027 step 4.3).
  for (const vp of HEADER_VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(track).toHaveAttribute("data-columns", "5");
    await snap(page, `round-header-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await chooseSetting(page, "Dark");
  await snap(page, "round-header-dark-1440x900");

  await nextRound(page);
  await expect(blink).toHaveCount(0);
  await expect(page.getByRole("status")).toContainText(
    "Expired effects: e2e Blink",
  );
});

test("Next round rolls the initiative; one Undo pops a round with its initiative; reroll replaces it; a cast corrects it", async ({
  page,
  stack,
}) => {
  const prepared = await fetch(`${stack.apiUrl}/api/spells/prepared`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ spells: ["stoneskin", "rarys_mnemonic_enhancer"] }),
  });
  expect(prepared.ok).toBe(true);
  await openDashboard(page);
  const row = page.getByTestId("initiative-row");
  await expect(row).toHaveText("Out of combat · initiative will be rolled");

  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(1);
  const value = page.getByTestId("initiative-value");
  await expect(value).toHaveText(/^-?\d+$/);
  const firstValue = await value.textContent();
  const firstId = await value.getAttribute("data-entry-id");
  const first = combatLog(page).locator(`[data-entry-id="${firstId}"]`);
  // ADR 0035 item 6: the d10 plus the weapon in hand's speed, named in the journal and in the
  // number's tooltip.
  await expect(first).toContainText(
    new RegExp(
      `Initiative ${firstValue} for round 1 \\(d10 \\d+, Bastard Sword \\+2: \\d+\\)`,
    ),
  );
  await value.hover();
  const breakdown = page.getByTestId("initiative-breakdown");
  await expect(breakdown).toContainText(/^d10 \d+Bastard Sword \+2: \d+/);
  await expect(breakdown).toContainText(
    `= ${firstValue}, the lowest acts first`,
  );
  await page.mouse.move(0, 0);

  // Next turn and the initiative it rolls are one transaction (ADR 0018): the Undo names the
  // turn, and one click brings back turn 1 with its initiative.
  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(2);
  await expect(value).not.toHaveAttribute("data-entry-id", firstId ?? "");
  await expectUndoOn(page, "Round 1 → 2");
  await snap(page, "undo-names-next-turn");
  await undoNextRound(page, logEntry(page, "Round 1 → 2"));
  await expect.poll(() => currentRound(page)).toBe(1);
  // The popped turn opening: each of its rows reads as undone.
  for (const text of ["Round 1 → 2", "for round 2"]) {
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
    `Initiative ${await value.textContent()} for round 1 (d10 `,
  );

  // ADR 0035 item 6.2: casting in the round corrects its initiative live, the same d10 plus the
  // casting time (Stoneskin: 1 segment) in place of the weapon's speed; one Undo pops the cast and
  // the correction.
  const before = await value.textContent();
  const rolledId = await value.getAttribute("data-entry-id");
  await value.hover();
  const d10 = Number(
    /^d10 (\d+)/.exec((await breakdown.textContent()) ?? "")?.[1],
  );
  await page.mouse.move(0, 0);
  await page.getByRole("tab", { name: "Spells" }).click();
  await page.getByRole("button", { name: "Cast Stoneskin" }).click();
  await expect(value).toHaveText(String(d10 + 1));
  const corrected = logEntry(page, "Round 1 initiative corrected");
  await expect(corrected).toContainText(
    `Round 1 initiative corrected: ${before} → ${d10 + 1} (Stoneskin: +1)`,
  );
  await snap(page, "initiative-corrected-by-a-cast");
  await undoEntry(page, "Cast Stoneskin");
  await expect(value).toHaveText(before ?? "");
  await expect(value).toHaveAttribute("data-entry-id", rolledId ?? "");

  // A spell of a round or more (Rary's: 1 turn) adds nothing and tags the number "end of round";
  // the tag goes with the round at Next Round.
  const endOfRound = page.getByTestId("initiative-end-of-round");
  await expect(endOfRound).toBeHidden();
  await page
    .getByRole("button", { name: "Cast Rary's Mnemonic Enhancer" })
    .click();
  await expect(endOfRound).toHaveText("end of round");
  await expect(value).toHaveText(String(d10));
  await nextRound(page);
  await expect.poll(() => currentRound(page)).toBe(2);
  await expect(endOfRound).toBeHidden();

  for (const vp of HEADER_VIEWPORTS) {
    await page.setViewportSize(vp);
    await expect(value).toBeVisible();
    await snap(page, `initiative-${vp.width}x${vp.height}`);
  }
});
