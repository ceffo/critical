import type { Page } from "@playwright/test";
import { expect } from "../e2e/fixtures";
import { attackForm, logEntry } from "../e2e/helpers";
import { clippedText } from "../e2e/noClip";
import { journal, recordHpT, seedTable, type T } from "./table";

/**
 * Capture helpers of the brand placement round (brand.spec.ts): the verdict annotation CSS can't
 * compute, the no-cut audit, and the seeded table (the mock-up round's, whose capture code was
 * removed with the four themes; see docs/design/ui-review/mockups/README.md, "Regenerating").
 */

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
