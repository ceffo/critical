import type { Locator, Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  apiCall,
  chooseLanguage,
  currentHp,
  logEntry,
  openDashboard,
  openFold,
  reloadSettled,
} from "./helpers";

/** "+2", "−2" (U+2212) or "0" as a number. */
function signed(text: string | null): number {
  return Number((text ?? "").trim().replace("−", "-").replace("+", ""));
}

function saveCard(page: Page): Locator {
  return page
    .getByRole("region", { name: "Detail", exact: true })
    .getByTestId("save-result");
}

/** d20, the sheet's target and margin as shown on the save card (first, second and last facts). */
async function facts(card: Locator) {
  const dd = card.locator("dd");
  return {
    d20: Number(await dd.nth(0).textContent()),
    target: Number(await dd.nth(1).textContent()),
    margin: signed(await dd.last().textContent()),
  };
}

async function rollSpellSave(page: Page) {
  // The card is already visible after an earlier roll, so wait for this roll's own response: once it
  // returns, the new save is recorded and a following read of /api/log sees it.
  const rolled = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/roll/save") && r.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Roll save vs Spells" }).click();
  expect((await rolled).ok()).toBe(true);
  await expect(saveCard(page)).toBeVisible();
}

async function adjust(page: Page, steps: number, reason?: string) {
  const form = await openFold(
    saveCard(page),
    "Adjust after the roll",
    "Adjust this save",
  );
  const button = form.getByRole("button", {
    name: steps > 0 ? "Increase adjustment" : "Decrease adjustment",
  });
  for (let i = 0; i < Math.abs(steps); i++) await button.click();
  if (reason !== undefined)
    await form.getByLabel("Reason (optional)").fill(reason);
  await form.getByRole("button", { name: "Apply" }).click();
}

const verdict = (margin: number) => (margin >= 0 ? "PASSED" : "FAILED");

test("a save is logged and its adjustment moves the target, never the roll", async ({
  page,
}) => {
  await openDashboard(page);
  const hp = await currentHp(page);
  await rollSpellSave(page);
  const card = saveCard(page);
  const rolled = await facts(card);
  const entry = logEntry(page, "Save vs Spells");
  await expect(entry).toContainText(
    `Save vs Spells: d20 ${rolled.d20} vs ${rolled.target}, ${verdict(rolled.margin)}`,
  );
  await expect(card.getByTestId("save-adjustment")).toHaveText("0");

  await adjust(page, 4, "Gaia fire");
  await expect
    .poll(async () => (await facts(card)).margin)
    .toBe(rolled.margin + 4);
  expect((await facts(card)).d20).toBe(rolled.d20);
  await expect(card).toContainText(
    rolled.margin + 4 >= 0 ? "Passed" : "Failed",
  );
  // ADR 0006 step 5: +4 (easier) lowers the target by 4; the roll side is unchanged.
  const adjusted = rolled.target - 4;
  expect((await facts(card)).target).toBe(rolled.target);
  await expect(card.getByTestId("save-adjusted-target")).toHaveText(
    String(adjusted),
  );
  await expect(card.getByTestId("save-vs")).toHaveText(
    `${rolled.d20} vs ${adjusted}+`,
  );
  await expect(card.getByTestId("save-adjustment-line")).toHaveText(
    /^Gaia fire\s*\+4$/,
  );
  await expect(card.getByTestId("save-modifiers")).toHaveCount(0);
  await expect(entry).toContainText(
    `d20 ${rolled.d20} vs ${adjusted} (target ${rolled.target}, adjusted +4 Gaia fire): ${verdict(rolled.margin)} → ${verdict(rolled.margin + 4)}`,
  );
  await expect(entry).toContainText("Amended");
  await snap(page, "save-adjusted");

  await adjust(page, -4);
  await expect.poll(async () => (await facts(card)).margin).toBe(rolled.margin);
  expect((await facts(card)).d20).toBe(rolled.d20);
  await expect(card).toContainText(rolled.margin >= 0 ? "Passed" : "Failed");
  await expect(card.getByTestId("save-adjusted-target")).toHaveCount(0);
  await expect(card.getByTestId("save-vs")).toHaveText(
    `${rolled.d20} vs ${rolled.target}+`,
  );
  await expect(entry).toContainText(
    `adjustment removed: ${verdict(rolled.margin + 4)} → ${verdict(rolled.margin)}`,
  );
  expect(await currentHp(page)).toBe(hp);
  await snap(page, "save-adjustment-removed");
});

/** Makes a fresh Wisdom-19 fighter the active character (Table 5: immune to command). */
async function wiseFighter(page: Page) {
  expect(
    await apiCall(page, "POST", "/api/characters", {
      name: "Sage",
      class: "fighter",
      raceCode: "human",
      details: { alignment: "neutral_good", sex: "female" },
      str: 12,
      dex: 12,
      con: 12,
      int: 10,
      wis: 19,
      cha: 10,
      hpMax: 10,
      xp: 0,
    }),
  ).toBe(true);
  await reloadSettled(page);
}

// ADR 0006 step 10.2 in the browser: the saves' Spell picker (shown once "Against the mind" is
// ticked) reaches the save request, the card and the last-roll column word the server's immunity
// (en and fr), and unticking the box or rolling clears the tag for the next save (vitest here has no
// DOM to click the box). The rule (which scores, which spells, the refused adjustment) is Go tests (ruletablesadpt,
// combatsvc, router).
test("a save against a spell the Wisdom makes the character immune to rolls nothing (en and fr)", async ({
  page,
}) => {
  await openDashboard(page);
  await wiseFighter(page);
  // Unticking "Against the mind" clears a chosen spell: the next save names none.
  await page.getByTestId("save-mind-affecting").check();
  await page.getByTestId("save-spell").click();
  await page.getByTestId("save-spell-search").fill("comma");
  // The picker words the class, never the catalogue code.
  const option = page.getByTestId("save-spell-option").first();
  await expect(option).toContainText("Priest");
  await expect(option).not.toContainText("priest");
  await page.getByRole("button", { name: "Save against Command" }).click();
  await expect(page.getByTestId("save-spell-tag")).toHaveText("vs Command");
  await page.getByTestId("save-mind-affecting").uncheck();
  await expect(page.getByTestId("save-spell-tag")).toHaveCount(0);
  const plain = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/roll/save") && r.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Roll save vs Spells" }).click();
  expect((await plain).request().postDataJSON()).toEqual({
    category: "spell",
    mindAffecting: false,
  });
  await page.getByTestId("save-mind-affecting").check();
  await expect(page.getByTestId("save-spell-tag")).toHaveCount(0);

  await page.getByTestId("save-spell").click();
  await page.getByTestId("save-spell-search").fill("comma");
  await page.getByRole("button", { name: "Save against Command" }).click();
  await expect(page.getByTestId("save-spell-tag")).toHaveText("vs Command");
  const rolled = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/roll/save") && r.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Roll save vs Spells" }).click();
  const res = await rolled;
  expect(res.request().postDataJSON()).toEqual({
    category: "spell",
    mindAffecting: true,
    spellId: "command",
  });
  expect((await res.json()).immunity).toEqual({ source: "wisdom", score: 19 });
  const card = saveCard(page);
  await expect(card.getByTestId("save-immune")).toHaveText("Immune");
  await expect(card.getByTestId("save-immunity")).toHaveText(
    "Wisdom 19: immune to Command, nothing to roll",
  );
  await expect(card.getByText("Adjust after the roll")).toHaveCount(0);
  await expect(page.getByTestId("save-spell-tag")).toHaveCount(0);
  await expect(page.getByTestId("save-mind-affecting")).not.toBeChecked();
  await expect(
    page.locator('[data-save="spell"]').getByTestId("save-last-immune"),
  ).toBeVisible();
  await expect(logEntry(page, "Save vs Spells").first()).toContainText(
    "Save vs Spells against Command: immune (Wisdom 19), PASSED",
  );

  await chooseLanguage(page, "fr");
  await expect(
    page
      .getByRole("region", { name: "Détail", exact: true })
      .getByTestId("save-immunity"),
  ).toHaveText(/^Sagesse 19 : immunité contre .+, aucun jet$/);
  await snap(page, "save-immune-fr");
});
