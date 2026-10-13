import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect, snap } from "./fixtures";
import { openDashboard } from "./helpers";
import type { Locator, Page } from "@playwright/test";

// The frozen fixture catalogue the stack seeds from (never the campaign seeds/).
const fixtureDir = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../api/testdata/seeds",
);
const fixtureSpells = JSON.parse(
  readFileSync(path.join(fixtureDir, "spells.json"), "utf8"),
) as unknown[];
const fixtureBook = JSON.parse(
  readFileSync(path.join(fixtureDir, "spellbook.json"), "utf8"),
) as { id: string }[];

/** A catalogue row appended to the fixture for these tests. */
function row(
  name: string,
  cls: "Wizard" | "Priest",
  level: number,
  school: string,
  sphere = "",
  extra: Record<string, string> = {},
) {
  return {
    name,
    class: cls,
    level,
    school,
    sphere,
    casting_time: "1",
    range: "10 yds.",
    duration: "1 rd.",
    area_of_effect: "1 creature",
    components: "V, S",
    saving_throw: "None",
    description: `${name} fixture text.`,
    ...extra,
  };
}

// The fixture has Fireball, Magic Missile, Burning Hands...; these add what the criteria name.
const named = [
  row("Project Image", "Wizard", 6, "Illusion/Phantasm", "", {
    casting_time: "6",
    range: "10 yds./level",
    duration: "1 rd./level",
    area_of_effect: "Special",
  }),
  row("Fire Trap", "Wizard", 4, "Abjuration, Evocation"),
  row("Fire Shield", "Wizard", 4, "Evocation, Alteration"),
  row("Wall of Fire", "Wizard", 4, "Evocation"),
  row("Faerie Fire", "Priest", 1, "Alteration", "Elemental Fire"),
  row("Flame Strike", "Priest", 5, "Evocation", "Elemental Fire"),
  row("Cure Light Wounds", "Priest", 1, "Necromancy", "Healing"),
  row("Dimensional Folding", "Wizard", 7, "Dimension"),
];

test.use({ extraSpells: { rows: named } });

async function openLookup(page: Page) {
  await openDashboard(page);
  await page.getByRole("tab", { name: "Spell Lookup" }).click();
  await expect(search(page)).toBeVisible();
  await expect(rows(page).first()).toBeVisible();
}

const search = (page: Page) => page.getByRole("combobox", { name: "Search spells..." });
const count = (page: Page) => page.getByTestId("lookup-count");
const rows = (page: Page) => page.getByTestId("spell-row");
const selected = (page: Page) =>
  page.getByRole("listbox", { name: "Spells" }).getByRole("option", { selected: true });
const detail = (page: Page) => page.getByTestId("spell-detail");
const classButton = (page: Page, name: string) =>
  page.getByRole("group", { name: "Spell list" }).getByRole("button", { name, exact: true });
const levelChip = (page: Page, n: number) =>
  page.getByRole("button", { name: `Level ${n}`, exact: true });

async function attr(list: Locator, name: string): Promise<string[]> {
  return list.evaluateAll(
    (els, a) => els.map((e) => e.getAttribute(a) ?? ""),
    name,
  );
}

async function names(list: Locator): Promise<string[]> {
  return list
    .getByTestId("spell-name")
    .evaluateAll((els) => els.map((e) => e.textContent ?? ""));
}

test("lookup: typing 'proj' selects Project Image with no click and shows its stats", async ({
  page,
}) => {
  await openLookup(page);
  await search(page).fill("proj");
  await expect(selected(page)).toHaveAttribute("data-id", "project_image");
  await expect(detail(page).getByRole("heading")).toHaveText("Project Image");
  // Values at the Bard's level (fixture: 15) next to the raw text.
  const range = detail(page).getByTestId("stat-range");
  await expect(range.getByTestId("stat-value")).toHaveText("150 yd");
  await expect(range.getByTestId("stat-raw")).toContainText("10 yds./level");
  await expect(detail(page).getByTestId("stat-duration").getByTestId("stat-value")).toHaveText(
    "15 rd",
  );
  await snap(page, "search-selected");
});

test("lookup: a French name finds the spell; the row shows both names in the UI language order", async ({
  page,
}) => {
  await openLookup(page);
  await search(page).fill("mains");
  const first = rows(page).first();
  await expect(first).toHaveAttribute("data-id", "burning_hands");
  await expect(first).toContainText("Burning Hands · Mains brûlantes");
  await page.getByRole("button", { name: "FR" }).click();
  await expect(first).toContainText("Mains brûlantes · Burning Hands");
  await expect(detail(page).getByRole("heading")).toHaveText("Mains brûlantes · Burning Hands");
  await snap(page, "french-name");
});

test("lookup: exact and prefix matches come before contains matches, level then name within a tier", async ({
  page,
}) => {
  await openLookup(page);
  await classButton(page, "All").click();
  await search(page).fill("fire");
  await expect(rows(page)).toHaveCount(5);
  expect(await names(rows(page))).toEqual([
    "Fireball", // prefix, L3
    "Fire Shield", // prefix, L4
    "Fire Trap", // prefix, L4
    "Faerie Fire", // contains, L1
    "Wall of Fire", // contains, L4
  ]);
  await expect(selected(page)).toHaveAttribute("data-id", "fireball");
});

test("lookup: the toggle defaults to Wizard; Mine is the spellbook; Priest and All work; class chip only on All", async ({
  page,
}) => {
  await openLookup(page);
  await expect(classButton(page, "Wizard")).toHaveAttribute("aria-pressed", "true");
  await expect(rows(page).first()).toBeVisible();
  expect(new Set(await attr(rows(page), "data-class"))).toEqual(new Set(["Wizard"]));
  await expect(page.getByTestId("spell-class")).toHaveCount(0);

  await classButton(page, "Mine").click();
  await expect(count(page)).toHaveText(`${fixtureBook.length} spells`);
  expect((await attr(rows(page), "data-id")).sort()).toEqual(
    fixtureBook.map((e) => e.id).sort(),
  );
  await expect(page.getByTestId("spell-class")).toHaveCount(0);
  await snap(page, "mine");

  await classButton(page, "Priest").click();
  await expect(rows(page).filter({ hasText: "Faerie Fire" })).toHaveCount(1);
  expect(new Set(await attr(rows(page), "data-class"))).toEqual(new Set(["Priest"]));
  await expect(page.getByTestId("spell-class")).toHaveCount(0);

  await classButton(page, "All").click();
  await expect(count(page)).toHaveText(`${fixtureSpells.length + named.length} spells`);
  expect(new Set(await attr(rows(page), "data-class"))).toEqual(
    new Set(["Wizard", "Priest"]),
  );
  await expect(page.getByTestId("spell-class").first()).toBeVisible();
  await expect(page.getByTestId("spell-class")).toHaveCount(await rows(page).count());
});

test("lookup: level chips multi-select; none selected means any level", async ({
  page,
}) => {
  await openLookup(page);
  const any = await count(page).textContent();
  await levelChip(page, 1).click();
  await levelChip(page, 3).click();
  await expect(levelChip(page, 1)).toHaveAttribute("aria-pressed", "true");
  await expect
    .poll(async () => [...new Set(await attr(rows(page), "data-level"))].sort())
    .toEqual(["1", "3"]);
  await expect(rows(page).filter({ hasText: "Fireball" })).toHaveCount(1);
  await snap(page, "levels-1-3");

  await levelChip(page, 1).click();
  await levelChip(page, 3).click();
  await expect(count(page)).toHaveText(any ?? "");
});

test("lookup: school groups take their aliases and Other; Priest mode filters by sphere", async ({
  page,
}) => {
  await openLookup(page);
  const school = page.getByRole("combobox", { name: "School" });
  await school.selectOption({ label: "Invocation/Evocation" });
  // "Evocation" in the data belongs to Invocation/Evocation.
  await expect(rows(page).filter({ hasText: "Magic Missile" })).toHaveCount(1);
  await expect(rows(page).filter({ hasText: "Fireball" })).toHaveCount(1);
  await expect(rows(page).filter({ hasText: "Burning Hands" })).toHaveCount(0);
  await snap(page, "school-evocation");

  await school.selectOption({ label: "Other" });
  await expect(rows(page)).toHaveCount(1);
  await expect(rows(page).first()).toHaveAttribute("data-id", "dimensional_folding");

  // All with a school set, then Priest: the school is dropped, the sphere dropdown shows.
  await classButton(page, "All").click();
  await expect(school).toHaveValue("other");
  await classButton(page, "Priest").click();
  const sphere = page.getByRole("combobox", { name: "Sphere" });
  await expect(page.getByRole("combobox", { name: "School" })).toHaveCount(0);
  await expect(sphere).toHaveValue("");
  await expect(rows(page).filter({ hasText: "Faerie Fire" })).toHaveCount(1);
  await expect(sphere.locator("option", { hasText: "Elemental Fire" })).toHaveCount(1);
  await expect(sphere.locator("option", { hasText: "Invocation/Evocation" })).toHaveCount(0);
  await sphere.selectOption("Elemental Fire");
  await expect(rows(page)).toHaveCount(2);
  expect(await names(rows(page))).toEqual(["Faerie Fire", "Flame Strike"]);
  await snap(page, "priest-sphere");
});

test("lookup: up/down in the search box moves the selection, a click selects; values at level with raw text; description collapsed then full", async ({
  page,
}) => {
  await openLookup(page);
  await search(page).fill("magic");
  // Prefix matches (Magic Missile, Magic Jar) before contains (Detect Magic...).
  await expect(selected(page)).toHaveAttribute("data-id", "magic_missile");
  const ids = await attr(rows(page), "data-id");
  expect(ids.length).toBeGreaterThan(2);
  await search(page).press("ArrowDown");
  await expect(selected(page)).toHaveAttribute("data-id", ids[1]);
  await search(page).press("ArrowUp");
  await expect(selected(page)).toHaveAttribute("data-id", ids[0]);

  await rows(page).filter({ hasText: "Dispel Magic" }).click();
  await expect(selected(page)).toHaveAttribute("data-id", "dispel_magic");
  await rows(page).filter({ hasText: "Magic Missile" }).click();
  await expect(selected(page)).toHaveAttribute("data-id", "magic_missile");
  const range = detail(page).getByTestId("stat-range");
  await expect(range.getByTestId("stat-value")).toHaveText("210 yd");
  await expect(range.getByTestId("stat-raw")).toHaveText("· 60 yds. + 10 yds./level");

  const description = detail(page).getByTestId("spell-description");
  const text = description.locator("p");
  await expect(text).toBeHidden();
  await description.getByText("Description").click();
  await expect(text).toBeVisible();
  const full = (await text.textContent()) ?? "";
  expect(full.length).toBeGreaterThan(200); // not cut
  expect(full.trimEnd().endsWith("as desired.")).toBe(true);
  await text.evaluate((el) => el.scrollIntoView({ block: "end" }));
  await snap(page, "description-open");
});

test("lookup: tab switch keeps search, filters and selection; Reset only when set; reload clears", async ({
  page,
}) => {
  await openLookup(page);
  const reset = page.getByRole("button", { name: "Reset", exact: true });
  await expect(reset).toHaveCount(0);

  await search(page).fill("fire");
  await classButton(page, "All").click();
  await levelChip(page, 4).click();
  await page.getByRole("combobox", { name: "School" }).selectOption({ label: "Abjuration" });
  await expect(rows(page)).toHaveCount(1);
  await expect(selected(page)).toHaveAttribute("data-id", "fire_trap");
  await expect(reset).toBeVisible();

  await page.getByRole("tab", { name: "Attack" }).click();
  await page.getByRole("tab", { name: "Spell Lookup" }).click();
  await expect(search(page)).toHaveValue("fire");
  await expect(classButton(page, "All")).toHaveAttribute("aria-pressed", "true");
  await expect(levelChip(page, 4)).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("combobox", { name: "School" })).toHaveValue("abjuration");
  await expect(selected(page)).toHaveAttribute("data-id", "fire_trap");

  await reset.click();
  await expect(search(page)).toHaveValue("");
  await expect(classButton(page, "Wizard")).toHaveAttribute("aria-pressed", "true");
  await expect(levelChip(page, 4)).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByRole("combobox", { name: "School" })).toHaveValue("");
  await expect(reset).toHaveCount(0);

  await search(page).fill("magic");
  await levelChip(page, 1).click();
  await page.reload();
  await page.getByRole("tab", { name: "Spell Lookup" }).click();
  await expect(search(page)).toHaveValue("");
  await expect(levelChip(page, 1)).toHaveAttribute("aria-pressed", "false");
  await expect(reset).toHaveCount(0);
});

test("lookup: the Receive picker finds spells by French and English names with the lookup's ranking", async ({
  page,
}) => {
  await openLookup(page);
  await classButton(page, "All").click();
  await search(page).fill("fire");
  await expect(rows(page)).toHaveCount(5);
  const lookupOrder = await attr(rows(page), "data-id");

  await page.getByRole("tab", { name: "Spells" }).click();
  const picker = page.getByRole("region", { name: "Spell cast on me" });
  const box = picker.getByLabel("Search for a spell cast on me");
  await box.fill("fire");
  await expect(picker.getByRole("listitem")).toHaveCount(5);
  await expect(picker.getByRole("listitem").first()).toContainText("Fireball");
  const receiveNames = await picker
    .getByRole("listitem")
    .evaluateAll((els) => els.map((e) => (e.textContent ?? "").split(" · ")[0]));
  expect(receiveNames).toEqual(["Fireball", "Fire Shield", "Fire Trap", "Faerie Fire", "Wall of Fire"]);
  expect(lookupOrder).toEqual(["fireball", "fire_shield", "fire_trap", "faerie_fire", "wall_of_fire"]);

  await box.fill("mains");
  await expect(picker.getByRole("listitem").first()).toContainText("Burning Hands");
});

// Enough synthetic Wizard rows that the list needs several 200-row pages.
const many = Array.from({ length: 450 }, (_, i) =>
  row(`Scroll Test ${String(i).padStart(3, "0")}`, "Wizard", (i % 9) + 1, "Alteration"),
);

test.describe("infinite scroll", () => {
  test.use({ extraSpells: { rows: [...named, ...many] } });

  test("lookup: the header shows the true total; scrolling to the bottom loads every spell, the last a level 9", async ({
    page,
  }) => {
    await openLookup(page);
    await classButton(page, "All").click();
    const total = fixtureSpells.length + named.length + many.length;
    await expect(count(page)).toHaveText(`${total} spells`);
    await expect(rows(page)).toHaveCount(200);
    await expect(page.getByRole("button", { name: /^(Next|Previous)$/ })).toHaveCount(0);

    const list = page.getByRole("listbox", { name: "Spells" });
    for (let i = 0; i < 10 && (await rows(page).count()) < total; i++) {
      const before = await rows(page).count();
      await list.evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
      await expect.poll(() => rows(page).count()).toBeGreaterThan(before);
    }
    await expect(rows(page)).toHaveCount(total);
    await expect(count(page)).toHaveText(`${total} spells`);
    await expect(rows(page).last()).toHaveAttribute("data-level", "9");
    await list.evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
    await snap(page, "scrolled-to-end");

    // A selection deep in the list is still in view after a tab switch.
    const deep = rows(page).nth(350);
    const deepId = (await deep.getAttribute("data-id")) ?? "";
    await deep.scrollIntoViewIfNeeded();
    await deep.click();
    await page.getByRole("tab", { name: "Attack" }).click();
    await page.getByRole("tab", { name: "Spell Lookup" }).click();
    await expect(selected(page)).toHaveAttribute("data-id", deepId);
    await expect(selected(page)).toBeInViewport();
    await snap(page, "deep-selection-after-tab-switch");
  });

});

// "Force" is Strength's French name; the synthetic rows match it in English too.
const forceRows = Array.from({ length: 260 }, (_, i) =>
  row(`Force Test ${String(i).padStart(3, "0")}`, "Wizard", (i % 9) + 1, "Alteration"),
);

test.describe("keyboard past a page", () => {
  test.use({ extraSpells: { rows: [...named, ...forceRows] } });

  test("lookup: a French name, then arrow-down past the 200-row page loads more and keeps the selection", async ({
    page,
  }) => {
    await openLookup(page);
    await search(page).fill("force");
    await expect(count(page)).toHaveText(`${forceRows.length + 1} spells`);
    // The French exact match ranks first.
    await expect(selected(page)).toHaveAttribute("data-id", "strength");
    await expect(rows(page)).toHaveCount(200);
    for (let i = 0; i < 205; i++) await search(page).press("ArrowDown");
    await expect(rows(page)).toHaveCount(forceRows.length + 1);
    const ids = await attr(rows(page), "data-id");
    await expect(selected(page)).toHaveAttribute("data-id", ids[205]);
    await expect(selected(page)).toBeInViewport();
  });
});

test("lookup: a slow response overtaken by a newer query is not written", async ({
  page,
}) => {
  await page.route("**/api/spells?*", async (route) => {
    if (new URL(route.request().url()).searchParams.get("q") === "fir")
      await new Promise((r) => setTimeout(r, 1500));
    await route.continue().catch(() => {}); // the page may be gone
  });
  await openLookup(page);
  const slow = page.waitForRequest((r) => new URL(r.url()).searchParams.get("q") === "fir");
  await search(page).fill("fir");
  await slow;
  await search(page).fill("fireball");
  await expect(rows(page)).toHaveCount(1);
  await expect(selected(page)).toHaveAttribute("data-id", "fireball");
  // Let the slow "fir" response land: the rows stay those of "fireball".
  await page.waitForResponse((r) => new URL(r.url()).searchParams.get("q") === "fir");
  await page.waitForTimeout(200);
  await expect(rows(page)).toHaveCount(1);
  await expect(count(page)).toHaveText("1 spell");
});
