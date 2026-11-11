import { test, expect, snap } from "./fixtures";
import {
  apiSettled,
  chooseLanguage,
  currentRound,
  LOOKUP_RAIL,
  openDashboard,
  openWeapons,
} from "./helpers";

// Keyboard order and arrow keys: a tablist and the rail are one Tab stop each, a skip link comes
// first (the WAI-ARIA tabs pattern). Only a browser proves focus order; the walk is headless.
test("Tab order, the skip link, and arrow keys in the rail and the tablist", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await openDashboard(page);

  const focusedName = () =>
    page.evaluate(() => {
      const e = document.activeElement as HTMLElement;
      return `${e.getAttribute("role") ?? e.tagName} ${e.getAttribute("aria-label") ?? e.textContent?.trim()}`;
    });
  const rail = page.getByRole("navigation", { name: "Activities" });
  const tabs = page.getByRole("tablist", { name: "Actions" });

  // The skip link is the first stop and is visible once focused.
  await page.keyboard.press("Tab");
  const skip = page.getByTestId("skip-link");
  await expect(skip).toBeFocused();
  await expect(skip).toBeVisible();
  await expect(skip).toHaveText("Skip to the action panel");

  // Walk to the first tab, counting the stops (the skip link is the first). The four rail buttons
  // are one stop: Act is reached, then the next Tab leaves the rail for the tablist.
  let stops = 1;
  let railStops = 0;
  const weapons = tabs.getByRole("tab", { name: /Weapons/ });
  while (!(await weapons.evaluate((e) => e === document.activeElement))) {
    await page.keyboard.press("Tab");
    stops++;
    if (await rail.locator(":focus").count()) railStops++;
    expect(stops).toBeLessThan(60);
  }
  expect(railStops).toBe(1);
  // 26 stops stood before the first tab with four rail stops and no skip link; now 24: the skip
  // link, and the rail once.
  expect(stops - 1).toBe(24);

  // The tablist is one stop: Right moves and selects, Home and End jump, and Tab leaves it.
  await page.keyboard.press("ArrowRight");
  await expect(tabs.getByRole("tab", { name: /Spells/ })).toBeFocused();
  await expect(tabs.getByRole("tab", { name: /Spells/ })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await page.keyboard.press("End");
  const last = tabs.getByRole("tab").last();
  await expect(last).toBeFocused();
  await expect(last).toHaveAttribute("aria-selected", "true");
  await page.keyboard.press("Home");
  await expect(weapons).toBeFocused();
  await page.keyboard.press("ArrowLeft"); // wraps to the last
  await expect(last).toBeFocused();
  await page.keyboard.press("Home");
  await page.keyboard.press("Tab");
  expect(await focusedName()).not.toMatch(/^tab /);

  // The rail is one stop: Down and Up move focus between the activities, End and Home jump; the
  // focused activity changes only on Enter.
  const act = rail.getByRole("button", { name: "Act", exact: true });
  await act.focus();
  await page.keyboard.press("ArrowDown");
  const spells = rail.getByRole("button", { name: LOOKUP_RAIL });
  await expect(spells).toBeFocused();
  await expect(act).toHaveAttribute("aria-current", "page");
  await page.keyboard.press("End");
  await expect(rail.getByRole("button").last()).toBeFocused();
  await page.keyboard.press("Home");
  await expect(act).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(spells).toHaveAttribute("aria-current", "page");

  // Back to Act; the skip link lands on the selected tab of the stage.
  await act.click();
  await page.keyboard.press("Tab");
  await page.keyboard.press("Shift+Tab");
  await page.getByTestId("skip-link").focus();
  await page.keyboard.press("Enter");
  await expect(weapons).toBeFocused();
});

// The skip link is seen when focused: inside the viewport, with room, in both languages.
for (const lang of ["en", "fr"] as const) {
  for (const [width, height] of [
    [1920, 1080],
    [2560, 1440],
  ]) {
    test(`skip link visible on focus, ${lang} ${width}x${height}`, async ({
      page,
    }) => {
      await page.setViewportSize({ width, height });
      await openDashboard(page);
      if (lang === "fr") await chooseLanguage(page, "fr");
      const skip = page.getByTestId("skip-link");
      // The language popover leaves focus on its trigger: start from the link itself.
      await skip.focus();
      await expect(skip).toBeFocused();
      await expect(skip).toHaveText(
        lang === "fr"
          ? "Aller au panneau d'actions"
          : "Skip to the action panel",
      );
      const box = await skip.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.width).toBeGreaterThan(100);
      expect(box!.height).toBeGreaterThan(20);
      expect(box!.x).toBeGreaterThanOrEqual(0);
      expect(box!.y).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(width);
      expect(box!.y + box!.height).toBeLessThanOrEqual(height);
      // Not cut: its text fits its box.
      expect(await skip.evaluate((e) => e.scrollWidth <= e.clientWidth)).toBe(
        true,
      );
      await snap(page, `skip-link-${lang}-${width}`);
    });
  }
}

// N is Next round and A is Attack: plain keys, when nothing takes typing or a popup is open. Each is
// the button's own click, so one Undo undoes it. Wording differs per language; the keys do not.
for (const lang of ["en", "fr"] as const) {
  test(`N and A: Next round and Attack, one Undo each, ${lang}`, async ({
    page,
  }) => {
    await openDashboard(page);
    if (lang === "fr") await chooseLanguage(page, "fr");
    else await openWeapons(page);
    if (lang === "fr") await page.getByRole("tab", { name: "Armes" }).click();
    const next = page.getByRole("button", {
      name: lang === "fr" ? "Tour suivant" : "Next round",
    });
    const attack = page.getByRole("button", {
      name: lang === "fr" ? "Attaquer" : "Attack",
      exact: true,
    });
    await expect(attack).toBeEnabled();
    // The key shows on both tooltips.
    await expect(next).toHaveAttribute("title", /\(N\)/);
    await expect(attack).toHaveAttribute("title", /\(A\)/);
    const round0 = await currentRound(page);
    const results = page.getByTestId("attack-result");
    const voided = page.locator('li[data-status="voided"]');

    // Where a key must do nothing: in a text field, in the dice composer, with a popover open.
    await page.getByLabel(/Target AC|CA de la cible/).focus();
    await page.keyboard.press("n");
    await page.keyboard.press("a");
    const dice = page.getByRole("form", {
      name: lang === "fr" ? "Dés" : "Dice",
    });
    await dice.getByRole("textbox").focus();
    await page.keyboard.press("n");
    await page.keyboard.press("a");
    await page.getByTestId("settings-trigger").click();
    await expect(page.getByTestId("settings-popover")).toBeVisible();
    await page.keyboard.press("n");
    await page.keyboard.press("a");
    await page.keyboard.press("Escape");
    await expect(page.getByTestId("settings-popover")).toBeHidden();
    // A modified N is the browser's, not ours.
    await page.locator("body").click({ position: { x: 1, y: 1 } });
    await page.keyboard.press("Control+n");
    await page.keyboard.press("Alt+a");
    await apiSettled(page);
    expect(await currentRound(page)).toBe(round0);
    await expect(results).toHaveCount(0);

    // A: one attack, one Undo.
    await page.keyboard.press("a");
    await expect(results).toHaveCount(1);
    await apiSettled(page);
    await page.getByTestId("undo").click();
    await expect(voided.first()).toBeVisible();
    await apiSettled(page);

    // N: one round, one Undo.
    await page.keyboard.press("n");
    await expect.poll(() => currentRound(page)).toBe(round0 + 1);
    await apiSettled(page);
    await page.getByTestId("undo").click();
    await expect.poll(() => currentRound(page)).toBe(round0);
  });
}
