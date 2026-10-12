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
    .getByRole("button", { name: kind === "damage" ? "Apply Damage" : "Apply Heal" })
    .click();
}

export function combatLog(page: Page): Locator {
  return page.getByRole("region", { name: "Combat Log" });
}

/** The log entry whose summary contains `source` (use unique sources per test). */
export function logEntry(page: Page, source: string): Locator {
  return combatLog(page).getByRole("listitem").filter({ hasText: source });
}
