import { expect, type Locator, type Page } from "@playwright/test";

/** Current HP as shown in the Moi column ("current / max"). */
export async function currentHp(page: Page): Promise<number> {
  const text = (await page.getByTestId("hp-value").textContent()) ?? "";
  const m = /^\s*(-?\d+)\s*\/\s*(\d+)/.exec(text);
  if (!m) throw new Error(`unexpected HP text: ${JSON.stringify(text)}`);
  return Number(m[1]);
}

/** Current turn (round) as shown in the header's turn block. */
export async function currentRound(page: Page): Promise<number> {
  const text = (await page.getByTestId("round-value").textContent()) ?? "";
  if (!/^\d+$/.test(text.trim())) throw new Error(`unexpected round: ${text}`);
  return Number(text);
}

/** Open the dashboard and wait until server state is loaded. */
export async function openDashboard(page: Page) {
  await page.goto("/");
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  await expect(page.getByTestId("round-value")).toHaveText(/^\d+$/);
}

/** Mark the window; `expectNoReload` later proves the page was never reloaded. */
export async function markNoReload(page: Page) {
  await page.evaluate(() => {
    (window as unknown as { __noReload: boolean }).__noReload = true;
  });
}

export async function expectNoReload(page: Page) {
  expect(
    await page.evaluate(
      () => (window as unknown as { __noReload?: boolean }).__noReload,
    ),
  ).toBe(true);
}

/** Record damage or healing through the quick HP entry. */
export async function recordHp(
  page: Page,
  kind: "damage" | "heal",
  amount: number,
  source: string,
) {
  const box = page.getByRole("group", { name: "HP change" });
  await box.getByLabel("Amount").fill(String(amount));
  await box.getByLabel("Source").fill(source);
  await box
    .getByRole("button", {
      name: kind === "damage" ? "Damage" : "Healing",
      exact: true,
    })
    .click();
}

export function combatLog(page: Page): Locator {
  return page.getByRole("region", { name: "Journal", exact: true });
}

/** The journal's group of turn n ("Turn 3"; turn 0 is "Out of combat"). */
export function turnGroup(page: Page, n: number | string): Locator {
  const turn = Number(n);
  return combatLog(page).getByRole("region", {
    name: turn > 0 ? `Turn ${turn}` : "Out of combat",
    exact: true,
  });
}

/** The log entry whose summary contains `source` (use unique sources per test). */
export function logEntry(page: Page, source: string): Locator {
  return combatLog(page).getByRole("listitem").filter({ hasText: source });
}

/** The row of the active buff called `name` in the header's effects timeline. */
export function buffBadge(page: Page, name: string): Locator {
  return page.locator(`[data-testid="buff"][data-buff="${name}"]`);
}

export interface NewBuff {
  name: string;
  rounds?: number;
  charges?: number;
  /** [effect key as on the wire, value], e.g. ["str_bonus", 6]. */
  effects?: [string, number][];
  note?: string;
  /** Tick "Save for reuse". */
  save?: boolean;
}

/** Open the timeline's Add popover (closed beforehand) on one of its tabs; returns the popover. */
export async function openAddEffect(
  page: Page,
  tab: "Spell cast on me" | "Saved effect" | "New effect",
): Promise<Locator> {
  const popover = page.getByTestId("add-effect-popover");
  // A popover still animating out of an earlier step must be gone before it is reopened.
  await expect(popover).toHaveCount(0);
  await page
    .getByRole("group", { name: "Effects" })
    .getByRole("button", { name: "Add effect" })
    .click();
  await popover.getByRole("tab", { name: tab }).click();
  return popover;
}

/** The spell search of the Add popover's "Spell cast on me" tab. */
export async function receivePicker(page: Page): Promise<Locator> {
  const popover = await openAddEffect(page, "Spell cast on me");
  return popover.getByRole("region", { name: "Spell cast on me" });
}

/** Add a buff through the Add popover's New effect tab and wait for its row. */
export async function addBuff(page: Page, buff: NewBuff) {
  const popover = await openAddEffect(page, "New effect");
  const form = popover.getByRole("form", { name: "Add effect" });
  await form.getByLabel("Buff name").fill(buff.name);
  await form.getByLabel("Turns").fill(String(buff.rounds ?? 0));
  await form.getByLabel("Charges").fill(String(buff.charges ?? 0));
  for (const [i, [key, value]] of (buff.effects ?? []).entries()) {
    await form.getByRole("button", { name: "Effect", exact: true }).click();
    const row = form.getByTestId("effect-row").nth(i);
    await row.getByLabel("Effect", { exact: true }).selectOption(key);
    await row.getByLabel("Effect value").fill(String(value));
  }
  if (buff.note) await form.getByLabel("Note").fill(buff.note);
  if (buff.save) await form.getByLabel("Save for reuse").check();
  await form.getByRole("button", { name: "Activate" }).click();
  // Past three effects the new row sits behind the timeline's expander: open it (left open).
  const closed = page.locator(
    "[data-testid=effects-expander][aria-expanded=false]",
  );
  await expect(buffBadge(page, buff.name).or(closed)).toBeVisible();
  if (await closed.isVisible()) await closed.click();
  await expect(buffBadge(page, buff.name)).toBeVisible();
}

/** Undo the log entry whose text contains `text`: it must be the one on top; the journal's one
 * Undo button pops it. */
export async function undoEntry(page: Page, text: string) {
  const entry = logEntry(page, text);
  await page.getByTestId("undo").click();
  await expect(entry).toHaveAttribute("data-status", "voided");
}

/** The journal's Undo tooltip names `text` (the entry on top), or, with `not`, does not. */
export async function expectUndoOn(page: Page, text: string, not = false) {
  const undo = page.getByTestId("undo");
  const re = new RegExp(text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  if (not) await expect(undo).not.toHaveAttribute("title", re);
  else await expect(undo).toHaveAttribute("title", re);
}

/** Effective STR as shown in the header. */
export async function effectiveStr(page: Page): Promise<number> {
  return Number((await page.getByTestId("str-value").textContent()) ?? "NaN");
}

/** Switch the workspace from the rail (English labels). */
export async function openWorkspace(
  page: Page,
  name: "Act" | "Inventory" | "Spells" | "Experience",
) {
  await page
    .getByRole("navigation", { name: "Workspaces" })
    .getByRole("button", { name, exact: true })
    .click();
}

/** Open the settings popover from the rail's gear; returns the popover. */
export async function openSettings(page: Page): Promise<Locator> {
  await page.getByTestId("settings-trigger").click();
  const popover = page.getByTestId("settings-popover");
  await expect(popover).toBeVisible();
  return popover;
}

/** Pick one option in the settings popover (by its visible name), then close it. */
export async function chooseSetting(page: Page, option: string) {
  const popover = await openSettings(page);
  const radio = popover.getByRole("radio", { name: option, exact: true });
  await radio.click();
  await expect(radio).toHaveAttribute("aria-checked", "true");
  await page.keyboard.press("Escape");
  await expect(popover).toBeHidden();
}

/** Switch the UI language through the settings popover. */
export async function chooseLanguage(page: Page, lang: "en" | "fr") {
  await chooseSetting(page, lang === "fr" ? "Français" : "English");
}

/** Next turn from the turn block (English UI). */
export async function nextTurn(page: Page) {
  await page.getByRole("button", { name: "Next turn" }).click();
}

/** Open the turn block's ⋯ menu and pick an action (English UI). */
export async function turnMenu(
  page: Page,
  item: "Time passes…" | "Set turn number…" | "End combat",
) {
  await page.getByRole("button", { name: "More turn actions" }).click();
  await page.getByRole("menuitem", { name: item }).click();
}

/**
 * Undo the turn's initiative entry, the one the turn block shows (Next turn logs it after the
 * advance, so it is popped first).
 */
export async function undoInitiative(page: Page) {
  const id = await page
    .getByTestId("initiative-value")
    .getAttribute("data-entry-id");
  const entry = page.locator(`li[data-entry-id="${id}"]`);
  await page.getByTestId("undo").click();
  await expect(entry).toHaveAttribute("data-status", "voided");
  await expect(page.getByTestId("initiative-value")).toHaveCount(0);
}

/** The Weapons tab's attack form (target bar, weapon in hand, switching). */
export function attackForm(page: Page): Locator {
  return page.getByRole("form", { name: "Attack" });
}

/** Open the Agir Weapons tab and wait for the weapon in hand (English UI). */
export async function openWeapons(page: Page) {
  await page.getByRole("tab", { name: "Weapons" }).click();
  await expect(attackForm(page).getByTestId("weapon-in-hand")).not.toBeEmpty();
}

/** Put a weapon in hand through "Other weapon" unless it already is; never attacks. */
export async function chooseWeapon(page: Page, name: string) {
  const held = attackForm(page).getByTestId("weapon-in-hand");
  await expect(held).not.toBeEmpty();
  if ((await held.textContent()) === name) return;
  await attackForm(page).getByRole("button", { name: "Other weapon" }).click();
  await page
    .getByRole("list", { name: "Other weapon" })
    .getByRole("button")
    .filter({ has: page.getByText(name, { exact: true }) })
    .click();
  await expect(held).toHaveText(name);
}

/** The ammunition chips of the weapon in hand. */
export function ammoChips(page: Page): Locator {
  return attackForm(page).getByRole("radiogroup", { name: "Ammunition" });
}

/** Choose the ammunition called `name` ("Arrows +3") and wait until it is the choice. */
export async function chooseAmmo(page: Page, name: string) {
  const chip = ammoChips(page).getByRole("radio", { name: `${name},` });
  await chip.click();
  await expect(chip).toHaveAttribute("aria-checked", "true");
}

/** Count left of the chosen ammunition, as its chip shows it. */
export async function chosenAmmoCount(page: Page): Promise<number> {
  const text = await ammoChips(page)
    .locator('[aria-checked="true"]')
    .getByTestId("ammo-count")
    .textContent();
  if (!/^\d+$/.test(text ?? ""))
    throw new Error(`unexpected ammo count: ${JSON.stringify(text)}`);
  return Number(text);
}

/**
 * Send every attack with target AC `ac`, beyond the form's ±20 (AC 1000 needs so low a roll that
 * every attack crits). The form still shows its own AC.
 */
export async function forceAttackAc(page: Page, ac: number) {
  await page.route("**/api/roll/attack", async (route) => {
    const body = route.request().postDataJSON() as Record<string, unknown>;
    await route.continue({
      postData: JSON.stringify({ ...body, target_ac: ac }),
    });
  });
}
