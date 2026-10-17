import { readFile } from "node:fs/promises";
import path from "node:path";
import type { Page } from "@playwright/test";
import { expect } from "../e2e/fixtures";
import { attackForm, chooseSetting, logEntry } from "../e2e/helpers";
import { clippedText } from "../e2e/noClip";
import { themeCss, type Option, type Theme } from "./mockup-tokens";
import { journal, recordHpT, seedTable, webDir, type T } from "./table";

/**
 * Painting a mock-up option over the running app, shared by the mock-up round (mockups.spec.ts)
 * and the brand placement round (brand.spec.ts): the option's tokens and component rules as a
 * last stylesheet, the verdict annotation CSS can't compute, the no-cut audit, and the seeded table.
 */

const cssDir = path.join(
  webDir,
  "..",
  "docs",
  "design",
  "ui-review",
  "mockups",
  "css",
);

export async function optionCss(o: Option): Promise<string> {
  const parts = await Promise.all(
    o.css.map((name) => readFile(path.join(cssDir, `${name}.css`), "utf8")),
  );
  return parts.join("\n");
}

/** Select the theme's base ambiance and mode in the app, then paint the option over it. */
export async function paint(page: Page, t: T, o: Option, theme: Theme) {
  await chooseSetting(page, t(`settings.mode.${theme.mode}`));
  await chooseSetting(page, t(`settings.ambiances.${theme.base}`));
  const css = o.id === "today" ? "" : themeCss(theme) + (await optionCss(o));
  await page.evaluate(
    ({ css, id }) => {
      let el = document.getElementById("mock-overlay");
      if (!el) {
        el = document.createElement("style");
        el.id = "mock-overlay";
      }
      document.head.appendChild(el); // last, after the app's styles
      el.textContent = css;
      if (css) document.documentElement.dataset.mockTheme = id;
      else delete document.documentElement.dataset.mockTheme;
    },
    { css, id: theme.id },
  );
}

/** Mark the header's last-result chip with the verdict the result card shows (CSS can't read text). */
export async function annotate(page: Page) {
  await page.evaluate(() => {
    const chip = document.querySelector("[data-testid=last-result-chip]");
    const verdict = document.querySelector("[data-testid=verdict]");
    if (!chip || !verdict) return;
    const good = verdict.classList.contains("text-success");
    const bad = verdict.classList.contains("text-danger");
    if (good || bad)
      (chip as HTMLElement).dataset.mockVerdict = good ? "good" : "bad";
  });
}

/** The no-cut audit of the whole page, plus "the page scrolls". */
export async function audit(page: Page): Promise<string[]> {
  const cut = await page.locator("body").evaluate(clippedText);
  const scrolls = await page.evaluate(
    () =>
      document.documentElement.scrollHeight >
      document.documentElement.clientHeight + 1,
  );
  return scrolls ? [...cut, "the page scrolls"] : cut;
}

/** Damage, undone, then a fresh attack: the journal shows an undone row, the card the attack. */
export async function seedMockTable(page: Page, apiUrl: string, t: T) {
  await seedTable(page, apiUrl);
  await recordHpT(page, t, "damage", 60, "Dragon breath");
  await expect(logEntry(page, "Dragon breath")).toBeVisible();
  await page.getByTestId("undo").click();
  await expect(journal(page, t).getByRole("listitem").first()).toHaveAttribute(
    "data-status",
    "voided",
  );
  await attackForm(page)
    .getByRole("button", { name: "Attack", exact: true })
    .click();
  await expect(
    journal(page, t).getByRole("listitem").first(),
  ).not.toHaveAttribute("data-status", "voided");
}
