import { test, expect, snap } from "./fixtures";
import { chooseLanguage, openDashboard } from "./helpers";
import { expectNoClippedText, expectWholeWords } from "./noClip";
import { psionicist } from "./psionicist";
import type { Page } from "@playwright/test";

// ADR 0009 step 3: the layout has no horizontal scroll down to 1024. The floor is checked here, in
// French and English, on every activity and on every tab of Act. A browser lays boxes out, so no
// Go or unit test can hold it.
const NARROW = { width: 1024, height: 768 };

/** Visible text boxes and scrollers that run past the right edge of the screen or scroll sideways. */
async function sidewaysOverflow(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const out: string[] = [];
    const vw = document.documentElement.clientWidth;
    const root = document.scrollingElement ?? document.documentElement;
    if (root.scrollWidth > root.clientWidth)
      out.push(`page scrolls sideways ${root.scrollWidth}>${root.clientWidth}`);
    for (const el of document.querySelectorAll("body *")) {
      if (!(el instanceof HTMLElement)) continue;
      if (el.closest("[data-scroll-x]") || el.closest("[hidden]")) continue;
      const r = el.getBoundingClientRect();
      if (r.width === 0 || r.height === 0) continue;
      const name = `<${el.tagName.toLowerCase()} ${(el.getAttribute("data-testid") ?? el.getAttribute("class") ?? "").slice(0, 50)}>`;
      if (r.right > vw + 1 || r.left < -1)
        out.push(
          `${name} spans ${Math.round(r.left)}-${Math.round(r.right)} of ${vw}`,
        );
      const ox = getComputedStyle(el).overflowX;
      if (
        (ox === "auto" || ox === "scroll") &&
        el.scrollWidth > el.clientWidth + 1
      )
        out.push(
          `${name} scrolls sideways ${el.scrollWidth}>${el.clientWidth}`,
        );
    }
    return out;
  });
}

/** Act's every tab, then (when `rest`) Spells, Inventory and Experience, in both languages. */
async function sweep(page: Page, rest: boolean, prefix: string) {
  await page.setViewportSize(NARROW);
  await openDashboard(page);
  const rail = page.getByTestId("rail").getByRole("button");
  for (const lang of ["fr", "en"] as const) {
    await chooseLanguage(page, lang);
    await page.keyboard.press("Escape");
    const check = async (what: string) => {
      const where = `${what}, ${lang}, 1024x768`;
      expect(await sidewaysOverflow(page), `sideways, ${where}`).toEqual([]);
      await expectNoClippedText(page.locator("body"), where);
      await expectWholeWords(page.getByTestId("stage"), where);
      await snap(
        page,
        `${prefix}-${what.replace(/\W+/g, "-").toLowerCase()}-${lang}`,
      );
    };
    await rail.nth(0).click();
    const tabs = page.getByTestId("stage").getByRole("tab");
    const count = await tabs.count();
    expect(count).toBeGreaterThan(1);
    for (let i = 0; i < count; i++) {
      await tabs.nth(i).click();
      await check(`act tab ${i}`);
    }
    if (!rest) continue;
    for (const [i, what] of [
      [1, "spells"],
      [2, "inventory"],
      [3, "experience"],
    ] as const) {
      await rail.nth(i).click();
      await check(what);
    }
  }
}

test("1024x768: Act, Spells, Inventory and Experience fit, French and English", async ({
  page,
}) => {
  test.setTimeout(90_000);
  await sweep(page, true, "default");
});

test.describe("psionicist", () => {
  test.use({ character: psionicist });
  test("1024x768: every Act tab fits, including Powers", async ({ page }) => {
    test.setTimeout(90_000);
    await sweep(page, false, "psionicist");
  });
});
