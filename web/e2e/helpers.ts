import { expect, type Locator, type Page } from "@playwright/test";

/** Current HP as shown in the header ("current / max"). */
export async function currentHp(page: Page): Promise<number> {
  const text = (await page.getByTestId("hp-value").textContent()) ?? "";
  const m = /^\s*(-?\d+)\s*\/\s*(\d+)/.exec(text);
  if (!m) throw new Error(`unexpected HP text: ${JSON.stringify(text)}`);
  return Number(m[1]);
}

/** Current round as shown in the header. */
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
      name: kind === "damage" ? "Apply Damage" : "Apply Heal",
    })
    .click();
}

export function combatLog(page: Page): Locator {
  return page.getByRole("region", { name: "Combat Log" });
}

/** The log entry whose summary contains `source` (use unique sources per test). */
export function logEntry(page: Page, source: string): Locator {
  return combatLog(page).getByRole("listitem").filter({ hasText: source });
}

/** The badge of the active buff called `name` in the header's buff strip. */
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

/** Add a buff through the buff strip's form and wait for its badge. */
export async function addBuff(page: Page, buff: NewBuff) {
  const strip = page.getByRole("group", { name: "Buffs" });
  await strip.getByRole("button", { name: "Add buff" }).click();
  const form = strip.getByRole("form", { name: "Add buff" });
  await form.getByLabel("Buff name").fill(buff.name);
  await form.getByLabel("Rounds").fill(String(buff.rounds ?? 0));
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
  await expect(buffBadge(page, buff.name)).toBeVisible();
}

/** Void the log entry whose text contains `text` (two clicks: Void, Confirm). */
export async function voidEntry(page: Page, text: string) {
  const entry = logEntry(page, text);
  await entry.getByRole("button", { name: "Void" }).click();
  await entry.getByRole("button", { name: "Confirm" }).click();
  await expect(entry).toHaveAttribute("data-status", "voided");
}

/** Effective STR as shown in the header. */
export async function effectiveStr(page: Page): Promise<number> {
  return Number((await page.getByTestId("str-value").textContent()) ?? "NaN");
}
