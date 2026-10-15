import { DatabaseSync } from "node:sqlite";
import type { Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  ammoChips,
  chooseAmmo,
  chooseLanguage,
  chooseWeapon,
  chosenAmmoCount,
  logEntry,
  openDashboard,
  openWeapons,
  voidEntry,
} from "./helpers";

function resultCard(page: Page) {
  return page
    .getByRole("region", { name: "Detail", exact: true })
    .getByTestId("attack-result");
}

const selectedAmmoCount = chosenAmmoCount;

async function openAttack(page: Page) {
  await openDashboard(page);
  await openWeapons(page);
}

test("a melee attack lists every modifier in the result card and the log", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  await chooseWeapon(page, "Long Sword +2");
  await expect(ammoChips(page)).toHaveCount(0);
  await form.getByLabel("Target AC").fill("5");
  await form.getByRole("radio", { name: "Large" }).click();
  await form.getByLabel("Situational", { exact: true }).fill("2");
  // Typed key by key: every keystroke is saved on the server, spaces included.
  await form.getByLabel("Reason").pressSequentially("Rear attack");
  await form.getByRole("button", { name: "Attack", exact: true }).click();

  const card = resultCard(page);
  await expect(card).toContainText("Long Sword +2 vs AC 5");
  await expect(card).toContainText("large target");
  await expect(card).toContainText(/Hit|Miss/);
  // The modifiers are in the calculation details, closed by default.
  await card.getByText("Calculation details").click();
  const mods = card.getByTestId("to-hit-modifiers");
  await expect(mods.getByRole("listitem")).toHaveCount(3);
  await expect(mods).toContainText(/Strength \d+/);
  await expect(mods.getByRole("listitem").nth(1)).toHaveText(
    /Long Sword \+2\s*\+2/,
  );
  await expect(mods.getByRole("listitem").nth(2)).toHaveText(
    /Rear attack\s*\+2/,
  );
  const log = (await (await page.request.get("/api/log")).json()) as {
    entries: { kind: string; payload: { modifiers: { kind: string; detail?: string }[] } }[];
  };
  expect(
    log.entries.find((e) => e.kind === "attack")!.payload.modifiers.find((m) => m.kind === "situational")
      ?.detail,
  ).toBe("Rear attack");

  // The attack is in the combat log, in the player's language, rendered from its payload.
  await expect(logEntry(page, "Long Sword +2 vs AC 5")).toBeVisible();
  await snap(page, "melee-modifiers");
});

test("a missile attack spends the chosen arrow", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await chooseWeapon(page, "Short Bow +2");
  await chooseAmmo(page, "Arrows +3");
  const before = await selectedAmmoCount(page);
  await form.getByLabel("Target AC").fill("2");
  await form.getByRole("button", { name: "Attack", exact: true }).click();

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
  await chooseWeapon(page, "Short Bow +2");
  await chooseAmmo(page, "Arrows +2");
  const before = await selectedAmmoCount(page);
  await form.getByLabel("Target AC").fill("3");
  await form.getByRole("button", { name: "Attack", exact: true }).click();
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
  const roll = form.getByRole("button", { name: "Attack", exact: true });
  await chooseWeapon(page, "Short Bow +2");
  await chooseAmmo(page, "Arrows +3");
  const left = await selectedAmmoCount(page);
  for (let i = left; i > 0; i--) {
    await roll.click();
    await expect.poll(() => selectedAmmoCount(page)).toBe(i - 1);
  }

  // The +3 arrows are spent: still the choice, no silent switch to other arrows.
  await expect(ammoChips(page).locator('[aria-checked="true"]')).toHaveAttribute("data-item", "arrows_plus3");
  await expect(form.getByTestId("ammo-spent")).toHaveText(
    "No Arrows +3 left: pick other ammunition",
  );
  await expect(roll).toBeDisabled();
  await snap(page, "ammo-spent");

  await chooseAmmo(page, "Arrows +1");
  await expect(form.getByTestId("ammo-spent")).toHaveCount(0);
  await expect(roll).toBeEnabled();
  // Once another pick replaces it, the spent arrows are no longer offered.
  await expect(ammoChips(page).locator('[data-item="arrows_plus3"]')).toHaveCount(0);
});

test("an earlier shot can only be voided once the later shot is popped", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  await chooseWeapon(page, "Short Bow +2");
  await form.getByLabel("Target AC").fill("7");

  await chooseAmmo(page, "Arrows +3");
  const plus3 = await selectedAmmoCount(page);
  await form.getByRole("button", { name: "Attack", exact: true }).click();
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus3 - 1);

  await chooseAmmo(page, "Arrows +1");
  const plus1 = await selectedAmmoCount(page);
  await form.getByRole("button", { name: "Attack", exact: true }).click();
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus1 - 1);

  // The log is a stack: only the newest entry offers Void.
  const earlier = logEntry(page, "Short Bow +2 (Arrows +3) vs AC 7");
  await expect(earlier.getByRole("button", { name: "Void" })).toHaveCount(0);
  await snap(page, "void-earlier-shot-not-offered");

  await voidEntry(page, "Short Bow +2 (Arrows +1) vs AC 7");
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus1);
  await voidEntry(page, "Short Bow +2 (Arrows +3) vs AC 7");
  await expect(page.getByRole("alert")).toHaveCount(0);
  await chooseAmmo(page, "Arrows +3");
  await expect.poll(() => selectedAmmoCount(page)).toBe(plus3);
  await snap(page, "void-earlier-shot");
});

test("the target AC is corrected on the card after the roll: same dice, no arrow spent", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  await chooseWeapon(page, "Short Bow +2");
  await chooseAmmo(page, "Arrows +2");
  const before = await selectedAmmoCount(page);
  // AC -20 can't be hit and AC 10 can't be missed (effective THAC0 11) whatever the d20, so the flips are certain.
  await form.getByLabel("Target AC").fill("-20");
  await form.getByRole("button", { name: "Attack", exact: true }).click();

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
  await acField.fill("10");
  await apply.click();
  await expect(card).toContainText("vs AC 10");
  await expect(card.getByTestId("verdict")).toHaveText("Hit");
  await expect(card.getByTestId("ac-corrected")).toHaveText("rolled vs AC -20");
  await expect(card.getByTestId("total-damage")).toHaveText(/^\d+$/);
  const damage = await card.getByTestId("total-damage").textContent();
  expect(await card.getByText("d20", { exact: true }).locator("xpath=..").locator("dd").textContent()).toBe(d20);
  await expect(entry).toContainText(/\(AC -20 → 10: MISS → (HIT|CRIT)\)/);
  await expect(entry).toContainText("Amended");
  await expect(card.getByTestId("ammo-left")).toHaveText(`Arrows +2: ${before - 1} left`);
  expect(await selectedAmmoCount(page)).toBe(before - 1);
  await snap(page, "attack-ac-corrected");

  // A second correction still reads from the rolled AC and outcome.
  await acField.fill("9");
  await apply.click();
  await expect(card).toContainText("vs AC 9");
  await expect(entry).toContainText(/\(AC -20 → 9: MISS → (HIT|CRIT)\)/);
  await expect(card.getByTestId("ac-corrected")).toHaveText("rolled vs AC -20");

  // Hit → miss: back to the rolled AC hides the damage and reads as rolled (still amended); back
  // to AC 10 shows the same damage again (no reroll).
  await acField.fill("-20");
  await apply.click();
  await expect(card.getByTestId("verdict")).toHaveText("Miss");
  await expect(card.getByTestId("total-damage")).toHaveText("–");
  await expect(card.getByTestId("ac-corrected")).toHaveCount(0);
  await expect(entry).toContainText("Short Bow +2 (Arrows +2) vs AC -20: miss");
  await expect(entry).not.toContainText("→ -20");
  await expect(entry).toContainText("Amended");
  await acField.fill("10");
  await apply.click();
  await expect(card.getByTestId("verdict")).toHaveText("Hit");
  await expect(card.getByTestId("total-damage")).toHaveText(damage!);
  await expect(entry).toContainText(/\(AC -20 → 10: MISS → (HIT|CRIT)\)/);
  expect(await selectedAmmoCount(page)).toBe(before - 1);

  // In French: the form and the log line.
  await chooseLanguage(page, "fr");
  const frCard = page.getByRole("region", { name: "Détail", exact: true }).getByTestId("attack-result");
  const frFix = frCard.getByRole("form", { name: "Corriger la CA de la cible" });
  await expect(frFix.getByLabel("CA de la cible corrigée")).toHaveValue("10");
  await expect(frFix.getByRole("button", { name: "Appliquer" })).toBeDisabled();
  await expect(frFix.getByRole("button", { name: "Monter la CA de la cible" })).toBeVisible();
  await expect(frCard.getByTestId("ac-corrected")).toHaveText("jet contre CA -20");
  const frEntry = page
    .getByRole("region", { name: "Journal", exact: true })
    .getByRole("listitem")
    .filter({ hasText: "Short Bow +2 (Arrows +2)" });
  await expect(frEntry).toContainText(
    /contre CA 10 : (touché|critique).*\(CA -20 → 10 : RATÉ → (TOUCHÉ|CRITIQUE)\)/,
  );
  await snap(page, "attack-ac-corrected-fr");
  await chooseLanguage(page, "en");

  // Each correction is its own step (ADR 0010 step 6): Annuler names the newest one and pops it,
  // and the card reads the attack as it was before it, here the rolled AC again (no arrow back).
  const undo = page.getByTestId("undo");
  await expect(undo).toHaveAttribute("title", "Undo: Correction AC -20 → 10 (Ctrl+Z / ⌘Z)");
  await undo.click();
  await expect(card).toContainText("vs AC -20");
  await expect(card.getByTestId("verdict")).toHaveText("Miss");
  await expect(card.getByTestId("ac-corrected")).toHaveCount(0);
  await expect(acField).toHaveValue("-20");
  await expect(entry).toContainText("Short Bow +2 (Arrows +2) vs AC -20: miss");
  expect(await selectedAmmoCount(page)).toBe(before - 1);
  await snap(page, "attack-ac-correction-undone");
  // The two corrections before it, then the first one: the attack is as rolled, still logged.
  for (const step of ["9 → -20", "10 → 9", "-20 → 10"]) {
    await expect(undo).toHaveAttribute("title", `Undo: Correction AC ${step} (Ctrl+Z / ⌘Z)`);
    await undo.click();
  }
  await expect(undo).toHaveAttribute("title", /^Undo: Short Bow \+2 \(Arrows \+2\) vs AC -20: miss/);
  await expect(entry).not.toContainText("Amended");

  // Voiding the attack then gives the arrow back and removes the correction form.
  await voidEntry(page, "Short Bow +2 (Arrows +2)");
  await expect.poll(() => selectedAmmoCount(page)).toBe(before);
  await expect(fix).toHaveCount(0);
  await snap(page, "attack-ac-corrected-voided");
});

test("only the latest roll's target AC can be corrected", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await chooseWeapon(page, "Long Sword +2");
  await form.getByLabel("Target AC").fill("5");
  await form.getByRole("button", { name: "Attack", exact: true }).click();
  await expect(resultCard(page)).toContainText("vs AC 5");
  await form.getByRole("button", { name: "Attack", exact: true }).click();
  await expect.poll(async () => (await logEntry(page, "Long Sword +2 vs AC 5").count())).toBe(2);

  const log = (await (await page.request.get("/api/log")).json()) as {
    entries: { id: number; kind: string }[];
  };
  const [, older] = log.entries.filter((e) => e.kind === "attack").map((e) => e.id);
  const res = await page.request.post(`/api/roll/attack/${older}/adjust`, { data: { targetAc: 7 } });
  expect(res.status()).toBe(409);
  expect((await res.json()).error).toContain("only the latest roll can be adjusted");

  // A save rolled after the attack replaces the card: the attack is no longer the latest roll.
  const [latest] = log.entries.filter((e) => e.kind === "attack").map((e) => e.id);
  expect((await page.request.post("/api/roll/save", { data: { category: "spell" } })).ok()).toBe(true);
  const afterSave = await page.request.post(`/api/roll/attack/${latest}/adjust`, { data: { targetAc: 7 } });
  expect(afterSave.status()).toBe(409);
  expect((await afterSave.json()).error).toContain("only the latest roll can be adjusted");
});

test("an attack logged before the multiplier d100 keeps the old crit text and can't be corrected", async ({
  page,
  stack,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  await chooseWeapon(page, "Long Sword +2");
  // The card of an old response (no multiplier d100): the old crit text and two dice facts. The
  // attack is sent at AC 1000 (beyond the form's ±20): so low a roll is needed that it crits.
  await page.route("**/api/roll/attack", async (route) => {
    const sent = route.request().postDataJSON() as Record<string, unknown>;
    const res = await route.fetch({ postData: JSON.stringify({ ...sent, target_ac: 1000 }) });
    const body = (await res.json()) as Record<string, unknown>;
    delete body.d100MultiplierRoll;
    await route.fulfill({ response: res, json: body });
  });
  await form.getByRole("button", { name: "Attack", exact: true }).click();
  const card = resultCard(page);
  await expect(card.getByTestId("crit-badge")).toHaveText("Critical ×2 (placeholder)");
  await expect(card.locator("dt", { hasText: "Crit check d100" })).toHaveCount(1);
  await expect(card.locator("dt", { hasText: "Multiplier d100" })).toHaveCount(0);
  await snap(page, "legacy-crit-card");
  await page.unroute("**/api/roll/attack");

  // Make the stored entry an old one, as logged before the multiplier d100 existed.
  const db = new DatabaseSync(stack.dbPath);
  try {
    const changed = db
      .prepare(
        "UPDATE action_log SET payload = json_remove(payload, '$.d100MultiplierRoll') WHERE kind = 'attack'",
      )
      .run();
    expect(changed.changes).toBe(1);
  } finally {
    db.close();
  }
  const log = (await (await page.request.get("/api/log")).json()) as {
    entries: { id: number; kind: string }[];
  };
  const id = log.entries.find((e) => e.kind === "attack")!.id;

  await page.reload();
  await expect(logEntry(page, "Long Sword +2 vs AC 1000")).toContainText(
    /Long Sword \+2 vs AC 1000: critical ×2, \d+ damage/,
  );
  await expect(logEntry(page, "Long Sword +2 vs AC 1000")).not.toContainText("multiplier");

  const res = await page.request.post(`/api/roll/attack/${id}/adjust`, { data: { targetAc: 5 } });
  expect(res.status()).toBe(400);
  expect((await res.json()).error).toContain("void it and roll again");
});

test("a miss corrected into a crit shows both d100s on the card and in the log", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  await chooseWeapon(page, "Long Sword +2");
  // AC -20 can't be hit (effective THAC0 11 needs 31). AC 10 needs 1: a crit when
  // d20 − check d100 ≥ 1 (ADR 0001 step 2). Roll until the dice make that a crit (~1 in 10).
  await form.getByLabel("Target AC").fill("-20");
  const card = resultCard(page);
  const fact = async (label: string) =>
    Number(
      await card
        .locator("dt", { hasText: label })
        .locator("xpath=following-sibling::dd")
        .textContent(),
    );
  let d20 = 0;
  let check = 100;
  for (let i = 0; i < 100 && d20 - check < 1; i++) {
    await form.getByRole("button", { name: "Attack", exact: true }).click();
    await expect.poll(async () => (await logEntry(page, "Long Sword +2 vs AC -20").count())).toBe(i + 1);
    d20 = await fact("d20");
    check = await fact("Crit check d100");
  }
  expect(d20 - check, "no roll in 100 makes AC 10 a crit").toBeGreaterThanOrEqual(1);
  await expect(card.getByTestId("verdict")).toHaveText("Miss");
  const mult = await fact("Multiplier d100");

  const fix = card.getByRole("form", { name: "Correct the target AC" });
  await fix.getByLabel("Corrected target AC").fill("10");
  await fix.getByRole("button", { name: "Apply" }).click();
  await expect(card).toContainText("vs AC 10");
  await expect(card.getByTestId("verdict")).toHaveText("Hit");
  await expect(card.getByTestId("crit-badge")).toHaveText(
    `Crit check ${check} · multiplier ${mult} → ×2 (placeholder)`,
  );
  expect(await fact("Crit check d100")).toBe(check);
  expect(await fact("Multiplier d100")).toBe(mult);
  await expect(logEntry(page, "Long Sword +2 vs AC 10")).toContainText(
    `critical (check ${check} · multiplier ${mult} → ×2)`,
  );
  await expect(logEntry(page, "Long Sword +2 vs AC 10")).toContainText("(AC -20 → 10: MISS → CRIT)");
  await snap(page, "miss-to-crit");

  // In French: the three dice facts and the crit text.
  await chooseLanguage(page, "fr");
  const frCard = page.getByRole("region", { name: "Détail", exact: true }).getByTestId("attack-result");
  await expect(frCard.getByTestId("crit-badge")).toHaveText(
    `Test critique ${check} · multiplicateur ${mult} → ×2 (provisoire)`,
  );
  await expect(frCard.locator("dt", { hasText: "d100 multiplicateur" })).toHaveCount(1);
  await expect(
    page
      .getByRole("region", { name: "Journal", exact: true })
      .getByRole("listitem")
      .filter({ hasText: "Long Sword +2 contre CA 10" }),
  ).toContainText(`(CA -20 → 10 : RATÉ → CRITIQUE)`);
  await snap(page, "miss-to-crit-fr");
});

// The form's choices are server state: a reload restores them all, and switching weapons
// (previous weapon, other weapon) never attacks.
test("the attack form is kept on the server across a reload; switching weapons never attacks", async ({
  page,
}) => {
  await openAttack(page);
  const form = attackForm(page);
  const held = form.getByTestId("weapon-in-hand");
  await chooseWeapon(page, "Long Sword +2");
  await chooseWeapon(page, "Short Bow +2");
  await expect(form.getByRole("button", { name: "Switch to Long Sword +2" })).toBeVisible();
  await chooseAmmo(page, "Arrows +2");
  await form.getByLabel("Target AC").fill("-3");
  await form.getByRole("radio", { name: "Large" }).click();
  await form.getByRole("button", { name: "Raise the modifier" }).click();
  await form.getByRole("button", { name: "Raise the modifier" }).click();
  await form.getByLabel("Reason").fill("cover");
  const thac0 = form.getByTestId("tile-thac0");
  await expect(form.getByTestId("tile-damage-l")).toHaveText("1d6");
  // Wait until the server has every change (the THAC0 tile is its preview).
  await expect
    .poll(async () => (await (await page.request.get("/api/attack/form")).json()).form)
    .toEqual({
      weaponId: "shortbow", previousWeaponId: "longsword", ammoItemId: "arrows_plus2", targetAc: -3,
      targetSize: "l", situational: { value: 2, label: "cover" },
    });
  const shown = await thac0.textContent();
  await expect(logEntry(page, "vs AC")).toHaveCount(0);

  await page.reload();
  await expect(held).toHaveText("Short Bow +2");
  await expect(ammoChips(page).locator('[aria-checked="true"]')).toHaveAttribute("data-item", "arrows_plus2");
  await expect(form.getByLabel("Target AC")).toHaveValue("-3");
  await expect(form.getByRole("radio", { name: "Large" })).toHaveAttribute("aria-checked", "true");
  await expect(form.getByLabel("Situational", { exact: true })).toHaveValue("+2");
  await expect(form.getByLabel("Reason")).toHaveValue("cover");
  await expect(thac0).toHaveText(shown!);
  await snap(page, "form-restored");

  // One click back to the sword: it is in hand, the bow is now the previous weapon, no attack.
  await form.getByRole("button", { name: "Switch to Long Sword +2" }).click();
  await expect(held).toHaveText("Long Sword +2");
  await expect(form.getByRole("button", { name: "Switch to Short Bow +2" })).toBeVisible();
  await expect(ammoChips(page)).toHaveCount(0);
  await expect(logEntry(page, "vs AC")).toHaveCount(0);
  await expect(resultCard(page)).toHaveCount(0);
  await snap(page, "weapon-switched");
});

// docs/adr/0012: the target AC is unknown by default; the card says which AC the roll hits and what
// to tell the GM, the GM's Hit settles it without a crit. Screenshots of the unsettled card at the
// four desktop sizes.
test("an attack against an unknown AC is settled by the GM's Hit", async ({ page }) => {
  await openAttack(page);
  const form = attackForm(page);
  await chooseWeapon(page, "Long Sword +2");
  const ac = form.getByLabel("Target AC");
  await expect(ac).toHaveValue("");
  await expect(ac).toHaveAttribute("placeholder", "?");
  // A known AC can be forgotten again with Unknown.
  await ac.fill("4");
  await expect(form.getByText("vs AC 4")).toBeVisible();
  await form.getByRole("button", { name: "Unknown" }).click();
  await expect(ac).toHaveValue("");
  await expect(form.getByText("AC unknown")).toBeVisible();

  const rolled = page.waitForResponse((r) => r.url().endsWith("/api/roll/attack"));
  await form.getByRole("button", { name: "Attack", exact: true }).click();
  const r = (await (await rolled).json()) as {
    hitsAc: number;
    critsFromAc: number;
    critPossible?: boolean;
    totalDamage: number;
  };
  const card = resultCard(page);
  await expect(card).toContainText("Long Sword +2 vs unknown AC");
  await expect(card.getByTestId("hits-ac")).toHaveText(`Hits AC ${r.hitsAc}`);
  await expect(card.getByTestId("tell-gm")).toHaveText(
    // The crit part is dropped when no real AC can crit (critPossible=false, random with the dice).
    r.critPossible === false
      ? `Tell the GM: “hits AC ${r.hitsAc}”.`
      : `Tell the GM: “hits AC ${r.hitsAc}”. A crit from AC ${r.critsFromAc}.`,
  );
  await expect(card.getByTestId("verdict")).toHaveText("?");
  await expect(card.getByText("Damage if it hits")).toBeVisible();
  await expect(logEntry(page, `Long Sword +2 vs unknown AC: hits AC ${r.hitsAc}`)).toBeVisible();
  for (const vp of [
    { width: 1280, height: 800 },
    { width: 1440, height: 900 },
    { width: 1920, height: 1080 },
    { width: 2560, height: 1440 },
  ]) {
    await page.setViewportSize(vp);
    await expect(card.getByTestId("tell-gm")).toBeVisible();
    await snap(page, `unknown-ac-${vp.width}x${vp.height}`);
  }
  await page.setViewportSize({ width: 1440, height: 900 });

  await card
    .getByRole("group", { name: "The GM’s hit or miss" })
    .getByRole("button", { name: "Hit" })
    .click();
  await expect(card.getByTestId("verdict")).toHaveText("Hit");
  await expect(card.getByTestId("verdict-by-gm")).toBeVisible();
  await expect(card.getByTestId("crit-badge")).toHaveCount(0);
  await expect(card.getByTestId("total-damage")).toHaveText(String(r.totalDamage));
  await expect(
    logEntry(page, `Long Sword +2 vs unknown AC: hit (GM), ${r.totalDamage} damage`),
  ).toBeVisible();
  await snap(page, "unknown-ac-gm-hit");
});

// docs/adr/0012 step 5: the GM gives the AC after the roll; at AC 10 the stored dice make a crit.
// A THAC0 of -100 keeps critsFromAc (effective THAC0 − d20 + crit-check d100) at most about -3, so
// every roll crits AC 10 whatever the dice.
test.describe("with a THAC0 low enough that AC 10 always crits", () => {
  test.use({ character: { thac0_base: -100 } });

  test("an attack against an unknown AC becomes a crit when the GM gives the AC", async ({ page }) => {
    await openAttack(page);
    const form = attackForm(page);
    await chooseWeapon(page, "Long Sword +2");
    const rolled = page.waitForResponse((res) => res.url().endsWith("/api/roll/attack"));
    await form.getByRole("button", { name: "Attack", exact: true }).click();
    const r = (await (await rolled).json()) as { critsFromAc: number };
    expect(r.critsFromAc).toBeLessThanOrEqual(10);

    const card = resultCard(page);
    await expect(card.getByTestId("verdict")).toHaveText("?");
    const give = card.getByRole("form", { name: "Give the real AC" });
    await give.getByLabel("Real target AC").fill("10");
    await give.getByRole("button", { name: "Correct" }).click();
    await expect(card).toContainText("Long Sword +2 vs AC 10");
    await expect(card.getByTestId("ac-given")).toBeVisible();
    await expect(card.getByTestId("tell-gm")).toHaveCount(0);
    await expect(card.getByRole("group", { name: "The GM’s hit or miss" })).toHaveCount(0);
    await expect(card.getByTestId("verdict")).toHaveText("Hit");
    await expect(card.getByTestId("crit-badge")).toBeVisible();
    await expect(logEntry(page, "Long Sword +2 vs AC 10: critical")).toBeVisible();
    await snap(page, "unknown-ac-given-crit");
  });
});
