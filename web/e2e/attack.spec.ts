import type { Locator, Page } from "@playwright/test";
import { test, expect, snap } from "./fixtures";
import {
  attackForm,
  ammoChips,
  chooseAmmo,
  chooseWeapon,
  chosenAmmoCount,
  logEntry,
  openDashboard,
  openWeapons,
  undoEntry,
  openTargetModifiers,
  openFold,
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
  // The size chips are on the target line; only the situational modifier is behind the toggle.
  await form.getByRole("radio", { name: "Large" }).click();
  await openTargetModifiers(page);
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
  // The attack is in the combat log, in the player's language, rendered from its payload.
  await expect(logEntry(page, "Long Sword +2 vs AC 5")).toBeVisible();
  await snap(page, "melee-modifiers");
});

test("a missile attack spends the chosen arrow; voiding it gives the arrow back", async ({
  page,
}) => {
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

  // Voiding the attack gives the arrow back.
  await undoEntry(page, "Short Bow +2 (Arrows +3) vs AC 2");
  await expect.poll(() => selectedAmmoCount(page)).toBe(before);
  await snap(page, "missile-void");
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
  await expect(card).toContainText("vs AC -20");
  // The correction is folded to one line until opened (ADR 0027 step 8.2).
  const fix = await openFold(card, "Correct the target AC");
  const acField = fix.getByLabel("Corrected target AC");
  const apply = fix.getByRole("button", { name: "Apply" });
  const entry = logEntry(page, "Short Bow +2 (Arrows +2)");
  await expect(card).toContainText("vs AC -20");
  await expect(card.getByTestId("verdict")).toHaveText("Miss");
  await expect(card.getByTestId("total-damage")).toHaveCount(0);
  await expect(acField).toHaveValue("-20");
  await expect(apply).toBeDisabled();
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);
  const d20 = await card
    .getByText("d20", { exact: true })
    .locator("xpath=..")
    .locator("dd")
    .textContent();

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
  expect(
    await card
      .getByText("d20", { exact: true })
      .locator("xpath=..")
      .locator("dd")
      .textContent(),
  ).toBe(d20);
  await expect(entry).toContainText(/\(AC -20 → 10: MISS → (HIT|CRIT)\)/);
  await expect(entry).toContainText("Amended");
  await expect(card.getByTestId("ammo-left")).toHaveText(
    `Arrows +2: ${before - 1} left`,
  );
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);
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
  await expect(card.getByTestId("total-damage")).toHaveCount(0);
  await expect(card.getByTestId("ac-corrected")).toHaveCount(0);
  await expect(entry).toContainText("Short Bow +2 (Arrows +2) vs AC -20: miss");
  await expect(entry).not.toContainText("→ -20");
  await expect(entry).toContainText("Amended");
  await acField.fill("10");
  await apply.click();
  await expect(card.getByTestId("verdict")).toHaveText("Hit");
  await expect(card.getByTestId("total-damage")).toHaveText(damage!);
  await expect(entry).toContainText(/\(AC -20 → 10: MISS → (HIT|CRIT)\)/);
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);

  // Each correction is its own step (ADR 0010 step 6): Annuler names the newest one and pops it,
  // and the card reads the attack as it was before it, here the rolled AC again (no arrow back).
  const undo = page.getByTestId("undo");
  await expect(undo).toHaveAttribute(
    "title",
    "Undo: Correction AC -20 → 10 (Ctrl+Z / ⌘Z)",
  );
  await undo.click();
  await expect(card).toContainText("vs AC -20");
  await expect(card.getByTestId("verdict")).toHaveText("Miss");
  await expect(card.getByTestId("ac-corrected")).toHaveCount(0);
  await expect(acField).toHaveValue("-20");
  await expect(entry).toContainText("Short Bow +2 (Arrows +2) vs AC -20: miss");
  await expect.poll(() => selectedAmmoCount(page)).toBe(before - 1);
  await snap(page, "attack-ac-correction-undone");
  // The two corrections before it, then the first one: the attack is as rolled, still logged.
  for (const step of ["9 → -20", "10 → 9", "-20 → 10"]) {
    await expect(undo).toHaveAttribute(
      "title",
      `Undo: Correction AC ${step} (Ctrl+Z / ⌘Z)`,
    );
    await undo.click();
  }
  await expect(undo).toHaveAttribute(
    "title",
    /^Undo: Short Bow \+2 \(Arrows \+2\) vs AC -20: miss/,
  );
  await expect(entry).not.toContainText("Amended");

  // Voiding the attack then gives the arrow back and removes the correction form.
  await undoEntry(page, "Short Bow +2 (Arrows +2)");
  await expect.poll(() => selectedAmmoCount(page)).toBe(before);
  await expect(fix).toHaveCount(0);
  await snap(page, "attack-ac-corrected-voided");
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
  await expect(
    form.getByRole("button", { name: "Switch to Long Sword +2" }),
  ).toBeVisible();
  await chooseAmmo(page, "Arrows +2");
  await form.getByLabel("Target AC").fill("-3");
  await form.getByRole("radio", { name: "Large" }).click();
  await openTargetModifiers(page);
  await form.getByRole("button", { name: "Raise the modifier" }).click();
  await form.getByRole("button", { name: "Raise the modifier" }).click();
  await form.getByLabel("Reason").fill("cover");
  const thac0 = form.getByTestId("tile-thac0");
  await expect(form.getByTestId("tile-damage-l")).toHaveText("1d6");
  // Wait until the server has every change (the THAC0 tile is its preview).
  await expect
    .poll(
      async () =>
        (await (await page.request.get("/api/attack/form")).json()).form,
    )
    .toEqual({
      weaponId: "shortbow",
      previousWeaponId: "longsword",
      ammoItemId: "arrows_plus2",
      targetAc: -3,
      targetSize: "l",
      situational: { value: 2, label: "cover" },
    });
  const shown = await thac0.textContent();
  await expect(logEntry(page, "vs AC")).toHaveCount(0);

  await page.reload();
  await expect(held).toHaveText("Short Bow +2");
  await expect(
    ammoChips(page).locator('[aria-checked="true"]'),
  ).toHaveAttribute("data-item", "arrows_plus2");
  await expect(form.getByLabel("Target AC")).toHaveValue("-3");
  // ADR 0027 steps 1.5 and 7.2: the size chips are on the target line (Large chosen), the
  // situational modifier shows as a tag, the toggle closed.
  await expect(form.getByRole("radio", { name: "Large" })).toHaveAttribute(
    "aria-checked",
    "true",
  );
  const tags = form.getByTestId("modifier-tag");
  await expect(tags).toHaveText(["Situational +2"]);
  await expect(tags.last()).toHaveAttribute("title", "cover");
  // One line: the size chips and the toggle sit beside the AC (same vertical centre).
  const middle = async (l: Locator) => {
    const b = (await l.boundingBox())!;
    return b.y + b.height / 2;
  };
  expect(
    Math.abs(
      (await middle(form.getByRole("radiogroup", { name: "Target size" }))) -
        (await middle(form.getByLabel("Target AC"))),
    ),
  ).toBeLessThanOrEqual(2);
  await snap(page, "target-tags");
  await openTargetModifiers(page);
  await expect(form.getByLabel("Situational", { exact: true })).toHaveValue(
    "+2",
  );
  await expect(form.getByLabel("Reason")).toHaveValue("cover");
  await expect(thac0).toHaveText(shown!);
  await snap(page, "form-restored");

  // One click back to the sword: it is in hand, the bow is now the previous weapon, no attack.
  await form.getByRole("button", { name: "Switch to Long Sword +2" }).click();
  await expect(held).toHaveText("Long Sword +2");
  await expect(
    form.getByRole("button", { name: "Switch to Short Bow +2" }),
  ).toBeVisible();
  await expect(ammoChips(page)).toHaveCount(0);
  await expect(logEntry(page, "vs AC")).toHaveCount(0);
  await expect(resultCard(page)).toHaveCount(0);
  await snap(page, "weapon-switched");
});

// docs/adr/0012: the target AC is unknown by default; the card says which AC the roll hits and what
// to tell the GM, the GM's Hit settles it without a crit. Screenshots of the unsettled card at the
// four desktop sizes.
test("an attack against an unknown AC is settled by the GM's Hit", async ({
  page,
}) => {
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

  const rolled = page.waitForResponse((r) =>
    r.url().endsWith("/api/roll/attack"),
  );
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
      ? `Tell the DM: “hits AC ${r.hitsAc}”.`
      : `Tell the DM: “hits AC ${r.hitsAc}”. A crit from AC ${r.critsFromAc}.`,
  );
  await expect(card.getByTestId("verdict")).toHaveText("?");
  await expect(card.getByText("Damage if it hits")).toBeVisible();
  await expect(
    logEntry(page, `Long Sword +2 vs unknown AC: hits AC ${r.hitsAc}`),
  ).toBeVisible();
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
    .getByRole("group", { name: "The DM’s hit or miss" })
    .getByRole("button", { name: "Hit" })
    .click();
  await expect(card.getByTestId("verdict")).toHaveText("Hit");
  await expect(card.getByTestId("verdict-by-gm")).toBeVisible();
  await expect(card.getByTestId("crit-badge")).toHaveCount(0);
  await expect(card.getByTestId("total-damage")).toHaveText(
    String(r.totalDamage),
  );
  await expect(
    logEntry(
      page,
      `Long Sword +2 vs unknown AC: hit (DM), ${r.totalDamage} damage`,
    ),
  ).toBeVisible();
  await snap(page, "unknown-ac-gm-hit");
});
