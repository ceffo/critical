import { test, expect, snap } from "./fixtures";
import { openDashboard } from "./helpers";
import type { Page } from "@playwright/test";

async function openLookup(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Spell Lookup" }).click();
  await expect(page.getByLabel("Search spells...")).toBeVisible();
}

const results = (page: Page) => page.getByTestId("spell-result");

test("lookup: same-named Wizard and Priest spells are told apart, one click drops Priest", async ({
  page,
}) => {
  await openLookup(page);
  await page.getByLabel("Search spells...").fill("Detect Magic");
  await expect(results(page).first()).toBeVisible();
  const classes = await results(page).evaluateAll((els) =>
    els.map((e) => e.getAttribute("data-class")),
  );
  expect(classes.sort()).toEqual(["Priest", "Wizard"]);
  await snap(page, "detect-magic-both-classes");

  await page.getByRole("button", { name: "Wizard only" }).click(); // the one click
  await expect(results(page)).toHaveCount(1);
  await expect(page.getByTestId("spell-class")).toHaveText("Wizard");
  await snap(page, "detect-magic-priest-excluded");
});

test("lookup: filters combine (Wizard, level 3, save 1/2)", async ({
  page,
}) => {
  await openLookup(page);
  await page.getByRole("button", { name: "Wizard only" }).click();
  await page.getByLabel("Level", { exact: true }).selectOption("3");
  await page.getByLabel("Save", { exact: true }).selectOption("1/2");
  await expect(results(page).filter({ hasText: "Fireball" })).toHaveCount(1);
  await expect(results(page).filter({ hasText: "Lightning Bolt" })).toHaveCount(
    1,
  );
  await expect(results(page).filter({ hasText: "Haste" })).toHaveCount(0);
  await snap(page, "wizard-level3-save-half");

  await page.getByRole("button", { name: "Components M" }).click();
  await expect(results(page).filter({ hasText: "Fireball" })).toHaveCount(1);
  await page.getByLabel("Search spells...").fill("zzzz-nothing");
  await expect(page.getByText("No spells match these filters")).toBeVisible();
  await page.getByRole("button", { name: "Reset filters" }).click();
  await expect(page.getByLabel("Save", { exact: true })).toHaveValue("");
});

test("lookup: stats follow the caster level, raw PHB text stays available", async ({
  page,
}) => {
  await openLookup(page);
  await page.getByLabel("Search spells...").fill("Magic Missile");
  await page.getByLabel("Caster level").fill("15");
  const mm = results(page).filter({ hasText: "Magic Missile" }).first();
  await expect(mm.getByTestId("v-range")).toContainText("210");
  await expect(mm.getByTestId("v-range")).toContainText("yard");
  await expect(mm).toContainText("At level 15");
  await mm.getByText("PHB text").click();
  await expect(mm.getByText("60 yds. + 10 yds./level")).toBeVisible();
  await snap(page, "magic-missile-level-15");

  await page.getByLabel("Caster level").fill("5");
  await expect(mm.getByTestId("v-range")).toContainText("110");
});

// Synthetic catalogue rows so the lookup needs several pages (the fixture has 24 spells).
const manySpells = Array.from({ length: 130 }, (_, i) => ({
  name: `Paging Test ${String(i).padStart(3, "0")}`,
  class: "Wizard",
  level: (i % 9) + 1,
  school: "Evocation",
  sphere: "",
  casting_time: "1",
  range: "10 yds.",
  duration: "1 rd.",
  area_of_effect: "1 creature",
  components: "V, S",
  saving_throw: "None",
  description: "Fixture spell for the paging test.",
}));

test.describe("paging", () => {
  test.use({ extraSpells: { rows: manySpells } });
  test("lookup: paging reaches the last pages (a level-9 spell) and a filter change returns to page 1", async ({
    page,
  }) => {
    await openLookup(page);
    await page.getByRole("button", { name: "Wizard only" }).click();
    const range = page.getByTestId("lookup-range");
    await expect(range).toHaveText(/^Showing 1–50 of \d+$/);
    await expect(results(page)).toHaveCount(50);
    await expect(
      page.getByRole("button", { name: "Previous", exact: true }),
    ).toBeDisabled();
    await snap(page, "paging-first-page");

    // A typed-stat filter, then page 2: the range and the total agree.
    await page.getByRole("button", { name: "Components V" }).click();
    await expect(range).toHaveText(/^Showing 1–50 of \d+$/);
    const next = page.getByRole("button", { name: "Next", exact: true });
    await next.click();
    await expect(range).toHaveText(/^Showing 51–100 of \d+$/);
    while (await next.isEnabled()) {
      const before = await range.textContent();
      await next.click();
      await expect(range).not.toHaveText(before ?? "");
    }
    await expect(next).toBeDisabled();
    await expect(
      results(page).filter({ hasText: "Lvl 9" }).first(),
    ).toBeVisible();
    await snap(page, "paging-last-page-level-9");

    await page.getByLabel("Level", { exact: true }).selectOption("2");
    await expect(range).toHaveText(/^Showing 1–/);
    await expect(
      page.getByRole("button", { name: "Previous", exact: true }),
    ).toBeDisabled();
  });
});
