import type { Locator, Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { addBuff, chooseLanguage, currentHp, logEntry, openDashboard, recordHp } from "./helpers";

/** "+2", "−2" (U+2212) or "0" as a number. */
function signed(text: string | null): number {
  return Number((text ?? "").trim().replace("−", "-").replace("+", ""));
}

function saveCard(page: Page): Locator {
  return page.getByRole("region", { name: "Last Result" }).getByTestId("save-result");
}

/** d20, target and margin as shown on the save card (its three facts, in that order). */
async function facts(card: Locator) {
  const dd = card.locator("dd");
  return {
    d20: Number(await dd.nth(0).textContent()),
    target: Number(await dd.nth(1).textContent()),
    margin: signed(await dd.nth(2).textContent()),
  };
}

async function rollSpellSave(page: Page) {
  // The card is already visible after an earlier roll, so wait for this roll's own response: once it
  // returns, the new save is recorded and a following read of /api/log sees it.
  const rolled = page.waitForResponse(
    (r) => r.url().endsWith("/api/roll/save") && r.request().method() === "POST",
  );
  await page.getByRole("button", { name: "Roll save vs Spells" }).click();
  expect((await rolled).ok()).toBe(true);
  await expect(saveCard(page)).toBeVisible();
}

async function adjust(page: Page, steps: number, reason?: string) {
  const form = saveCard(page).getByRole("form", { name: "Adjust this save" });
  const button = form.getByRole("button", { name: steps > 0 ? "Increase adjustment" : "Decrease adjustment" });
  for (let i = 0; i < Math.abs(steps); i++) await button.click();
  if (reason !== undefined) await form.getByLabel("Reason (optional)").fill(reason);
  await form.getByRole("button", { name: "Apply" }).click();
}

const verdict = (margin: number) => (margin >= 0 ? "PASSED" : "FAILED");

test("a save is logged and its adjustment flips the verdict, never the d20", async ({ page }) => {
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
  await expect.poll(async () => (await facts(card)).margin).toBe(rolled.margin + 4);
  expect((await facts(card)).d20).toBe(rolled.d20);
  await expect(card).toContainText(rolled.margin + 4 >= 0 ? "Passed" : "Failed");
  await expect(card.getByTestId("save-modifiers")).toContainText("Adjustment Gaia fire");
  await expect(entry).toContainText(
    `adjusted +4 Gaia fire: ${verdict(rolled.margin)} → ${verdict(rolled.margin + 4)}`,
  );
  await expect(entry).toContainText("Amended");
  await snap(page, "save-adjusted");

  await adjust(page, -4);
  await expect.poll(async () => (await facts(card)).margin).toBe(rolled.margin);
  expect((await facts(card)).d20).toBe(rolled.d20);
  await expect(card).toContainText(rolled.margin >= 0 ? "Passed" : "Failed");
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
  const [latest, older] = log.entries.filter((e) => e.kind === "save").map((e) => e.id);
  const hpEntry = log.entries.find((e) => e.kind === "hp_change")!.id;

  const adjustById = (id: number) =>
    page.request.post(`/api/roll/save/${id}/adjust`, { data: { adjustment: 4 } });
  const notLatest = await adjustById(older);
  expect(notLatest.status()).toBe(409);
  expect((await notLatest.json()).error).toContain("only the latest roll can be adjusted");
  const notSave = await adjustById(hpEntry);
  expect(notSave.status()).toBe(400);

  // Void the latest save (the one on the card) from the log, newest first: the card loses its
  // adjust control and the API refuses it.
  const form = saveCard(page).getByRole("form", { name: "Adjust this save" });
  await expect(form).toBeVisible();
  const newest = logEntry(page, "Save vs Spells").first();
  await newest.getByRole("button", { name: "Void" }).click();
  await newest.getByRole("button", { name: "Confirm" }).click();
  await expect(newest).toHaveAttribute("data-status", "voided");
  await expect(form).toHaveCount(0);
  const voided = await adjustById(latest);
  expect(voided.status()).toBe(409);
  expect((await voided.json()).error).toContain("already voided");
  // Voiding the newer save does not make the older one adjustable again.
  const stillOlder = await adjustById(older);
  expect(stillOlder.status()).toBe(409);
  expect((await stillOlder.json()).error).toContain("only the latest roll can be adjusted");
  await snap(page, "save-voided");
});

test("a save is no longer adjustable once another roll (an attack) follows it", async ({ page }) => {
  await openDashboard(page);
  await rollSpellSave(page);
  const save = (
    (await (await page.request.get("/api/log")).json()) as { entries: { id: number; kind: string }[] }
  ).entries.find((e) => e.kind === "save")!.id;

  await page.getByRole("tab", { name: "Attack" }).click();
  await page.getByLabel("Target AC").fill("4");
  await page.getByRole("button", { name: "Roll Attack" }).click();
  await expect(page.getByRole("region", { name: "Last Result" }).getByTestId("attack-result")).toBeVisible();

  const res = await page.request.post(`/api/roll/save/${save}/adjust`, { data: { adjustment: 4 } });
  expect(res.status()).toBe(409);
  expect((await res.json()).error).toContain("only the latest roll can be adjusted");
});

test("a save-bonus buff is labelled by its name on the save card (en and fr)", async ({ page }) => {
  await openDashboard(page);
  await addBuff(page, { name: "e2e Gaia ward", effects: [["save_bonus", 2]] });
  await rollSpellSave(page);
  const mods = saveCard(page).getByTestId("save-modifiers");
  await expect(mods.getByRole("listitem")).toHaveCount(1);
  await expect(mods.getByRole("listitem").first()).toHaveText(/^e2e Gaia ward\s*\+2$/);
  await expect(logEntry(page, "Save vs Spells")).toBeVisible();

  await chooseLanguage(page, "fr");
  const frCard = page.getByRole("region", { name: "Dernier résultat" }).getByTestId("save-result");
  await expect(frCard).toContainText("Sauvegarde contre Sorts");
  await expect(frCard.getByTestId("save-modifiers").getByRole("listitem").first()).toHaveText(
    /^e2e Gaia ward\s*\+2$/,
  );
  await snap(page, "save-buff-fr");
});
