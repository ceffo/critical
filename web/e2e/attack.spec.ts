import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import { logEntry, openDashboard, voidEntry } from "./helpers";

function attackForm(page: Page) {
  return page.getByRole("form", { name: "Attack" });
}

function resultCard(page: Page) {
  return page
    .getByRole("region", { name: "Last Result" })
    .getByTestId("attack-result");
}

/** Remaining count shown in the selected ammunition option ("Arrows +3 (2 left)"). */
async function selectedAmmoCount(page: Page): Promise<number> {
  const text =
    (await attackForm(page)
      .getByLabel("Ammunition")
      .locator("option:checked")
      .textContent()) ?? "";
  const m = /\((\d+) left\)/.exec(text);
  if (!m) throw new Error(`unexpected ammo option: ${JSON.stringify(text)}`);
  return Number(m[1]);
}

async function openAttack(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Attack" }).click();
  await expect(attackForm(page).getByLabel("Weapon")).toBeVisible();
}

test("a melee attack lists every modifier in the result card and the log", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  await form.getByLabel("Weapon").selectOption("longsword");
  await expect(form.getByLabel("Ammunition")).toHaveCount(0);
  await form.getByLabel("Target AC").fill("5");
  await form.getByLabel("Target size").selectOption("l");
  await form.getByLabel("Situational").fill("2");
  await form.getByLabel("Reason").fill("Rear attack");
  await form.getByRole("button", { name: "Roll Attack" }).click();

  const card = resultCard(page);
  await expect(card).toContainText("Long Sword +2 vs AC 5");
  await expect(card).toContainText("large target");
  await expect(card).toContainText(/Hit|Miss/);
  const mods = card.getByTestId("to-hit-modifiers");
  await expect(mods.getByRole("listitem")).toHaveCount(3);
  await expect(mods).toContainText(/Strength \d+/);
  await expect(mods.getByRole("listitem").nth(1)).toHaveText(
    /Long Sword \+2\s*\+2/,
  );
  await expect(mods.getByRole("listitem").nth(2)).toHaveText(
    /Rear attack\s*\+2/,
  );

  // The attack is in the combat log, in the player's language, rendered from its payload.
  await expect(logEntry(page, "Long Sword +2 vs AC 5")).toBeVisible();
  await snap(page, "melee-modifiers");
});

test("a missile attack spends the chosen arrow", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await form.getByLabel("Weapon").selectOption("shortbow");
  await form.getByLabel("Ammunition").selectOption("arrows_plus3");
  const before = await selectedAmmoCount(page);
  await form.getByLabel("Target AC").fill("2");
  await form.getByRole("button", { name: "Roll Attack" }).click();

  const card = resultCard(page);
  await expect(card).toContainText("Short Bow +2 with Arrows +3 vs AC 2");
  const mods = card.getByTestId("to-hit-modifiers");
  await expect(mods).toContainText(/Dexterity \d+ \(missile\)/);
  await expect(mods).toContainText("Short Bow +2");
  await expect(mods).toContainText("Arrows +3");
  await expect(mods).not.toContainText("Strength");
  await expect(card.getByTestId("ammo-left")).toHaveText(
    `Arrows +3: ${before - 1} left`,
  );
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);
  await expect(
    logEntry(page, "Short Bow +2 (Arrows +3) vs AC 2"),
  ).toBeVisible();
  await snap(page, "missile-ammo");
});

test("voiding a missile attack gives the arrow back", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await form.getByLabel("Weapon").selectOption("shortbow");
  await form.getByLabel("Ammunition").selectOption("arrows_plus2");
  const before = await selectedAmmoCount(page);
  await form.getByLabel("Target AC").fill("3");
  await form.getByRole("button", { name: "Roll Attack" }).click();
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);

  await voidEntry(page, "Short Bow +2 (Arrows +2) vs AC 3");
  await expect.poll(() => selectedAmmoCount(page)).toBe(before);
  await snap(page, "missile-void");
});

test("spent ammunition stays selected and must be replaced by an explicit pick", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  const roll = form.getByRole("button", { name: "Roll Attack" });
  await form.getByLabel("Weapon").selectOption("shortbow");
  const ammo = form.getByLabel("Ammunition");
  await ammo.selectOption("arrows_plus3");
  const left = await selectedAmmoCount(page);
  for (let i = left; i > 0; i--) {
    await roll.click();
    await expect.poll(() => selectedAmmoCount(page)).toBe(i - 1);
  }

  // The +3 arrows are spent: still the selection, no silent switch to other arrows.
  await expect(ammo).toHaveValue("arrows_plus3");
  await expect(form.getByTestId("ammo-spent")).toHaveText(
    "No Arrows +3 left: pick other ammunition",
  );
  await expect(roll).toBeDisabled();
  await snap(page, "ammo-spent");

  await ammo.selectOption("arrows_plus1");
  await expect(form.getByTestId("ammo-spent")).toHaveCount(0);
  await expect(roll).toBeEnabled();
});

test("a later shot with other arrows does not block voiding an earlier shot", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  const ammo = form.getByLabel("Ammunition");
  await form.getByLabel("Weapon").selectOption("shortbow");
  await form.getByLabel("Target AC").fill("7");

  await ammo.selectOption("arrows_plus3");
  const plus3 = await selectedAmmoCount(page);
  await form.getByRole("button", { name: "Roll Attack" }).click();
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus3 - 1);

  await ammo.selectOption("arrows_plus1");
  const plus1 = await selectedAmmoCount(page);
  await form.getByRole("button", { name: "Roll Attack" }).click();
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus1 - 1);

  await voidEntry(page, "Short Bow +2 (Arrows +3) vs AC 7");
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus1 - 1);
  await ammo.selectOption("arrows_plus3");
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus3);
  await snap(page, "void-earlier-shot");
});

test("the target AC is corrected on the card after the roll: same dice, no arrow spent", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  await form.getByLabel("Weapon").selectOption("shortbow");
  await form.getByLabel("Ammunition").selectOption("arrows_plus2");
  const before = await selectedAmmoCount(page);
  // AC -20 can't be hit and AC 20 can't be missed whatever the d20, so the flips are certain.
  await form.getByLabel("Target AC").fill("-20");
  await form.getByRole("button", { name: "Roll Attack" }).click();

  const card = resultCard(page);
  const fix = card.getByRole("form", { name: "Correct the target AC" });
  const acField = fix.getByLabel("Corrected target AC");
  const apply = fix.getByRole("button", { name: "Apply" });
  const entry = logEntry(page, "Short Bow +2 (Arrows +2)");
  await expect(card).toContainText("vs AC -20");
  await expect(card.getByTestId("verdict")).toHaveText("Miss");
  await expect(card.getByTestId("total-damage")).toHaveText("–");
  await expect(acField).toHaveValue("-20");
  await expect(apply).toBeDisabled();
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);
  const d20 = await card.getByText("d20", { exact: true }).locator("xpath=..").locator("dd").textContent();

  // − / + step the field; nothing is sent until Apply.
  await fix.getByRole("button", { name: "Raise target AC" }).click();
  await expect(acField).toHaveValue("-19");
  await fix.getByRole("button", { name: "Lower target AC" }).click();
  await expect(acField).toHaveValue("-20");

  // Miss → hit: the damage shown is the dice rolled with the attack.
  await acField.fill("20");
  await apply.click();
  await expect(card).toContainText("vs AC 20");
  await expect(card.getByTestId("verdict")).toHaveText("Hit");
  await expect(card.getByTestId("ac-corrected")).toHaveText("was AC -20");
  await expect(card.getByTestId("total-damage")).toHaveText(/^\d+$/);
  const damage = await card.getByTestId("total-damage").textContent();
  expect(await card.getByText("d20", { exact: true }).locator("xpath=..").locator("dd").textContent()).toBe(d20);
  await expect(entry).toContainText(/\(AC -20 → 20: MISS → (HIT|CRIT)\)/);
  await expect(entry).toContainText("Amended");
  await expect(card.getByTestId("ammo-left")).toHaveText(`Arrows +2: ${before - 1} left`);
  expect(await selectedAmmoCount(page)).toBe(before - 1);
  await snap(page, "attack-ac-corrected");

  // Hit → miss hides the damage; back to AC 20 shows the same damage again (no reroll).
  await acField.fill("-20");
  await apply.click();
  await expect(card.getByTestId("verdict")).toHaveText("Miss");
  await expect(card.getByTestId("total-damage")).toHaveText("–");
  await expect(entry).toContainText(/\(AC 20 → -20: (HIT|CRIT) → MISS\)/);
  await acField.fill("20");
  await apply.click();
  await expect(card.getByTestId("verdict")).toHaveText("Hit");
  await expect(card.getByTestId("total-damage")).toHaveText(damage!);
  expect(await selectedAmmoCount(page)).toBe(before - 1);

  // Voiding the corrected attack still gives the arrow back and removes the correction form.
  await voidEntry(page, "Short Bow +2 (Arrows +2)");
  await expect.poll(() => selectedAmmoCount(page)).toBe(before);
  await expect(fix).toHaveCount(0);
  await snap(page, "attack-ac-corrected-voided");
});

test("only the latest roll's target AC can be corrected", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await form.getByLabel("Weapon").selectOption("longsword");
  await form.getByLabel("Target AC").fill("5");
  await form.getByRole("button", { name: "Roll Attack" }).click();
  await expect(resultCard(page)).toContainText("vs AC 5");
  await form.getByRole("button", { name: "Roll Attack" }).click();
  await expect.poll(async () => (await logEntry(page, "Long Sword +2 vs AC 5").count())).toBe(2);

  const log = (await (await page.request.get("/api/log")).json()) as {
    entries: { id: number; kind: string }[];
  };
  const [, older] = log.entries.filter((e) => e.kind === "attack").map((e) => e.id);
  const res = await page.request.post(`/api/roll/attack/${older}/adjust`, { data: { targetAc: 7 } });
  expect(res.status()).toBe(409);
  expect((await res.json()).error).toContain("only the latest roll can be adjusted");
});
