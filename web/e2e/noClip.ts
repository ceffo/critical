import { expect, type Locator } from "@playwright/test";

/**
 * The no-cut audit (ADR 0022 step 1.3), the same as docs/design/ui-directions/shots.mjs: every
 * element under `root` that has text and is wider inside than its box (`scrollWidth > clientWidth +
 * 1`), or hides its vertical overflow and is taller inside than its box. A box that scrolls is fine
 * vertically (its text scrolls, it is not cut); one that scrolls sideways on purpose (the dice
 * favourites strip, ADR 0022 step 1.2) carries `data-scroll-x`. Screen-reader-only text (1 px wide)
 * is not shown, so it is skipped.
 */
export function clippedText(root: Element): string[] {
  const clipped: string[] = [];
  const describe = (el: Element, how: string) =>
    `<${el.tagName.toLowerCase()} class="${el.getAttribute("class") ?? ""}"> ${how}: ${(el.textContent ?? "").trim().slice(0, 60)}`;
  for (const el of [root, ...root.querySelectorAll("*")]) {
    if (!(el instanceof HTMLElement)) continue;
    if (el.closest("[data-scroll-x]") || !(el.textContent ?? "").trim())
      continue;
    if (el.clientWidth <= 1) continue;
    if (el.scrollWidth > el.clientWidth + 1)
      clipped.push(describe(el, `${el.scrollWidth}>${el.clientWidth} wide`));
    const oy = getComputedStyle(el).overflowY;
    if (
      (oy === "hidden" || oy === "clip") &&
      el.clientHeight > 0 &&
      el.scrollHeight > el.clientHeight + 1
    )
      clipped.push(describe(el, `${el.scrollHeight}>${el.clientHeight} tall`));
  }
  return clipped;
}

/** Fails, listing them, when text inside `region` is cut (ADR 0022 step 1). */
export async function expectNoClippedText(region: Locator, what: string) {
  const clipped = await region.evaluate(clippedText);
  expect(clipped, `cut text in ${what}`).toEqual([]);
}
