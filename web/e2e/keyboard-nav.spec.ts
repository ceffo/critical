import { test, expect } from "./fixtures";
import { LOOKUP_RAIL, openDashboard } from "./helpers";

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
  // 26 stops stood before the first tab with four rail stops and no skip link; then 24: the skip
  // link, and the rail once; now 25: the saves' spell wand beside the mind box.
  expect(stops - 1).toBe(25);

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
