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

// ADR 0006 step 10.2 in the browser: the saves' Spell picker (shown once "Mind" is
// ticked) reaches the save request, the card and the last-roll column word the server's immunity
// (en and fr), and unticking the box or rolling clears the tag for the next save (vitest here has no
// DOM to click the box). The rule (which scores, which spells, the refused adjustment) is Go tests (ruletablesadpt,
// combatsvc, router).
test("a save against a spell the Wisdom makes the character immune to rolls nothing (en and fr)", async ({
  page,
}) => {
  await openDashboard(page);
  await wiseFighter(page);
  // Unticking "Mind" clears a chosen spell: the next save names none.
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

/** Picks the spell the save is against from the wand's spell line. */
async function againstSpell(page: Page, search: string, name: string) {
  await page.getByTestId("save-spell").click();
  await page.getByTestId("save-spell-search").fill(search);
  await page.getByRole("button", { name: `Save against ${name}` }).click();
  await expect(page.getByTestId("save-spell-tag")).toHaveText(`vs ${name}`);
}

/** Creates a human level 5 wizard of the class (its four level-ups rolled) with Fireball prepared, the
 * current character. */
async function wizardWithFireball(page: Page, cls: string) {
  for (const [method, url, body] of [
    [
      "POST",
      "/api/characters",
      {
        name: cls,
        class: cls,
        raceCode: "human",
        details: { alignment: "neutral_good", sex: "male" },
        str: 9,
        dex: 16,
        con: 16,
        int: 18,
        wis: 12,
        cha: 10,
        hpMax: 4,
        xp: 0,
      },
    ],
    ["POST", "/api/xp", { base: 20000, bonus: false }],
    ...[2, 3, 4, 5].map(
      (level) => ["POST", "/api/character/levelup/roll", { level }] as const,
    ),
    ["POST", "/api/character/spellbook/add", { spellId: "fireball" }],
    ["PUT", "/api/spells/prepared", { spells: ["fireball"] }],
  ] as const)
    expect(await apiCall(page, method, url, body)).toBe(true);
  await reloadSettled(page);
}

// ADR 0050 steps 12 and 13 in the browser: the wand opens the spell line without the mind box, the
// named spell and the "Specialist caster" box reach the save request, the card lists the server's
// terms, the line clears after the roll, a refusal words its code; a cast's targets' -1 shows on the
// card and in the journal. The rule (the school, opposed wins, priest spells, the cancellation) is Go
// tests (saves, combatsvc, castsvc); the wording and hidden states are component tests.
test("a specialist's +1 against his school, a specialist caster's -1 on the save and the cast", async ({
  page,
}) => {
  await openDashboard(page);
  // A transmuter (alteration) saves vs Petrify/Polymorph against Slow, an alteration wizard spell.
  expect(
    await apiCall(page, "POST", "/api/characters", {
      name: "Morphée",
      class: "transmuter",
      raceCode: "human",
      details: { alignment: "neutral_good", sex: "male" },
      str: 9,
      dex: 16,
      con: 12,
      int: 17,
      wis: 10,
      cha: 10,
      hpMax: 4,
      xp: 0,
    }),
  ).toBe(true);
  await reloadSettled(page);
  const card = saveCard(page);
  const terms = card.getByTestId("save-modifiers").getByRole("listitem");
  const roll = async () => {
    const rolled = page.waitForResponse(
      (r) =>
        r.url().endsWith("/api/roll/save") && r.request().method() === "POST",
    );
    await page
      .getByRole("button", { name: "Roll save vs Petrify/Polymorph" })
      .click();
    return rolled;
  };

  const wand = page.getByTestId("save-spell-toggle");
  await wand.click();
  await expect(page.getByTestId("save-mind-affecting")).not.toBeChecked();
  await againstSpell(page, "slow", "Slow");
  const own = await roll();
  expect(own.request().postDataJSON()).toEqual({
    category: "petri_poly",
    mindAffecting: false,
    spellId: "slow",
  });
  await expect(terms).toHaveCount(1);
  await expect(terms).toHaveText(["Specialist, his school+1"]);
  await snap(page, "specialist-save");

  // The same with "Specialist caster": both terms, net 0; then the line, the wand and the box clear.
  await wand.click();
  await againstSpell(page, "slow", "Slow");
  await page.getByTestId("save-caster-specialist").check();
  const both = await roll();
  expect(both.request().postDataJSON()).toEqual({
    category: "petri_poly",
    mindAffecting: false,
    spellId: "slow",
    casterSpecialist: true,
  });
  const res = await both.json();
  expect(res.rollTotal).toBe(res.d20Roll);
  await expect(terms).toHaveText([
    "Specialist, his school+1",
    "Specialist caster−1",
  ]);
  await expect(page.getByTestId("save-spell-line")).toHaveCount(0);
  await expect(wand).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByTestId("save-caster-specialist")).toHaveCount(0);
  await snap(page, "specialist-caster-save");

  // "Specialist caster" against a priest spell is refused, worded from its code.
  await wand.click();
  await againstSpell(page, "comma", "Command");
  await page.getByTestId("save-caster-specialist").check();
  const refused = await roll();
  expect(refused.status()).toBe(400);
  await expect(
    page.getByText(
      "Name the wizard spell the save is against: the specialist caster’s −1 needs it.",
    ),
  ).toBeVisible();

  // An invoker's Fireball tells its targets' -1 on the card and in the journal; a mage's tells nothing.
  for (const [cls, modifier] of [
    ["invoker", 1],
    ["mage", 0],
  ] as const) {
    await wizardWithFireball(page, cls);
    await page.getByRole("tab", { name: "Spells" }).click();
    await page
      .getByRole("region", { name: "Level 3" })
      .getByRole("button", { name: "Cast Fireball" })
      .click();
    const cast = page
      .getByRole("region", { name: "Detail", exact: true })
      .getByTestId("cast-result");
    await expect(cast).toContainText("Fireball");
    const said = modifier ? ["(targets −1: specialist caster)"] : [];
    await expect(cast.getByTestId("cast-save-modifier")).toHaveText(said);
    if (modifier) await snap(page, "invoker-fireball");
    // The journal's card for the entry, after a reload: drawn from the logged payload.
    await reloadSettled(page);
    await logEntry(page, "Cast Fireball").first().click();
    const entry = page
      .getByRole("region", { name: "Detail", exact: true })
      .getByTestId("cast-entry");
    await expect(entry).toBeVisible();
    await expect(entry.getByTestId("cast-entry-save-modifier")).toHaveText(
      said,
    );
  }
});
