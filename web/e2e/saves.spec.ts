import type { Locator, Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  addBuff,
  apiCall,
  chooseLanguage,
  currentHp,
  logEntry,
  openDashboard,
  recordHp,
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

// The live card reads the save endpoint's answer, the reloaded one the entry's payload: one shape
// (the answer is the payload plus its entry id), so a natural 1 or 20 reads the same in both.
test("a natural 1 or 20 save reads the same live and after a reload", async ({
  page,
}) => {
  await openDashboard(page);
  const card = saveCard(page);
  let live = { d20: 0, target: 0, margin: 0 };
  for (let i = 0; i < 100 && live.d20 !== 1 && live.d20 !== 20; i++) {
    await rollSpellSave(page);
    await expect
      .poll(async () => await logEntry(page, "Save vs Spells").count())
      .toBe(Math.min(i + 1, 50)); // the page reads the journal's newest 50 (the default page)
    live = await facts(card);
  }
  expect([1, 20], "no natural 1 or 20 in 100 saves").toContain(live.d20);
  const vs = await card.getByTestId("save-vs").textContent();
  const outcome = live.margin >= 0 ? "Passed" : "Failed";
  await expect(card).toContainText(outcome);

  await page.reload();
  await expect(card).toBeVisible();
  expect(await facts(card)).toEqual(live);
  await expect(card.getByTestId("save-vs")).toHaveText(vs ?? "");
  await expect(card).toContainText(outcome);
  await expect(logEntry(page, "Save vs Spells").first()).toContainText(
    `Save vs Spells: d20 ${live.d20} vs ${live.target}, ${verdict(live.margin)}`,
  );
});

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

test("a voided save can no longer be adjusted; older saves and other entries are refused", async ({
  page,
}) => {
  await openDashboard(page);
  await recordHp(page, "damage", 1, "e2e save neighbour");
  await rollSpellSave(page);
  await rollSpellSave(page);

  const log = (await (await page.request.get("/api/log")).json()) as {
    entries: { id: number; kind: string }[];
  };
  const [latest, older] = log.entries
    .filter((e) => e.kind === "save")
    .map((e) => e.id);
  const hpEntry = log.entries.find((e) => e.kind === "hp_change")!.id;

  const adjustById = (id: number) =>
    page.request.post(`/api/roll/save/${id}/adjust`, {
      data: { adjustment: 4 },
    });
  const notLatest = await adjustById(older);
  expect(notLatest.status()).toBe(409);
  expect((await notLatest.json()).error).toContain(
    "only the latest roll can be adjusted",
  );
  const notSave = await adjustById(hpEntry);
  expect(notSave.status()).toBe(400);

  // Void the latest save (the one on the card) from the log, newest first: the card loses its
  // adjust control and the API refuses it.
  const form = await openFold(
    saveCard(page),
    "Adjust after the roll",
    "Adjust this save",
  );
  const newest = logEntry(page, "Save vs Spells").first();
  await page.getByTestId("undo").click();
  await expect(newest).toHaveAttribute("data-status", "voided");
  await expect(form).toHaveCount(0);
  const voided = await adjustById(latest);
  expect(voided.status()).toBe(409);
  expect((await voided.json()).error).toContain("already undone");
  // Voiding the newer save does not make the older one adjustable again.
  const stillOlder = await adjustById(older);
  expect(stillOlder.status()).toBe(409);
  expect((await stillOlder.json()).error).toContain(
    "only the latest roll can be adjusted",
  );
  await snap(page, "save-voided");
});

test("a save is no longer adjustable once another roll (an attack) follows it", async ({
  page,
}) => {
  await openDashboard(page);
  await rollSpellSave(page);
  const save = (
    (await (await page.request.get("/api/log")).json()) as {
      entries: { id: number; kind: string }[];
    }
  ).entries.find((e) => e.kind === "save")!.id;

  await page.getByRole("tab", { name: "Weapons" }).click();
  await page
    .getByRole("form", { name: "Attack" })
    .getByLabel("Target AC")
    .fill("4");
  await page.getByRole("button", { name: "Attack", exact: true }).click();
  await expect(
    page
      .getByRole("region", { name: "Detail", exact: true })
      .getByTestId("attack-result"),
  ).toBeVisible();

  const res = await page.request.post(`/api/roll/save/${save}/adjust`, {
    data: { adjustment: 4 },
  });
  expect(res.status()).toBe(409);
  expect((await res.json()).error).toContain(
    "only the latest roll can be adjusted",
  );
});

test("a save-bonus buff is labelled by its name on the save card (en and fr)", async ({
  page,
}) => {
  await openDashboard(page);
  await addBuff(page, { name: "e2e Gaia ward", effects: [["save_bonus", 2]] });
  await rollSpellSave(page);
  const mods = saveCard(page).getByTestId("save-modifiers");
  await expect(mods.getByRole("listitem")).toHaveCount(1);
  await expect(mods.getByRole("listitem").first()).toHaveText(
    /^e2e Gaia ward\s*\+2$/,
  );
  await expect(logEntry(page, "Save vs Spells")).toBeVisible();

  await chooseLanguage(page, "fr");
  const frCard = page
    .getByRole("region", { name: "Détail", exact: true })
    .getByTestId("save-result");
  await expect(frCard).toContainText("Sauvegarde contre Sorts");
  await expect(
    frCard.getByTestId("save-modifiers").getByRole("listitem").first(),
  ).toHaveText(/^e2e Gaia ward\s*\+2$/);
  await snap(page, "save-buff-fr");
});

// ADR 0006 step 9 in the browser: the "Against the mind" box reaches the save request, the card
// words the server's Wisdom modifier (en and fr), and the box clears for the next save. The values
// (Table 5, which saves, every class) are Go tests (combatsvc, ruletablesadpt, router).
test("a save against the mind carries the Wisdom modifier on its card (en and fr)", async ({
  page,
}) => {
  await openDashboard(page);
  expect(
    await apiCall(page, "POST", "/api/characters", {
      name: "Wren",
      class: "fighter",
      raceCode: "human",
      details: { alignment: "neutral_good", sex: "female" },
      str: 12,
      dex: 12,
      con: 12,
      int: 10,
      wis: 17,
      cha: 10,
      hpMax: 10,
      xp: 0,
    }),
  ).toBe(true);
  await reloadSettled(page);
  const mind = page.getByTestId("save-mind-affecting");
  await mind.check();
  const rolled = page.waitForResponse(
    (r) =>
      r.url().endsWith("/api/roll/save") && r.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Roll save vs Spells" }).click();
  const res = await rolled;
  expect(res.request().postDataJSON()).toEqual({
    category: "spell",
    mindAffecting: true,
  });
  expect((await res.json()).mindAffecting).toBe(true);
  const mods = saveCard(page).getByTestId("save-modifiers");
  await expect(mods.getByRole("listitem")).toHaveCount(1);
  await expect(mods.getByRole("listitem").first()).toHaveText(
    /^Wisdom 17 \(magical defence\)\s*\+3$/,
  );
  await expect(mind).not.toBeChecked();

  await chooseLanguage(page, "fr");
  const frCard = page
    .getByRole("region", { name: "Détail", exact: true })
    .getByTestId("save-result");
  await expect(
    frCard.getByTestId("save-modifiers").getByRole("listitem").first(),
  ).toHaveText(/^Sagesse 17 \(défense contre la magie\)\s*\+3$/);
  await expect(page.getByText("Contre l’esprit")).toBeVisible();
  await snap(page, "save-mind-fr");
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

  // The picker words the class in French too.
  await page.getByTestId("save-mind-affecting").check();
  await page.getByTestId("save-spell").click();
  await page.getByTestId("save-spell-search").fill("comma");
  await expect(page.getByTestId("save-spell-option").first()).toContainText(
    "Prêtre",
  );
  await expect(page.getByTestId("save-spell-option").first()).not.toContainText(
    "priest",
  );
  await page.setViewportSize({ width: 1920, height: 1080 });
  await snap(page, "save-spell-picker-fr");
});
