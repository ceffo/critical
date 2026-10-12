import { test, expect, snap } from "./fixtures";
import type { Page } from "@playwright/test";

/** Current HP as shown in the HP bar ("current / max"). */
async function currentHp(page: Page): Promise<number> {
  const text = (await page.getByTestId("hp-value").textContent()) ?? "";
  const m = /^\s*(-?\d+)\s*\/\s*(\d+)/.exec(text);
  if (!m) throw new Error(`unexpected HP text: ${JSON.stringify(text)}`);
  return Number(m[1]);
}

test("app loads and shows the character's HP", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).not.toHaveText(
    "Loading...",
  );
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  expect(await currentHp(page)).toBeGreaterThan(0);
  await snap(page, "dashboard");
});

test("entering damage lowers current HP by that amount without reload", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  const before = await currentHp(page);
  await page.evaluate(() => {
    (window as unknown as { __noReload: boolean }).__noReload = true;
  });

  const damage = page.getByRole("group", { name: "Damage" });
  await damage.getByRole("spinbutton").fill("3");
  await damage.getByRole("textbox").fill("e2e goblin");
  await damage.getByRole("button", { name: "Apply Damage" }).click();

  await expect.poll(() => currentHp(page)).toBe(before - 3);
  // The page was not reloaded: the marker set above is still there.
  expect(
    await page.evaluate(
      () => (window as unknown as { __noReload?: boolean }).__noReload,
    ),
  ).toBe(true);
  await snap(page, "after-damage");
});

test("fr smoke: French toggle shows no raw i18n keys", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("hp-value")).toHaveText(/\d+ \/ \d+/);
  await page.getByRole("button", { name: "FR" }).click();
  await expect(
    page.getByRole("button", { name: "Infliger les dégâts" }),
  ).toBeVisible();

  const body = (await page.locator("body").innerText()).split("\n");
  // A raw key looks like `ns.key` / `hp.applyDamage`: dotted lowercase identifier.
  const raw = body.filter((l) => /^[a-z]+(\.[a-zA-Z]+)+$/.test(l.trim()));
  expect(raw).toEqual([]);
  // ...same for placeholders / titles / aria-labels.
  const attrs = await page.evaluate(() =>
    [
      ...document.querySelectorAll("[placeholder],[title],[aria-label]"),
    ].flatMap((el) =>
      ["placeholder", "title", "aria-label"].map(
        (a) => el.getAttribute(a) ?? "",
      ),
    ),
  );
  expect(attrs.filter((v) => /^[a-z]+(\.[a-zA-Z]+)+$/.test(v))).toEqual([]);
  await snap(page, "dashboard-fr");
});
